# mod 元信息展示(ModName/PackageName/Tags)

## Goal

mod 下载/部署成功后，在服务器管理页的 Mods 区块展示该 mod `Info.json` 中的
`ModName`、`PackageName`、`Tags`，让用户能直观确认下载到的 mod 是什么，而不只看到
用户输入的 workshop_id / 自定义名。**不展示 Thumbnail 缩略图**（用户已确认排除）。

## Background / Confirmed Facts

- `Info.json` 示例字段（真实样本，工作区已打开）：`ModName`、`PackageName`、
  `Thumbnail`、`Version`、`Tags`（字符串数组）、`InstallRule` 等。
- 后端解析 [internal/palmod/info.go](../../../internal/palmod/info.go) 的 `ParseInfo`
  目前仅取 `PackageName`、`Version`、`IsServer`（`Info` 结构体 L25-29）。解析刻意宽容：
  缺字段不 panic，`Version` 兼容字符串/数字。
- 数据模型 [internal/models/server.go:32](../../../internal/models/server.go#L32) 的
  `Mod` 只有 `name`(用户输入) / `package_name` / `version`；GORM 模型直接 `c.JSON`
  作为 API 响应返回。
- 下载后回填在 [internal/process/manager.go:435-462](../../../internal/process/manager.go#L435-L462)：
  `ParseInfo(dst)` 后把 `package_name`/`version`/`install_path` 写库。
- DB 迁移用 GORM `AutoMigrate`（[internal/database/database.go:32-42](../../../internal/database/database.go#L32-L42)），
  **加列式、幂等**：给 `Mod` 增字段即自动建列，无需手写迁移。
- 前端类型 [ui/src/types/server.ts:27-38](../../../ui/src/types/server.ts#L27-L38) 的 `Mod`；
  列表渲染 [ui/src/components/server-manage/ModsSection.tsx:193-213](../../../ui/src/components/server-manage/ModsSection.tsx#L193-L213)
  目前显示 `name || workshop_id`、`workshop_id`、`version`。
- i18n 文案 key 在 `serverConfig.mods.*`（`ui/messages/{zh,en,ja}.json`）。

## Requirements

- R1 后端 `palmod.Info` 增加 `ModName string` 与 `Tags []string`；`ParseInfo` 从
  `Info.json` 解析这两个字段，保持现有宽容策略（缺失→零值，不报错）。
- R2 `models.Mod` 增加 `ModName`(列 `mod_name`) 与 `Tags`(列 `tags`，`serializer:json`
  存储，JSON 输出为字符串数组) 两个字段，默认空。
- R3 `Manager.UpdateMods` 回填时把 `mod_name`、`tags` 与既有 `package_name`/`version`
  一并写库；解析失败或缺字段时保持为空，不阻断其它 mod（沿用现有 partial-success 语义）。
- R4 前端 `Mod` 类型增加 `mod_name: string`、`tags: string[]`；Mods 列表按下述层级展示：
  - 主标题：优先 `mod_name`；为空时回退 `name`；再回退 `workshop_id`。
  - 次级灰色小字行：`PackageName`（有值时）+ `workshop_id` + `version`。
  - Tags：有 tag 时以小徽章(badge)形式展示在条目下方；无 tag 不显示占位。
- R5 未下载（`package_name === ''`）的 mod 维持现有“未下载”告警图标行为不变。
- R6 mod 更新完成 → 全部成功时**关闭日志弹窗 + 刷新 mod 列表**；有失败时保持弹窗打开
  （错误在日志里可见）并仍刷新列表（部分成功）。**不使用轮询**——用现有 steamcmd SSE 流
  推送完成事件（用户明确要求避免轮询）。
- R7 SSE 通道从纯 `log` 文本升级为“带事件名的消息”：`UpdateMods` 后台任务返回后，
  handler 依其返回值（nil=全成功）广播 `done` 事件（`ok`/`error`）到 steamcmd 流；
  前端 `ServerLogsDialog` 监听 `done` 事件回调，`ModsSection` 据此关弹窗/刷新。
  完成结果取自 `process.UpdateMods` 返回值，**不读库、不匹配日志文本**。

## Technical Notes

- Tags 存储：`Tags []string` + `gorm:"column:tags;serializer:json"`，SQLite 存 JSON 文本，
  API 天然输出数组，前端无需再解析。
- 无破坏性迁移：仅新增列，旧库旧行的新列为空，`AutoMigrate` 自动补列。
- 展示不新增接口（缩略图排除，故无需图片服务端点）。

## Acceptance Criteria

- [x] AC1 下载成功后，`GET /servers/:id/mods` 返回的 mod 含非空 `mod_name`、
      `package_name`、`tags`（对应 Info.json）。回填走 `Select`+结构体 `Updates`；
      `TestModTagsSerializerRoundTrip` 验证 `[]string` 存取无损。
- [x] AC2 Mods 列表 UI 显示 ModName（主标题）、PackageName（次级行），及 Tags（badge，
      有 tag 才显示）。[ModsSection.tsx](../../../ui/src/components/server-manage/ModsSection.tsx) 已实现，`bun run build` 通过。
      *（真机视觉待下载真实 mod 后确认。）*
- [x] AC3 `Info.json` 缺 `ModName`/`Tags` 时不报错，零值降级；`TestParseInfoNumericVersionAndMissingFields`
      断言 `ModName==""`、`Tags==nil`；前端 `tags ?? []` 容错。
- [x] AC4 旧库 AutoMigrate 加列启动，旧行新字段为空；`TestInitializeLegacyDB` 通过。
- [x] AC5 `go test ./...` 全绿、`go build .` 通过、`bun run lint`/`bun run build` 通过。
- [x] AC6 mod 更新完成→全成功自动关日志弹窗+刷新列表；失败保持弹窗。经 SSE `done`
      事件（`logger.Msg{Event,Data}` + `BroadcastEvent`），无轮询。构建/类型检查通过；
      *运行时行为待 Windows 真机 + 真实 mod 下载端到端确认。*

## Out of Scope

- Thumbnail 缩略图展示（已确认排除）。
- 新增任何图片服务端点。
- mod 的 Author / Dependencies / MinRevision 等其它 Info.json 字段。

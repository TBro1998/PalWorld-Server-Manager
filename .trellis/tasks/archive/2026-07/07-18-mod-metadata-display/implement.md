# Implement — mod 元信息展示(ModName/PackageName/Tags)

## 有序清单

1. **palmod 解析** — [internal/palmod/info.go](../../../internal/palmod/info.go)
   - `Info` 加 `ModName string`、`Tags []string`。
   - `rawInfo` 加 `ModName json:"ModName"`、`Tags []string json:"Tags"`。
   - `ParseInfo` 拷贝 `raw.ModName`、`raw.Tags` 到 `Info`。
   - 更新 [palmod_test.go](../../../internal/palmod/palmod_test.go)：在
     `TestParseInfoTolerant` 断言 ModName/Tags 被解析；新增/扩展一例覆盖缺 ModName/Tags
     时为零值不报错。

2. **数据模型** — [internal/models/server.go](../../../internal/models/server.go)
   - `Mod` 加 `ModName string gorm:"column:mod_name;default:''" json:"mod_name"`。
   - `Mod` 加 `Tags []string gorm:"column:tags;serializer:json" json:"tags"`。
   - 更新结构体注释说明二者来源 Info.json、下载后回填。

3. **回填** — [internal/process/manager.go](../../../internal/process/manager.go) `UpdateMods`
   - 成功分支 `Updates(map[string]any{...})` 增加 `mod_name`、`tags`。
   - 本地 `mod.ModName`/`mod.Tags` 同步。
   - 校验 serializer 生效（见校验项）；不生效则改结构体 `Updates`+`Select`。

4. **前端类型** — [ui/src/types/server.ts](../../../ui/src/types/server.ts)
   - `Mod` 加 `mod_name: string`、`tags: string[]`（注释标注可能为空/null）。

5. **前端 UI** — [ModsSection.tsx](../../../ui/src/components/server-manage/ModsSection.tsx)
   - 主标题 `m.mod_name || m.name || m.workshop_id`。
   - 次级行加入 `package_name`（有值时）。
   - 条目下方渲染 tags badge（`(m.tags ?? []).length > 0` 时）。

6. **i18n** — `ui/messages/{zh,en,ja}.json`
   - 如 UI 需要“PackageName / Tags”标签文案，在 `serverConfig.mods.*` 增加对应 key
     （三语）。若采用纯值展示（tag 文本直接显示、package 前无标签）可少加或不加。

## 校验命令

- 后端：`go test ./internal/palmod/ ./internal/process/ ./internal/api/`
- serializer round-trip：在 process 或 models 层加一个写入含 Tags 的 mod 再读回断言相等
  的用例（或临时脚本验证），确认 `[]string` 正确 JSON 存取。
- 构建：`go build .`
- 前端：`cd ui && bun run lint`（如改动 UI）；`bun run build` 确认静态导出通过。

## 风险文件 / 回滚点

- [internal/process/manager.go](../../../internal/process/manager.go)：`Updates` 的
  serializer 行为是唯一不确定点；先跑 round-trip 单测确认。回滚点 = 该文件单处编辑。
- 其余均为加法式改动，逐文件可独立还原。

## start 前检查

- prd.md 收敛完成、AC 可测。
- design.md/implement.md 就位。
- 本任务为 inline 工作流，跳过 implement.jsonl/check.jsonl 硬门槛。

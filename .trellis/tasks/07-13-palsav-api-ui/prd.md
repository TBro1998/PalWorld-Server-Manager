# 存档数据 REST 接口与前端展示

> 父任务:`07-13-palworld-sav-reader-go`。**依赖子任务 A `palsav-core-lib`**(需其公开结构体 Player/Pal/Guild/Item)。A 跑通后再补 `design.md` + `implement.md`。

## Goal

把子任务 A 解析出的玩家/公会/背包/帕鲁数据,通过 REST API 暴露,并在前端 Web UI 展示。

## Requirements(初稿,A 完成后细化)

- R1 REST 端点(Gin `protected` 组,遵循 `internal/api/router.go` 现有约定):
  - `GET /api/servers/:serverId/save/players` — 玩家列表(含等级、公会、在线时间等)。
  - `GET /api/servers/:serverId/save/players/:playerUid/pals` — 该玩家的帕鲁列表。
  - `GET /api/servers/:serverId/save/players/:playerUid/inventory` — 该玩家背包。
  - `GET /api/servers/:serverId/save/guilds` — 公会列表(含成员)。
  - (端点形态最终以 A 的结构体与前端需求定;可能合并为一个概览端点。)
- R2 服务端定位存档路径:由 serverId → `Pal/Saved/SaveGames/0/<worldid>/{Level.sav,Players/*.sav}`,复用现有服务器目录逻辑(`internal/palconfig`/`server-manage` 相关)。
- R3 解析结果**缓存**(存档大、解析有成本):按文件 mtime 失效;避免每次请求全量解析 Level.sav。
- R4 前端页面:在服务器管理界面新增"存档/玩家"视图,列表 + 详情(帕鲁、背包)。走 `apiClient`,i18n(zh/en/ja),shadcn/ui。
- R5 错误与边界:存档不存在/损坏/解析失败时返回结构化错误,前端可提示。

## Acceptance Criteria(初稿)

- [ ] AC1 端点返回真实存档的四类数据 JSON,字段与 A 结构体一致。
- [ ] AC2 前端页面可列出玩家/公会,并查看某玩家帕鲁与背包。
- [ ] AC3 二次请求命中缓存(存档未变时不重复全量解析)。
- [ ] AC4 前端构建 `bun run build` 通过,嵌入后端可用;i18n 三语齐全。

## 约束

- 遵循 CLAUDE.md:前端静态导出 + 嵌入;API 相对路径;JWT 保护;Windows 优先。
- 只读:不修改存档。

## Notes

- 细化时机:子任务 A 的 `model.go` 稳定后。届时补 design(端点契约/缓存策略/前端组件)与 implement(有序清单)。

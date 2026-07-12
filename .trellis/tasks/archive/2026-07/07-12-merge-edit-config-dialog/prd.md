# 合并编辑与配置界面并清理 RCON

## Goal

把服务器卡片上的"编辑"和"配置"两个按钮及其弹窗合并为**单个"设置"（齿轮）按钮 + 单弹窗多标签**界面，消除重复参数，并彻底移除已弃用的 RCON 配置。让每个可配置项在 UI 上只有一个编辑入口，数据来源统一到"实际生效"的层。

## Background

当前服务器卡片在 `idle`（stopped/error）状态下同时显示"编辑"和"配置"两个按钮，打开两个独立弹窗：

- 编辑弹窗 [EditServerDialog.tsx](ui/src/components/EditServerDialog.tsx)：名称、安装目录、游戏端口、查询端口、RCON端口、启用RCON。写 `servers` 表（`PUT /servers/:id`）。
- 配置弹窗 [ServerConfigDialog.tsx](ui/src/components/ServerConfigDialog.tsx)：PalWorldSettings.ini 结构化参数（4 分类）+ 启动参数标签 + 原始文本标签。写 INI + `launch_args`（`PUT /servers/:id/config`）。

两弹窗之间与配置弹窗内部都存在重复参数；RCON 官方已弃用（改用 REST API，https://docs.palworldgame.com/category/rest-api）。

## Confirmed Facts (code inspection)

- **`servers` 表列 `port` / `query_port` / `rcon_port` / `rcon_enabled` 是惰性展示元数据**：仅在 [handlers.go](internal/api/handlers.go) CRUD 与前端展示中读写；进程启动 [manager.go:73](internal/process/manager.go#L73) 只读 `install_path, pid, launch_args`；全仓库无代码把它们写入 INI 或命令行。→ 不影响真正运行的服务器。
- **真正决定服务器行为的两处**：`servers.launch_args`(JSON)→命令行 [launchargs.go](internal/palconfig/launchargs.go)（`-port/-players/-publicip/-publicport/-logformat/线程类/-publiclobby`）；`PalWorldSettings.ini` OptionSettings [schema.go](internal/palconfig/schema.go)（含 `RCONEnabled/RCONPort/RESTAPIEnabled/RESTAPIPort/PublicIP/PublicPort/LogFormatType/ServerName` 等）。
- **参数语义澄清**：`-port`（游戏绑定端口）INI 无对应项，是启动参数独有；`publicIP/publicPort/logFormat` 在启动参数与 INI 各一份（运行时启动参数覆盖 INI）——这才是真正的重复。
- schema 已内置 `RESTAPIEnabled`(默认 False) 与 `RESTAPIPort`(默认 8212)。
- INI 读写：`serializeInner` 先按 registry 顺序写已知参数，未知键排序后**逐字透传**；`LoadSettings` 用 `Defaults()` 与文件值取并集。→ 把 RCON 移出 schema 后，已存在 INI 里的 `RCONEnabled` 会作为未知键被逐字保留（非破坏、值为 False 时无害），不会被我们主动写回。
- DB：`modernc.org/sqlite v1.33.1`（SQLite ≥3.46，支持 `ALTER TABLE DROP COLUMN`）。`servers` 表 CREATE 语句含 port/query_port/rcon_port(NOT NULL) 与 rcon_enabled。
- 入口时机：`servers`/编辑/配置按钮仅在 `idle` 显示（[ServerCard.tsx](ui/src/components/ServerCard.tsx)），合并弹窗天然只在停服时打开——满足 INI/目录"仅停止时可改"的后端约束（[handlers.go](internal/api/handlers.go) UpdateServerConfig 要求 status==stopped）。
- 前端 `server.port` 仅用于 [ServerCard.tsx:66](ui/src/components/ServerCard.tsx#L66) 与 EditServerDialog。
- 平台：项目当前仅面向 Windows；保留 Linux 分支代码不破坏（CLAUDE.md）。

## Requirements

- **R1 单一入口**：服务器卡片 `idle` 状态用**一个"设置"（齿轮图标）按钮**取代原"编辑"+"配置"两个按钮；打开单个统一弹窗。
- **R2 统一弹窗标签**：单弹窗含标签：`基础` / `性能` / `服务器管理` / `玩法` / `平衡` / `启动参数` / `原始文本`。`基础`承接服务器名称、安装目录、游戏端口(`-port`)。
- **R3 名称统一**：UI 只保留单一"服务器名称"（绑定 `servers.name`）；保存时若已安装，自动写入 INI `ServerName`；结构化表单不再单列 `ServerName`。
- **R4 网络参数去重**：`启动参数`标签移除 `publicIP/publicPort/logFormat`，这三项只在 INI(`服务器管理`) 编辑。启动参数标签保留：`players / usePerfThreads / noAsyncLoadingThread / useMultithreadForDS / numberOfWorkerThreadsServer / publicLobby`；游戏端口 `-port` 移到`基础`标签。
- **R5 数据来源归一**：以 `INI + launch_args` 为唯一来源。移除 `servers` 表惰性列 `port/query_port/rcon_port/rcon_enabled`（含 CREATE 语句、DROP 迁移、model、handlers、前端类型）。不把惰性列接线到启动流程。
- **R6 彻底移除 RCON**：`RCONEnabled/RCONPort` 移出 `schema.Params`；前端不再展示；DB 列随 R5 一并移除。REST API 参数（`RESTAPIEnabled/RESTAPIPort`）保留并正常展示于`服务器管理`。
- **R7 保存编排**：合并弹窗一次"保存"同时落库：名称/安装目录/启动参数 → `PUT /servers/:id`；INI settings/raw（含同步的 `ServerName`）→ `PUT /servers/:id/config`。任一失败给出明确错误且不静默吞掉。
- **R8 卡片端口展示**：[ServerCard.tsx](ui/src/components/ServerCard.tsx) 的端口展示改为从 `launch_args` 解析 `-port`（无则显示默认 8211）。
- **R9 i18n**：`zh/en/ja` 三语言同步——移除 `editServer` 里 RCON/查询端口相关键、`serverConfig.launch` 里 publicIP/publicPort/logFormat 键、以及 `params.RCONEnabled/RCONPort`；新增合并弹窗所需键（基础标签、齿轮按钮 `settings` 等）。

## Acceptance Criteria

- [ ] AC1：卡片 `idle` 状态只有一个"设置"齿轮按钮；点击打开含 7 个标签的单弹窗；不再存在独立的"编辑"和"配置"按钮/弹窗。
- [ ] AC2：`基础`标签可编辑名称、安装目录、游戏端口；保存后 `servers.name`、`install_path`、`launch_args.port` 正确更新。
- [ ] AC3：已安装服务器保存后，INI `ServerName` 等于 UI 名称；结构化表单中不再出现单独的 `ServerName` 项。
- [ ] AC4：`启动参数`标签不含 publicIP/publicPort/logFormat；`服务器管理`标签仍可编辑 `PublicIP/PublicPort/LogFormatType`。
- [ ] AC5：UI 任一标签、schema 接口、DB 表均不含 RCON（`RCONEnabled/RCONPort`、`rcon_port/rcon_enabled`）；`RESTAPIEnabled/RESTAPIPort` 仍在`服务器管理`可编辑。
- [ ] AC6：`servers` 表不再有 `port/query_port/rcon_port/rcon_enabled` 列（全新库 CREATE 不含；旧库经 DROP 迁移移除）；后端编译通过，`go test ./...` 通过。
- [ ] AC7：`GET /servers` 返回体不含 port/query_port/rcon_port/rcon_enabled 字段；前端 `Server` 类型同步；卡片端口从 launch_args 展示。
- [ ] AC8：`bun run build` 与 `bun run lint` 通过；三语言 JSON 合法且无残留失效键被引用。
- [ ] AC9：一次保存能同时更新元数据与 INI；停服状态下保存成功，错误路径有可见提示。

## Out of Scope

- 不新增/改造 REST API 后端逻辑，仅保证 `RESTAPIEnabled/RESTAPIPort` 在现有 INI 表单里可编辑。
- 不主动改写游戏自带 INI 里已存在的 RCON 键（透传即可）。
- 不处理 `error` 状态下 INI 保存被后端拒绝的既有行为（与本任务无关，维持现状）。
- 不改动 mods/auth/system 等其他模块；不清理未使用的 `servers.status` 列。

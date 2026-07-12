# PRD: 后端数据库层迁移到 GORM

## Goal

将后端数据库访问从手写 `database/sql` 迁移到 GORM，driver 使用 `github.com/glebarez/sqlite`（纯 Go，无 CGO，底层 `modernc.org/sqlite`），在**不改变外部行为**的前提下简化迁移逻辑与消除 Scan 样板。

## User Value

- 用 GORM `AutoMigrate` 替代手写的 `addColumnIfMissing`/`dropColumnIfExists`，未来加表/加字段更省心。
- 消除 `rows.Scan(&a,&b,...)` 与 `serverColumns`/`scanServer` 的隐式契约，字段增删不再易错。
- 保持单文件分发核心架构（无 CGO）。

## Confirmed Facts（代码已确认）

- Go 1.26；当前 driver `modernc.org/sqlite v1.33.1`。
- 三张表：`servers`、`mods`、`users`。`mods`/`users` 相关 handler 目前是 stub（`ListMods` 返回空、`Login/Register` 未实现）。
- `models.Server.Status` 是派生值（`process.DeriveStatus`），**从不持久化**；schema 里 `status` 列存在但运行时从不读写。
- 状态派生依据的持久化事实仅：`pid`、`last_error`；启动只读 `install_path, pid, launch_args`。
- 现有 migrate 行为：`CREATE TABLE IF NOT EXISTS` + 加列（launch_args/installed/last_error）+ 删除弃用列（port/query_port/rcon_port/rcon_enabled）。
- 决策（Q1）：迁移后**移除删列逻辑**，依赖 `AutoMigrate`（只做加法）。旧库残留弃用列与未用 `status` 列无害（运行时从不读），无需清理。
- `db` 流转链：`main.go` → `server.New` → `api.NewRouter`(Router.db) 与 `process.NewManager`(Manager.db)。

## 改动面（调用点清单）

- `main.go` — `database.Initialize` 返回类型。
- `internal/database/database.go` — 重写为 GORM 打开 + `AutoMigrate`。
- `internal/server/server.go` — `db` 字段与 `New` 签名。
- `internal/api/router.go` — `db` 字段与 `NewRouter` 签名。
- `internal/api/handlers.go` — `ListServers`/`CreateServer`/`GetServer`/`UpdateServer`/`DeleteServer`/`InstallServer`/`loadServerPathState`/`UpdateServerConfig`；移除 `serverColumns`/`scanServer`。
- `internal/process/manager.go` — `loadServer`/`setPID`/`setError`/`clearError`/`InstallServer` 内的 SQL。
- `internal/process/monitor.go` — `monitor`(setPID)/`ReconcileOnStartup`/`ReconcileInstalled`。
- `internal/models/*.go` — 为 Server/Mod/User 补 GORM tag。

## Requirements

- R1: 引入 `gorm.io/gorm` + `github.com/glebarez/sqlite`；`modernc.org/sqlite` 作为 glebarez 间接依赖保留。
- R2: `database.Initialize` 返回 `*gorm.DB`；启动用 `AutoMigrate(&Server{}, &Mod{}, &User{})`。
- R3: 所有持有/传递 `*sql.DB` 的结构体与构造函数改为 `*gorm.DB`。
- R4: 8 个 handler 与 process 层全部 SQL 改为 GORM API；删除 `serverColumns`/`scanServer`。
- R5: `models.Server.Status` 标 `gorm:"-"`；不引入任何新持久化列。
- R6: 保留派生状态语义——`status` 不写库；`pid`/`last_error` 仍是事实源。
- R7: 无 CGO：`go build` 与现有构建方式不需要 CGO。

## Acceptance Criteria

- AC1: `go build ./...` 通过；二进制不依赖 CGO。
- AC2: `GET /servers` 返回既有字段与格式不变（含派生 status）。
- AC3: 创建/更新/删除 server、安装、启动/停止、配置读写行为与迁移前一致。
- AC4: 对**已有旧库**启动不报错（AutoMigrate 幂等，旧库残留列不影响）。
- AC5: 派生 status 正确（installing > running > error > stopped）。

## Out of Scope

- 实现 mods/users 的业务逻辑（仍是 stub）。
- 新增功能、新增表或列。
- 更换其他 ORM（已定 GORM）。
- 清理旧库残留列（Q1 已决策：不清理）。

## Open Questions

- 无（Q1 已决策）。

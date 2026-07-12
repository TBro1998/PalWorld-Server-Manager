# Database Guidelines

> Database patterns and conventions for this project.

---

## Overview

- Driver: `modernc.org/sqlite`（纯 Go，无 CGO）。DB 通过 `database.Initialize(dbPath)` 打开并在启动时 `migrate()`。
- 无 ORM：直接用 `database/sql` 的 `db.Query` / `db.QueryRow` / `db.Exec`，手写 SQL。
- 迁移即代码：schema 变更写在 `internal/database/database.go` 的 `migrate()` 里，随进程启动幂等执行。

---

## Query Patterns

### Convention: `serverColumns` 常量必须与 `scanServer` 的 Scan 目标严格对齐

**What**: `internal/api/handlers.go` 用一个 `serverColumns` 字符串常量集中定义 `servers` 表的查询列，所有 `SELECT` 复用它；`scanServer` 按同一顺序 `Scan(...)` 到 `models.Server`。

**Why**: 列清单与 Scan 目标是两处手写、必须一一对应的隐式契约。任何一侧增删/改序而另一侧没同步，会导致运行期列错位（字段被写进错误目标或 `Scan` 报错），且编译期无法发现。

**Rule**: 改动 `servers` 表列时，必须同时改这三处并保持数量+顺序一致：`serverColumns` 常量、`scanServer` 的 `Scan(...)`、`models.Server` 结构体字段。改完立即 `go build ./...` 并对 `GET /servers` 冒烟。

**Example**:
```go
// 9 列，顺序与 scanServer 完全对应
const serverColumns = "id, name, install_path, last_error, pid, launch_args, installed, created_at, updated_at"

func scanServer(sc interface{ Scan(dest ...any) error }) (models.Server, error) {
    var s models.Server
    err := sc.Scan(&s.ID, &s.Name, &s.InstallPath, &s.LastError, &s.PID,
        &s.LaunchArgs, &s.Installed, &s.CreatedAt, &s.UpdatedAt) // 顺序与上面逐一对齐
    return s, err
}
```

---

## Migrations

### Convention: 加列用 `addColumnIfMissing`（致命），删列用 `dropColumnIfExists`（告警、非致命）

**What**: 既有库的结构演进走幂等 helper，均先 `PRAGMA table_info(<table>)` 判断列是否存在再动作。`CREATE TABLE IF NOT EXISTS` 只影响全新库——**改表结构时 CREATE 语句与迁移 helper 两处都要改**。

**Why**:
- 加列失败意味着后续读写会缺列 → 返回 error 让启动失败（fail fast）。
- 删列是清理惰性数据，列残留无害（运行时不读）→ DROP 失败只记 `warning` 并继续，避免因个别环境 SQLite 差异破坏启动韧性。

**Signatures**:
```go
func addColumnIfMissing(db *sql.DB, table, column, definition string) error // 失败返回 error（致命）
func dropColumnIfExists(db *sql.DB, table, column string) error              // DROP 失败记 warning 后返回 nil（非致命）
```

**Example**:
```go
// migrate() 末尾
_ = dropColumnIfExists(db, "servers", "port")
_ = dropColumnIfExists(db, "servers", "query_port")
_ = dropColumnIfExists(db, "servers", "rcon_port")
_ = dropColumnIfExists(db, "servers", "rcon_enabled")
```

> **Note**: `ALTER TABLE ... DROP COLUMN` 需 SQLite ≥ 3.35；本项目 `modernc.org/sqlite v1.33.1` 满足。仅对无索引/约束依赖的列使用。

---

## Naming Conventions

<!-- Table names, column names, index names -->

(To be filled by the team)

---

## Common Mistakes

### Gotcha: `servers` 表的端口/RCON 列不影响运行——生效源是 launch_args + INI

> **Warning**: 服务器运行时真正生效的配置只有两处：`servers.launch_args`（JSON → 命令行参数，见 `internal/palconfig/launchargs.go`）与 `PalWorldSettings.ini` 的 OptionSettings（见 `internal/palconfig/schema.go`）。进程启动 `internal/process/manager.go` 只读 `install_path, pid, launch_args`。
>
> 历史上 `servers` 表存在 `port/query_port/rcon_port/rcon_enabled` 列，但它们**只在 CRUD 与 UI 展示中读写，从不进入启动命令或 INI**，即惰性展示元数据。这些列已被移除。
>
> **教训**：要改服务器实际行为（端口、公网、日志格式等），改的是 `launch_args` 或 INI 参数，**不要**新增"看起来像配置"的 `servers` 表列——那样只会造出又一个不生效的惰性列。游戏绑定端口 `-port` 只有 launch arg，INI 无对应项；`PublicIP/PublicPort/LogFormatType` 在 launch arg 与 INI 各有一份，运行时 launch arg 覆盖 INI。

### Convention: RCON 已弃用，用 REST API

**What**: 项目不再暴露 RCON（`RCONEnabled/RCONPort` 已移出 `palconfig.Params`，`servers` 表 RCON 列已删）。远程管理走 Palworld 官方 REST API。

**Why**: RCON 官方已弃用（https://docs.palworldgame.com/category/rest-api）。

**Related**: REST API 通过 INI 参数 `RESTAPIEnabled` / `RESTAPIPort` 开启，这两项仍在 `palconfig.Params` 的 `serverManagement` 分类中，由结构化配置表单渲染。

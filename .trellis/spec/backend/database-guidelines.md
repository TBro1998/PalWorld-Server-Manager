# Database Guidelines

> Database patterns and conventions for this project.

---

## Overview

- ORM: **GORM**（`gorm.io/gorm`）。Driver：`github.com/glebarez/sqlite`（纯 Go，无 CGO，底层 `modernc.org/sqlite`）。
- DB 通过 `database.Initialize(dbPath)` 打开，返回 `*gorm.DB`，并在启动时 `migrate()` 跑 `AutoMigrate`。
- 迁移即代码：schema 由 `internal/models` 的结构体定义，`AutoMigrate(&Server{}, &Mod{}, &User{})` 随进程启动幂等执行。
- 关闭：`*gorm.DB` 无 `Close()`；取底层 `sqlDB, _ := db.DB(); sqlDB.Close()`。

---

## Models

### Convention: 每个持久化字段必须显式 `gorm:"column:<name>"`

**What**: `internal/models` 的 Server/Mod/User 为每个入库字段写死 `column` tag。

**Why**: 代码在 `Select`/`Where`/`Update`/`Order` 里大量用**裸字符串列名**（如 `Update("pid", 0)`、`Select("install_path","pid","launch_args")`、`Order("created_at DESC")`）。这些字符串必须与 GORM 实际建的列名逐一对齐，否则运行期报 `no such column`，编译期发现不了。

> **Gotcha（已踩）**: `PID int` 若不加 column tag，GORM 命名策略会生成 `p_id`（`PID` 不在 GORM 的 initialism 列表里），而代码里写的是 `"pid"` → `Create` 成功但 `Update("pid", ...)` 报 `no such column: pid`。显式 `gorm:"column:pid"` 消除该分歧。旧 `db:"..."` tag 对 GORM 无效，不要依赖。

**Rule**: 新增/改动入库字段时，同时确认三处一致：结构体 `column` tag、代码里引用该列的裸字符串、`json` tag（对外契约）。改完 `go build ./...` 并跑 `internal/database` 的迁移/CRUD 测试。

### Convention: 派生字段用 `gorm:"-"`，不要用 `gorm.Model`

**What**: `Server.Status` 标 `gorm:"-"`（派生值，见 `process.DeriveStatus`），从不入库/出库。模型用显式字段而非内嵌 `gorm.Model`。

**Why**:
- `Status` 是运行期从内存状态 + `pid`/`last_error` 派生的，持久化会造成双源真相。
- `gorm.Model` 内嵌 `DeletedAt` 会触发**软删除**，改变 `DeleteServer` 的硬删语义。项目要硬删，故不用 `gorm.Model`。

---

## Query Patterns

### Convention: 部分列更新用单列 `Update` 或 `Updates(map)`，禁用 `Updates(struct)`

**What**: 只改个别列时用 `db.Model(&models.Server{}).Where("id = ?", id).Update("col", val)`，多列用 `Updates(map[string]any{...})`。

**Why**: `Updates(struct{})` 会**跳过零值字段**。而本项目里 `pid=0`、`last_error=""`、`installed=false` 都是合法目标值（停服、清错、标记未安装），用 struct 更新会被静默忽略。单列 `Update` 与 `map` 不跳零值。`updated_at` 由 GORM 自动刷新，不用手写。

**Example**:
```go
// 停服：pid 归零必须写入
db.Model(&models.Server{}).Where("id = ?", id).Update("pid", 0)
// 目录 + installed 一起改
db.Model(&models.Server{}).Where("id = ?", id).
    Updates(map[string]any{"install_path": p, "installed": ok})
```

### Convention: not-found 判断用 `errors.Is(err, gorm.ErrRecordNotFound)`

**What**: `First`/`Take` 查不到返回 `gorm.ErrRecordNotFound`。判断用 `errors.Is`，不要再比较 `err.Error() == "sql: no rows in result set"`。

### Convention: 创建用 `Create`，ID 与时间戳自动回填

**What**: `db.Create(&server)` 后 `server.ID`、`CreatedAt`、`UpdatedAt` 由 GORM 自动填充；无需手写 `time.Now()` 或 `LastInsertId()`。

---

## Migrations

### Convention: 只用 `AutoMigrate`，它只做加法、不删列

**What**: schema 演进靠 `db.AutoMigrate(...)`：创建缺失的表/列/索引，幂等，新旧库都安全，每次启动都跑。

**Why / Gotcha**: `AutoMigrate` **不会删列**。历史遗留的弃用列（曾经的 `port`/`query_port`/`rcon_port`/`rcon_enabled`）与未用的 `status` 列在旧库中会残留——但**运行时从不读，无害**，无需清理（曾用的手写 `addColumnIfMissing`/`dropColumnIfExists` 已随 GORM 迁移移除）。若将来确需删列，用 `db.Migrator().DropColumn(&Model{}, "col")` 显式处理，并评估韧性（个别环境 DROP 失败不应阻断启动）。

### Gotcha（已踩）: glebarez migrator 无法重建带 `FOREIGN KEY` 子句的旧表

**What**: 旧手写 schema 的 `mods` 表带 `FOREIGN KEY (server_id) REFERENCES servers(id) ON DELETE CASCADE`。当 AutoMigrate 需要**重建**该表（SQLite 无法 ALTER COLUMN，故走 建临时表→拷贝→改名）时，glebarez SQLite migrator 会**误把 `FOREIGN` 关键字当成列名**，启动崩溃：`table mods__temp has no column named FOREIGN`。

**Fix**: `migrate()` 在 AutoMigrate 前调用 `dropLegacyModsIfEmpty(db)`——`mods` 是 stub、从未写入，仅当表**为空**时 drop 掉让 AutoMigrate 干净重建（无 FK 子句），非空表绝不删（防数据丢失）。见 `internal/database/database.go`，回归用例 `TestInitializeLegacyModsFK`。

**Rule**: GORM 模型不要用原始 `FOREIGN KEY` DDL 表达关联；需要外键时用 GORM 关联（`foreignKey`/`references`）由 GORM 自己生成，避免 migrator 重建时的解析坑。

---

### Gotcha（已踩，2026-07-20）: 读旧表迁移数据时只 SELECT 该旧 schema 真有的列

**What**: `collectLegacyMods`（把 per-server `mods` 旧行迁到 全局库+`server_mods` 新 schema）曾无条件 `SELECT id, server_id, workshop_id, name, enabled, install_path, package_name, mod_name, version`。但 `package_name/mod_name/version` 是**全局库重构时才加的列**，最老的 per-server `mods` 表根本没有它们 → SQLite 报 `no such column: package_name`，`Initialize` 失败。此前该分支被"测试包编译不过、`TestInitializeLegacyModsFK` 从未真跑"掩盖，修好测试后才暴露。

**Fix**: 迁移读取按 `hasRawColumn(db,"mods",col)` **动态拼列**——固定列（`id/server_id/workshop_id/name/enabled/install_path`）+ 仅存在时才追加的可选列（`package_name/mod_name/version`），再 `SELECT strings.Join(cols)`。缺失的可选列在 `legacyModRow` 里留零值，`insertLegacyMods` 照常。

**Rule**: 任何"读旧 schema 行做迁移"的 SQL 都不能假设新列存在；跨 schema 版本读取按列探测（`hasRawColumn`）拼 SELECT。改迁移逻辑务必让 `internal/database` 测试包**能编译**，否则 `TestInitializeLegacyMods*` 这类回归用例会静默不执行。

---

## Common Mistakes

### Gotcha: `servers` 表的端口/RCON 列不影响运行——生效源是 launch_args + INI

> **Warning**: 服务器运行时真正生效的配置只有两处：`servers.launch_args`（JSON → 命令行参数，见 `internal/palconfig/launchargs.go`）与 `PalWorldSettings.ini` 的 OptionSettings（见 `internal/palconfig/schema.go`）。进程启动 `internal/process/manager.go` 只读 `install_path, pid, launch_args`。
>
> **教训**：要改服务器实际行为（端口、公网、日志格式等），改的是 `launch_args` 或 INI 参数，**不要**新增"看起来像配置"的 `servers` 表列——那样只会造出又一个不生效的惰性列。游戏绑定端口 `-port` 只有 launch arg，INI 无对应项；`PublicIP/PublicPort/LogFormatType` 在 launch arg 与 INI 各有一份，运行时 launch arg 覆盖 INI。

### Convention: RCON 已弃用，用 REST API

**What**: 项目不再暴露 RCON（`RCONEnabled/RCONPort` 已移出 `palconfig.Params`，`servers` 表 RCON 列已删）。远程管理走 Palworld 官方 REST API。

**Why**: RCON 官方已弃用（https://docs.palworldgame.com/category/rest-api）。

**Related**: REST API 通过 INI 参数 `RESTAPIEnabled` / `RESTAPIPort` 开启，这两项仍在 `palconfig.Params` 的 `serverManagement` 分类中，由结构化配置表单渲染。

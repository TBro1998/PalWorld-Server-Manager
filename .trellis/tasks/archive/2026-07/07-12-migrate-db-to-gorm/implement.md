# Implement: 后端数据库层迁移到 GORM

## 执行顺序

按依赖自底向上：依赖 → 模型 → database 层 → 各调用点 → 编译 → 验证。

### 1. 依赖

- [ ] `go get gorm.io/gorm github.com/glebarez/sqlite`
- [ ] 确认 go.mod 更新；`modernc.org/sqlite` 转为间接依赖。

### 2. models（补 GORM tag）

- [ ] `internal/models/server.go`：Server/Mod/User 加 `primaryKey` 等 tag；`Server.Status` 加 `gorm:"-"`；**不使用 `gorm.Model`**。清理旧 `db:"..."` tag。

### 3. database 层

- [ ] `internal/database/database.go`：重写 `Initialize` → `gorm.Open(sqlite.Open(dbPath))` + `AutoMigrate(&Server,&Mod,&User)`；删除 `migrate`/`addColumnIfMissing`/`dropColumnIfExists`。
- [ ] `main.go`：`db` 变量类型随 `Initialize` 返回 `*gorm.DB`；`defer db.Close()` 改为 `sqlDB, _ := db.DB(); defer sqlDB.Close()`。

### 4. 结构体与构造函数签名

- [ ] `internal/server/server.go`：`db *gorm.DB` + `New(cfg, db *gorm.DB, ...)`。
- [ ] `internal/api/router.go`：`db *gorm.DB` + `NewRouter(db *gorm.DB, ...)`。
- [ ] `internal/process/manager.go`：`db *gorm.DB` + `NewManager(db *gorm.DB, ...)`。

### 5. handlers.go 改写

- [ ] 删除 `serverColumns` 常量与 `scanServer` 函数。
- [ ] `ListServers` → `Find(&servers).Order(...)`；空列表仍返回 `[]`。
- [ ] `CreateServer` → `Create(&s)`；空 path 回填走二次 `Updates`。
- [ ] `GetServer` → `First(&s, id)`；not-found 用 `gorm.ErrRecordNotFound`。
- [ ] `UpdateServer` → 分段 `Model(...).Updates(map[string]any{...})`；保留“仅 stopped 可改目录”校验。
- [ ] `DeleteServer` → 先 `First` 判存在+派生状态校验，再 `Delete(&Server{}, id)`。
- [ ] `InstallServer` → 存在性用 `Select("id").First`。
- [ ] `loadServerPathState` → `Select(...).First(&s, id)` 后取字段。
- [ ] `UpdateServerConfig` 内 launch_args 更新 → `Model(...).Updates(map{...})`。
- [ ] 全文件 not-found 判断统一改 `errors.Is(err, gorm.ErrRecordNotFound)`。

### 6. process 层改写

- [ ] `manager.go`：`loadServer` → `Select("install_path","pid","launch_args").First`；`setPID`/`setError` → `Model(&Server{ID:id}).Updates(map{...})`（pid=0/空字符串必须用 map）；`InstallServer` 内 installed 0/1 更新同理。
- [ ] `monitor.go`：`ReconcileOnStartup`/`ReconcileInstalled` → `Find` + 循环 `Updates`；`monitor` 的 setPID(0) 已在 manager 复用。
- [ ] 移除 `database/sql` import（若不再使用）与 `sql.ErrNoRows` 判断。

## 验证命令

- [ ] `go build ./...`（AC1）
- [ ] `go vet ./...`
- [ ] `cd ui && bun run build`（若需完整二进制）后 `cd .. && go build .`
- [ ] 运行 `go run .`，冒烟：
  - [ ] `GET /api/servers` 字段/格式不变、含派生 status（AC2、AC5）
  - [ ] 创建→更新→配置读写→删除 一轮（AC3）
  - [ ] 用**已存在的旧 db 文件**启动不报错（AC4）

## 风险文件 / 回滚点

- 高风险：`handlers.go`（改动点最多，易漏 not-found 判断）、`manager.go`（部分列更新的零值陷阱）。
- 回滚：所有改动在一次提交内，`git revert` 即可；不触碰数据、不删列，无数据破坏风险。

## 完成前检查

- [ ] 无遗留 `*sql.DB` / `db.Query|QueryRow|Exec` / `sql.ErrNoRows`（grep 确认）。
- [ ] `models.Server.Status` 未入库（建库后 `PRAGMA table_info(servers)` 新库无 status 列亦可）。
- [ ] 更新 spec：`.trellis/spec/backend/database-guidelines.md`（Phase 3.3）。

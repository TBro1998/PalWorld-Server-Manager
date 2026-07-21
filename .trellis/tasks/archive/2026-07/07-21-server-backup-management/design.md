# 服务器备份管理 — 技术设计

## 架构与边界

分层，复用现有范式（`process.Manager` + `internal/api` handler + `internal/config` + GORM model）：

```
internal/backup/            新增包：纯备份领域逻辑，不依赖 gin
  archive.go                CreateZip(scope, saveDir, configDir, dest) / ExtractZip(zip, targets)
  archive_test.go
  retention.go              Prune(records, keepCount, keepDays) -> 待删列表
  retention_test.go
  manifest.go               Manifest 结构（scope/serverID/createdAt/source/hot）写入/读取 zip 内 manifest.json
  reconcile.go              启动对账：DB 记录 <-> 磁盘 zip
  scheduler.go              per-server ticker 调度器（间隔触发 create + prune）
  scheduler_test.go
internal/models/backup.go   Backup、BackupSchedule 两个 GORM model
internal/config/config.go   新增 BackupDir 字段（yaml:"backup_dir" env:"BACKUP_DIR" envDefault:"./backups"）
internal/api/backup_handlers.go   HTTP handlers（薄层，调用 internal/backup + process.Manager）
internal/api/router.go      注册 /servers/:id/backups 子路由
internal/server/server.go   启动装配：backup.Reconcile + scheduler.Start
ui/src/components/server-manage/BackupSection.tsx  替换占位为真实 UI
ui/src/lib/api.ts           backupsApi
ui/src/types/server.ts      Backup / BackupSchedule 类型
ui/messages/{zh,en,ja}.json 扩充 serverManage.backup 键
```

设计原则：`internal/backup` 只做文件与策略计算，不碰 DB/HTTP；DB 与进程状态判断留在 handler 层，与 `save_handlers.go` / `mod_handlers.go` 现有分工一致。

## 数据模型（新增 GORM model，加入 database.go migrate 列表）

```go
// internal/models/backup.go
type Backup struct {
    ID        int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
    ServerID  int64     `gorm:"column:server_id;not null;index" json:"server_id"`
    Scope     string    `gorm:"column:scope;not null" json:"scope"`   // "save" | "config" | "all"
    Source    string    `gorm:"column:source;not null" json:"source"` // "manual" | "auto" | "pre-restore"
    Hot       bool      `gorm:"column:hot;default:false" json:"hot"`  // 服务器运行中所做备份
    SizeBytes int64     `gorm:"column:size_bytes;default:0" json:"size_bytes"`
    FilePath  string    `gorm:"column:file_path;not null" json:"-"`   // 绝对/相对 zip 路径，前端不需要
    CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

type BackupSchedule struct {
    ServerID        int64     `gorm:"column:server_id;primaryKey" json:"server_id"` // 每服务器一行
    Enabled         bool      `gorm:"column:enabled;default:false" json:"enabled"`
    IntervalMinutes int       `gorm:"column:interval_minutes;default:60" json:"interval_minutes"`
    Scope           string    `gorm:"column:scope;default:'all'" json:"scope"`
    KeepCount       int       `gorm:"column:keep_count;default:10" json:"keep_count"` // 0 = 不限
    KeepDays        int       `gorm:"column:keep_days;default:0" json:"keep_days"`    // 0 = 不限
    UpdatedAt       time.Time `gorm:"column:updated_at" json:"updated_at"`
}
```

`backupID` 即 `Backup.ID`；zip 落盘名用 `<id>.zip`（先建记录拿到自增 ID，再写盘并回填 file_path/size）。

## 数据流 / 契约

### 创建备份（手动 & 自动共用）
1. handler 解析 serverID，加载 server；`palconfig.ConfigDir` / `palsave.LocateWorld` 求源目录（按 scope）。
2. 若 `process.Manager.IsRunning(serverID)`：尽力 `RestSave`（复用 rest_handlers 逻辑），置 hot=true；REST 不可用则告警继续。
3. `db.Create(&Backup{...FilePath 暂空})` 拿 ID → `backup.CreateZip(...)` 写入 `<backup_dir>/<serverID>/<id>.zip`（含 manifest.json）→ 回填 size/file_path。
4. `backup.Prune` 依据该 server 的 schedule 保留策略清理旧记录+文件（手动备份也触发清理）。
5. 返回新 Backup DTO。

### 恢复
1. handler 校验：`IsRunning` → 409 拒绝（提示先停服）。
2. 自动创建 source=pre-restore 备份（scope=all）。
3. `backup.ExtractZip` 按 zip 内 scope 覆盖 saveDir/configDir（先写临时目录再原子替换，失败回滚）。
4. 返回结果。

### 列表 / 下载 / 删除
- GET `/servers/:id/backups` → `[]Backup` 按 created_at desc。
- GET `/servers/:id/backups/:backupId/download` → `c.FileAttachment(file_path, name)`。
- DELETE `/servers/:id/backups/:backupId` → 删 zip + 删记录。

### 自动备份配置
- GET/PUT `/servers/:id/backups/schedule` → BackupSchedule。PUT 后调度器 reload 该 server 的 ticker。

### 路由（router.go，protected 组内 servers.Group("/:id")）
```
backups := servers.Group("/:id/backups")
backups.GET("",                r.ListBackups)
backups.POST("",               r.CreateBackup)          // body: {scope}
backups.GET("/schedule",       r.GetBackupSchedule)
backups.PUT("/schedule",       r.UpdateBackupSchedule)
backups.GET("/:backupId/download", r.DownloadBackup)
backups.DELETE("/:backupId",   r.DeleteBackup)
backups.POST("/:backupId/restore", r.RestoreBackup)
```
注意 `:backupId` 与既有 `:id` 同段路径下需保证 gin 路由不冲突（子路径不同段，安全）。

## 调度器

- `scheduler.Scheduler` 持有 `map[int64]*time.Ticker` + mutex + 依赖（db、创建备份的回调）。
- `Start()`：读所有 enabled schedule，为每个起一个 goroutine，按 interval 调 create+prune。
- `Reload(serverID)`：PUT schedule 后停旧 ticker、按新配置起新 ticker。
- 复用 `process.Manager` 判断运行态；备份失败只记日志，不 panic（对齐 update checker 的"后台非阻塞、结果不阻断"风格）。
- 在 `server.go:setupRoutes` 装配，紧随 `Checker().StartBackgroundCheck()`。

## 兼容性 / 迁移

- 新增两张表经 `AutoMigrate` 自动建，additive；旧库启动即补表，无数据迁移。
- 新增 `backup_dir` 配置项，默认 `./backups`，缺省即可用；env `BACKUP_DIR` 覆盖（需在 `config.Load()` 的显式 env 重载段补一行，对齐现有字段写法）。
- 前端 BackupSection 从占位替换为真实实现，路由 tab 已存在，无路由改动。

## 关键权衡

- **zip 立即压缩 vs 目录快照**：选 zip——下载零额外打包、单文件易管理；代价是大存档压缩耗时，MVP 可接受（同步执行，前端 loading）。
- **进程内 ticker vs cron 库**：选 ticker——零新依赖、固定间隔满足 R2；cron 表达式留后续。
- **DB 权威 + zip 内 manifest**：双写，DB 供快速列表，manifest 供脱离工具辨识与对账。
- **恢复要求停服**：牺牲便利换一致性与安全（避免覆盖正在被写的存档）。

## 运维 / 回滚

- 回滚点：备份为纯新增功能，禁用调度 + 不暴露路由即可完全关闭，不影响既有存档/进程逻辑。
- 恢复采用"临时目录 + 原子替换"，失败不留半覆盖状态。
- 磁盘/DB 不一致由启动 `Reconcile` 纠正：DB 有记录但 zip 缺失→标记删除该记录；zip 存在但无记录→忽略（不自动补录，避免误纳入无 manifest 的杂文件）。

## 安全

- 所有路由在 `protected`（JWT）组下，与现有一致。
- `:backupId` 必须校验归属该 serverID，防越权下载/删除他人备份。
- 路径拼接使用 `filepath.Join` + serverID/backupID 数值化，杜绝路径穿越。

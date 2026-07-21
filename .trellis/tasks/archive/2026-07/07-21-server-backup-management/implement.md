# 服务器备份管理 — 执行计划

## 有序实现清单

### 后端
1. `internal/config/config.go`：加 `BackupDir` 字段（yaml `backup_dir` / env `BACKUP_DIR` / default `./backups`），并在 `Load()` 的显式 env 重载段补 `BACKUP_DIR` 分支。
2. `internal/models/backup.go`：新增 `Backup`、`BackupSchedule`（见 design）。
3. `internal/database/database.go`：`migrate()` 的 `AutoMigrate(...)` 列表追加两 model。
4. `internal/backup/manifest.go` + `archive.go`：`Manifest` 结构、`CreateZip(scope, saveDir, configDir, dest)`、`ExtractZip(zip, saveDir, configDir)`；zip 内写 `manifest.json`。
5. `internal/backup/retention.go`：`Prune(records, keepCount, keepDays)` 计算待删列表（纯函数，好测）。
6. `internal/backup/reconcile.go`：`Reconcile(db, backupDir)` 对账。
7. `internal/backup/scheduler.go`：`Scheduler` per-server ticker，`Start()` / `Reload(serverID)` / `Stop()`。
8. `internal/api/backup_handlers.go`：List/Create/Download/Delete/Restore/Get+UpdateSchedule handlers；范围求路径复用 `palconfig.ConfigDir`、`palsave.LocateWorld`；运行态判断用 `r.process`；热备份前尽力 `RestSave`。
9. `internal/api/router.go`：注册 `/servers/:id/backups` 子路由组。
10. `internal/api/router.go` 的 `Router` struct + `NewRouter`：持有 `*backup.Scheduler`（或在 server.go 装配后注入），暴露 getter 供 server.go 启动。
11. `internal/server/server.go:setupRoutes`：调用 `backup.Reconcile` + `scheduler.Start()`（紧随 update checker）。

### 前端
12. `ui/src/types/server.ts`：`Backup`、`BackupSchedule` 类型。
13. `ui/src/lib/api.ts`：`backupsApi`（list/create/download/delete/restore/getSchedule/updateSchedule）。
14. `ui/src/components/server-manage/BackupSection.tsx`：替换占位——manage 面板（列表+创建范围选择+下载+删除+恢复确认）、auto 面板（启用/间隔/范围/保留策略表单）。sync 面板暂保留占位或并入 auto（对齐 i18n 现有键）。
15. `ui/messages/{zh,en,ja}.json`：扩充 `serverManage.backup` 下的操作/字段/提示键（三语）。

### Swagger
16. 为新 handlers 加 swagger 注释；按 CLAUDE.md 运行 `swag init -g internal/api/docs.go --parseInternal --parseDependency` 重生成并提交 `docs/`。

## 验证命令

```bash
# 后端
go build .
go test ./internal/backup/... ./internal/api/...

# 前端
cd ui && bun run build && bun run lint
```

手动验收：创建各 scope 备份→列表出现→下载 zip 可解压→停服后恢复→pre-restore 记录出现→启用间隔自动备份并到期→保留策略清理。

## 风险文件 / 回滚点

- `internal/database/database.go`（migrate）：只追加 model，勿动既有 legacy 迁移逻辑。
- `internal/api/router.go`：`Router` struct 改动影响面广，改动集中在新增字段/路由，勿动既有路由。
- `internal/server/server.go`：启动装配顺序，scheduler 起在 DB/reconcile 之后。
- 恢复逻辑（覆盖 saveDir/configDir）：务必临时目录 + 原子替换 + 失败回滚，最高破坏性操作。
- 回滚：整功能可通过不注册路由 + 不启动 scheduler 完全禁用，不影响既有功能。

## start 前检查

- [ ] `prd.md` 收敛通过、AC 可测。
- [ ] `design.md`、`implement.md` 完成。
- [ ] `implement.jsonl` / `check.jsonl` 已填真实 spec 条目（非种子行）。
- [ ] 用户 review 通过后再 `task.py start`。

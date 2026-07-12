# 执行计划 - SteamCMD修复与服务器生命周期管理

## 阶段 1: SteamCMD 命令顺序修复 [P0]

- [x] `internal/steamcmd/install.go`：交换 `+force_install_dir` 与 `+login anonymous` 顺序（force_install_dir 在前）
- [x] 更新注释说明顺序要求

**验证**: `go build .` 通过

**回滚点**: 此阶段独立，可单独提交发布

---

## 阶段 2: 进程管理包 (internal/process)

- [x] `platform.go`：`serverExecutable()`、`isProcessAlive()`、`killProcess()`（Windows/Linux 分支）
- [x] `manager.go`：`Manager` 结构 + `NewManager` + `StartServer` + `StopServer` + `RestartServer`
- [x] `monitor.go`：`monitor()` goroutine（等待退出更新 DB）+ `ReconcileOnStartup()`

**验证**: `go build ./internal/process/` 通过

---

## 阶段 3: 日志管理包 (internal/logger)

- [x] `capture.go`：`Capture` 实现 `io.Writer` + 轮转 + `ReadLogs()`
- [x] `stream.go`：`StreamManager` + `Subscribe`/`Unsubscribe`/`Broadcast`
- [x] `manager.go` 集成：启动时组合 `io.MultiWriter(capture, broadcaster)`

**验证**: `go build ./internal/logger/` 通过

---

## 阶段 4: API 集成 (internal/api)

- [x] `router.go`：Router 持有 `*process.Manager` 和 `*logger.StreamManager`，`NewRouter` 注入
- [x] `handlers.go`：实现 `StartServer` / `StopServer` / `RestartServer` / `GetLogs` / `StreamLogs`
- [x] `main.go`：初始化 StreamManager 和 Manager，调用 `ReconcileOnStartup()`

**验证**: `go build .` 通过

---

## 阶段 5: 构建验证与测试

- [x] `go build .` 全量构建通过
- [x] `go vet ./...` 无警告
- [x] 手动验证：安装 → 启动 → 查看日志 → 停止 → 重启
- [x] SSE 流验证
- [x] 更新 CLAUDE.md 实现状态

**验证命令**:
```bash
go build .
go vet ./...
```

---

## Review Gates

- 阶段 1 完成后：确认构建通过，可独立提交
- 阶段 4 完成后：全量构建通过再进入测试
- 阶段 5 完成后：运行 trellis-check 质量检查

---

## 阶段 6: 状态模型重构 v2（存事实，推导状态）

详见 design.md 第 7 节。目标：消除 DB 持久化运行时瞬态导致的 installing 卡死。

### 6.1 数据库 + 模型
- [ ] `internal/database/database.go`：`addColumnIfMissing(db, "servers", "last_error", "TEXT DEFAULT ''")`
- [ ] `internal/models/server.go`：新增 `LastError string json:"last_error,omitempty" db:"last_error"`；更新 `Status` 注释为"派生，非持久"

### 6.2 进程管理（internal/process）
- [ ] `manager.go`：引入 `procHandle{cmd,pid}`；`running map[int64]*procHandle`；新增 `installing map[int64]struct{}`；`NewManager` 增加 `steamcmdPath`；新增 `IsInstalling`、`DeriveStatus(id,lastError)`、`InstallServer(id,out)`；`StartServer`/`StopServer` 改为只持久化 pid/last_error（`setPID`/`setError`/`clearError` 辅助），不写 status
- [ ] `monitor.go`：`monitor` 兼容 cmd==nil（轮询 `isProcessAlive`）；`ReconcileOnStartup` 改为按 PID 接管（存活登记 running+轮询；死亡清零 pid），移除 installing 处理

### 6.3 API（internal/api）
- [ ] `router.go`：`NewManager(db, streams, cfg.LogDir, cfg.SteamCMDPath)`
- [ ] `handlers.go`：`serverColumns` 用 `last_error` 替换 `status`；扫描后经 `r.process.DeriveStatus` 填充 `s.Status`；`InstallServer` 改为经 Manager 编排、去掉 status 写、守卫改为派生状态且允许 error/stopped；`DeleteServer`/`UpdateServer`/`UpdateServerConfig` 守卫改用派生状态

### 6.4 前端（ui/src）
- [ ] `types/server.ts`：`Server` 增加 `last_error?: string`
- [ ] `components/ServerCard.tsx`：`error` 与 `stopped` 共用可操作分支；安装按钮在 `!installed` 且非 running/installing 时显示；`error` 时展示 `last_error`
- [ ] i18n（zh/en/ja）：如需新增错误提示 key 则补齐

### 6.5 验证
- [ ] `go build .`、`go vet ./...` 通过
- [ ] `cd ui && bun run build` 通过，回根 `go build .` 通过
- [ ] 手动/逻辑验证：安装中杀后端→重启→不再卡 installing（派生 stopped，可重装）；安装失败→error 可重试；running 孤儿→重启后仍识别为 running 且可停止

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

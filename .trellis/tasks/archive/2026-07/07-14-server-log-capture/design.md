# Design — 服务器运行日志捕获（修正版）

> 初版设计基于错误诊断（"缺 -log + 平台差异化 tail"）。经负责人确认，正解是**两平台统一直接接管进程 stdout/stderr、废弃 tail**。本文件为最终实现设计。

## 总体思路

保持 `out = io.MultiWriter(capture, broadcaster)` 这一"落盘 + SSE"汇聚点不变，把服务器进程自身的 stdout/stderr 直接接管进 `out`：

```go
cmd := exec.Command(exe, args...)
cmd.Dir = srv.installPath
cmd.SysProcAttr = sysProcAttr()
cmd.Stdout = out
cmd.Stderr = out
```

两平台一致，无平台分支，不注入任何额外启动参数。

## 改动点

1. `internal/process/manager.go` `StartServer`：
   - 设 `cmd.Stdout = out; cmd.Stderr = out`（接管进程输出）。
   - 删除 tail 相关 wiring（`stopTail`/`tailDone`/`go tailFile(...)`）。
   - `monitor` 调用改为 `go m.monitor(serverID, handle, capture)`（复用已创建的 handle）。
2. `internal/process/monitor.go`：
   - `monitor` 签名去掉 `stopTail chan struct{}, tailDone <-chan struct{}`，删除停 tailer 逻辑，仅保留 `capture.Close()`。
   - `ReconcileOnStartup` 调用同步改为三参数 `go m.monitor(s.ID, ..., nil)`。
   - 更新注释：adopted 进程（cmd==nil）无 capture（无法附加到已运行进程的 stdout）。
3. `internal/process/platform.go`：删除仅服务于 tail 的 `gameLogPath`。
4. 删除 `internal/process/tail.go`（`tailFile`/`waitOrStop`/`tailPollInterval` 全部随之移除，删除前已确认无其它引用）。

## 关键正确性论证

- **写-关闭竞态**：`cmd.Stdout = out` 时 `os/exec` 以 goroutine 把子进程输出拷贝到 `out`，`cmd.Wait()` 会等这些拷贝 goroutine 结束才返回；`monitor` 在 `Wait()` 之后才 `capture.Close()`，所有写都在 Close 前完成，无竞态（R5）。
- **Stdout/Stderr 同写一处**：`Stdout == Stderr`（同一 interface 值）时 `os/exec` 复用单管道，写入串行、行不交错（R6）。
- **不重复、不回放**：不再 tail 文件；stdout 只含本次进程输出（R3/R6）。
- **adopted 进程**：重启管理器后被 PID 收养的进程无法接管其 stdout（进程已在运行），故无 live 日志——与改造前一致，属既有限制，非回归。

## 风险与后续

- Windows 上 `PalServer.exe` 若把真实服务器作为**独立新控制台**子进程拉起，则接管启动器 stdout 可能仍拿不到孙进程输出。负责人确认当前环境有可捕获的输出，故先按直接接管实现；若实测 Windows 仍为空，后续可改为直接运行 `Pal\Binaries\Win64\PalServer-Win64-Shipping-Cmd.exe`（控制台版）以保证 stdout 可捕获（记入 spec 作为 follow-up）。

## 兼容性 / 回滚

- API、SSE、前端、日志路径、`ReadLogs`/`Subscribe` 全不变，向后兼容。
- 回滚：`git revert` 本次改动即可恢复 tail 方案，无数据迁移。

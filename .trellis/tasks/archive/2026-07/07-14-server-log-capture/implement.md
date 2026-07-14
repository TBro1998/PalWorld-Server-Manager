# Implement — 服务器运行日志捕获（修正版）

## 执行清单（已完成）

- [x] S1 `manager.go` `StartServer`：设 `cmd.Stdout = out; cmd.Stderr = out`，删除 tail wiring，`monitor` 改三参调用并复用 `handle`，更新注释。
- [x] S2 `monitor.go`：`monitor` 去掉 `stopTail/tailDone` 参数与停 tailer 逻辑，仅 `capture.Close()`；`ReconcileOnStartup` 调用同步；更新注释。
- [x] S3 `platform.go`：删除 `gameLogPath`。
- [x] S4 删除 `internal/process/tail.go`（确认无其它引用后）。

## 验证（AC5，已通过）

```bash
go build ./...
go vet ./...
CGO_ENABLED=0 GOOS=linux  go build -o /dev/null .   # linux OK
              GOOS=windows go build -o /tmp/psm.exe . # windows OK
```

全绿。

## 人工验证（AC1/AC2/AC3/AC4，需真实 Palworld 服）

- Windows：启动服务器 → `GET /api/servers/:id/logs?kind=server` 非空、`/logs/stream` 有实时行；停止后 pid 归零、可再次启动拿到日志。
- Docker/Linux：容器内启动服务器 → 同两接口非空且实时；无重复行。
- 若 Windows 实测 stdout 仍为空 → 触发 design.md「风险与后续」中的 `-Cmd.exe` 直启方案。

## 回滚点

- `git revert` 本次提交恢复 tail 方案，无数据迁移。

# 服务器运行日志捕获

## Goal

服务器（Palworld 专用服）启动后，管理器必须"接管"其运行时输出，使得该输出既能落盘（滚动日志文件）又能通过 SSE 实时推送给前端。当前实现启动后抓不到任何日志数据，需修复，Windows 原生与 Docker/Linux 两种运行环境都必须能拿到日志。

## Background / 现状

现有链路（`internal/process/manager.go` `StartServer`）：

1. 启动进程时 **丢弃** 进程自身 stdout/stderr（`cmd.Stdout`/`Stderr` 保持 nil）。
2. 改为 tail 游戏日志文件 `<install>/Pal/Saved/Logs/Pal.log`，把行写入 `out = MultiWriter(capture, broadcaster)`。
3. `capture` 落盘到 `<logDir>/server_<id>/server/current.log`；`broadcaster` 推 SSE。API 侧 `ReadLogs`（读文件）+ `Subscribe`（订阅 SSE）消费。

抓不到数据的根因（经与项目负责人确认，修正为）：

- **未接管进程输出本身**：服务器进程**本来就有日志输出**（Windows/Linux 皆然），问题不在于缺少 `-log`。当前实现却把进程自身的 stdout/stderr 丢弃，改绕道 tail 文件，导致抓不到——这才是根因。
- **tail 文件方案不可靠**：完全依赖 tail `Pal.log`，实测在两平台上都拿不到数据。正解是让管理器**直接接管进程的 stdout/stderr**（用户所说"接管服务器输出"），两平台一致。

> 修正说明：初版 PRD 曾把根因归为"缺 `-log` 参数 + 平台差异化（Windows 只能 tail）"。负责人指出 Windows 服务器有输出、只是没被接管，且不是 `-log` 问题。故方案改为**两平台统一直接接管 stdout/stderr、废弃 tail**。

## Requirements

- R1 服务器启动后，其运行时日志必须进入现有 `out` 管道（同时落盘 + SSE），无需改动 API/前端消费方。
- R2 **两平台统一**直接接管子进程 stdout/stderr（`cmd.Stdout`/`Stderr` → `out`），废弃 tail `Pal.log` 方案。
- R3 不新增/不注入额外启动参数（`-log` 等）：服务器本身已产出日志，不改动 `LaunchArgs`/持久化配置。
- R4 保持业务代码平台无关；不新增无谓的平台分支。
- R5 不破坏既有生命周期：monitor 退出时关闭 capture、清理 pid，无 goroutine 泄漏、无写-关闭竞态（依赖 `cmd.Wait` 等待 os/exec 输出拷贝 goroutine 结束后再 Close capture）。
- R6 不重复行、不回放上一轮日志（stdout 天然只含本次进程输出）。

## Non-Goals

- 不改造 SSE/前端消费链路（`broadcast.go`/`stream.go`/API handler/前端）。
- 不新增运行模式配置项；不引入 RCON。
- 不做日志内容解析/结构化（仅原样捕获行）。
- 不改动 SteamCMD 安装日志（KindSteamCMD）路径。

## Acceptance Criteria

- [x] AC1 Linux（Docker）下启动服务器后，`GET /api/servers/:id/logs?kind=server` 能返回非空日志，`/logs/stream` 有实时行推送。（真机验证通过）
- [x] AC2 Windows 原生下启动服务器后，同样两个接口能拿到服务器进程 stdout/stderr 的日志数据。（真机验证通过）
- [x] AC3 不再 tail 任何日志文件，不产生重复行。
- [x] AC4 停止/重启服务器后：capture 已关闭、pid 归零，无 goroutine 泄漏；再次启动能继续拿到日志。
- [x] AC5 `go build ./...`、`go vet ./...` 通过；`CGO_ENABLED=0 GOOS=linux go build` 交叉编译通过；`GOOS=windows go build` 通过。

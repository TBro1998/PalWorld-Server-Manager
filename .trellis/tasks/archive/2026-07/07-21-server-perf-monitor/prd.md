# 服务器性能监控 (CPU/内存)

## Goal

为管理器补齐 README 计划中的「实时监控：服务器 CPU、内存使用情况」。在线玩家数、FPS、
帧时间、运行时长已由游戏 REST `/metrics` 实现（见 OverviewSection 的 tiles），本任务**不重复**，
只新增两类基于操作系统的资源指标：

1. **每个游戏服进程**的 CPU / 内存占用（按 `process.Manager` 记录的 PID 聚合其整棵进程树）。
2. **宿主机整体**的 CPU / 内存（及数据盘磁盘）使用率，填充现有 `/api/system/stats` 空实现。

数据下发采用**轮询接口**（与现有 `/metrics`、`/players` 一致的 React Query 定时拉取），不引入 SSE。

## Requirements

### 采集层
- R1 使用 `github.com/shirou/gopsutil/v4`（纯 Go、无 CGO，符合单二进制约束）采集指标，版本锁定 `v4.26.6`。
- R2 **每服务器进程指标**：以 `Manager` 记录的 PID 为根，递归聚合子进程（Windows 上 `PalServer.exe`
  是启动器，真正吃资源的是子进程 `PalServer-Win64-Shipping-Cmd.exe`；不聚合会严重偏低）。
  返回：cpuPercent（占单核百分比，多核可 >100）、memoryRSS 字节、进程树进程数、numCPU。
- R3 **宿主机指标**：整体 CPU 使用率（0-100，跨核归一）、内存 used/total/usedPercent、
  数据目录所在盘的磁盘 used/total/usedPercent。
- R4 CPU 百分比按「两次采样间的差值」计算：采集器缓存上次采样的 CPU 时间与时间戳，
  每次请求返回自上次调用以来的增量百分比。首次调用无基线时返回 0（下次轮询自动校正），不阻塞请求。
- R5 采集失败必须优雅降级：宿主机某项失败不影响其他项；进程不存在/已退出返回 running=false 而非 500。

### API 层
- R6 `GET /api/system/stats`（已存在路由，当前 501 stub）返回宿主机指标（R3）。鉴权沿用 protected 组。
- R7 `GET /api/servers/:id/stats` 新增，返回该服务器进程树指标（R2）。未运行时返回结构化
  `{ running: false, reason: "not_running" }` + 200，与 REST status 的降级风格一致；服务器不存在返回 404。
- R8 两个接口都补 swagger 注解（`@Tags system` / `servers`），并重新生成 docs。

### 前端层
- R9 类型：`ProcessStats` / `HostStats` 加入 `types/`，`serversApi.stats(id)` 与
  `systemApi.stats()` 加入 `lib/api.ts`。
- R10 Overview 区新增「资源占用」面板：进程 CPU% 与内存（MB/GB 自适应）tile；
  宿主机 CPU / 内存 / 磁盘一行紧凑展示。服务器未运行或无数据时显示占位 `—`，不报错。
- R11 轮询：进程指标仅在 `server.status === 'running'` 时启用，间隔 5s；宿主机指标常驻 5s 轮询。
  离开页面自动停止（React Query 默认行为）。
- R12 i18n：zh / en / ja 三语补齐新增文案键（`overview.resource.*`）。

### 约束
- R13 平台差异必须走 gopsutil 的跨平台 API，**不得**新增 `platform_*.go` 分支或调用 `tasklist/wmic/ps`。
- R14 `CGO_ENABLED=0` 下 Windows 与 Linux 均能编译通过（gopsutil 满足）。

## Acceptance Criteria

- [ ] `go build ./...` 与 `CGO_ENABLED=0 GOOS=linux go build ./...` 均通过。
- [ ] `go vet ./...` 通过；新增采集器有单元测试（至少覆盖：进程树聚合的空树/退出进程降级、CPU 增量首帧为 0）。
- [ ] `GET /api/system/stats` 返回真实宿主机 CPU/内存/磁盘，字段非全零（运行态下）。
- [ ] `GET /api/servers/:id/stats`：运行中的服务器返回 cpuPercent（有负载时 >0）且 memoryRSS 反映整棵进程树；
      停止的服务器返回 `running:false, reason:"not_running"`；不存在的 id 返回 404。
- [ ] 前端 Overview 展示进程与宿主机资源，5s 自动刷新，未运行时优雅占位。
- [ ] zh/en/ja 三语文案齐全，无缺键告警。
- [ ] `ui` 端 lint / tsc 通过。

## Notes

- 采集器放在新包 `internal/sysstat/`（职责单一：OS 资源采样），API 层调用它。
  进程树聚合复用 `Manager` 已持久化的 PID，不改动 `Manager` 的生命周期逻辑。
- 进程 CPU「占单核百分比」语义随 gopsutil；返回 numCPU 让前端可选择显示归一化值。当前 UI 直接展示原始值 + 说明。
- 磁盘只报数据目录所在盘（`config.Config` 的数据根），避免枚举全部挂载点。

# Design — 服务器性能监控 (CPU/内存)

## 1. 边界与分层

```
gopsutil/v4 (cpu, mem, disk, process)
        │
internal/sysstat/           ← 新包：纯 OS 资源采样，无 gin/gorm 依赖
  ├─ sysstat.go             ← Collector：Host() / Process(pid) + CPU 增量状态
  ├─ tree_windows.go / tree_unix.go  ← 仅当跨平台子进程枚举需要区分时才拆；优先用 gopsutil 统一 API（见 §4）
  └─ sysstat_test.go
        │
internal/api/               ← HTTP 层
  ├─ system_handlers.go     ← GetSystemStats 改为真实实现 (host)
  └─ handlers.go / 新 stats_handlers.go ← GetServerStats (per-process)
        │
ui/                         ← 轮询展示
```

采集器是**有状态单例**（持有上次 CPU 采样基线），由 `api.Router` 持有一个实例，在 `NewRouter` 构造。
不放进 `process.Manager`：Manager 只管生命周期，采样是只读旁路，职责分离。

## 2. 数据契约

### HostStats (`GET /api/system/stats`)
```go
type HostStats struct {
    CPUPercent  float64 `json:"cpuPercent"`  // 0..100 跨核归一
    NumCPU      int     `json:"numCpu"`
    MemUsed     uint64  `json:"memUsed"`     // bytes
    MemTotal    uint64  `json:"memTotal"`
    MemPercent  float64 `json:"memPercent"`
    DiskUsed    uint64  `json:"diskUsed"`
    DiskTotal   uint64  `json:"diskTotal"`
    DiskPercent float64 `json:"diskPercent"`
}
```
每个子项独立采集，失败则该组字段留零值 + 记 warning（R5），整体仍返回 200。

### ProcessStats (`GET /api/servers/:id/stats`)
```go
type ProcessStats struct {
    Running     bool    `json:"running"`
    Reason      string  `json:"reason,omitempty"` // "not_running" 等
    PID         int     `json:"pid,omitempty"`
    CPUPercent  float64 `json:"cpuPercent"`        // 占单核，多核可 >100
    NumCPU      int     `json:"numCpu"`
    MemoryRSS   uint64  `json:"memoryRss"`         // bytes，进程树求和
    ProcessCount int    `json:"processCount"`      // 进程树节点数
}
```
未运行：`{running:false, reason:"not_running", numCpu:N}` + 200。id 不存在：404。

## 3. CPU 增量算法 (R4)

gopsutil 的 `Process.Percent(0)` 依赖 Process 结构体内缓存的上次调用时间；我们每次请求聚合的是
**一整棵树**且每次新建 Process 对象，故自行维护基线：

- Collector 持有 `map[key]cpuSample{ totalCPUSeconds float64; at time.Time }`，key = `"host"` 或 `serverID`。
- 单次采样：累加进程树每个进程的 `Times()` 的 `User+System`（CPU 秒）。
- 增量：`deltaCPU / deltaWall / numCPU * 100`（host 归一到 0-100）；进程指标不除 numCPU（占单核语义）。
- 首帧（无基线或基线过期 >30s）返回 0，写入基线；下次轮询即出真实值。5s 轮询天然提供稳定 delta。
- Host CPU 直接用 `cpu.Percent(0, false)`（gopsutil 内部维护全局基线，进程内跨调用有效），首帧同样可能为 0。

并发：Collector 用 `sync.Mutex` 保护基线 map（多个前端标签页可能并发拉取）。

## 4. 进程树聚合 (R2) — 跨平台

用 gopsutil 统一 API，**不新增 platform_*.go**（R13）：

- `process.NewProcess(int32(pid))` → 根进程。若返回 err（进程已退出）→ 视为 not_running 降级。
- 递归 `proc.Children()` 收集整棵树（Windows 上启动器→Cmd 子进程；Linux 上 sh→binary）。
- 对每个节点取 `MemoryInfo().RSS` 求和、`Times()` 累加。单个子进程取值失败跳过（进程可能刚退出），不整体失败。
- `Children()` 在两平台均由 gopsutil 实现（Windows 走 toolhelp 快照，Linux 走 /proc）。经确认无需 CGO。

> 若实测某平台 `Children()` 行为异常，才退化为 `_windows.go/_unix.go` 对偶；初版不预设拆分。

## 5. 磁盘目标 (R3)

`config.Config` 无独立数据根字段。数据分散在 `DatabasePath` / `LogDir` / `SteamCMDPath`（默认均在 `.` 下）。
初版报**当前工作目录所在盘**：`disk.Usage(".")`（Windows 返回该盘符卷，Linux 返回挂载点）。简单且贴合「程序数据盘」。

## 6. API 接线

- Router 增字段 `sys *sysstat.Collector`，`NewRouter` 中 `sysstat.New()`。
- `GetSystemStats`：调 `r.sys.Host(ctx)` 返回 HostStats。
- 新路由 `servers.GET("/:id/stats", r.GetServerStats)`（在 handlers 分组内，紧邻 logs）。
  - 解析 id → `serverExists` 校验（404）→ 取 PID（`DeriveStatus`/DB pid）→ 未运行返回降级结构 → 运行则 `r.sys.Process(ctx, serverID, pid)`。
- swagger 注解 + `swag init`（复用现有 docs 生成方式，见 internal/api/docs.go 约定）。

## 7. 前端

- 类型：`HostStats` / `ProcessStats` 入 `types/system.ts`（host）与 `types/server.ts`（process），
  或统一放 system.ts。`systemApi.stats()`、`serversApi.stats(id)` 入 api.ts。
- Overview 新增「资源占用」PanelCard：
  - 进程行：CPU%（`cpuPercent.toFixed(0)%`）、内存（自适应 MB/GB）。仅 running 时轮询（`enabled: isRunning`）。
  - 宿主机行：CPU% / 内存 used/total / 磁盘 used/total。常驻 5s 轮询。
  - 复用现有 tile 卡片样式；无数据 `—`。
- i18n：`serverManage.overview.resource.{title,processCpu,processMem,hostCpu,hostMem,hostDisk,cores}` × zh/en/ja。

## 8. 兼容性 / 回滚

- 纯新增：新包 + 一个新路由 + 一个 stub 填充 + 前端只加面板。无 DB 迁移，无既有契约变更。
- 回滚：移除路由注册与前端面板即可；`sysstat` 包与依赖可保留不影响其他功能。
- 依赖体积：gopsutil 增加约几百 KB，可接受（换来跨平台正确性，符合既定决策）。

## 9. 风险

- R-1 Windows `Children()` 首次快照开销：进程树小（2-3 节点），5s 一次可忽略。
- R-2 CPU 首帧为 0：可接受，文档化；轮询第二帧即正确。
- R-3 容器内 `disk.Usage(".")` 反映的是容器可见卷 —— 正是期望（数据卷 `/data`）。

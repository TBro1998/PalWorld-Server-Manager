# Implement — 服务器性能监控 (CPU/内存)

执行顺序：后端采集 → API → 前端 → i18n → 全量验证。每步后跑对应验证。

## Step 1 — 依赖

- [ ] `go get github.com/shirou/gopsutil/v4@v4.26.6`
- [ ] `go mod tidy`
- 验证：`go list -m github.com/shirou/gopsutil/v4` 显示 v4.26.6。

## Step 2 — 采集器包 `internal/sysstat/`

- [ ] `sysstat.go`：
  - `type Collector struct{ mu sync.Mutex; samples map[string]cpuSample }`
  - `func New() *Collector`
  - `func (c *Collector) Host(ctx) (HostStats, error)`：cpu.Percent(0,false) + mem.VirtualMemory + disk.Usage(".")，逐项降级。
  - `func (c *Collector) Process(ctx, key string, pid int) (ProcessStats, error)`：进程树聚合 + CPU 增量。
  - `HostStats` / `ProcessStats` 结构体（见 design §2）。
  - 进程树 walk：`gatherTree(pid)` 递归 `Children()`，累加 RSS 与 CPU 秒，收集节点数。
  - CPU 增量算法（design §3），基线过期阈值 30s，首帧返回 0。
- [ ] `sysstat_test.go`：
  - 空/退出进程 → running 相关降级不 panic（用当前测试进程 os.Getpid() 做正例）。
  - CPU 首帧为 0；第二帧（sleep 后）>=0 且不 panic。
  - Host() 返回 numCpu>0、memTotal>0。
- 验证：`go test ./internal/sysstat/... -v`。

## Step 3 — API 层

- [ ] `internal/api/router.go`：Router 加 `sys *sysstat.Collector`；`NewRouter` 初始化；`import` sysstat。
- [ ] `internal/api/system_handlers.go`：`GetSystemStats` 改真实实现，调 `r.sys.Host`。补/更新 swagger 注解。
- [ ] 新增 `internal/api/stats_handlers.go`（或加到 handlers.go）：`GetServerStats`：
  - parse id → `r.serverExists(c,id)`（404）
  - 取 pid + running：`loadServerPathState` 拿 last_error → `DeriveStatus`；running 时从 DB 读 pid
    （`Manager` 无导出的 pid getter → 直接查 `models.Server.PID`，或加一个 `Manager.PID(id)` getter）。
    **决策**：加 `Manager.PID(serverID) int` 只读 getter（查 running map 的 handle.pid，回退 DB），最贴合来源。
  - 未运行 → `{running:false, reason:"not_running", numCpu}` 200；运行 → `r.sys.Process(ctx, strconv(id), pid)`。
  - swagger 注解 `@Tags servers` `@Router /servers/{id}/stats [get]`。
- [ ] `internal/api/router.go` 注册 `servers.GET("/:id/stats", r.GetServerStats)`（logs 路由附近）。
- [ ] 重新生成 swagger：按项目现有方式 `swag init`（确认 docs 目录 & 命令，见 07-19-swagger-openapi 归档）。
- 验证：`go build ./...`、`go vet ./...`。手测两个接口（运行/停止/不存在三态）。

## Step 4 — Manager PID getter

- [ ] `internal/process/manager.go`：新增 `func (m *Manager) PID(serverID int64) int`：
  锁下查 `m.running[serverID]`，有则 handle.pid；否则查 DB `models.Server.PID`。只读，不改状态。
- 验证：并入 Step 3 build。

## Step 5 — 前端类型与 API

- [ ] `ui/src/types/system.ts`：加 `HostStats`；`ui/src/types/server.ts`：加 `ProcessStats`（或都放 system.ts）。
- [ ] `ui/src/lib/api.ts`：`systemApi.stats = () => apiClient.get<HostStats>('/api/system/stats')`；
  `serversApi.stats = (id) => apiClient.get<ProcessStats>('/api/servers/${id}/stats')`。

## Step 6 — 前端 Overview 面板

- [ ] `OverviewSection.tsx`：新增「资源占用」PanelCard。
  - `useQuery(['server-stats', id], serversApi.stats, { enabled: isRunning, refetchInterval: 5000 })`
    （isRunning 从已有 `['server', id]` 查询派生，勿新增轮询源）。
  - `useQuery(['host-stats'], systemApi.stats, { refetchInterval: 5000, refetchOnWindowFocus:false })`。
  - 展示：进程 CPU%/内存 tile + 宿主机 CPU/内存/磁盘紧凑行。无数据 `—`。
  - 内存格式化 helper：bytes → MB/GB 自适应（新增 `formatBytes`）。

## Step 7 — i18n

- [ ] `messages/{zh,en,ja}.json` 的 `serverManage.overview` 下加 `resource.*` 键（title/processCpu/processMem/hostCpu/hostMem/hostDisk/cores）。
  注意 zh.json 现有内容为 UTF-8（编辑器显示乱码是终端编码问题，用 JSON 工具写入确保正确）。

## Step 8 — 全量验证 (Phase 2.2 final)

- [ ] `go build ./...`
- [ ] `CGO_ENABLED=0 GOOS=linux go build ./...`（跨平台编译验证 R14）
- [ ] `go vet ./...`
- [ ] `go test ./...`
- [ ] `cd ui && bun run lint`（或项目实际 lint 命令）+ 类型检查
- [ ] 手测：运行中服务器 CPU>0 且内存反映进程树；停止态降级；不存在 404；宿主机三项非零。
- [ ] 三语无缺键。

## Review Gates

- G1（Step 2 后）：采集器测试全绿、CPU 增量语义正确。
- G2（Step 3-4 后）：Go 全量 build+vet 通过，三态接口手测正确。
- G3（Step 6-7 后）：前端 lint/tsc 通过，UI 展示与占位正确。
- G4（Step 8）：跨平台编译通过，验收标准逐条打钩。

## Rollback Points

- 任一步 build 失败且定位困难 → 回退该步改动（git checkout 对应文件）。
- gopsutil 在目标平台行为异常 → design §4 的 platform 对偶方案，或缩范围到仅 Host（去掉 per-process）。

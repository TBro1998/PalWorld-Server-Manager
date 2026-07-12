# 技术设计 - SteamCMD修复与服务器生命周期管理

## 1. 包结构

```
internal/
├── process/              # 新增：进程管理
│   ├── manager.go       # 进程生命周期（Start/Stop/Restart）
│   ├── monitor.go       # 进程退出监控 + 启动时状态校正
│   └── platform.go      # 平台特定（Windows taskkill / Linux signal）
├── logger/              # 新增：日志管理
│   ├── capture.go       # 日志捕获 + 轮转 + 读取
│   └── stream.go        # SSE 广播管理
└── api/
    ├── router.go        # 更新：注入 Manager/StreamManager
    └── handlers.go      # 更新：实现 5 个 handler
```

## 2. 边界与契约

### process.Manager
```go
type Manager struct {
    db      *sql.DB
    streams *logger.StreamManager
    mu      sync.Mutex
    running map[int64]*exec.Cmd  // serverID -> 运行中的进程
}

func NewManager(db *sql.DB, streams *logger.StreamManager) *Manager
func (m *Manager) StartServer(serverID int64) error
func (m *Manager) StopServer(serverID int64) error
func (m *Manager) RestartServer(serverID int64) error
func (m *Manager) ReconcileOnStartup() error  // 应用启动时校正残留状态
```

### 平台抽象 (platform.go)
```go
// killProcess 优雅关闭：先 term，超时 timeout 后强制 kill
func killProcess(pid int, timeout time.Duration) error
// isProcessAlive 检查进程是否存活
func isProcessAlive(pid int) bool
// serverExecutable 返回平台对应的服务器可执行文件路径
func serverExecutable(installPath string) (string, error)
```

### logger.Capture
```go
type Capture struct {
    serverID int64
    logDir   string
    mu       sync.Mutex
    file     *os.File
    written  int64
}

func NewCapture(serverID int64, logDir string) *Capture
func (c *Capture) Write(p []byte) (int, error)  // 实现 io.Writer
func (c *Capture) Close() error
func ReadLogs(logDir string, serverID int64, lines int) ([]string, error)
```

### logger.StreamManager
```go
type StreamManager struct {
    mu      sync.RWMutex
    clients map[int64]map[string]chan string  // serverID -> clientID -> chan
}

func NewStreamManager() *StreamManager
func (sm *StreamManager) Subscribe(serverID int64) (id string, ch chan string)
func (sm *StreamManager) Unsubscribe(serverID int64, id string)
func (sm *StreamManager) Broadcast(serverID int64, line string)
```

## 3. 数据流

### 启动流程
```
API StartServer
  → Manager.StartServer(id)
    → 查 DB 获取 server（校验 status == stopped）
    → serverExecutable(installPath) 定位可执行文件
    → 创建 Capture（io.Writer，写文件 + Broadcast 到 SSE）
    → exec.Command，cmd.Dir = installPath，Stdout/Stderr = 组合 writer
    → cmd.Start()
    → DB: status=running, pid=cmd.Process.Pid
    → go monitor(id, cmd)  // 等待退出 → DB status=stopped
```

### 日志双写
- `cmd.Stdout/Stderr` 指向一个组合 writer：`io.MultiWriter(capture, broadcastWriter)`
- capture 负责落盘 + 轮转
- broadcastWriter 按行拆分后调用 `streams.Broadcast`

### SSE 流程
```
API StreamLogs
  → 设置 SSE 头
  → streams.Subscribe(serverID) 得到 chan
  → for { select ch → c.SSEvent; ctx.Done → Unsubscribe return }
```

## 4. 关键设计决策

1. **running map 而非仅依赖 PID**：进程句柄保存在内存 map，便于直接 Signal/Kill 和 Wait，避免跨平台 PID 查找的不确定性。DB 中 PID 仅用于启动时校正。

2. **启动时状态校正 (ReconcileOnStartup)**：应用重启后，DB 中可能残留 `running` 状态但进程已不在。启动时遍历这些记录，用 `isProcessAlive(pid)` 校验，不存活则置为 `stopped`。

3. **优雅关闭超时**：默认 10 秒。Windows 用 `taskkill /PID /T`（含子进程），超时 `taskkill /F`；Linux 发 SIGTERM 到进程组，超时 SIGKILL。

4. **日志轮转**：Capture.Write 累计字节数，超过 10MB 时关闭当前文件、按时间戳重命名、开新文件；清理保留最近 10 个。

## 5. 兼容性与回滚

- **兼容性**：新增包不影响现有安装功能；handlers 从 501 改为实际实现，前端调用契约不变。
- **回滚**：阶段 1（SteamCMD 修复）独立可回滚；阶段 2-4 若出问题，handlers 可退回 501 占位。
- **配置**：新增可选 `log_dir`（默认 `./logs`）、`shutdown_timeout`（默认 10s），走现有配置优先级。

## 6. 数据库

现有 servers 表已含 `status`、`pid` 字段，无需迁移。状态取值：`stopped` / `running` / `installing` / `error`。

---

## 7. 状态模型重构 v2：存事实，推导状态（2026-07-12）

### 背景
把运行时瞬态（`running` / `installing`）当作**权威状态**写入 DB，导致后端在安装过程中被关闭后 DB 永久停留 `installing`、前端卡死。`ReconcileOnStartup` 只能对 `running` 用 PID 存活兜底，对 `installing` 无解——因为安装子进程随后端一起死亡，没有可校验的持久进程。根因是把"活状态"固化进了持久层。

### 新模型
- **持久化的只有"事实"**：
  - `pid`：上次由本工具启动的进程号（0 表示无）。
  - `installed`：磁盘事实的缓存（launcher 是否存在）。
  - `last_error`（**新增列**）：最近一次安装/启动失败信息；成功启动/安装/停止即清空。
- **`status` 不再持久化为真值**，由 Manager 按内存态 + 事实**派生**：
  1. 在内存 `installing` 集合 → `installing`
  2. 在内存 `running` 表 → `running`
  3. `last_error` 非空 → `error`
  4. 否则 → `stopped`
- **install 编排移入 Manager**：用内存 `installing` 集合跟踪；后端重启后不存在"安装中"记录，卡死**结构性消失**，无需对 installing 做任何 reconcile。
- **running 按 PID 接管**：启动时对 `pid` 存活的记录登记为 `running` 并起轮询 monitor（非子进程无法 `Wait`，改用 `isProcessAlive` 轮询）；不存活则清零 pid。

### 契约变化
- **DB**：新增列 `last_error TEXT DEFAULT ''`（走 `addColumnIfMissing`，additive）。`status` 列保留但代码不再读写作为真值（旧的 stuck `installing` 行因 pid=0 且 last_error 空，自动派生为 stopped）。
- **models.Server**：新增 `LastError string`；`Status` 字段仍在 JSON 输出中，但由派生填充而非 DB 扫描。
- **process.Manager**：
  - `NewManager(db, streams, logDir, steamcmdPath)`（新增 steamcmdPath）。
  - `running map[int64]*procHandle`（procHandle 含 `cmd *exec.Cmd`（nil 表示接管的孤儿）与 `pid int`）。
  - 新增 `installing map[int64]struct{}`、`IsInstalling(id)`、`DeriveStatus(id, lastError)`、`InstallServer(id, out io.Writer) error`。
  - `StartServer`/`StopServer` 只持久化事实（pid、last_error），不再写 status。
  - `ReconcileOnStartup` 改为**按 PID 接管**（存活登记 running + 轮询 monitor；死亡清零 pid），删除对 installing 的处理。
- **API**：`ListServers`/`GetServer` 读 `last_error` 后派生 status 填入响应；`DeleteServer`/`UpdateServer`/`UpdateServerConfig` 的"必须 stopped"守卫改用派生状态；`InstallServer` 允许在 `stopped` 或 `error`（即非 installing/非 running）下发起，且不再写 status='installing'。
- **前端**：`Server` 增加 `last_error?: string`；`error` 与 `stopped` 一样可操作（显示安装/启动/编辑/配置）；`error` 时在卡片展示 `last_error`。安装按钮在 `!installed` 且非 running/installing 时显示（覆盖 error 重试）。

### 兼容性与回滚
- 迁移 additive，旧库直接可用；旧 stuck 行自动愈合。
- 若回滚，可恢复到写 status 的旧逻辑；`last_error` 列留存无害。

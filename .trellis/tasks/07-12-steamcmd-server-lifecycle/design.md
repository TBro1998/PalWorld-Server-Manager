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

# SteamCMD 修复与服务器生命周期管理实施计划

**创建时间**: 2026-07-12  
**状态**: 规划中  
**优先级**: P0（阻塞性bug + 核心功能）

## 1. 问题分析

### 1.1 即时问题：SteamCMD 命令顺序错误

**位置**: `internal/steamcmd/install.go:28-33`

**当前代码**:
```go
args := []string{
    "+login", "anonymous",
    "+force_install_dir", installPath,
    "+app_update", "2394010", "validate",
    "+quit",
}
```

**错误原因**: SteamCMD 要求 `+force_install_dir` 必须在 `+login` **之前**执行，否则会报错：
```
Please use force_install_dir before logon!
ERROR! Failed to install app '2394010' (Missing configuration)
```

**影响**: 阻塞所有 Palworld 服务器安装功能

**修复**: 交换 `+force_install_dir` 和 `+login` 的顺序

---

### 1.2 核心功能缺失：服务器生命周期管理

**未实现的 API 端点**:
- `POST /api/servers/:serverId/start` - 启动服务器
- `POST /api/servers/:serverId/stop` - 停止服务器
- `POST /api/servers/:serverId/restart` - 重启服务器
- `GET /api/servers/:serverId/logs` - 获取历史日志
- `GET /api/servers/:serverId/logs/stream` - 流式日志（SSE）

**当前状态**: 所有handlers返回 501 Not Implemented

---

## 2. 目标

### 2.1 立即目标
- ✅ 修复 SteamCMD 命令顺序bug，使服务器安装功能正常工作

### 2.2 核心目标
- ✅ 实现服务器进程启动、停止、重启功能
- ✅ 实现进程状态跟踪和PID管理
- ✅ 实现日志捕获和存储
- ✅ 实现日志流式传输（Server-Sent Events）

### 2.3 质量目标
- 跨平台支持（Windows/Linux）
- 优雅关闭（先SIGTERM，超时后SIGKILL）
- 进程状态一致性（数据库与实际进程状态同步）
- 日志轮转和大小限制

---

## 3. 架构设计

### 3.1 新增包结构

```
internal/
├── process/              # 新增：进程管理
│   ├── manager.go       # 进程生命周期管理
│   ├── monitor.go       # 进程状态监控
│   └── platform.go      # 平台特定实现（Windows/Linux）
├── logger/              # 新增：日志管理
│   ├── capture.go       # 日志捕获（stdout/stderr）
│   ├── storage.go       # 日志存储和轮转
│   └── stream.go        # SSE流式传输
└── api/
    └── handlers.go      # 更新：实现服务器管理handlers
```

### 3.2 核心组件设计

#### 3.2.1 进程管理器 (process.Manager)

**职责**:
- 启动 Palworld 服务器进程
- 跟踪进程PID和状态
- 优雅关闭进程（SIGTERM → SIGKILL）
- 检测进程是否存活

**关键方法**:
```go
type Manager struct {
    db *sql.DB
}

func (m *Manager) StartServer(serverID int64) error
func (m *Manager) StopServer(serverID int64, graceful bool) error
func (m *Manager) RestartServer(serverID int64) error
func (m *Manager) IsProcessRunning(pid int) bool
func (m *Manager) GetServerExecutable(installPath string) (string, error)
```

#### 3.2.2 日志捕获器 (logger.Capture)

**职责**:
- 捕获服务器进程的 stdout/stderr
- 写入日志文件（按服务器ID分离）
- 日志轮转（大小限制：10MB/文件）
- 提供日志读取接口

**关键方法**:
```go
type Capture struct {
    serverID int64
    logDir   string
    file     *os.File
}

func (c *Capture) Start(cmd *exec.Cmd) error
func (c *Capture) Stop() error
func (c *Capture) GetLogPath() string
func (c *Capture) ReadLogs(offset, limit int) ([]string, error)
```

#### 3.2.3 SSE 流式传输 (logger.StreamManager)

**职责**:
- 管理多个客户端的日志流连接
- 实时推送新日志行到已连接客户端
- 处理客户端断开和重连

**关键方法**:
```go
type StreamManager struct {
    clients map[int64][]*StreamClient
    mu      sync.RWMutex
}

func (sm *StreamManager) AddClient(serverID int64, client *StreamClient)
func (sm *StreamManager) RemoveClient(serverID int64, clientID string)
func (sm *StreamManager) BroadcastLog(serverID int64, logLine string)
```

---

## 4. 技术方案

### 4.1 服务器可执行文件路径检测

**Windows**:
```
<installPath>/PalServer.exe
```

**Linux**:
```
<installPath>/PalServer.sh
```

**实现**:
```go
func GetServerExecutable(installPath string) (string, error) {
    if runtime.GOOS == "windows" {
        exe := filepath.Join(installPath, "PalServer.exe")
        if _, err := os.Stat(exe); err == nil {
            return exe, nil
        }
    } else {
        sh := filepath.Join(installPath, "PalServer.sh")
        if _, err := os.Stat(sh); err == nil {
            return sh, nil
        }
    }
    return "", errors.New("server executable not found")
}
```

### 4.2 进程启动流程

1. **验证前置条件**:
   - 检查服务器状态（必须是 `stopped`）
   - 检查可执行文件存在性
   - 验证端口未被占用（可选）

2. **启动进程**:
   ```go
   cmd := exec.Command(executable, args...)
   cmd.Dir = installPath  // 设置工作目录
   cmd.SysProcAttr = getProcAttr()  // 平台特定属性
   ```

3. **日志捕获**:
   ```go
   cmd.Stdout = logWriter
   cmd.Stderr = logWriter
   ```

4. **异步启动**:
   ```go
   if err := cmd.Start(); err != nil {
       return err
   }
   ```

5. **更新数据库**:
   - 保存 PID: `cmd.Process.Pid`
   - 更新状态: `running`
   - 更新时间戳

6. **后台监控**:
   - goroutine 等待进程退出
   - 退出时更新数据库状态为 `stopped`

### 4.3 优雅关闭流程

**Windows**:
```go
// Windows 使用 taskkill
exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T").Run()
time.Sleep(5 * time.Second)
// 超时后强制关闭
exec.Command("taskkill", "/F", "/PID", strconv.Itoa(pid), "/T").Run()
```

**Linux**:
```go
// Linux 使用信号
process.Signal(syscall.SIGTERM)
time.Sleep(5 * time.Second)
// 超时后强制关闭
process.Kill()
```

### 4.4 日志存储方案

**日志目录结构**:
```
./logs/
├── server_1/
│   ├── current.log      # 当前日志
│   ├── 2026-07-12_01.log  # 轮转日志
│   └── 2026-07-12_02.log
└── server_2/
    └── current.log
```

**轮转策略**:
- 单文件最大 10MB
- 达到限制时重命名为带时间戳的文件
- 保留最近 10 个日志文件

### 4.5 SSE 实现方案

**Gin SSE 模式**:
```go
func (r *Router) StreamLogs(c *gin.Context) {
    c.Header("Content-Type", "text/event-stream")
    c.Header("Cache-Control", "no-cache")
    c.Header("Connection", "keep-alive")
    
    clientChan := make(chan string, 10)
    // 注册客户端到 StreamManager
    defer streamManager.RemoveClient(serverID, clientID)
    
    for {
        select {
        case msg := <-clientChan:
            c.SSEvent("message", msg)
            c.Writer.Flush()
        case <-c.Request.Context().Done():
            return
        }
    }
}
```

---

## 5. 实施步骤

### 阶段 1: 紧急修复（5分钟）

**目标**: 修复 SteamCMD 命令顺序bug

**文件**: `internal/steamcmd/install.go`

**修改**:
```go
// 修改前
args := []string{
    "+login", "anonymous",
    "+force_install_dir", installPath,
    // ...
}

// 修改后
args := []string{
    "+force_install_dir", installPath,
    "+login", "anonymous",
    // ...
}
```

**验证**: 运行服务器安装测试，确认不再报错

---

### 阶段 2: 进程管理核心（60分钟）

**2.1 创建进程管理包** (`internal/process/manager.go`)
- 实现 `StartServer()` - 启动服务器进程
- 实现 `StopServer()` - 停止服务器进程
- 实现 `RestartServer()` - 重启服务器
- 实现 `IsProcessRunning()` - 检查进程状态
- 实现 `GetServerExecutable()` - 获取可执行文件路径

**2.2 平台特定实现** (`internal/process/platform.go`)
- Windows: 使用 `taskkill` 命令
- Linux: 使用 SIGTERM/SIGKILL 信号
- 实现 `getProcAttr()` - 获取平台特定进程属性

**2.3 进程监控** (`internal/process/monitor.go`)
- 后台 goroutine 监控进程生命周期
- 进程退出时自动更新数据库状态
- 实现 `startMonitoring(serverID, pid)` 方法

---

### 阶段 3: 日志管理（45分钟）

**3.1 日志捕获** (`internal/logger/capture.go`)
- 实现 `Capture` 结构体
- 捕获进程 stdout/stderr
- 写入日志文件（`./logs/server_{id}/current.log`）
- 集成到进程启动流程

**3.2 日志存储和轮转** (`internal/logger/storage.go`)
- 实现日志文件轮转逻辑（10MB限制）
- 实现 `ReadLogs()` - 读取历史日志
- 实现日志文件清理（保留最近10个文件）

**3.3 SSE 流式传输** (`internal/logger/stream.go`)
- 实现 `StreamManager` 结构体
- 管理多客户端连接
- 实现 `BroadcastLog()` - 广播日志到所有客户端
- 实现客户端连接/断开管理

---

### 阶段 4: API 集成（30分钟）

**更新文件**: `internal/api/handlers.go`

**4.1 实现 StartServer handler**
```go
func (r *Router) StartServer(c *gin.Context) {
    serverID := parseServerID(c)
    pm := process.NewManager(r.db)
    lm := logger.NewCapture(serverID, "./logs")
    
    if err := pm.StartServer(serverID, lm); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"message": "Server started"})
}
```

**4.2 实现其他 handlers**
- `StopServer` - 调用 `pm.StopServer()`
- `RestartServer` - 调用 `pm.RestartServer()`
- `GetLogs` - 调用 `lm.ReadLogs()`
- `StreamLogs` - 使用 SSE 流式传输

---

### 阶段 5: 测试验证（30分钟）

**5.1 单元测试**
- `internal/process/manager_test.go`
- `internal/logger/capture_test.go`

**5.2 集成测试**
- 创建测试服务器
- 测试启动/停止/重启流程
- 测试日志捕获
- 测试 SSE 流式传输

**5.3 手动测试清单**
- [ ] SteamCMD 安装服务器成功
- [ ] 启动服务器进程
- [ ] 进程PID正确记录到数据库
- [ ] 日志正确捕获到文件
- [ ] 停止服务器进程
- [ ] 数据库状态正确更新
- [ ] 重启服务器
- [ ] SSE 日志流正常工作
- [ ] 跨平台测试（Windows/Linux）

---

## 6. 风险评估与缓解

### 6.1 进程管理风险

**风险**: 进程僵尸或孤儿进程
- **缓解**: 使用平台特定的进程组管理（Windows: CREATE_NEW_PROCESS_GROUP, Linux: setpgid）
- **缓解**: 应用退出时清理所有已启动进程

**风险**: 优雅关闭超时
- **缓解**: 设置5秒超时，超时后强制 kill
- **缓解**: 提供配置项让用户自定义超时时间

### 6.2 日志管理风险

**风险**: 日志文件无限增长
- **缓解**: 实现日志轮转（10MB限制）
- **缓解**: 自动清理旧日志（保留最近10个）

**风险**: 并发写入冲突
- **缓解**: 使用互斥锁保护文件写入
- **缓解**: 每个服务器独立日志文件

### 6.3 状态一致性风险

**风险**: 数据库状态与实际进程状态不一致
- **缓解**: 应用启动时扫描数据库中标记为 `running` 的服务器，验证进程是否存在
- **缓解**: 进程监控 goroutine 自动更新状态

### 6.4 跨平台兼容性风险

**风险**: Windows/Linux 行为差异
- **缓解**: 抽象平台特定逻辑到 `platform.go`
- **缓解**: 在两个平台上都进行完整测试

---

## 7. 成功标准

### 7.1 功能标准
- ✅ SteamCMD 安装命令成功执行
- ✅ 服务器可以通过 API 启动/停止/重启
- ✅ 进程 PID 正确追踪
- ✅ 服务器日志正确捕获和存储
- ✅ SSE 日志流正常工作

### 7.2 质量标准
- ✅ 所有单元测试通过
- ✅ 集成测试覆盖主要流程
- ✅ 跨平台测试通过（Windows + Linux）
- ✅ 内存泄漏检查通过
- ✅ 并发测试通过（多服务器同时启动）

### 7.3 文档标准
- ✅ API 文档更新
- ✅ 代码注释完整
- ✅ CLAUDE.md 更新实现状态

---

## 8. 时间估算

| 阶段 | 任务 | 预估时间 |
|------|------|----------|
| 1 | SteamCMD 修复 | 5 分钟 |
| 2 | 进程管理核心 | 60 分钟 |
| 3 | 日志管理 | 45 分钟 |
| 4 | API 集成 | 30 分钟 |
| 5 | 测试验证 | 30 分钟 |
| **总计** | | **170 分钟 (~3小时)** |

---

## 9. 下一步行动

### 立即执行
1. ✅ 提交此计划给用户审核
2. ⏳ 获得用户批准后开始实施
3. ⏳ 按阶段顺序执行（阶段1优先，可独立发布）

### 后续优化（P1）
- 添加服务器健康检查（ping/query端口）
- 实现 RCON 命令接口
- 添加性能监控（CPU/内存使用）
- 实现 Mod 安装功能（通过 SteamCMD）

---

## 10. 总结

此实施计划分为5个阶段，从紧急修复到完整功能实现：

**核心价值**:
1. **立即解决阻塞问题**: 阶段1修复 SteamCMD bug，恢复安装功能
2. **完整生命周期管理**: 阶段2-4实现启动、停止、重启、日志功能
3. **生产就绪**: 阶段5确保质量和跨平台兼容性

**架构亮点**:
- 清晰的包职责分离（process/logger/api）
- 跨平台抽象设计
- 实时日志流式传输（SSE）
- 优雅关闭机制

**实施策略**:
- 分阶段交付，每个阶段可独立验证
- 阶段1可立即发布修复
- 阶段2-5可合并为一个完整的功能版本

---

**计划状态**: ✅ 就绪，等待用户审核


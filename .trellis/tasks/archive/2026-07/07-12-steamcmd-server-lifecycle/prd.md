# SteamCMD修复与服务器生命周期管理

## Goal

修复 SteamCMD 安装命令顺序 bug，并实现 Palworld 服务器的完整生命周期管理（启动/停止/重启/日志）。

## Requirements

### R1: SteamCMD 命令顺序修复（P0 阻塞）
- `+force_install_dir` 必须在 `+login` 之前执行
- 修复后服务器安装不再报错 "Please use force_install_dir before logon!"

### R2: 服务器进程管理
- 启动 Palworld 服务器进程（Windows: PalServer.exe / Linux: PalServer.sh）
- 停止服务器进程（优雅关闭：SIGTERM → 超时 → SIGKILL）
- 重启服务器
- PID 追踪并持久化到数据库
- 进程状态与数据库状态一致

### R3: 日志管理
- 捕获服务器进程 stdout/stderr 到日志文件
- 日志按服务器 ID 分离存储
- 日志轮转（单文件 10MB，保留最近 10 个）
- 历史日志读取接口

### R4: 实时日志流（SSE）
- 通过 Server-Sent Events 实时推送日志
- 支持多客户端连接
- 客户端断开自动清理

## Constraints

- 保持单二进制架构（纯 Go，无 CGO）
- 跨平台支持（Windows/Linux）
- 复用现有 exec.Command 模式和配置系统

## Acceptance Criteria

- [ ] SteamCMD 安装命令成功执行，不再报配置错误
- [ ] 服务器可通过 API 启动，PID 正确记录到数据库
- [ ] 服务器可通过 API 停止，状态正确更新为 stopped
- [ ] 服务器可重启
- [ ] 服务器日志正确捕获到 `./logs/server_{id}/current.log`
- [ ] GET /api/servers/:id/logs 返回历史日志
- [ ] SSE 日志流实时推送新日志行
- [ ] 应用启动时校正数据库中残留的 running 状态
- [ ] Windows 与 Linux 均通过测试

## Notes

- 详细技术设计见 design.md，执行步骤见 implement.md
- Palworld 专用服务器 App ID: 2394010

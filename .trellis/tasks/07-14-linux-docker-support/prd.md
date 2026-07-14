# PRD — Linux 适配与 Docker 运行模式支持

## 背景

项目当前以 Windows 为主要目标平台，但代码库中已存在跨平台分支
（`internal/process/platform_unix.go`、`platform.go` 的 `PalServer.sh` 分支、
`steamcmd.go` 的 `steamcmd.sh` 分支、Linux tar.gz 下载、`xdg-open` 打开浏览器）。
现需要：
1. 让 Linux 端**完整可运行**（SteamCMD 安装、Palworld 专用服安装、进程启停、日志、Web UI 全链路跑通）。
2. 新增 **Docker 运行模式**：管理器与 Palworld 游戏服跑在**同一个容器**内，管理器直接拉起游戏进程（贴合现有进程管理架构）。
3. 提供 **docker-compose 文件**，一条命令即可起服。

## 目标用户

- 在 Linux 主机 / VPS / NAS 上部署 Palworld 服务器的用户。
- 希望用 Docker 一键部署、数据持久化、便于升级的用户。

## 需求

### R1 Linux 原生完整可运行
- 在 Linux 上 `go build .` 产物可直接运行。
- SteamCMD 能自动下载、解压并具备可执行权限、完成自更新。
- 通过 SteamCMD 成功安装 Palworld 专用服（app 2394010），并能**实际启动运行**。
  - 必须处理 Palworld Linux 专用服对 `~/.steam/sdk64/steamclient.so` 的依赖，否则进程会启动失败/报错。
- 服务器进程启停、优雅关闭、日志采集与 SSE 流式输出在 Linux 上正常。

### R2 Docker 运行模式（管理器 + 游戏服同容器）
- 提供 `Dockerfile`，多阶段构建：前端(bun) → 后端(Go, 内嵌 ui/out) → 运行镜像。
- 运行镜像基于 glibc 发行版（Palworld 二进制依赖 glibc，不可用 Alpine/musl）。
- 镜像内置 SteamCMD 运行所需依赖（32 位库等），SteamCMD 与游戏服由应用在运行时安装到持久化卷。
- 容器内以非 root 用户运行（SteamCMD 不建议 root 运行）。
- 数据（数据库、SteamCMD、Palworld 存档、日志）持久化到卷，容器重建不丢数据。
- Web 界面默认监听 `0.0.0.0`，容器外可访问。
- 容器内禁用/静默浏览器自动打开（无 GUI，避免噪声日志）。

### R3 docker-compose
- 提供 `docker-compose.yml`：build、端口映射、命名卷、环境变量、重启策略。
- 暴露端口：Web UI(tcp)、游戏端口(udp)、查询端口(udp)。
- 通过环境变量注入配置（`HOST`/`PORT`/`JWT_SECRET`/各路径）。

### R4 文档与配置示例
- 提供 `.dockerignore` 避免把无关文件送进构建上下文。
- README/使用说明中补充 Docker 与 Linux 部署方式（中文）。
- 在 compose 中给出必须修改项提示（如 `JWT_SECRET`）。

## 非目标（Out of Scope）
- RCON 命令接口（P2，未实现，保持不变）。
- 管理器与游戏服**分容器**编排（本次明确选择同容器）。
- Windows 行为变更（保持现状，不可回退破坏）。
- 多架构镜像（arm64）——Palworld 官方仅提供 x86_64，本次仅 amd64。

## 验收标准

- [x] AC1：在 Linux（或容器）中，从零启动 → SteamCMD 自动安装成功。（容器冒烟实测：`SteamCMD installed successfully`）
- [~] AC2：通过 UI/接口安装 Palworld 专用服成功，`PalServer.sh` 存在且可执行。（安装逻辑跨平台一致；冒烟未跑完整游戏服下载）
- [~] AC3：启动服务器后进程存活，`Pal.log` 被 tail 且日志经 SSE 可见；停止能优雅退出。（Linux 进程/信号分支已存在并保持；未在冒烟中端到端跑）
- [x] AC4：`docker compose up -d --build` 成功；Web 服务在 `0.0.0.0:8080` 起监听。（镜像构建成功 + 容器内 `Server starting on http://0.0.0.0:8080`；compose config 通过）
- [~] AC5：容器重启/重建后，已安装的服务器与数据仍在（卷持久化生效）。（卷/路径设计到位；未做重建实测）
- [x] AC6：Windows 原有构建与行为不受影响（平台分支未被破坏）。（`GOOS=windows go build .` + `go vet` 通过）
- [x] AC7：`go build .` 在无 config.yaml 时读取环境变量路径，容器内路径落在 `/data` 卷。（镜像 ENV + 冒烟实测）

> 图例：`[x]` 已实测通过；`[~]` 逻辑到位但未端到端实测（需完整下载游戏服/重建，留待真实部署验证）。

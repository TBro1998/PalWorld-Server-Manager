# 幻兽帕鲁服务器开服工具

> [English](./README.en.md) | 中文 | [日本語](./README.ja.md)

一个功能完善的幻兽帕鲁专用服务器管理工具，支持模组管理、多服务器管理和现代化的 Web 界面。

> ⚠️ **项目状态：开发阶段**
>
> 本项目目前尚未正式上线，仍处于开发阶段。功能可能不稳定、存在变动或缺失，暂不建议在生产环境中使用。欢迎试用与反馈，敬请留意后续正式发布。

## 功能特性

### 已实现

- 🚀 **一键服务器管理** - 轻松启动、停止、重启服务器
- 📥 **一键安装服务端** - 通过 SteamCMD 下载安装 Palworld 专用服务器
- 🎮 **多服务器支持** - 管理多个独立配置、独立存档、独立端口的服务器
- ⚙️ **可视化配置编辑** - 图形化编辑启动参数与 `PalWorldSettings.ini`
- 📝 **实时日志** - 通过 SSE 实时查看服务器日志（含历史日志）
- 🎛️ **REST API 命令** - 替代 RCON 的 broadcast / save / shutdown / kick / ban
- 🌍 **多语言支持** - 支持中文、英文、日文
- 📦 **单一可执行文件** - 前端嵌入 Go 二进制文件（约 15-25MB）
- 🖥️ **跨平台架构** - 代码层跨平台，由于 Mod 原因当前仅正式支持 Windows

- 🔧 **模组管理** - 输入创意工坊 ID，通过 SteamCMD 自动下载、安装与启停模组
- ⬆️ **自动更新** - 基于 GitHub Releases 的更新检测与一键更新

### 计划开发中

- 🔒 **登录鉴权** - JWT 认证与用户管理，保护远程访问场景
- 📊 **实时监控** - CPU、内存和在线玩家统计

## 安装使用

### 下载安装

1. 前往 [Releases](https://github.com/TBro1998/PalWorld-Server-Manager/releases) 页面下载最新版本
2. 解压下载的压缩包到任意目录
3. 双击运行 `palworld-server-manager.exe`（Windows）

### 首次使用

1. 程序启动后，在浏览器访问 `http://127.0.0.1:8080`（Docker/远程部署请访问 `http://<主机IP>:8080`）
2. 首次访问需要创建管理员账号
3. 登录后即可开始管理您的 Palworld 服务器

### Docker 部署（Linux 推荐）

管理器与 Palworld 游戏服运行在**同一个容器**内，数据持久化到卷，重建容器不丢档。

镜像已发布到 Docker Hub：[`tbro98/palsm`](https://hub.docker.com/r/tbro98/palsm)

```bash
# 1. 下载 docker-compose.yml
curl -O https://raw.githubusercontent.com/TBro1998/PalWorld-Server-Manager/main/docker-compose.yml

# 2. 修改 JWT_SECRET（必须）
#    编辑 docker-compose.yml，将 JWT_SECRET 改为随机强密码

# 3. 拉取镜像并启动（自动从 Docker Hub 拉取，无需本地构建）
docker compose up -d

# 4. 浏览器访问 http://<主机IP>:8080，创建管理员账号后即可安装/管理服务器
```

要点：

- **务必修改 `docker-compose.yml` 中的 `JWT_SECRET`** 再用于生产。
- SteamCMD 与 Palworld 服务端由程序在容器内**首次运行时自动下载**到 `/data` 卷；无需手动预装。
- 端口映射默认：`8080/tcp`（管理界面）、`8211/udp`（游戏）、`27015/udp`（查询）。
  若在界面里修改了服务器的 `-port` / `-QueryPort`，需同步调整 compose 的端口映射。
- 数据目录 `./psm-data` 挂载到容器 `/data`，包含数据库、SteamCMD、存档与日志。备份该目录即可备份全部数据。
- 镜像基于 Debian（glibc），已内置 SteamCMD 与 Palworld Linux 服务端所需运行库；容器以非 root 用户 `steam` 运行。
- 更新到最新版本：`docker compose pull && docker compose up -d`

### Linux 原生部署

无需 Docker 时，也可直接在 Linux 主机运行（需 x86_64、glibc）：

```bash
# 依赖（Debian/Ubuntu 示例）：SteamCMD 为 32 位程序
sudo dpkg --add-architecture i386 && sudo apt-get update
sudo apt-get install -y ca-certificates lib32gcc-s1 libstdc++6 libstdc++6:i386

# 运行（对外访问设置 HOST=0.0.0.0）
HOST=0.0.0.0 PORT=8080 JWT_SECRET=your-secret ./palworld-server-manager
```

程序会在 `~/.steam/sdk64` 自动建立 Palworld 所需的 `steamclient.so` 软链接，安装完成后即可启动服务器。

## 主要功能

### 服务器管理（已实现）
- 一键安装 Palworld 专用服务器
- 启动、停止、重启服务器
- 可视化编辑服务器参数、启动参数和端口
- 查看服务器运行状态
- 多服务器独立管理

### 日志（已实现）
- 实时查看服务器日志（SSE 推送）
- 查看历史日志

### REST API 命令（已实现）
- broadcast / save / shutdown / kick / ban（替代 RCON）

### 计划开发中
- **模组管理**：输入创意工坊 ID 自动下载安装模组、一键启用/禁用、管理已安装列表
- **登录鉴权**：用户名密码登录与 JWT 会话保护
- **系统监控**：服务器 CPU、内存使用情况与在线玩家数量
- **自动更新**：基于 GitHub Releases 的更新检测与一键更新

## 配置说明

程序支持两种配置方式，优先级为：**配置文件 > 环境变量 > 默认值**

### 方式一：配置文件（推荐）

在程序目录下创建 `config.yaml` 文件：

```yaml
# Web 界面设置
host: "127.0.0.1"  # 监听地址
port: 8080          # 端口

# 路径设置
steamcmd_path: "./steamcmd"        # SteamCMD 安装路径
palworld_base_path: "./palworld"   # Palworld 服务器目录

# 数据库
database_path: "./palworld.db"

# JWT 密钥（生产环境请修改）
jwt_secret: "your-secure-secret-key"
```

参考 `config.example.yaml` 文件获取完整配置示例。

### 方式二：环境变量

如果没有 `config.yaml` 文件，程序将使用环境变量：

- `HOST` - Web 界面监听地址
- `PORT` - Web 界面端口
- `STEAMCMD_PATH` - SteamCMD 安装路径
- `PALWORLD_BASE_PATH` - Palworld 服务器安装目录

## 开发文档

如果您想参与开发或了解技术细节，请查看：

- [技术方案](./PalWorld_TECHNICAL_PROPOSAL.md) - 详细技术设计
- [后端开发文档](./server/README.md) - Go 后端开发指南
- [前端开发文档](./ui/README.md) - Next.js 前端开发指南

## 许可证

[GNU Affero General Public License v3.0](./LICENSE)

## 贡献

欢迎贡献代码！请随时提交 Pull Request。
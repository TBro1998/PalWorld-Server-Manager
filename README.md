# 幻兽帕鲁服务器开服工具

> [English](./README.en.md) | 中文 | [日本語](./README.ja.md)

一个功能完善的幻兽帕鲁专用服务器管理工具，支持模组管理、多服务器管理和现代化的 Web 界面。

> ⚠️ **项目状态：开发阶段**
>
> 本项目目前尚未正式上线，仍处于开发阶段。功能可能不稳定、存在变动或缺失，暂不建议在生产环境中使用。欢迎试用与反馈，敬请留意后续正式发布。

## 功能特性

### 🚀 服务器管理（已实现）
- 一键安装 Palworld 专用服务器（通过 SteamCMD）
- 启动、停止、重启服务器
- 多服务器支持 - 独立配置、独立存档、独立端口
- 可视化编辑服务器参数、启动参数与端口

### 📝 日志（已实现）
- 实时查看服务器日志（SSE 推送）
- 查看历史日志

### 🎛️ REST API 命令（已实现）
- broadcast / save / shutdown / kick / ban（替代 RCON）

### 🔧 模组管理（已实现）
- 输入创意工坊 ID，通过 SteamCMD 自动下载、安装与启停模组

### 🔒 登录鉴权（已实现）
- 用户名密码登录与 JWT 会话保护，适用于远程访问场景

### ⬆️ 自动更新（已实现）
- 基于 GitHub Releases 的更新检测与一键更新

### 🌍 其他（已实现）
- 多语言支持 - 中文、英文、日文
- 单一可执行文件 - 前端嵌入 Go 二进制（约 15-25MB）
- 跨平台架构 - 代码层跨平台，由于 Mod 原因当前仅正式支持 Windows

### 📋 计划开发中
- 📊 **实时监控**：服务器 CPU、内存使用情况与在线玩家数量
- ⏰ **定时任务**：定时重启、定时存档等计划任务配置
- 💾 **备份管理**：存档自动备份与一键还原
- 🔄 **崩溃自启**：服务器异常退出时自动重启与告警通知
- 🌐 **玩家自助页面**：面向游戏玩家的独立登录门户，可查看服务器状态、提交申请等自助操作
- 🐳 **游戏服务器容器化运行**：将 Palworld 游戏服务端运行在独立 Docker 容器中，实现更好的隔离与资源控制
- 🛡️ **PalDefender 支持**：集成 PalDefender 反作弊系统，提供配置管理与状态监控

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

# 2. 拉取镜像并启动（自动从 Docker Hub 拉取，无需本地构建）
docker compose up -d

# 3. 浏览器访问 http://<主机IP>:8080，创建管理员账号后即可安装/管理服务器
```

要点：

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
HOST=0.0.0.0 PORT=8080 ./palworld-server-manager
```

程序会在 `~/.steam/sdk64` 自动建立 Palworld 所需的 `steamclient.so` 软链接，安装完成后即可启动服务器。

## AI Agent 运维技能（palworld-ops）

本项目提供一个 AI Agent 技能 [`skills/palworld-ops`](./skills/palworld-ops/)，让 Claude Code、Claude Desktop 等 Agent 能够通过 REST API 直接运维你的服务器（健康检查、性能调优、引导建服、玩家管理、故障排查、模组流程、自动化）。

**如何安装**：你不需要手动配置，把面向 Agent 的安装说明文档交给你的 Agent，让它自行完成安装即可。

将下面这句话（或文档链接）发给你的 Agent：

> 请阅读 https://raw.githubusercontent.com/TBro1998/PalWorld-Server-Manager/main/skills/palworld-ops/INSTALL.zh.md 并按其中步骤为我安装 palworld-ops 技能。

若你已克隆本仓库，也可直接让 Agent 阅读本地文件 [`skills/palworld-ops/INSTALL.zh.md`](./skills/palworld-ops/INSTALL.zh.md) 并执行。

安装完成并重启 Agent 会话后，你就可以让 Agent 帮你管理帕鲁服务器了。使用前请确保管理器正在运行，并向 Agent 提供管理员密码。

**如何更新**：工具升级后，接口变化会自动生效（Agent 运行时读取最新的 API 文档），无需操作；仅当技能内容本身更新时，让 Agent 重新拉取最新技能覆盖旧版本、重启会话即可。详见 [INSTALL.zh.md](./skills/palworld-ops/INSTALL.zh.md) 的“更新已安装的技能”一节。

## 配置说明

程序支持两种配置方式，优先级为：**环境变量 > 配置文件 > 默认值**

> **说明**：环境变量始终优先于 `config.yaml`，适合在 Docker 场景下通过环境变量覆盖配置文件中的特定字段，两种方式可以混用。

### 首次运行流程

无需手动创建任何配置文件。程序启动后，首次访问 Web 界面会引导你设置管理员密码；设置完成后，程序会**自动生成 JWT 密钥**并将所有配置**写入 `config.yaml`**。此后重启均从该文件加载，无需再次设置。

### 配置文件

在程序目录下创建 `config.yaml`（如需在首次运行**前**自定义端口、路径等）：

```yaml
# Web 界面
host: "127.0.0.1"  # 监听地址；对外访问或 Docker 部署改为 0.0.0.0
port: 8080

# 路径
steamcmd_path: "./steamcmd"    # SteamCMD 安装路径
database_path: "./palworld.db" # SQLite 数据库路径
log_dir: "./logs"              # 日志目录

# 自动更新：仅需 fork 本项目时修改
github_repo: "TBro1998/PalWorld-Server-Manager"
```

> `jwt_secret` 与 `password_hash` 由程序在首次 Web 设置时自动写入，**无需手动填写**。

### 环境变量（适用于 Docker / 覆盖配置文件）

环境变量始终生效，优先级高于 `config.yaml`，可在不修改配置文件的情况下覆盖任意字段：

| 环境变量 | 说明 | 默认值 |
|---|---|---|
| `HOST` | Web 界面监听地址 | `127.0.0.1` |
| `PORT` | Web 界面端口 | `8080` |
| `DATABASE_PATH` | SQLite 数据库文件路径 | `./palworld.db` |
| `STEAMCMD_PATH` | SteamCMD 安装路径 | `./steamcmd` |
| `LOG_DIR` | 日志存放目录 | `./logs` |
| `GITHUB_REPO` | 自动更新源仓库 | `TBro1998/PalWorld-Server-Manager` |

`jwt_secret` 和 `password_hash` 无论哪种方式，均在首次 Web 设置时自动生成并写入 `config.yaml`，不需要通过环境变量提供。

## 许可证

[GNU Affero General Public License v3.0](./LICENSE)

## 贡献

欢迎贡献代码！请随时提交 Pull Request。

## 致谢

本项目的开发离不开以下开源项目的启发与参考，特此致谢：

- [PalworldSaveTools](https://github.com/deafdudecomputers/PalworldSaveTools) - 幻兽帕鲁存档解析与工具库
- [palworld-save-pal](https://github.com/oMaN-Rod/palworld-save-pal) - 幻兽帕鲁存档管理工具
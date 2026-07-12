# 幻兽帕鲁服务器开服工具

> [English](./README.en.md) | 中文 | [日本語](./README.ja.md)

一个功能完善的幻兽帕鲁专用服务器管理工具，支持模组管理、多服务器管理和现代化的 Web 界面。

## 功能特性

- 🚀 **一键服务器管理** - 轻松启动、停止、重启服务器
- 🎮 **多服务器支持** - 管理多个独立配置的服务器
- 🔧 **模组管理** - 通过 SteamCMD 安装和管理创意工坊模组
- 📊 **实时监控** - CPU、内存和玩家统计
- 📝 **实时日志** - 通过 SSE 实时查看服务器日志
- 🌍 **多语言支持** - 支持中文、英文、日文
- 🔒 **安全可靠** - JWT 认证和用户管理
- 📦 **单一可执行文件** - 前端嵌入 Go 二进制文件（约 15-25MB）
- 🖥️ **跨平台** - 由于Mod原因，当前仅支持Windows

## 安装使用

### 下载安装

1. 前往 [Releases](https://github.com/zhuzhenghan/palworld-server-manager/releases) 页面下载最新版本
2. 解压下载的压缩包到任意目录
3. 双击运行 `palworld-server-manager.exe`（Windows）

### 首次使用

1. 程序启动后会自动打开浏览器访问 `http://127.0.0.1:8080`
2. 首次访问需要创建管理员账号
3. 登录后即可开始管理您的 Palworld 服务器

## 主要功能

### 服务器管理
- 一键安装 Palworld 专用服务器
- 启动、停止、重启服务器
- 配置服务器参数和端口
- 查看服务器运行状态

### 模组管理
- 输入创意工坊 ID 自动下载安装模组
- 一键启用/禁用模组
- 管理已安装的模组列表

### 监控与日志
- 实时查看服务器日志
- 监控服务器 CPU 和内存使用情况
- 查看在线玩家数量

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
database_path: "./data/palworld.db"

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

MIT

## 贡献

欢迎贡献代码！请随时提交 Pull Request。
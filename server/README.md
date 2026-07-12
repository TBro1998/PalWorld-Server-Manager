# Palworld 服务器管理器 - 后端

> [English](./README.en.md) | 中文 | [日本語](./README.ja.md)

用于管理 Palworld 专用服务器的 Go 后端服务。

## 技术栈

- **框架**: Gin
- **数据库**: SQLite (modernc.org/sqlite - 纯 Go，无 CGO)
- **认证**: JWT
- **实时日志**: Server-Sent Events (SSE)

## 目录结构

```
server/
├── main.go         # 应用入口
├── internal/
│   ├── api/           # HTTP 处理器和路由
│   ├── auth/          # 认证逻辑
│   ├── config/        # 配置管理
│   ├── database/      # 数据库初始化和迁移
│   ├── models/        # 数据模型
│   ├── server/        # HTTP 服务器设置
│   ├── steamcmd/      # SteamCMD 集成
│   └── i18n/          # 国际化
└── pkg/
    ├── logger/        # 日志工具
    └── utils/         # 通用工具

## 构建

```bash
go mod download
go build .
```

## 运行

```bash
./bin/palworld-server-manager
```

## 环境变量

- `HOST` - 服务器主机地址（默认：127.0.0.1）
- `PORT` - 服务器端口（默认：8080）
- `DATABASE_PATH` - SQLite 数据库路径（默认：./data/palworld.db）
- `JWT_SECRET` - JWT 签名密钥（生产环境请更改！）
- `STEAMCMD_PATH` - SteamCMD 安装路径
- `PALWORLD_BASE_PATH` - Palworld 服务器基础目录

## 嵌入前端

前端使用 Go 的 `embed` 包嵌入。首先构建 Next.js 前端：

```bash
cd ../ui
npm run build
```

然后构建 Go 二进制文件 - 它将自动包含前端。

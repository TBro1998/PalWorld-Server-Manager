# 幻兽帕鲁服务器开服工具 — 技术方案文档


---

## 一、需求梳理

| 编号 | 需求 | 优先级 | 备注 |
|------|------|--------|------|
| FR-1 | 通过 SteamCMD 下载安装 Palworld 专用服务器 | P0 | AppID: `2394010` |
| FR-2 | 编辑服务器启动参数 & `PalWorldSettings.ini` | P0 | |
| FR-3 | 一键启动 / 停止 / 重启服务器 | P0 | |
| FR-4 | **用户提供 mod ID，工具通过 SteamCMD 自动下载 + 安装** | P0 | 不做搜索，不要 API Key |
| FR-5 | 多服务器管理（独立配置 / 独立存档 / 独立端口） | P0 | |
| FR-6 | 查看服务器日志（历史日志 + 实时日志 SSE 推送） | P1 | 实时日志用 SSE（避免 WebSocket 复杂度） |
| FR-7 | REST API 命令下发（替代 RCON） | P2 | broadcast / save / shutdown / kick / ban |
| FR-8 | 服务器状态监控（CPU / 内存 / 在线人数） | P2 | 轮询即可，秒级刷新可接受 |
| FR-9 | 登录鉴权（用户名密码） | P0 | 保护远程访问场景 |
| FR-10 | 监听地址可配置（仅本地 / 局域网 / 远程） | P0 | 用户自己选 |
| NFR-1 | **前后端单二进制打包** | — | 强约束 |
| NFR-2 | 跨平台：Windows / Linux（macOS 暂不做） | — | 当前版本只跑 Windows，但代码层要跨平台 |
| NFR-3 | Linux 用 Web 界面操作（不要 CLI） | — | NAS / VPS 场景 |
| NFR-4 | **多语言支持（中文/英文/日文）** | — | 使用 next-intl 实现 |
| NFR-5 | **内置自动更新检测 + 一键更新** | — | 基于 GitHub Releases |
| NFR-6 | 不依赖 Steam 客户端（使用 SteamCMD） | — | 强约束 |
| NFR-7 | 不需要 Docker 支持 | — | 当前版本 |
| NFR-8 | mod 列表手动维护：用户输入 Workshop ID，工具负责下载 + 部署 + 启停 | — | 强约束 |

---

## 二、技术选型

### 2.1 后端：Go

| 用途 | 选型 | 备注 |
|------|------|------|
| HTTP 框架 | **Gin** | 生态成熟，标准库兼容，SSE 支持好 |
| 数据库 | SQLite (`modernc.org/sqlite`) | 纯 Go 免 CGO |
| SteamCMD 调用 | `os/exec` + 自封装 | |
| INI 解析 | `go-ini/ini` | |
| 进程监控 | `shirou/gopsutil/v3` | 跨平台 CPU/内存 |
| 文件监听 | `fsnotify/fsnotify` | 监听 mod 文件变化 |
| 日志流 | 自实现 SSE 推送 | 使用 Gin 的 Stream API |
| 嵌入前端 | 标准库 `embed` | |
| WebSocket | **不用** | 改用 SSE + 轮询 |
| 密码哈希 | `golang.org/x/crypto/bcrypt` | 登录鉴权 |
| Session 管理 | JWT (`golang-jwt/jwt/v5`) | 替代 Cookie 更适合前后端分离 |
| 配置管理 | `caarlos0/env` | 环境变量配置 |
| 自动更新 | `minio/selfupdate` | 自更新二进制 |
| 版本比较 | `Masterminds/semver/v3` | 语义化版本解析 |
| 多语言支持 | 内置消息映射 | 根据 Accept-Language 返回对应错误消息 |

### 2.2 前端：Next.js 16

| 用途 | 选型 | 备注 |
|------|------|------|
| 框架 | **Next.js 16 App Router** | `output: 'export'` 静态导出 |
| UI 组件 | **shadcn/ui** | Radix UI + Tailwind，按需引入，体积小 |
| 样式方案 | Tailwind CSS v4 | |
| 多语言 | **next-intl** | 支持 App Router + 静态导出 |
| 状态管理 | Zustand | 轻量 |
| 数据请求 | TanStack Query + axios | |
| 表单 | react-hook-form + zod | |
| 实时日志 | SSE（原生 EventSource API） | 不引入第三方库 |
| INI 编辑 | **自定义可视化表单组件** | 分类折叠面板 + 字段描述 |
| 路由保护 | 客户端路由守卫 + token | |

### 2.3 单二进制打包

**核心思路：** Go 后端用 `embed.FS` 把 Next.js 构建产物打包进二进制，启动时同一进程同时提供：
- 静态文件服务（Next.js 构建产物）
- REST API
- SSE 实时日志流

**实现方式：**
- 使用 Go 标准库 `embed.FS` 嵌入前端静态文件
- Gin 的 `StaticFS` 提供静态文件服务
- 单一可执行文件约 15-25 MB（含前端 + 压缩后）

**跨平台编译：**
- Windows x64
- Linux x64
- Linux ARM64（NAS / 树莓派）

## 三、幻兽帕鲁官方服务器文档
[Palworld Server Guide](https://docs.palworldgame.com/category/settings-and-operations)
# 软件自更新机制

## Goal

让 PalWorld Server Manager 管理工具本身能从 GitHub Release 获取最新版本并自我更新：

- 启动时后台自动检查是否有新版本（非阻塞）。
- Web UI 支持手动「检查更新 / 立即更新」，展示当前版本、可用新版本、release notes 与更新进度。
- 更新对象是**本管理工具自身的二进制**（Windows / Linux），不是 Palworld 服务端（后者已有独立安装/更新功能）。

## Background / Confirmed Facts

- 版本号通过 ldflags 注入 `main` 包级变量：`Version`（默认 `dev`）、`BuildTime`、`GitCommit`（见 [version.go](version.go)）。这些变量在 `main` 包，需通过 `server.New` → `api.NewRouter` 传入 API 层。
- Release workflow（[.github/workflows/release.yml](.github/workflows/release.yml)）：
  - 触发：push `v*` tag。
  - 产物：`psm_windows_amd64.exe`、`psm_linux_amd64`，上传到 GitHub Release，`make_latest: true`。
  - 同时构建并推送 Docker 镜像到 Docker Hub（`palsm:<version>` / `:latest`）。
- config 已有 `GitHubRepo`，默认 `TBro1998/PalWorld-Server-Manager`（[internal/config/config.go:39](internal/config/config.go#L39)）。
- 设置持久化已有 key/value 模式：`models.Setting` 表 + [internal/settings/settings.go](internal/settings/settings.go) 的 `Get/Set`。
- SSE 流式已有基础设施：[internal/logger/stream.go](internal/logger/stream.go) 的 `StreamManager`（按 `serverID + kind` 分组，支持 `Broadcast` 日志与 `BroadcastEvent` 命名事件）。
- API 路由集中在 [internal/api/router.go](internal/api/router.go)；`protected` 组的 JWT 中间件当前在 [router.go:49](internal/api/router.go#L49) 被注释，全部路由实际无鉴权。
- 前端导航为侧栏（[ui/src/components/Sidebar.tsx](ui/src/components/Sidebar.tsx)）：首页 / 服务器 / 模组，底部语言与主题控件；暂无设置/关于页。API client 在 [ui/src/lib/api.ts](ui/src/lib/api.ts)，i18n 走 `useTranslations`（[ui/src/contexts/LanguageContext.tsx](ui/src/contexts/LanguageContext.tsx)），文案在 `ui/messages/{en,zh,ja}.json`。
- 项目当前主要面向 Windows，Linux 代码路径保留；此功能需同时支持 Windows 与 Linux。

## Requirements

- **R1 版本查询**：提供 API 返回当前 `version` / `buildTime` / `gitCommit`。
- **R2 检查更新**：调用 GitHub Release API（`/repos/{repo}/releases/latest`）取最新版本，用 semver 与当前版本比较，返回 `hasUpdate`、`latestVersion`、`releaseNotes`、`assetUrl`（对应本平台）、`isDev`（当前为开发版本时跳过判定）。
- **R3 启动自动检查**：进程启动时在后台 goroutine 执行一次检查（非阻塞、失败不影响启动），结果缓存在内存供 UI 读取。
- **R4 执行更新**：下载对应平台二进制 → 校验 → 用 `minio/selfupdate` 原子替换当前可执行文件 → re-exec 重启进程。下载进度经 SSE 上报。
- **R5 下载镜像设置**：提供 setting `download_mirror`，前端界面可读写；非空时作为下载 URL 前缀，为空则直连 GitHub。检查更新（API 元数据）始终走 GitHub 官方 API，不经镜像。
- **R6 前端交互**：新增设置/关于页（侧栏入口），展示当前版本、检查更新按钮、发现新版本时展示版本号与 notes、「立即更新」按钮、更新进度与重启后自动恢复；提供 `download_mirror` 输入框。开发版本显示「开发版本，跳过更新检查」。
- **R7 跨平台自替换**：Windows 与 Linux 均能替换运行中的二进制并重启（`minio/selfupdate` 处理改名/落盘/回滚，re-exec 处理重启）。

## Acceptance Criteria

- [ ] **AC1 (R1)**：`GET /api/system/version` 返回当前 `version`、`buildTime`、`gitCommit`。
- [ ] **AC2 (R2)**：`GET /api/system/update/check` 在有更高 release 时返回 `hasUpdate=true` + `latestVersion` + `releaseNotes` + 本平台 `assetUrl`；无更高版本时 `hasUpdate=false`。
- [ ] **AC3 (R2/R3)**：当前 `Version=="dev"` 或版本串非法时，检查不报错且 `hasUpdate=false`，响应含 `isDev=true`（或等价标识）；进程启动时后台自动执行一次检查且不阻塞启动、失败仅记日志。
- [ ] **AC4 (R5)**：`GET /api/system/settings` 返回 `download_mirror` 当前值；`PUT`/`POST` 可写入并持久化到 `models.Setting`。设置非空时，执行更新的下载 URL 带该前缀；为空时直连 GitHub asset URL。
- [ ] **AC5 (R4/R7)**：`POST /api/system/update/apply` 在 Windows 与 Linux 下能下载→替换→重启，重启后 `GET /api/system/version` 返回新版本号；下载/替换过程经 SSE（`GET /api/system/update/stream`）上报进度与完成/失败事件。
- [ ] **AC6 (R4)**：更新失败（下载失败、校验失败、替换失败）时回滚到原二进制并保持进程可用，SSE 上报错误事件，进程不崩溃。
- [ ] **AC7 (R6)**：前端设置页展示当前版本；点击「检查更新」触发 R2；有新版本时展示版本与 notes 并可点「立即更新」；更新中显示进度；重启后页面轮询 `version` 恢复到新版本；含可保存的 `download_mirror` 输入；开发版本显示跳过提示。三语言（en/zh/ja）文案齐备。
- [ ] **AC8**：`go build .` 通过；新增后端逻辑（版本比较、asset 选择、镜像前缀拼接）有单元测试且通过。

## Technical Decisions

- **TD1 自替换 + 重启**：用 [`minio/selfupdate`](https://github.com/minio/selfupdate)（原子替换 + 失败回滚，跨 Windows/Linux）替换二进制；替换成功后程序自身 re-exec（`os.StartProcess` 拉起同路径、同参数、同环境的新二进制，随后当前进程退出）完成重启。
- **TD2 版本比较**：用 `golang.org/x/mod/semver`（官方维护，原生支持 `v` 前缀）。`Version=="dev"` 或 `semver.IsValid` 为假时视为开发/未知版本，不判定有更新、不报错。
- **TD3 下载来源与镜像**：直连 GitHub Release asset URL；`download_mirror` 存于 `models.Setting`（复用 [settings.Get/Set](internal/settings/settings.go)，新增 key `download_mirror`），前端读写；非空时作为 asset URL 前缀拼接。检查更新的 API 元数据请求不走镜像。
- **TD4 鉴权**：更新相关端点注册到现有 `protected` 组，跟随全项目现状（当前无鉴权，JWT 启用后自动覆盖）。在触发替换的 handler 上加注释标注安全风险。
- **TD5 容器**：不对容器环境做特殊处理，容器内视同普通 Linux 走 Linux 更新路径。
- **TD6 进度上报**：复用 `StreamManager`，用一个哨兵 `serverID`（如 `0` 或专用常量）+ 新 `kind`（如 `KindUpdate`）承载更新进度事件；下载百分比用 `log` 事件，完成/失败/即将重启用命名事件（`done`/`error`/`restarting`）。
- **TD7 校验**：若 release 附带 checksums 文件则校验 SHA256（best-effort：存在即校验，缺失则跳过并记日志），避免对历史 release 的硬依赖。

## Out of Scope

- Palworld 服务端的安装/更新（已有独立功能）。
- Docker 镜像自身的自更新（由容器编排/手动拉取负责；容器内点更新走 Linux 二进制替换路径，效果不持久，用户自负）。
- 更新端点的独立鉴权强化（跟随全项目 JWT 计划）。
- 跨架构（arm64 等）与 amd64 之外的平台；当前 release 仅产出 amd64。

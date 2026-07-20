# 技术设计：软件自更新机制

## 架构总览

```
main.go (BuildInfo: Version/BuildTime/GitCommit via ldflags)
   │  传入
   ▼
server.New(cfg, db, staticFiles, buildInfo)
   │  传入
   ▼
api.NewRouter(db, cfg, buildInfo, streams...)
   │  使用
   ▼
internal/update/  ← 新建更新领域包（无 gin/gorm 依赖，纯逻辑）
   ├─ github.go    GitHub Release API 客户端：拉 latest release、解析 assets
   ├─ version.go   semver 比较、dev 判定、本平台 asset 选择
   ├─ apply.go     下载 + 校验 + minio/selfupdate 替换 + re-exec 重启
   └─ *_test.go    单元测试（版本比较 / asset 选择 / 镜像前缀拼接）
```

前端：新增 `/settings` 页 + 侧栏入口，复用 `apiClient` 与 `useTranslations`。

## 后端设计

### 1. BuildInfo 传递

`main` 包的 `Version/BuildTime/GitCommit` 不能被其他包直接引用（会造成 import cycle 且 ldflags 只注入 main）。定义一个可传递结构：

- 在 `internal/update`（或新建 `internal/buildinfo`）定义：
  ```go
  type BuildInfo struct {
      Version   string
      BuildTime string
      GitCommit string
  }
  ```
- `main.go` 构造 `update.BuildInfo{Version, BuildTime, GitCommit}` 传入 `server.New`。
- `server.New` 与 `api.NewRouter` 增加 `buildInfo` 参数，`Router` 持有它。

> 决策：放 `internal/update` 内即可，避免多建一个包；`update` 是唯一消费者。

### 2. GitHub Release 客户端（github.go）

- 端点：`GET https://api.github.com/repos/{repo}/releases/latest`，`repo` 来自 `cfg.GitHubRepo`。
- 请求头：`Accept: application/vnd.github+json`，`User-Agent: PalWorld-Server-Manager`（GitHub 要求 UA）。带超时 `http.Client{Timeout: 15s}`。
- 解析关注字段：`tag_name`、`name`、`body`(release notes)、`published_at`、`assets[]`（`name` + `browser_download_url` + `size`）。
- 不做鉴权（公共仓库匿名即可；受 GitHub 匿名限流约束，可接受）。
- 返回结构 `ReleaseInfo{TagName, Body, PublishedAt, Assets []Asset}`。

### 3. 版本比较与 asset 选择（version.go）

- `IsDev(v string) bool`：`v == "dev"` 或 `!semver.IsValid(normalize(v))`。`normalize` 保证有 `v` 前缀（tag 已带，本地 `dev` 直接判 dev）。
- `HasUpdate(current, latest string) bool`：两者都合法且 `semver.Compare(latest, current) > 0`。current 为 dev/非法时返回 false。
- `AssetName(goos, goarch string) string`：映射到 release 资产名：
  - `windows/amd64` → `psm_windows_amd64.exe`
  - `linux/amd64` → `psm_linux_amd64`
  - 其他 → 空（判定为不支持自更新）。
  - 运行时用 `runtime.GOOS` / `runtime.GOARCH`。
- `SelectAsset(release, goos, goarch) (Asset, bool)`：在 `release.Assets` 里按名字匹配。
- `ResolveDownloadURL(rawURL, mirror string) string`：`mirror` 去空白后非空时返回 `strings.TrimRight(mirror,"/") + "/" + rawURL`（前缀拼接约定：镜像服务通常接受 `https://mirror/https://github.com/...`，故直接拼原始完整 URL）。为空返回原 URL。

### 4. 检查更新聚合 + 启动缓存

- `Checker` 持有 `repo`、`buildInfo`、`http.Client`、内存缓存 `atomic.Pointer[CheckResult]`。
- `CheckResult{CurrentVersion, IsDev, HasUpdate, LatestVersion, ReleaseNotes, AssetName, AssetURL, Size, CheckedAt, Err string}`。
- `Check(ctx) (*CheckResult, error)`：拉 release → 组装结果 → 存入缓存 → 返回。
- 启动时：`server` 初始化后 `go checker.Check(ctx)`（非阻塞，失败仅 `log.Printf`）。R3。
- `Cached() *CheckResult`：UI 首次加载读缓存，避免每次打页面都打 GitHub。

### 5. 执行更新（apply.go）

流程（R4/R7）：

1. 读缓存或现查，确认 `HasUpdate`，取本平台 asset。
2. 拼接下载 URL（含 mirror 前缀）。
3. `http.Get` 流式下载到内存/临时文件，边下边算 SHA256，按 `Content-Length` 计算百分比 → `progress func(pct int, msg string)` 回调（handler 里桥接到 SSE）。
4. **校验（TD7）**：若 release 含 checksums 资产（如 `checksums.txt` / `SHA256SUMS`），下载并比对本 asset 的 SHA256；不匹配则中止。缺失则跳过校验并记日志。
5. `selfupdate.Apply(reader, selfupdate.Options{})`：默认替换 `os.Executable()` 指向的文件，内部改名旧文件、落盘新文件、失败 `RollbackError` 回滚。
6. 成功 → 通知 SSE `restarting` 事件 → 触发 re-exec。
7. re-exec：`os.StartProcess(exePath, os.Args, &os.ProcAttr{Files: {stdin,stdout,stderr}, Env: os.Environ()})`，成功后当前进程 `os.Exit(0)`（给一个短延时让 SSE flush）。
   - 注意：re-exec 会中断当前 HTTP 连接，前端据此转入轮询（见前端设计）。

失败处理（AC6）：任一步出错 → `progress`/SSE 发 `error` 事件 → 返回错误，进程继续运行（selfupdate 保证二进制回滚）。

### 6. API 端点（handlers + router）

注册到 `protected` 组（TD4），前缀 `/api/system`：

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/system/version` | R1：返回 buildInfo |
| GET | `/system/update/check` | R2：强制现查（或带 `?cached=1` 读缓存） |
| POST | `/system/update/apply` | R4：触发下载+替换+重启；异步执行，进度走 SSE |
| GET | `/system/update/stream` | SSE：更新进度（TD6） |
| GET | `/system/settings` | R5：返回 `download_mirror` |
| PUT | `/system/settings` | R5：写入 `download_mirror` |

- SSE handler 复用 `steam_handlers.go` 里的模式（`c.Header` 四件套 + `r.streams.Subscribe` + `c.Stream`）。
- `apply` 触发替换的 handler 加注释：**「此端点会下载并替换二进制并重启进程，JWT 启用后必须覆盖」**（TD4 风险标注）。
- `GetSystemStats`（现为 501 stub）不在本任务范围，保持不动。

### 7. settings 扩展

- `internal/settings/settings.go` 增加 `KeyDownloadMirror = "download_mirror"`。
- 复用 `Get/Set`，无需新表。

### 8. SSE kind 扩展

- `internal/logger`（`capture.go` 里定义了 `KindServer`/`KindSteamCMD`）新增 `KindUpdate = "update"`。
- 用哨兵 `serverID`（定义 `const updateStreamID int64 = 0`，与真实 server id 不冲突——真实 id 从 1 起）。

## 前端设计

### 页面 `/settings`

- 新建 `ui/src/app/settings/page.tsx`（`'use client'`）。
- 侧栏 [Sidebar.tsx](ui/src/components/Sidebar.tsx) `links` 增加 `{ href: '/settings', label: t('settings'), icon: Settings }`（`lucide-react` 的 `Settings` 图标）。
- 内容分区：
  1. **关于/版本**：当前 version / buildTime / gitCommit（来自 `GET /system/version`）。
  2. **更新**：
     - 开发版本（`isDev`）→ 显示「开发版本，跳过更新检查」，隐藏检查/更新按钮。
     - 否则显示「检查更新」按钮 → 调 `check`。
     - `hasUpdate` → 展示 `latestVersion` + `releaseNotes`（markdown 可先纯文本/预格式化展示）+「立即更新」按钮。
     - 「立即更新」→ 先 `new EventSource(streamUrl)` 订阅进度，再 `POST apply`；进度条读 `log` 事件百分比；收到 `restarting` → 进入「正在重启，请稍候」并开始轮询 `GET /system/version`，版本变为新版本即提示成功并可刷新。
  3. **下载镜像设置**：输入框 + 保存按钮（`GET/PUT /system/settings`）。
- 更新期间的连接中断属预期：apply 触发 re-exec 后请求会失败，前端不报错，直接转轮询。

### API client（api.ts）

新增 `systemApi`：
```ts
export const systemApi = {
  version: () => apiClient.get('/api/system/version'),
  checkUpdate: () => apiClient.get('/api/system/update/check'),
  applyUpdate: () => apiClient.post('/api/system/update/apply'),
  updateStreamUrl: () => '/api/system/update/stream',
  getSettings: () => apiClient.get('/api/system/settings'),
  setSettings: (data: { download_mirror: string }) => apiClient.put('/api/system/settings', data),
}
```
配套 TS 类型放 `ui/src/types/`。

### i18n

`ui/messages/{en,zh,ja}.json` 增加 `nav.settings` 与 `settings.*`（版本、检查更新、有新版本、立即更新、更新中、重启中、镜像地址、保存、开发版本跳过等）。

## 依赖

- `github.com/minio/selfupdate`（新增）。
- `golang.org/x/mod/semver`（新增；`golang.org/x/mod` 常已间接存在，需确认 go.mod）。
- 前端无新依赖。

## 兼容性 / 回滚

- selfupdate 内建回滚：替换失败自动还原旧二进制。
- 校验 best-effort，不破坏历史 release 更新能力。
- 若目标平台无对应 asset（如未来 arm64），`SelectAsset` 返回 false → API 返回「当前平台不支持自更新」，不崩溃。
- 新端点全部新增，不改动既有 API 行为。

## 安全考量

- 更新端点无独立鉴权（跟随全项目现状），host 默认 `127.0.0.1` 缓解；代码注释标注风险（TD4）。
- 下载仅走 HTTPS（GitHub / 用户配置镜像）；校验 SHA256（存在时）。
- `download_mirror` 为用户自填，属低敏配置，明文存 setting 表可接受。

## 需要 Windows 实测验证的点

- Windows 下 `selfupdate.Apply` 对运行中 `.exe` 的改名替换。
- re-exec 后新进程正常监听端口、旧进程退出、无端口占用残留。

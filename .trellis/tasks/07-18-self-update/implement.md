# 实施计划：软件自更新机制

## 前置：依赖

```bash
go get github.com/minio/selfupdate
go get golang.org/x/mod/semver   # 确认 go.mod 是否已间接引入
go mod tidy
```

## 实施顺序

### 阶段 A — 后端更新领域包（可独立单测）

1. **`internal/update/version.go`**
   - `BuildInfo` 结构体。
   - `normalize`、`IsDev`、`HasUpdate`（用 `golang.org/x/mod/semver`）。
   - `AssetName(goos, goarch)`、`SelectAsset(release, goos, goarch)`。
   - `ResolveDownloadURL(rawURL, mirror)`。
2. **`internal/update/github.go`**
   - `ReleaseInfo` / `Asset` 结构体。
   - `fetchLatestRelease(ctx, client, repo) (*ReleaseInfo, error)`：调 `releases/latest`，带 UA + Accept + 超时。
3. **`internal/update/checker.go`**
   - `Checker` 结构体（repo、buildInfo、client、`atomic.Pointer[CheckResult]`）。
   - `CheckResult` 结构体。
   - `New(repo, buildInfo)`、`Check(ctx)`、`Cached()`。
4. **`internal/update/apply.go`**
   - `Apply(ctx, result, mirror, progress func(pct int, msg string)) error`：下载→(校验)→`selfupdate.Apply`。
   - `Restart() error`：`os.StartProcess` re-exec + 延时 `os.Exit(0)`。
   - checksums best-effort 校验辅助函数。
5. **`internal/update/version_test.go`**
   - 表驱动测试：`IsDev`、`HasUpdate`（含 dev/非法/大小版本）、`AssetName`、`ResolveDownloadURL`（空/带斜杠/不带斜杠 mirror）。

### 阶段 B — settings / logger 扩展

6. **`internal/settings/settings.go`**：加 `KeyDownloadMirror = "download_mirror"`。
7. **`internal/logger`**（`capture.go` 中 Kind 常量处）：加 `KindUpdate = "update"`。

### 阶段 C — 接线 BuildInfo

8. **`main.go`**：构造 `update.BuildInfo{Version, BuildTime, GitCommit}`，传入 `server.New`。
9. **`internal/server/server.go`**：`New` 增参 `buildInfo update.BuildInfo`；存字段；传入 `api.NewRouter`；初始化后 `go checker.Check(...)` 做启动检查（R3）。
10. **`internal/api/router.go`**：`NewRouter` 增参 `buildInfo`；`Router` 持有 `buildInfo` 与 `*update.Checker`；注册新路由。

### 阶段 D — API handlers

11. **`internal/api/system_handlers.go`**（新建，避免污染 handlers.go）：
    - `GetVersion`、`CheckUpdate`（支持 `?cached=1`）、`ApplyUpdate`（异步 + SSE 进度 + 触发 restart）、`UpdateStream`（SSE，复用 steam 模式）、`GetSettings`、`UpdateSettings`。
    - `ApplyUpdate` 顶部注释标注 JWT 风险（TD4）。
12. **`internal/api/router.go`**：在 `protected` 组注册 `/system/*` 路由（见 design 表）。

### 阶段 E — 前端

13. **`ui/src/lib/api.ts`**：加 `systemApi`。
14. **`ui/src/types/`**：加 version/update/settings TS 类型。
15. **`ui/src/app/settings/page.tsx`**：新建设置页（版本 / 更新 / 镜像三区）。
16. **`ui/src/components/Sidebar.tsx`**：`links` 加 `/settings` 项（`Settings` 图标 + `t('settings')`）。
17. **`ui/messages/{en,zh,ja}.json`**：加 `nav.settings` + `settings.*` 文案。

### 阶段 F — 发布流水线校验（TD7，可选增强）

18. **`.github/workflows/release.yml`**：构建后生成 `checksums.txt`（`sha256sum psm_*`）并加入 `files:` 上传。使未来 release 支持校验；缺失时更新逻辑跳过校验，向后兼容。

## 验证计划

### 编译 / 单测
```bash
go build ./...
go test ./internal/update/... ./internal/settings/...
cd ui && bun run build   # 前端静态导出通过（嵌入前提）
```

### 后端联调（不触发真实替换）
- `GET /api/system/version` 返回注入的版本（本地为 dev）。
- `GET /api/system/update/check` 对真实仓库返回 latest；dev 下 `hasUpdate=false`。
- `GET /api/system/settings` / `PUT` 往返 `download_mirror`。

### Windows 实测（source of truth，AC1/AC5）
1. 用旧 tag 构建注入低版本：`go build -ldflags "-X main.Version=v0.0.1 -o psm.exe ."`。
2. 运行 → 设置页显示有新版本 → 点「立即更新」。
3. 观察进度 SSE → 替换 → re-exec → 轮询到新版本号。
4. 确认旧进程退出、端口无残留、新二进制版本正确。
5. 失败注入（断网/坏 mirror）→ 前端显示错误，进程存活，二进制未损坏（回滚验证）。

### Linux 实测（AC5，尽力）
- 同上流程，`psm_linux_amd64`。

## 风险 / 回滚点

- **re-exec 时序**：`os.Exit` 过早会截断 SSE flush；给 ~500ms 延时或在 flush 后退出。
- **Windows 文件占用**：selfupdate 已处理改名替换；若失败依赖其 `RollbackError`。
- **端口重绑定**：新进程启动到旧进程退出之间可能短暂端口占用；re-exec 前应确保旧进程尽快 `os.Exit`（不做 graceful HTTP shutdown 以避免长等待，进程整体退出即释放端口）。
- 回滚：所有改动为新增文件 + 少量接线；如需回退，移除 `/system` 路由注册与 update 包即可，不影响既有功能。

## 验收映射

| AC | 验证方式 |
|---|---|
| AC1 | Windows 实测步骤 1-4 |
| AC2 | `GET /system/version` 手测 + 前端展示 |
| AC3 | dev 构建下检查不报错、UI 显示开发版本 |
| AC4 | 启动日志显示后台检查执行、UI 读缓存 |
| AC5 | Windows + Linux 实测 |
| AC6 | 失败注入实测：进程存活、二进制完好 |
| AC7 | `PUT settings` 设 mirror 后 apply 走镜像 URL（可通过日志/坏 mirror 报错间接验证） |

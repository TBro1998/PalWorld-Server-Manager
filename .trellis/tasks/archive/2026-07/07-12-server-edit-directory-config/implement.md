# 执行计划：服务器目录与配置编辑

> 顺序执行。每个阶段末尾有验证命令与回滚点。后端先行，前端后接，最后构建嵌入。

## 阶段 0：准备
- [ ] 确认 `bun`、`go` 可用；`go build .` 基线通过。

## 阶段 1：后端参数注册表与 INI 读写（`internal/palconfig`）
- [ ] 1.1 新建 `schema.go`：`ParamDef` 类型 + `Params` 全量数组（四大类，含 type/default/category/options）。
- [ ] 1.2 新建 `default_settings.ini` + `embed.go`（`//go:embed default_settings.ini`）作为内置播种模板。
- [ ] 1.3 新建 `ini.go`：`configDir`、`LoadSettings`、`SaveSettings`、`LoadRaw`、`SaveRaw`、`parseOptionSettings`（状态机）、`serializeOptionSettings`。
- [ ] 1.4 新建 `launchargs.go`：`LaunchArgs` 结构、`ParseLaunchArgs(json)`、`ToArgs()`、`Marshal`。
- [ ] **验证**：为 `parseOptionSettings`/`serializeOptionSettings` 写最小 round-trip 单测（含 `DenyTechnologyList`、`ServerName="x,y"` 逗号、`CrossplayPlatforms=(...)`）。`go test ./internal/palconfig/`。
- [ ] **回滚点**：本阶段纯新增包，删除目录即可回滚。

## 阶段 2：DB 与模型
- [ ] 2.1 `database.go`：`migrate()` 增 `launch_args TEXT DEFAULT ''` 与 `installed BOOLEAN DEFAULT 0` 列（幂等 ALTER，查 `PRAGMA table_info`）。
- [ ] 2.2 `models/server.go`：增 `LaunchArgs`、`Installed`（均 db 落库）字段。
- [ ] **验证**：`go build .`；用既有库确认启动不报错、旧数据可读、新列默认值正确。

## 阶段 3：进程管理（探测 + 启动校正 + 启动参数）
- [ ] 3.1 `process`：导出 `IsInstalled(installPath) bool`（复用 `serverExecutable` 内部逻辑）。
- [ ] 3.2 `manager.go`：新增 `ReconcileInstalled()` 遍历 servers 回写 `installed`；`serverRow` 增 `launchArgs` 并在 `loadServer` 查询；`StartServer` 用 `palconfig.ParseLaunchArgs(...).ToArgs()` 追加到 `exec.Command`。
- [ ] 3.3 `server/server.go`：`setupRoutes` 中在 `ReconcileOnStartup()` 后调用 `ReconcileInstalled()`。
- [ ] **验证**：`go build .`；启动后 `installed` 列被正确校正；配置 `-port` 后启动命令行含参数。

## 阶段 4：API 层
- [ ] 4.1 `handlers.go`：`ListServers`/`GetServer` 的 SELECT 增 `launch_args`/`installed` 并直接返回（不 stat）。
- [ ] 4.2 扩展 `UpdateServerRequest` + `UpdateServer`：支持 `installPath`（改目录需 stopped，变化后 `process.IsInstalled` 重算并落库 `installed`）、`launchArgs`（校验解析）。
- [ ] 4.3 `InstallServer` 成功分支追加 `installed = 1`。
- [ ] 4.4 新增 `GetServerConfig`、`UpdateServerConfig`、`GetConfigSchema` 处理器。
- [ ] 4.5 `router.go`：注册 `GET/PUT /servers/:id/config`、`GET /config/schema`。
- [ ] **验证**：`go run .` 后 curl 走通：GET config（首次播种）、PUT settings、PUT raw、改目录到未安装路径（`installed:false`）、改回已安装路径（`installed:true`）。
- [ ] **回滚点**：git 分阶段提交前不 commit；异常可 `git checkout` 相关文件。

## 阶段 5：前端类型与 API
- [ ] 5.1 `types/server.ts`：`Server` 增 `launch_args`/`installed`；新增 config/schema/launchArgs 类型；扩展 `UpdateServerData`。
- [ ] 5.2 `lib/api.ts`：扩展 `update`；新增 `getConfig`/`updateConfig`/`configSchema`。

## 阶段 6：前端组件
- [ ] 6.1 补齐缺失 shadcn 组件（`switch`/`select`/`tabs`/`textarea`，仅缺则补）。
- [ ] 6.2 `EditServerDialog.tsx`：编辑 name/install_path/端口/启动参数；stopped 才能改目录。
- [ ] 6.3 `ServerConfigDialog.tsx`：schema 驱动的分类 Tabs 表单 + 原始文本 Tab + 启动参数区。
- [ ] 6.4 `ServerCard.tsx`：增"编辑""配置"按钮（stopped 可用）；`!installed` 显示"需要安装"提示。
- [ ] 6.5 `servers/page.tsx`：接线 update/config mutation 与对话框开合。
- [ ] **验证**：`cd ui && bun run lint`。

## 阶段 7：i18n（含全量参数说明）
- [ ] 7.1 `messages/{zh,en,ja}.json` 增 `editServer`/`serverConfig` 段（分类标题、按钮、提示、"需要安装"）。
- [ ] 7.2 `serverConfig.params.<Key>` 三语录入：每个参数 `label`+`desc`（en 取官方文档描述，zh/ja 翻译），覆盖全部 OptionSettings 参数与启动参数。
- [ ] **验证**：抽查若干参数在三语下 label/desc 均有值，无缺 Key。

## 阶段 8：构建与联调
- [ ] 8.1 `cd ui && bun run build`（生成 `ui/out/` 供 Go 嵌入）。
- [ ] 8.2 项目根 `go build .`。
- [ ] 8.3 `go run .` 端到端手测：编辑目录（已装/未装两路径）、结构化改配置并核对磁盘 INI、原始文本改配置、启动参数生效。
- [ ] **验证**：对照 prd.md 全部验收项打勾。

## 阶段 9：质量检查与收尾
- [ ] 9.1 触发 `trellis-check`（spec 合规、类型、构建、跨层数据流）。
- [ ] 9.2 修复问题后更新 spec（如有新约定）。
- [ ] 9.3 提交（Phase 3.4）。

## 关键校验命令
```bash
go build .
go test ./internal/palconfig/
cd ui && bun run lint && bun run build
```

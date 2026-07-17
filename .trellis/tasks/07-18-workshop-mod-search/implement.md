# Implement — 创意工坊 mod 搜索器

执行顺序：先后端（含小调研敲定 QueryFiles 参数）→ 打通端点 → 前端 → i18n → 校验。每步可独立验证。

## Step 0 — 调研敲定 Steam 参数（阻塞后端搜索）
- [ ] 用一个真实 Web API Key 对 `IPublishedFileService/QueryFiles/v1?appid=1623730&search_text=...` 试请求，确认：`query_type` 文本搜索取值（候选 `12`）、cursor 分页字段（`cursor=*` / 响应 `next_cursor`）、`publishedfiledetails[]` 里可用字段名（title/preview_url/short_description/tags/subscriptions/time_updated/creator）。
- [ ] `GetDetails/v1` 确认 `publishedfileids[i]` 数组参 + `children[].publishedfileid`。
- 验证：`curl` 或临时 Go 脚本能取回 JSON；把确认结果记入 design.md（更新候选值）。

## Step 1 — settings 加 key（后端）
- [ ] `internal/settings/settings.go` 加 `KeySteamWebAPIKey`；放宽顶部注释口径。
- 验证：`go build ./internal/settings`。

## Step 2 — `internal/steamworkshop` 纯逻辑包
- [ ] `client.go`：`AppID`、`Item`/`SearchResult`/`DetailItem`/`DepItem`、`Search`、`GetDetails`、`ResolveDependencies`（httpClient 注入、visited+maxDepth）。
- [ ] `client_test.go`：用 `httptest.Server` 造 QueryFiles/GetDetails 假响应，测：搜索解析、分页 cursor、依赖递归/去重/防环/深度上限。
- 验证：`go test ./internal/steamworkshop/`。

## Step 3 — API handlers + 路由
- [ ] `internal/api/steam_handlers.go`（或新 `workshop_handlers.go`）：`WorkshopSearch`、`WorkshopDependencies`、`SetWebAPIKey`；扩展 `SteamStatus` 加 `webApiKeyConfigured`。空 key/ Steam 错误规范化返回，key 明文不外泄。
- [ ] `internal/api/router.go` 的 `steam` 组注册 3 个新路由。
- [ ] handler 层测试（可选，参照 `steam_handlers_test.go`）：空 key 分支、status 增补字段。
- 验证：`go build .`；`go test ./internal/api/`。

## Step 4 — 前端 api/types
- [ ] `ui/src/lib/api.ts`：`steamApi` 加 `workshopSearch`/`workshopDependencies`/`setWebApiKey`，`status()` 返回类型加 `webApiKeyConfigured`。
- [ ] `ui/src/types/server.ts`：加 `WorkshopItem`/`WorkshopDep`。

## Step 5 — 前端组件
- [ ] `WorkshopBrowserDialog.tsx`：搜索输入（防抖）、结果列表、外链、分页、"添加"流程（add → 解析依赖 → 缺失前置弹窗 → 一键补全 → invalidate mods）。
- [ ] `MissingDepsDialog`（可内联在同文件）：列出缺失前置 + "一键添加全部"。
- [ ] `SteamAccountSection` 加 Web API Key 输入（`PasswordInput`）+ 保存/清除；用 `webApiKeyConfigured` 显示状态。
- [ ] `ModsSection.tsx`：加"浏览创意工坊"按钮（未配置 key 禁用+提示），打开弹窗，透传 `server`/`mods`。
- 验证：`cd ui && bun run lint`。

## Step 6 — i18n
- [ ] `ui/messages/zh.json`/`en.json`/`ja.json` 补齐 workshop.* / deps.* / errors.* 文案，三语对齐无缺键。

## Step 7 — 构建与端到端校验
- [ ] `cd ui && bun run build`（静态导出，供 Go 内嵌）。
- [ ] `go build .`。
- [ ] （Windows 真机）配 key → 搜索 → 添加 → 依赖弹窗 → 一键补全 → 列表刷新；未配 key 禁用态。

## 验证命令汇总
- 后端：`go build .`、`go test ./internal/steamworkshop/ ./internal/api/ ./internal/settings/`
- 前端：`cd ui && bun run lint && bun run build`

## 风险点 / 回滚
- Step 0 若 QueryFiles 参数与候选不符：只影响 `steamworkshop.Search` 的参数拼装，改动局部。
- 前置检测依赖 Steam 层声明（见 prd Risks）：若真机验证发现 Palworld mod 普遍不声明 Steam 依赖，功能保留但"通常不弹窗"，需在 UI 文案上诚实说明（不夸大）。
- 回滚点：各 Step 独立提交；后端包/端点与前端组件解耦，可分别回退。
- 新增写 `PalModSettings.ini` 的路径本任务**不涉及**（添加只写 DB），无需持 `iniMu`。

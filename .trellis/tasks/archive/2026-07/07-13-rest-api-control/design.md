# Design — 服务器 REST API 控制功能

## Architecture / Boundaries
三层，均沿用现有约定：

```
前端 RestApiSection ──/api/servers/:id/rest/*──▶ Go Router(handlers) ──palapi 客户端──▶ 127.0.0.1:RESTAPIPort (游戏服 REST API, Basic Auth)
```

- **internal/palapi**（新增）：纯 HTTP 客户端，不依赖 gin/gorm。输入 `Client{BaseURL, Password, HTTPClient}`，方法对应 11 个端点。职责单一、可单测（对 httptest.Server）。
- **internal/api**（扩展）：新增 rest 相关 handler；负责“从 DB 取 installPath → palconfig.LoadSettings 读端口/密码/开关 → 构造 palapi.Client → 转发/校验”。复用现有 handler 骨架与错误约定。
- **前端**：serversApi 新增方法 + RestApiSection 重写；复用 shared.tsx / ui 原语。

## Backend Contracts

### palapi 包
```go
package palapi

type Client struct { BaseURL string; Password string; HTTP *http.Client }
func New(port int, password string) *Client // BaseURL = http://127.0.0.1:<port>/v1/api, 默认 5s 超时

// GET
func (c *Client) Info(ctx) (Info, error)
func (c *Client) Metrics(ctx) (Metrics, error)
func (c *Client) Players(ctx) (Players, error)   // {Players []Player}
func (c *Client) Settings(ctx) (map[string]any, error)
// POST
func (c *Client) Announce(ctx, message string) error
func (c *Client) Kick(ctx, userid, message string) error
func (c *Client) Ban(ctx, userid, message string) error
func (c *Client) Unban(ctx, userid string) error
func (c *Client) Save(ctx) error
func (c *Client) Shutdown(ctx, waittime int, message string) error
func (c *Client) Stop(ctx) error
```
- 所有请求带 `Authorization: Basic base64("admin:"+password)`。
- 非 2xx：读取 body 归一化为 `error`（保留状态码语义，401→凭证错误，其它→透传消息）。
- 连接被拒/超时 → 明确的 unreachable 错误（供上层判定 reachable=false）。

### 新增路由（internal/api/router.go，protected 组内 servers 子组下）
```
GET  /servers/:id/rest/status
GET  /servers/:id/rest/info
GET  /servers/:id/rest/metrics
GET  /servers/:id/rest/players
GET  /servers/:id/rest/settings
POST /servers/:id/rest/announce   {message}
POST /servers/:id/rest/kick       {userid, message}
POST /servers/:id/rest/ban        {userid, message}
POST /servers/:id/rest/unban      {userid}
POST /servers/:id/rest/save
POST /servers/:id/rest/shutdown   {waittime, message}
POST /servers/:id/rest/stop
```

### handler 公共逻辑（新增 internal/api/rest_handlers.go）
1. 解析 `:id`，查 `install_path, last_error`。
2. `status := r.process.DeriveStatus(id, lastError)`；非 running → 409/400 `{"error":"server not running"}`（status 端点除外，仍返回结构化状态）。
3. `settings := palconfig.LoadSettings(installPath)`；取 `RESTAPIEnabled`(bool)、`RESTAPIPort`(int)、`AdminPassword`(string)。
4. 未启用/密码空 → 4xx `{"error"}`（status 端点返回 enabled=false/reason）。
5. `client := palapi.New(port, password)`；调用对应方法；成功透传 JSON，失败 502/400 + `{"error"}`。

### /rest/status 响应
```json
{ "enabled": true, "running": true, "reachable": true, "port": 8212, "reason": "" }
```
- reason 用于前端提示（如 "restapi_disabled" / "admin_password_empty" / "not_running" / "unreachable"）。
- **绝不返回 AdminPassword**（满足 AC8）。

## Frontend Contracts

> 变更：**不再新建/重写单一 REST 面板**。移除独立 `restapi` 分区，接口能力分散接入 Overview/Players/Operations 三分区，并以共享 hook + 提示组件统一“可用性/引导”。

### serversApi 扩展（ui/src/lib/api.ts）
```ts
restStatus, restInfo, restMetrics, restPlayers, restSettings,
restAnnounce(id,{message}), restKick(id,{userid,message}), restBan(id,{userid,message}),
restUnban(id,{userid}), restSave(id), restShutdown(id,{waittime,message}), restStop(id)
```
路径 `/api/servers/${id}/rest/...`。新增 TS 类型放 ui/src/types/server.ts（PalInfo/PalMetrics/PalPlayer/RestStatus）。

### 共享层（新增）
- `ui/src/hooks/useRestStatus.ts`：`useQuery(['rest-status', id], serversApi.restStatus, {refetchInterval:5000})`，返回 `{status, isAvailable}`（isAvailable = enabled && running && reachable）。
- `ui/src/components/server-manage/RestUnavailableNotice.tsx`：依据 `status.reason` 渲染统一提示（未启用/密码空/未运行/不可达；含“去设置分区开启”指引）。三分区在 `!isAvailable` 时统一渲染它。

### 分区改造（复用现有 SectionShell/PanelCard）
- **OverviewSection**（只读）：
  - `useRestStatus(id)` + `useQuery ['rest-info'|'rest-metrics'|'rest-players']`（enabled: isAvailable, refetchInterval 5000）。
  - 指标磁贴填 online(`currentplayernum`/`maxplayernum`)、FPS(`serverfps`)、uptime、帧时间；信息卡填 servername/version/description。
  - 在线玩家卡：`players` 只读列表（名字+level+ping）。
  - `!isAvailable` → 磁贴显示占位 + 顶部 `RestUnavailableNotice`。
- **PlayersSection**：
  - `players` 标签：玩家表（name/level/ping/userId），每行 Kick/Ban（Ban 弹窗含 message）；表上方“解封 userId”输入+按钮。
  - guilds/pals/inventory 标签保持占位（Out of Scope）。
  - `!isAvailable` → `RestUnavailableNotice`。
- **OperationsSection**（写）：四张 PanelCard —
  - 公告：Input/Textarea + 发送；保存：Button；优雅关服：Dialog(waittime number + message)；立即停止：Button。
  - kick/ban 从本分区移除（迁往 Players）；破坏性操作二次确认 Dialog。
  - `!isAvailable` → 卡片禁用 + `RestUnavailableNotice`。
- Mutations 用 `useMutation`，成功后 `invalidateQueries`（如 players/metrics）；反馈用**内联状态文本/Badge**（项目无 toast 库）。

### 导航与清理（ui/src/app/servers/manage/page.tsx）
- `SECTIONS` 移除 `{ key: 'restapi', icon: Webhook }`；移除对应 import 与 `active === 'restapi'` 分支。
- 删除 `ui/src/components/server-manage/RestApiSection.tsx`。
- 清理 `serverManage.restapi.*` 中不再使用的 i18n 键（保留/新增 REST 提示所需键）。

## 复用 / 约定
- 后端错误：`{"error": string}`，状态码沿用现有风格。
- 前端：Card/Button/Input/Textarea/Dialog/Badge/Label 均已存在；布局复用 shared.tsx 的 PanelCard。
- i18n：`serverManage.restapi.*` 三语键（zh/en/ja），沿用 useTranslations。

## Trade-offs / Risks
- **代理而非直连**：前端不直接碰游戏服端口，凭证不出后端（安全，满足 AC8），代价是多一跳。选择代理。
- **轮询 5s**：与页面既有节奏一致；面板关闭时组件卸载自动停止查询。
- **无 toast 库**：不引新依赖，用内联反馈，降低范围。
- **Windows 优先**：palapi 走 127.0.0.1，跨平台无差异；无平台专属代码。
- **settings 值解析**：`RESTAPIEnabled` INI 中为 `True/False` 字符串，需大小写不敏感解析为 bool；`RESTAPIPort` 为字符串转 int，失败回退 8212。

## Rollback
- 后端纯增量（palapi 包、rest_handlers.go、路由）：回滚即删除。
- 前端：改造 Overview/Players/Operations 三分区 + 新增 hook/notice + 删除 RestApiSection 与导航项。回滚 = 用 git 恢复三分区与 page.tsx、恢复 RestApiSection.tsx、删除新增文件。三分区原为占位，回滚风险低。

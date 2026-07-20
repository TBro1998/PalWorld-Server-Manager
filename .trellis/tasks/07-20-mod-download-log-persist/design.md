# Design: 修复 mod 下载刷新后日志丢失

## 架构与边界

复用项目已有的"历史 + 实时"日志模式（服务器运行日志即如此：`GetLogs` 拉历史 + `StreamLogs` 接实时）。本次把该模式补齐到全局 mod 下载流，不引入新机制、不改动落盘/广播路径。

改动集中在 4 个文件，前后端各 2 处：

- 后端 `internal/api/mod_handlers.go`：新增 downloading 字段计算 + 新增历史日志端点。
- 后端 `internal/api/router.go`：注册 `GET /api/mods/:modId/logs`。
- 前端 `ui/src/lib/api.ts`：`globalModsApi` 增加 `getLogs`，`Mod` 类型补 `downloading`。
- 前端 `ui/src/app/mods/page.tsx`：列表初始化 `downloadingIds` + `ModRow` 先回填历史再连 SSE。

## 数据流与契约

### R1 — 列表返回 downloading（后端）

`ModWithStatus` 增加字段：

```go
type ModWithStatus struct {
    models.Mod
    ServerCount int  `json:"server_count"`
    Downloading bool `json:"downloading"`
}
```

在 `ListGlobalMods` 组装 result 时逐条计算：`Downloading: r.process.IsDownloadingGlobalMod(m.WorkshopID)`。`IsDownloadingGlobalMod` 内部持锁，N 条 mod 即 N 次加锁；mod 数量级很小（个位到几十），可接受，不做批量优化。

### R2 — 历史日志端点（后端）

新增 `GET /api/mods/:modId/logs`，形状对齐 `GetLogs`，但 kind 固定为 `KindSteamCMD`（mod 下载只有这一种流），因此不解析 `kind` query：

```go
func (r *Router) GetModLogs(c *gin.Context) {
    modID, err := strconv.ParseInt(c.Param("modId"), 10, 64)
    if err != nil { /* 400 Invalid mod ID */ }
    lines := 200
    if v := c.Query("lines"); v != "" {
        if n, err := strconv.Atoi(v); err == nil && n > 0 { lines = n }
    }
    logs, err := logger.ReadLogs(r.config.LogDir, modID, logger.KindSteamCMD, lines)
    if err != nil { /* 500 */ }
    c.JSON(http.StatusOK, gin.H{"modId": modID, "logs": logs})
}
```

`ReadLogs` 对缺失文件返回空切片（`os.IsNotExist` → `[]string{}`），因此从未下载过的 mod 请求该端点得到 `{ "logs": [] }`，安全。

路由注册（`router.go` 全局 mod 组内，紧邻 stream 路由）：

```go
globalMods.GET("/:modId/logs", r.GetModLogs)
globalMods.GET("/:modId/logs/stream", r.ModLogStream)  // 已存在
```

### R3 — 列表初始化下载态（前端）

`Mod` 类型增加可选 `downloading?: boolean`。页面在拿到 `data.mods` 后，用一个 effect 把 `downloading === true` 的 mod id 并入 `downloadingIds`（并集而非覆盖，避免打断本地已在跟踪的下载）：

```ts
useEffect(() => {
  const serverDownloading = mods.filter(m => m.downloading).map(m => m.id)
  if (serverDownloading.length === 0) return
  setDownloadingIds(prev => {
    const next = new Set(prev)
    serverDownloading.forEach(id => next.add(id))
    return next
  })
}, [mods])
```

由于 `refetchInterval` 依赖 `downloadingIds.size > 0`，刷新后首帧列表返回 downloading=true → 集合非空 → 轮询自动恢复；下载结束后端 downloading 转 false，`handleDownloadDone`（由 SSE done 触发）已负责从集合移除。

### R4 — ModRow 先回填历史再连 SSE（前端）

现有 `ModRow` 有两个 effect：一个在 `downloading` 变 true 时重置并打开面板，一个在 `downloading` 为 true 时连 SSE。问题：重置 effect 无条件 `setLogLines([])`，会清掉回填的历史。

调整为：`downloading` 为 true 时，SSE effect 内先 `getLogs` 回填、再连 SSE。用一个 `cancelled` 标志防止竞态与卸载后 setState。移除"重置 effect"里对 `setLogLines([])` 的清空（改由回填流程统一管理），保留打开面板/清 done 状态。

```ts
useEffect(() => {
  if (!downloading) return
  let cancelled = false

  globalModsApi.getLogs(mod.id)
    .then(res => { if (!cancelled) setLogLines(res.data.logs ?? []) })
    .catch(() => { /* SSE 仍会补新行 */ })

  const es = new EventSource(globalModsApi.logStreamUrl(mod.id))
  es.addEventListener('log', e => setLogLines(prev => [...prev, (e as MessageEvent).data]))
  es.addEventListener('done', e => {
    setLogDone((e as MessageEvent).data === 'ok' ? 'ok' : 'error')
    onDownloadDone(); es.close()
  })
  es.onerror = () => { setLogDone('error'); onDownloadDone(); es.close() }

  return () => { cancelled = true; es.close() }
}, [downloading, mod.id, onDownloadDone])
```

注意去重顺序：`getLogs` 回填的历史行 + SSE 实时行理论上可能有一行重叠窗口，但 `ReadLogs` 读的是刷新前落盘的历史、SSE 只推订阅之后的新行，两者时间不交叠（订阅在回填请求发出后建立，最坏情况漏读极少量行而非重复）。与 `LogsSection` 同等语义，不额外去重。

api.ts 新增：

```ts
getLogs: (modId: number, lines = 200) =>
  apiClient.get<{ modId: number; logs: string[] }>(`/api/mods/${modId}/logs`, { params: { lines } }),
```

## 兼容性与迁移

- 无数据库 schema 改动。`downloading` 是运行时计算字段，不落库。
- 新增字段/端点是纯增量，旧前端忽略 `downloading` 亦不报错。
- Swagger：`GetModLogs` 加 godoc 注解；按 CLAUDE.md，改了 handler 需 `swag init` 重新生成并提交 `docs/`。

## 权衡

- **为何不在 SSE 流里做历史回填**：`ModLogStream` 用 `c.Stream` 逐条转发 channel，塞入历史需要额外读文件并在建流时先 flush，破坏其单一职责；项目既有模式就是"两个端点"，遵循之。
- **为何 downloading 走列表字段而非独立端点**：前端本就轮询列表（`refetchInterval`），复用同一响应零额外请求；独立 status 端点会多一次往返。

## 运维 / 回滚

- 单一提交，回滚即 `git revert`。无状态迁移、无副作用文件。
- 风险文件：`ModRow` 的 effect 依赖数组与 `onDownloadDone` 引用稳定性。当前父组件传入 `() => handleDownloadDone(mod.id)`（每次 render 新建箭头函数），若留在 SSE effect 依赖里会导致轮询重渲染时 SSE 反复重连、历史反复回填。**遵循 mod-handling spec（line 76）既有约定：用 ref 承接 `onDownloadDone`**，将其移出 SSE effect 依赖，effect 仅依赖 `[downloading, mod.id]`。

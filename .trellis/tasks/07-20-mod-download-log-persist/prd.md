# PRD: 修复 mod 下载刷新后日志丢失

## Goal / 用户价值

当某个 mod 正在下载时，用户刷新 mod 管理界面（`/mods`）后，前端应仍能识别该 mod 处于下载中，并继续展示下载日志（含刷新前已产生的历史日志），直到下载结束。

## Background / 确认事实（来自代码勘查）

下载本身运行在后端 goroutine 中，与前端页面生命周期无关；日志同时落盘并实时广播。刷新丢失的只是**前端内存态**，后端状态完好，缺的是把后端状态捞回前端的通道。

- 下载在后端 goroutine 执行：[internal/api/mod_handlers.go:231-243](../../../internal/api/mod_handlers.go#L231-L243)。日志经 `io.MultiWriter(capture, broadcaster)` 同时落盘与广播。
- 落盘：`logger.NewCapture(modID, KindSteamCMD, LogDir)` 写入 `<LogDir>/server_<modID>/steamcmd/current.log`（[internal/logger/capture.go:49-58](../../../internal/logger/capture.go#L49-L58)）。历史日志刷新后仍在磁盘上。
- 下载中状态在后端有权威来源：`Manager.IsDownloadingGlobalMod(workshopID)`（[internal/process/manager.go:362-369](../../../internal/process/manager.go#L362-L369)），键为 workshopID。
- 前端"下载中"仅存于内存：`downloadingIds` 是 `useState`（[ui/src/app/mods/page.tsx:49](../../../ui/src/app/mods/page.tsx#L49)），刷新即清空 → `ModRow` 的 `downloading` 变 false，日志面板关闭且不再连 SSE。
- SSE 流 `ModLogStream` 只订阅后续实时行，无历史回填（[internal/api/mod_handlers.go:264-292](../../../internal/api/mod_handlers.go#L264-L292)）。`globalModsApi` 也没有对应的 getLogs（[ui/src/lib/api.ts:153-162](../../../ui/src/lib/api.ts#L153-L162)）。
- `ListGlobalMods` 返回的 `ModWithStatus` 不含 downloading 标志（[internal/api/mod_handlers.go:21-68](../../../internal/api/mod_handlers.go#L21-L68)）。
- 参照实现：服务器运行日志查看器先 `getLogs()` 拉历史、再连 SSE（[ui/src/components/server-manage/LogsSection.tsx:31-54](../../../ui/src/components/server-manage/LogsSection.tsx#L31-L54)）；对应后端历史端点 `GetLogs` 用 `logger.ReadLogs`（[internal/api/handlers.go:579-610](../../../internal/api/handlers.go#L579-L610)）。这是本项目已确立的"历史+实时"模式。

## Requirements

- R1: `GET /api/mods` 每条 mod 返回 `downloading` 布尔字段，由 `IsDownloadingGlobalMod(mod.WorkshopID)` 计算。
- R2: 新增 `GET /api/mods/:modId/logs` 历史日志回填端点，复用 `logger.ReadLogs(LogDir, modID, KindSteamCMD, lines)`，对齐服务器 `GetLogs` 的形状。
- R3: 前端 `/mods` 页面加载时，用列表返回的 `downloading` 标志初始化 `downloadingIds`，使刷新后仍标记下载中并保持轮询/日志面板打开。
- R4: `ModRow` 在 `downloading` 为真时，先调用新的 getLogs 回填历史日志，再连 SSE 接续实时行（对齐 `LogsSection` 模式）。

## Acceptance Criteria

- AC1: 触发某 mod 下载后刷新 `/mods`，该行仍显示"下载中"（spinner + 日志面板展开），无需再次点击。（覆盖 R1、R3）
- AC2: 刷新后日志面板包含刷新前已产生的历史日志行，且随后继续追加新行直到 done。（覆盖 R2、R4）
- AC3: 下载在后端结束（`done`）后，无论刷新与否，该行最终回到非下载态且状态标志正确。（覆盖 R1、R3、R4）
- AC4: 无下载进行时刷新 `/mods` 行为不变，`downloadingIds` 为空、不触发多余轮询。（回归，覆盖 R1、R3）

## Out of Scope

- 服务器 mod 部署流（`DeployServerMods`）的同类刷新问题——本次仅针对全局 mod 库下载。
- 多标签页/多客户端同时观看同一下载的一致性优化（当前 SSE 广播已支持多订阅者，不额外处理）。
- 下载队列、断点续传等能力。

## Open Questions

（无阻塞项。范围已确认：仅全局 mod 下载 `/mods`，deploy 弹窗同类问题不在本次范围内。）

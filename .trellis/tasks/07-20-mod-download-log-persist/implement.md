# Implement: 修复 mod 下载刷新后日志丢失

## 执行清单（按序）

### 后端

1. `internal/api/mod_handlers.go` — `ModWithStatus` 增加 `Downloading bool \`json:"downloading"\`` 字段；在 `ListGlobalMods` 组装 result 循环中填 `Downloading: r.process.IsDownloadingGlobalMod(m.WorkshopID)`。（R1）
2. `internal/api/mod_handlers.go` — 新增 `GetModLogs` handler：解析 `modId`、可选 `lines`（默认 200），调用 `logger.ReadLogs(r.config.LogDir, modID, logger.KindSteamCMD, lines)`，返回 `{modId, logs}`。加 godoc 注解（`@Router /mods/{modId}/logs [get]`）。（R2）
3. `internal/api/router.go` — 在全局 mod 组注册 `globalMods.GET("/:modId/logs", r.GetModLogs)`，置于 `/:modId/logs/stream` 之前或相邻。（R2）

### 前端

4. `ui/src/lib/api.ts` — `globalModsApi` 增加 `getLogs(modId, lines=200)`，形状 `{ modId: number; logs: string[] }`。（R4）
5. `ui/src/types/server.ts`（`Mod` 类型定义处） — 增加可选 `downloading?: boolean`。（R1/R3）
6. `ui/src/app/mods/page.tsx`：
   - `ModRow` 内用 ref 承接 `onDownloadDone`（遵循 mod-handling spec line 76 既有约定），使其移出 SSE effect 依赖，避免父组件轮询重渲染导致 SSE 反复重连、历史反复回填。（R4 前置修复）
   - 新增 effect：从 `mods` 中筛 `downloading===true` 的 id 并入 `downloadingIds`（并集，见 design）。（R3）
7. `ui/src/app/mods/page.tsx` `ModRow`：
   - 合并/调整 effect：`downloading` 为 true 时先 `globalModsApi.getLogs(mod.id)` 回填 `logLines`，再连 SSE；用 `cancelled` 标志防竞态。移除原"重置 effect"中无条件 `setLogLines([])`（保留打开面板、清 logDone）。（R4）

### 收尾

8. `swag init -g internal/api/docs.go --parseInternal --parseDependency` 重新生成并提交 `docs/`（改了 handler）。

## 校验命令

```bash
# 后端编译 + 测试（mod handler 已有测试）
cd e:/ZZH/PalWorld-Server-Manager
go build ./...
go test ./internal/api/... ./internal/logger/... ./internal/process/...

# 前端 lint + 构建（静态导出，后端嵌入依赖）
cd ui
bun run lint
bun run build
```

## 手动验证（对应 AC）

- 起一个 mod 下载 → 刷新 `/mods`：该行仍 spinner + 日志面板展开（AC1）；面板含刷新前历史行并继续追加（AC2）。
- 等下载 done：行回非下载态，标志正确（AC3）。
- 无下载时刷新：无 spinner、无多余轮询（AC4）。

## 风险文件 / 回滚点

- `ui/src/app/mods/page.tsx`：effect 依赖与回调引用稳定性是主要风险（`onDownloadDone` 引用不稳会致 SSE 反复重连、历史反复回填）。改动后确认下载期间 Network 面板 `/logs` 只请求一次、SSE 只连一次。
- 单提交，回滚 = `git revert`。无 DB / 磁盘状态迁移。

## task.py start 前置检查

- 本任务为 inline 工作流（直接实现，非 sub-agent dispatch）→ 跳过 implement.jsonl / check.jsonl 门槛。
- prd.md / design.md / implement.md 三件齐备。

# PRD — 服务器 REST API 控制功能

## Goal / User Value
通过 Palworld 官方 REST API，把服务器实时信息与运维操作直接做成“服务器管理”页各功能分区里的可视化交互（概览看实时信息、玩家分区管理在线玩家、运维分区执行公告/保存/关服等），无需外部工具，也**不**保留一个单独罗列端点的 “REST API” 分区。

## Background / Confirmed Facts (from code + official docs)
- 官方 REST API（文档：https://docs.palworldgame.com/category/rest-api）：
  - 前提：服务器配置 `RESTAPIEnabled=True`。
  - 认证：HTTP Basic Auth，用户名固定 `admin`，密码为 `AdminPassword`。
  - 端口：`RESTAPIPort`（默认 8212）。
  - 端点：
    - `GET  /v1/api/info` — {version, servername, description, worldguid}
    - `GET  /v1/api/metrics` — {serverfps, currentplayernum, serverframetime, maxplayernum, uptime, days}
    - `GET  /v1/api/players` — {players:[{name, accountName, playerId, userId, ip, ping, location_x, location_y, level, building_count}]}
    - `GET  /v1/api/settings` — 当前生效设置对象
    - `POST /v1/api/announce` — {message}
    - `POST /v1/api/kick` — {userid, message}
    - `POST /v1/api/ban` — {userid, message}
    - `POST /v1/api/unban` — {userid}
    - `POST /v1/api/save` — 无 body
    - `POST /v1/api/shutdown` — {waittime:int(秒), message:string}
    - `POST /v1/api/stop` — 无 body，立即停止
- 后端可用 `palconfig.LoadSettings(installPath)`（internal/palconfig/ini.go:75）读取每台服务器 INI 的 `RESTAPIEnabled`/`RESTAPIPort`/`AdminPassword`，据此连接目标 REST API。
- 管理器与游戏服务器同机部署（当前仅 Windows），代理目标为 `127.0.0.1:<RESTAPIPort>`。
- 现有后端 handler 模式（internal/api/handlers.go）：解析 `:id` → 查库 → `r.process.DeriveStatus(id, lastError)` 派生状态 → 返回 JSON，错误统一 `{"error": ...}`。
- 路由分 `auth`（公开）与 `protected` 组（internal/api/router.go:46），服务器子路由挂在 `/servers/:id/*`。
- 前端管理页（ui/src/app/servers/manage/page.tsx）左侧分区导航 `SECTIONS` 含 overview/players/operations/map/restapi/backup/settings，每 5s 轮询服务器状态。
- 现有分区均为占位，且天然对应本功能：
  - OverviewSection：指标磁贴（cpu/memory/online/uptime）+ 信息卡 + 在线玩家卡。
  - PlayersSection：players/guilds/pals/inventory 标签。
  - OperationsSection：kick/ban/broadcast/shutdown 磁贴。
  - RestApiSection：仅静态罗列端点（本任务将**移除**）。
- 前端 API 客户端 ui/src/lib/api.ts 走相对路径 `/api`，带 JWT 拦截器。
- UI 原语已具备：button/input/textarea/dialog/badge/card/select/label；**无 toast 库**（反馈用内联状态）。

## Decisions
- D1 前置未满足处理：当 `RESTAPIEnabled=False` 或 `AdminPassword` 为空时，**仅展示引导提示**（去“设置”开启 RESTAPIEnabled 并设置 AdminPassword 后重启），**不新增自动写入 INI 能力**。
- D2 关闭语义：提供**优雅关服**（`/shutdown`，弹窗输入倒计时秒数+广播消息）与**立即停止**（`/stop`）两个操作。
- D3 **移除独立 REST API 分区**：从 `SECTIONS` 导航删除 `restapi`，删除 `RestApiSection` 组件；接口能力分散接入现有功能分区。
- D4 分区映射（“概览读 + 玩家管理 + 运维写”）：
  - **概览 Overview（只读）**：`/info` + `/metrics` 驱动指标磁贴与信息卡；`/players` 驱动“在线玩家”只读预览。
  - **玩家 Players**：`/players` 驱动完整玩家表；每行“踢出/封禁”，另有“解封 userId”输入。
  - **运维 Operations（写）**：公告 `/announce`、保存 `/save`、优雅关服 `/shutdown`、立即停止 `/stop`。
- D5 刷新策略：
  - **概览 Overview**：**不自动轮询**其 REST 数据（info/metrics/players），改为提供**手动“刷新”按钮**由用户触发拉取。
  - **玩家 Players**：玩家表按 5s 轮询（保留）。
  - 共享 `useRestStatus`（可用性状态）：仍按 5s 轮询以驱动各分区可用/不可用切换。
- D6 破坏性操作（kick/ban/unban/shutdown/stop）前端需二次确认弹窗；成功/失败内联反馈。
- D7 后端为“代理转发”：读取该服务器 INI 得端口/密码，向 `127.0.0.1:port` 发起 Basic Auth 请求转发 JSON；HTTP 超时约 5s；**不向前端返回 AdminPassword 明文**。
- D8 `/settings` 端点：后端 palapi 与代理端点保留（成本低），但 UI 暂不单独呈现（设置编辑已由 Settings 分区/配置编辑器覆盖）；列入 Out of Scope 的 UI 部分。
- D9 REST 可用性状态与引导提示由**共享 hook + 共享提示组件**统一提供，供概览/玩家/运维三分区复用，避免重复。

## Requirements
- R1 后端新增 `internal/palapi` 客户端包：给定 (host=127.0.0.1, port, adminPassword)，封装全部 11 个端点，含 5s 超时、Basic Auth、非 2xx 与连接失败(unreachable) 错误归一化。
- R2 后端新增受保护路由 `/servers/:id/rest/*`：
  - `GET  /servers/:id/rest/status` — 返回 {enabled, reachable, running, port, reason}，不含密码。
  - `GET  /servers/:id/rest/info`、`/metrics`、`/players`、`/settings` — 透传。
  - `POST /servers/:id/rest/announce`、`/kick`、`/ban`、`/unban`、`/save`、`/shutdown`、`/stop` — 透传，校验必填项。
  - 统一前置校验：服务器存在、running、RESTAPIEnabled=True、AdminPassword 非空；否则 4xx + 明确 `{"error"}`（status 端点仍返回结构化状态）。
- R3 前端 `serversApi` 扩展 REST 方法；新增共享 `useRestStatus(serverId)` hook 与 `RestUnavailableNotice` 提示组件（D9）。
- R4 分区改造（D4）：
  - Overview：指标磁贴填入在线数/FPS/uptime；信息卡填 servername/version/description；在线玩家卡渲染只读列表；REST 不可用时磁贴显示占位并给提示。
  - Players：`players` 标签渲染玩家表 + 每行踢出/封禁（封禁含 message）+ 解封 userId 输入；其余标签（guilds/pals/inventory）保持占位。
  - Operations：公告（输入+发送）、保存、优雅关服（弹窗 waittime+message）、立即停止；四项均接入接口，破坏性操作二次确认。
- R5 导航与清理（D3）：`SECTIONS` 移除 `restapi`；删除 `RestApiSection.tsx`；移除相关未用 i18n/图标引用。
- R6 i18n：为新增/改造文案补充 zh/en/ja `serverManage.*` 键（概览信息字段、玩家表列/操作、运维操作与弹窗、REST 不可用提示）。

## Acceptance Criteria
- [ ] AC1 服务器已启用 REST API 且 running 时，概览分区显示 info/metrics（在线/上限、FPS、uptime、服名/版本）；数据**不自动轮询**，点击“刷新”按钮后拉取最新值（含加载态）。
- [ ] AC2 玩家分区显示在线玩家表（name/level/ping/userId 等），并每 5s 刷新。
- [ ] AC3 未启用（RESTAPIEnabled=False 或 AdminPassword 空）或未运行时，概览/玩家/运维分区展示统一的引导/状态提示，不显示会失败的控制按钮。
- [ ] AC4 运维-公告：输入消息发送后游戏内收到广播，前端提示成功。
- [ ] AC5 玩家-踢出/封禁：对在线玩家执行成功并生效；运维/玩家处“解封 userId”成功。
- [ ] AC6 运维-保存：调用后返回成功。
- [ ] AC7 运维-优雅关服：输入倒计时与消息后游戏内出现倒计时广播并到期关服；立即停止可即时停服。
- [ ] AC8 破坏性操作均有二次确认；失败时展示后端返回的错误信息。
- [ ] AC9 后端不向前端返回 AdminPassword 明文。
- [ ] AC10 独立 REST API 分区已从导航移除且 `RestApiSection.tsx` 删除，无残留引用与死代码。
- [ ] AC11 `go build .`、`go test ./internal/palapi/...`、`cd ui && bun run build` 通过；`bun run lint` 无新增错误。

## Out of Scope
- RCON 通道（本任务仅走 REST API）。
- 自动写入/生成 AdminPassword 或 RESTAPIEnabled（见 D1）。
- 将 REST API 暴露到公网。
- `/settings` 的 UI 呈现（后端端点保留，见 D8）。
- PlayersSection 的 guilds/pals/inventory 存档解析数据实现。

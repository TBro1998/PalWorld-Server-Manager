# Implement — 服务器 REST API 控制功能

## 执行顺序（后端 → 前端 → 联调）

### 阶段 A：后端 palapi 客户端包
- [ ] A1 新建 `internal/palapi/client.go`：`Client` 结构 + `New(port, password)`（BaseURL=`http://127.0.0.1:<port>/v1/api`，`http.Client{Timeout:5s}`）。
- [ ] A2 内部 `doJSON`/`doPost` 助手：设置 Basic Auth、Content-Type，处理 2xx/非2xx/连接错误（区分 unreachable）。
- [ ] A3 实现 11 个方法 + 响应结构体（Info/Metrics/Player/Players）。
- [ ] A4 新建 `internal/palapi/client_test.go`：用 `httptest.Server` 覆盖 GET info、POST announce、401、connection-refused/unreachable 归一化。

### 阶段 B：后端 handler + 路由
- [ ] B1 新建 `internal/api/rest_handlers.go`：公共 `resolveRest(id)`（查库→DeriveStatus→LoadSettings→解析 enabled/port/password），返回 (client, status, enabled, err)。
- [ ] B2 实现 `RestStatus`（结构化，不返回密码）与 4 个 GET 透传 handler。
- [ ] B3 实现 7 个 POST handler，含 body 绑定与必填校验（message/userid/waittime）。
- [ ] B4 `internal/api/router.go`：在 servers 子组注册 `/rest/*` 路由。
- [ ] B5 `go build .` 通过；`go test ./internal/palapi/...` 通过。

### 阶段 C：前端（分散接入，不建独立面板）
- [ ] C1 `ui/src/types/server.ts`：新增 `PalInfo`/`PalMetrics`/`PalPlayer`/`RestStatus` 类型。
- [ ] C2 `ui/src/lib/api.ts`：serversApi 增加 rest* 方法。
- [ ] C3 共享层：`ui/src/hooks/useRestStatus.ts`（含 isAvailable 派生 + 5s 轮询）与 `ui/src/components/server-manage/RestUnavailableNotice.tsx`（按 reason 渲染引导）。
- [ ] C4 改造 `OverviewSection.tsx`：info/metrics 磁贴与信息卡 + 在线玩家只读列表；不可用时提示。
- [ ] C5 改造 `PlayersSection.tsx`：players 标签玩家表 + 每行踢出/封禁 + 解封 userId 输入；确认 Dialog。
- [ ] C6 改造 `OperationsSection.tsx`：公告/保存/优雅关服(Dialog)/立即停止；破坏性操作二次确认；移除 kick/ban 磁贴。
- [ ] C7 导航清理：`ui/src/app/servers/manage/page.tsx` 从 `SECTIONS` 移除 `restapi` 及其 import/分支；删除 `RestApiSection.tsx`。
- [ ] C8 i18n：`ui/messages/{zh,en,ja}.json` 增删相应 `serverManage.*` 键（概览字段、玩家表/操作、运维操作与弹窗、REST 不可用提示；清理废弃的 restapi.* 键）。
- [ ] C9 `cd ui && bun run lint` 无新增错误；`bun run build` 通过。

### 阶段 D：联调 / 验收
- [ ] D1 对照 PRD AC1–AC9 自查。
- [ ] D2 verify：启用 REST API 的运行中服务器上，实测 info/players/announce/kick/save/shutdown 路径（或对 handler 层做最小驱动验证）。

## 验证命令
```bash
# 后端
go build .
go test ./internal/palapi/... ./internal/api/...

# 前端
cd ui && bun run lint && bun run build
```

## 风险文件 / 回滚点
- `ui/src/app/servers/manage/page.tsx`：改导航 SECTIONS + 移除分支——注意别破坏其余分区渲染。
- `Overview/Players/Operations` 三分区：改造占位为功能实现，回滚用 git 恢复。
- `RestApiSection.tsx`：删除文件——回滚即从 git 恢复。
- `internal/api/router.go`：仅新增路由行，风险低。
- 新增文件（palapi/*、rest_handlers.go、useRestStatus.ts、RestUnavailableNotice.tsx）：回滚即删除。

## Review Gates
- 阶段 B 结束：后端可 build + 单测过，再进前端。
- 阶段 C 结束：lint/build 过，再联调。
- 提交前：full-scope check（trellis-check）。

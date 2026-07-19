# PRD: 后端 OpenAPI/Swagger 集成（子任务 A）

> 父任务：`07-19-mcp-server-ai-ops`。本子任务可独立验收。

## Goal

为现有 REST API 提供一份**与代码同源、可运行时拉取**的 OpenAPI 契约，并对外提供 Swagger UI。作为 AI 运维 skill（子任务 B）的精确契约来源，降低 skill 手写 API 表带来的漂移。

## Confirmed Facts

- handler 均为 `*Router` 方法，分布在 `internal/api/*.go`（`handlers.go` 19、`rest_handlers.go` 15、`mod_handlers.go` 10、`system_handlers.go` 7、`save_handlers.go` 5、`whitelist_handlers.go` 4、`auth_handlers.go` 4、`workshop_handlers.go` 3、`steam_handlers.go` 3、`router.go` 3；含辅助方法，路由端点约 40）。
- 路由在 `internal/api/router.go` 统一注册到 `/api` 组，受 `auth.Middleware` 保护（除 `/auth/status|setup|login`）。
- 多数 handler 返回 `gin.H{}` 匿名 map，非 typed 结构体。
- 无现存 `docs/` 目录与 OpenAPI。

## Requirements

- R2.1 引入 swaggo：`github.com/swaggo/swag`（CLI）、`github.com/swaggo/gin-swagger`、`github.com/swaggo/files`。
- R2.2 通用 API 信息注解（title/version/description/BasePath=`/api`/securityDefinitions BearerAuth）放在专用 `internal/api/docs.go` 或 `main.go`。
- R2.3 为 `router.go` 注册的**全部**端点在其 handler 上添加操作注解：`@Summary/@Tags/@Param/@Success/@Failure/@Router`，受保护端点标 `@Security BearerAuth`。响应体统一用 `{object} map[string]interface{}`（或对少数关键端点定义响应 DTO），不强行重构 handler 返回类型。
- R2.4 `swag init` 生成 `docs` 包并**提交入库**，使 `go build .` 无需预装 swag 即可构建；API 变更时重跑 `swag init`。
- R2.5 在 `internal/server/server.go` 挂载 `GET /swagger/*any`（`ginSwagger.WrapHandler`），blank-import 生成的 `docs` 包。**该路由公开、不经 JWT**。
- R2.6 spec 的 `basePath` 正确（`/api`；host 留空由 UI 相对解析或运行时设置）。
- R2.7 更新 `CLAUDE.md` 构建说明：新增 `swag init`（及 `go install github.com/swaggo/swag/cmd/swag@latest`）步骤与依赖说明。

## Acceptance Criteria

- [ ] `go build .`（不预跑 swag）成功——因 `docs` 包已入库。
- [ ] 运行后 `GET /swagger/index.html` 打开 UI；`GET /swagger/doc.json` 返回合法 OpenAPI JSON，**无需鉴权**。
- [ ] spec 覆盖 `router.go` 全部端点；抽查 `POST /api/servers/:id/start`、`GET /api/system/stats`、`POST /api/auth/login` 三端点的 path/method/参数/鉴权标注与实现一致。
- [ ] `swag init` 可从干净状态重新生成且与提交产物一致（无手改生成物）。
- [ ] 现有前端与 API 行为不回归（`/swagger/*` 不与 `NoRoute` 静态回退冲突）。
- [ ] Windows 下 `go build .` 通过。

## Out of Scope

- 把 handler 返回值重构为 typed DTO（仅按需为个别关键端点定义响应模型）。
- SSE 流式端点（`/logs/stream` 等）响应体的精确建模（标为 text/event-stream，说明为流即可）。
- Linux 专门验证。

## 依赖
- 无上游依赖。是子任务 B 的上游（B 依赖本任务产出的 spec）。

# Design: 后端 OpenAPI/Swagger 集成（子任务 A）

## 架构与边界

- 不改变现有路由结构与 handler 行为。仅**新增注解 + 新增 `/swagger/*any` 路由 + 生成 `docs` 包**。
- swaggo 工作流：注解（Go 注释）→ `swag init` 解析 → 生成 `docs/docs.go`（内嵌 spec 字符串，暴露 `docs.SwaggerInfo`）→ 运行时 `gin-swagger` 提供 UI + `/swagger/doc.json`。

## 包与依赖

- `github.com/swaggo/swag`（CLI，`go install .../cmd/swag@latest`；库部分随 go.mod）
- `github.com/swaggo/gin-swagger`（Gin 中间件）
- `github.com/swaggo/files`（Swagger UI 静态资源）
- 生成物：`docs` 包（默认 `./docs`）。**入库**，`go build .` 不依赖预跑 swag。

## 通用信息注解（`internal/api/docs.go`，新建）

```go
// Package api ...
// @title           PalWorld Server Manager API
// @version         1.0
// @description     幻兽帕鲁服务器开服工具 REST API
// @BasePath        /api
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
```
> `swag init -g internal/api/docs.go --parseInternal --parseDependency` 确保解析到 `internal/models` 等内部类型。

## 操作注解规范（每个 handler 上）

```go
// StartServer godoc
// @Summary   启动服务器
// @Tags      servers
// @Produce   json
// @Param     id   path   string  true  "server id"
// @Success   200  {object}  map[string]interface{}
// @Failure   400  {object}  map[string]interface{}
// @Failure   401  {object}  map[string]interface{}
// @Security  BearerAuth
// @Router    /servers/{id}/start [post]
func (r *Router) StartServer(c *gin.Context) { ... }
```

约定：
- `@Router` 路径相对 BasePath `/api`；gin 的 `:id` 写成 `{id}`。
- 受保护端点加 `@Security BearerAuth`；`/auth/status|setup|login` 不加。
- 请求体端点用 `@Param body body <Type|map[string]interface{}> true "..."`；已有请求 DTO（如 `SetupRequest`/`LoginRequest`）直接引用。
- 响应统一 `map[string]interface{}`；`/save`、存档解析等如已有 `internal/models`/`palsave` 返回类型可引用以获得更好 schema。
- SSE 端点（`/logs/stream`、`/steam/logs/stream`、`/mods/:modId/logs/stream`、`/update/stream`）：`@Produce text/event-stream`，`@Success 200 {string} string "SSE stream"`，注明为事件流。
- 用 `@Tags` 分组：auth / servers / config / rest / save / whitelist / mods / steam / workshop / system。

## 路由挂载（`internal/server/server.go`）

- 在 `setupRoutes()` 内、`NoRoute` 静态回退**之前**注册：
  ```go
  s.router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
  ```
- blank import 生成包：`_ "github.com/TBro1998/PalWorld-Server-Manager/docs"`。
- 与静态回退关系：`NoRoute` 仅在无匹配路由时触发；显式 `/swagger/*any` 优先，无冲突。

## 已知限制/取舍

- 多数响应为 `gin.H`，schema 松（`map[string]interface{}`）；本期不重构为 typed 响应。skill 侧提示 agent「响应字段以实际返回为准，spec 仅保证 path/method/参数/鉴权准确」。
- 注解是注释、不受编译器约束，需靠 `swag init` 报错 + review 保证准确。
- Go 1.26 较新：首步即验证 `swag init` 能解析本仓库（风险点，前置到实现第 1 步）。

## 兼容/回滚

- 纯增量：删除 `docs.go` 注解、`/swagger` 路由、`docs` 包、go.mod 三个依赖即可完全回滚。
- 不改数据库、不改现有端点契约。

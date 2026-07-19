# Implementation Plan: 后端 OpenAPI/Swagger 集成（子任务 A）

## 执行顺序

### 阶段 0：前置验证
- [ ] 0.1 安装 swag CLI：`go install github.com/swaggo/swag/cmd/swag@latest`，验证 `swag version` 可执行。
- [ ] 0.2 添加 go.mod 依赖（`go get github.com/swaggo/gin-swagger github.com/swaggo/files`），跑 `go mod tidy`。
- [ ] 0.3 创建最小注解 `internal/api/docs.go`（仅 `@title/@version/@BasePath/@securityDefinitions BearerAuth`）。
- [ ] 0.4 试跑 `swag init -g internal/api/docs.go --parseInternal --parseDependency`，验证能成功生成 `docs` 包（即 Go 1.26 + 本仓库结构无阻塞点）。删除生成物后进入正式阶段。

### 阶段 1：通用信息与路由挂载
- [ ] 1.1 完善 `internal/api/docs.go` 通用注解（title/version/description/contact/license 按需）。
- [ ] 1.2 在 `internal/server/server.go` 挂载 `/swagger/*any`（`ginSwagger.WrapHandler`），blank-import `"github.com/TBro1998/PalWorld-Server-Manager/docs"`。放在 `NoRoute` 之前。
- [ ] 1.3 试跑 `swag init`，验证 UI 可访问（端点列表空）。

### 阶段 2：批量注解（按文件拆步，减少单次失败面）
**顺序建议**：简单文件→复杂文件；优先核心端点。

- [ ] 2.1 `internal/api/auth_handlers.go`（3 端点，无 Security）：AuthStatus / Setup / Login。
- [ ] 2.2 `internal/api/handlers.go`（服务器 CRUD + 安装/启停，约 10 端点，有 Security）：ListServers / CreateServer / GetServer / UpdateServer / DeleteServer / InstallServer / StartServer / StopServer / RestartServer / GetLogs。
- [ ] 2.3 `internal/api/handlers.go`（配置，2 端点）：GetServerConfig / UpdateServerConfig。
- [ ] 2.4 `internal/api/handlers.go`（SSE 日志流，1 端点，text/event-stream）：StreamLogs。
- [ ] 2.5 `internal/api/rest_handlers.go`（Palworld REST 代理，约 11 端点）：RestStatus / RestInfo / RestMetrics / RestPlayers / RestSettings / RestAnnounce / RestKick / RestBan / RestUnban / RestSave / RestShutdown / RestStop。
- [ ] 2.6 `internal/api/save_handlers.go`（存档解析，4 端点）：SavePlayers / SaveGuilds / SavePlayerPals / SavePlayerInventory。
- [ ] 2.7 `internal/api/whitelist_handlers.go`（3 端点）：GetWhitelist / AddWhitelistEntry / RemoveWhitelistEntry。
- [ ] 2.8 `internal/api/mod_handlers.go`（全局 Mod 库，约 5 端点）：ListGlobalMods / AddGlobalMod / DeleteGlobalMod / DownloadGlobalMod / ModLogStream（SSE）。
- [ ] 2.9 `internal/api/mod_handlers.go`（每服 Mod，约 5 端点）：ListServerMods / LinkServerMod / UnlinkServerMod / ToggleServerMod / DeployServerMods。
- [ ] 2.10 `internal/api/steam_handlers.go`（2 端点 + 1 SSE）：SteamStatus / SteamLogin / SteamLogStream（SSE） / SetWebAPIKey。
- [ ] 2.11 `internal/api/workshop_handlers.go`（2 端点）：WorkshopSearch / WorkshopDependencies。
- [ ] 2.12 `internal/api/system_handlers.go`（约 7 端点）：GetSystemStats / GetVersion / CheckUpdate / GetUpdateStatus / ApplyUpdate / UpdateStream（SSE） / GetSystemSettings / UpdateSystemSettings。
- [ ] 2.13 `internal/api/handlers.go`（配置 schema，1 端点）：GetConfigSchema。

每步后跑 `swag init` 验证可生成；若报错修正后再进下一步。

### 阶段 3：验收与文档
- [ ] 3.1 完整跑 `swag init`，提交生成的 `docs` 包入库（`docs/docs.go`、`docs/swagger.json`、`docs/swagger.yaml`）。
- [ ] 3.2 `go build .` 无需预跑 swag 即成功。运行后访问 `/swagger/index.html` UI、`/swagger/doc.json` spec，均公开无需鉴权。
- [ ] 3.3 抽查 3 端点（`POST /api/servers/{id}/start`、`GET /api/system/stats`、`POST /api/auth/login`）的 path/method/参数/鉴权/响应与实现一致。
- [ ] 3.4 更新 `CLAUDE.md` 构建说明，新增：
  ```markdown
  ## Build Commands
  
  ### OpenAPI/Swagger (如需重新生成 spec)
  
  ```bash
  # 安装 swag CLI
  go install github.com/swaggo/swag/cmd/swag@latest
  
  # 生成/更新 OpenAPI spec（API 变更时需重跑）
  swag init -g internal/api/docs.go --parseInternal --parseDependency
  
  # 生成物已提交入库；日常 go build 无需预跑 swag
  ```
  
  运行后访问 `http://127.0.0.1:8080/swagger/index.html` 查看 API 文档。
  ```
- [ ] 3.5 验证现有前端与 API 行为不回归（`/swagger/*` 不影响其他路由）。

## 风险缓解

- **风险**：Go 1.26 + swaggo 兼容性。**缓解**：阶段 0 前置验证；失败时回退方案是降 Go 版本或等 swaggo 更新。
- **风险**：注解遗漏或错误。**缓解**：阶段 2 按文件拆小步；每步后 `swag init` 立即验证；抽查代表端点（阶段 3.3）。
- **风险**：生成物体积过大（若注释冗长）。**缓解**：实测后若 `docs.go` > 500KB 考虑精简注释或拆 spec（本期端点约 40，预计 < 200KB，可接受）。

## 回滚点
- 阶段 0 后：删 `docs.go`、`go.mod` 三个依赖、`docs` 包；还原 `server.go`。
- 阶段 1 后：删 `/swagger` 路由与 import；删 `docs` 包；保留 `docs.go`（无影响）或一并删。
- 阶段 2/3 任意步：`git restore` 当前注解批次。

## 验收清单（再次确认）

与 PRD Acceptance Criteria 一致：
- [ ] `go build .` 不预跑 swag 即成功。
- [ ] `/swagger/index.html` UI + `/swagger/doc.json` spec，公开无鉴权。
- [ ] spec 覆盖全部端点；抽查 3 端点契约与实现一致。
- [ ] `swag init` 可重新生成且与提交产物一致。
- [ ] 现有前后端行为不回归。

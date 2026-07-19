# PRD: AI 运维 skill 与 OpenAPI 契约（PalWorld Server Manager）

## Goal / User Value

在现有前后端之外，为幻兽帕鲁服务器开服工具新增一个**面向外部 AI agent 的运维能力层**，让使用者脱离 Web 前端、用自然语言 + 自己的 agent（OpenClaw / Hermes / Claude 等）完全控制本工具，并获得服务器健康诊断、性能优化规划、自动化运维等更智能的管理体验。

本期通过两件事达成：(1) 后端集成 OpenAPI/Swagger，提供与代码同源、可运行时拉取的 API 契约；(2) 交付一份 Anthropic Agent Skill 格式的 `palworld-ops` skill，承载鉴权、运维 playbook 与安全护栏，精确契约指向 live spec。**智能全部由外部 agent 承担，本工具零模型依赖。**

## Confirmed Facts (from code inspection)

- 单二进制架构：Go 1.26 + Gin，前端静态导出内嵌（`//go:embed all:ui/out`），入口 `main.go` → `internal/server/server.go` → `internal/api`。
- HTTP 监听 `config.Host:config.Port`，默认 `127.0.0.1:8080`（`internal/config/config.go`）。
- 鉴权：JWT 中间件 `auth.Middleware`（`internal/auth/middleware.go`），token 来自 `Authorization: Bearer` 或 `?token=`。登录 `POST /api/auth/login` 传 `{"password":"..."}` → 返回 `{"token":"<jwt>"}`，有效期 7 天，凭密码可刷新（`internal/api/auth_handlers.go`）。
- 能力集中在 `internal/api/router.go`（约 40 个端点），handler 依赖常驻内存态管理器：`process.Manager`、`logger.StreamManager`、`update.Checker`、`saveCache`。
- 现有能力面：服务器 CRUD、install、start/stop/restart、logs(+SSE)、config schema/读写、Palworld REST 代理（status/info/metrics/players/settings/announce/kick/ban/unban/save/shutdown/stop）、存档解析（players/guilds/pals/inventory）、whitelist、全局 Mod 库、每服 Mod 引用、Steam 登录与创意工坊搜索、system/stats、version/update。
- 无 OpenAPI/Swagger，无独立 docs 目录。仓库现有 skill 均为 SKILL.md + frontmatter（Anthropic Agent Skill 格式，位于 `.claude/skills/`）。
- 平台：当前仅 Windows 为准，保留 Linux 代码路径。

## Requirements

### R0 智能边界
- 所有 LLM 推理/规划/自动化决策由**外部 agent** 承担。本工具**零模型依赖**：不接入模型 API、不管理 key/成本、不做 prompt 工程、前端不加 AI 面板。
- 交付物仅两类：(1) OpenAPI 契约（执行通道说明书）；(2) skill（方法论 + 工具编排知识）。

### R1 执行通道
- **不建 MCP Server**。执行通道 = 外部 agent 直接调用现有 HTTP REST API。
- 面向具备 shell/HTTP 能力的通用 agent host（Claude Code / OpenClaw / Hermes）；不为 MCP-only host 适配。MCP 层留作后续可选增强，不在本期。

### R2 OpenAPI/Swagger（子任务 A）
- 后端集成 **swaggo**（`github.com/swaggo/swag` + `gin-swagger` + `swaggo/files`）：在 handler 上写注解，`swag init` 生成 spec。
- 对外提供 **Swagger UI + JSON spec**（`/swagger/index.html` 与 `/swagger/doc.json`），**公开、不加鉴权**（只描述形状、无机密）。
- 覆盖 `router.go` 全部约 40 个端点的注解（路径/方法/参数/请求体/响应/鉴权标注）。
- 更新 `CLAUDE.md` 构建说明：新增 `swag init` 步骤与依赖。

### R3 skill 交付物（子任务 B）
- **Anthropic Agent Skill 格式**：`skills/palworld-ops/SKILL.md`（frontmatter: name/description + 正文）+ 渐进式披露附属文件（如 `reference.md` 速查、`playbooks.md`）。
- 目录置于项目根 `skills/palworld-ops/`（面向分发，独立于 `.claude/skills/`）。
- skill 内容语言：**英文正文**（可移植、模型理解最佳），关键术语可中英并存。
- 内容包含：
  - **接入与鉴权**：base URL、登录换 token、Bearer 头、401 重登；指引 agent 运行时拉 `/swagger/doc.json` 获取精确契约。
  - **安全护栏**（见 R5）。
  - **常用端点速查**（薄，指向 live spec 求精确契约）。
  - **七个运维 playbook**（见 R6）。
- 依赖子任务 A 的 spec 产物（依赖写入子任务 artifact）。

### R4 鉴权
- agent 复用现有登录，**零后端改动**：base URL + admin 密码 → `POST /api/auth/login` 换 token → `Authorization: Bearer`；401 凭密码自动重登。
- 不新增长期 API token（默认 127.0.0.1 绑定 + agent 已持密码）。

### R5 安全护栏（三级分档，由 skill 承载并要求 agent 遵守）
- **只读/诊断**（list/get/logs/stats/metrics/存档解析/workshop 搜索/config 读/version&update check/spec）：agent 可自由执行。
- **常规变更**（create server、config 写、mod link·toggle·deploy、announce、save、whitelist add）：先说明改动点再执行。
- **危险·不可逆**（stop/restart/shutdown 影响在线玩家、kick/ban/unban、delete server、delete mod、system/update/apply 替换二进制并重启）：**必须先向用户明确确认并说明影响面**（如"将断开 N 名在线玩家"）。

### R6 运维 playbook（skill 内置七项）
1. 健康体检/诊断：综合 system/stats + REST metrics/players + 最近日志给出状态诊断。
2. 性能优化规划：读 config schema + 当前设置 + system/stats，给出 PalWorldSettings 调优建议与权衡。
3. 引导式开服：create→install→配置→Mod 部署→start 全流程编排。
4. 玩家管理：列玩家、公告、踢/封/解封、白名单（危险操作走三级确认）。
5. 日志故障排查：基于日志+状态定位启动失败/崩溃/Mod 冲突等。
6. Mod 管理工作流：搜索创意工坊→加入全局库→链接到服→启用→部署。
7. 自动化运维模式：定时存档/公告/重启等由 agent host 调度器驱动，skill 只给命令编排与模式（本工具不做调度）。

## Acceptance Criteria

### 子任务 A（Swagger）
- [ ] `go build .` 成功，且构建流程（含 `swag init`）在 `CLAUDE.md` 有记载并可复现。
- [ ] 运行后 `GET /swagger/index.html` 可打开 UI；`GET /swagger/doc.json` 返回合法 OpenAPI JSON，无需鉴权。
- [ ] spec 覆盖 `router.go` 全部端点，含路径/方法/参数/请求体/响应与鉴权标注；抽查 3 个代表端点（如 `POST /api/servers/:id/start`、`GET /api/system/stats`、`POST /api/auth/login`）契约与实现一致。
- [ ] 现有前端与 API 行为不回归。

### 子任务 B（skill）
- [ ] `skills/palworld-ops/SKILL.md` 存在且 frontmatter 合法（name/description）。
- [ ] skill 覆盖：接入鉴权流程、三级安全护栏、指向 live spec 的说明、七个 playbook。
- [ ] 每个危险操作在 skill 中都标注"需用户确认 + 影响面说明"。
- [ ] 端到端可用性验证：按 skill 指引，agent 能完成"登录 → 拉 spec → 列服务器 → 读一次 system/stats → 产出一次健康诊断"闭环（用真实运行实例或 curl 逐步走通）。

## Out of Scope

- MCP Server / MCP 传输层（后续可选任务）。
- 工具内置 LLM 调用、AI 前端面板、模型密钥/成本管理。
- 长期 API token、新鉴权机制。
- agent host 侧的定时调度实现（由使用者的 agent 平台承担）。
- Linux 专门验证（Windows 为准，保留 Linux 代码路径不破坏）。

## 任务结构
- 父任务：`07-19-mcp-server-ai-ops`，持有需求集、跨子任务验收、集成审查。
- 子任务 A：后端 OpenAPI/Swagger 集成。
- 子任务 B：`skills/palworld-ops/` skill（依赖 A）。

# PRD: palworld-ops 运维 skill（子任务 B）

> 父任务：`07-19-mcp-server-ai-ops`。依赖子任务 A（Swagger）。

## Goal

交付一份 **Anthropic Agent Skill 格式**的 `palworld-ops` skill，供外部 agent（Claude / OpenClaw / Hermes 等）加载后，用自然语言控制 PalWorld Server Manager 工具，并提供七个运维 playbook（健康诊断、性能优化、引导开服、玩家管理、故障排查、Mod 管理、自动化运维模式）。**智能全部由外部 agent 承担，skill 只承载鉴权、安全护栏、方法论与工具编排知识。**

## Confirmed Facts

- 仓库现有 skill 格式：`SKILL.md`（YAML frontmatter: name/description + markdown 正文），位于 `.claude/skills/`，均为英文。
- 子任务 A 将提供 `/swagger/doc.json` spec（公开、可运行时拉取），作为精确 API 契约来源。
- 现有能力面（来自父 PRD）：服务器 CRUD/install/启停、logs(+SSE)、config、Palworld REST 代理、存档解析、whitelist、Mod、Steam/创意工坊、system/stats、version/update。
- 三级安全护栏（来自父 R5）：只读自由、常规变更先说明、危险操作必须确认+说明影响面。
- 七个 playbook（来自父 R6）：健康体检、性能优化规划、引导式开服、玩家管理、日志故障排查、Mod 管理工作流、自动化运维模式。

## Requirements

### R3.1 文件结构
- 目录：`skills/palworld-ops/`（项目根，面向分发，独立于 `.claude/skills/`）。
- 主文件：`SKILL.md`（frontmatter + 正文），英文为主（关键术语可中英并存，如"PalWorldSettings.ini"）。
- 附属文件（渐进式披露，可选）：
  - `reference.md`：常用端点速查表（薄，指向 live spec）。
  - `playbooks.md`：七个 playbook 详细步骤（若 SKILL.md 过长可拆出）。

### R3.2 frontmatter
```yaml
---
name: palworld-ops
description: AI-powered operations skill for PalWorld Server Manager. Provides authentication, safety guardrails, and seven operational playbooks (health check, performance tuning, guided setup, player management, troubleshooting, mod workflow, automation patterns).
---
```

### R3.3 SKILL.md 内容结构（正文）

#### § 接入与鉴权
- base URL 配置（默认 `http://127.0.0.1:8080`，agent 从用户获取）。
- admin 密码配置（agent 安全存储）。
- 登录流程：`POST /api/auth/login {"password":"..."}` → `{"token":"<jwt>"}` → 后续请求 `Authorization: Bearer <token>`，401 时凭密码自动重登。
- 指引 agent 首次拉取 `GET /swagger/doc.json` 获取完整 API 契约（该端点无需鉴权）。

#### § 安全护栏（三级分档，agent 必须遵守）
列明三级操作分类（来自父 R5），对每个危险操作要求：
1. 先向用户描述将执行的操作。
2. 说明影响面（如"将断开 3 名在线玩家"、"将替换运行中的二进制并重启进程"）。
3. 等待用户明确确认后再执行。

#### § 常用端点速查（薄，核心几个端点的 path/method/用途）
- 列服务器：`GET /api/servers`
- 创建服务器：`POST /api/servers`
- 启动/停止/重启：`POST /api/servers/{id}/start|stop|restart`
- 读日志：`GET /api/servers/{id}/logs`，SSE 流：`GET /api/servers/{id}/logs/stream`
- 系统监控：`GET /api/system/stats`
- 在线玩家：`GET /api/servers/{id}/rest/players`
- 配置读写：`GET|PUT /api/servers/{id}/config`
- 更多端点详见 `/swagger/doc.json`（指向 live spec，不在 skill 内手抄完整 API 表）。

#### § 七个运维 playbook
每个 playbook 一个小节（或拆到 `playbooks.md`）：

1. **健康体检/诊断**：综合 `GET /api/system/stats`（系统资源）、`GET /api/servers/{id}/rest/metrics`（游戏指标）、`GET /api/servers/{id}/rest/players`（在线玩家）、`GET /api/servers/{id}/logs?limit=100`（最近日志），给出状态诊断（正常/警告/异常）与建议。

2. **性能优化规划**：读 `GET /api/config/schema`（配置项元数据）、`GET /api/servers/{id}/config`（当前配置）、`GET /api/system/stats`（硬件），基于 PalWorldSettings.ini 常见调优维度（MaxPlayers / DayTimeSpeedRate / NightTimeSpeedRate / ExpRate / PalCaptureRate / DropItemMaxNum 等）给出建议与权衡（如提高 ExpRate 降低难度，减小 DropItemMaxNum 省资源）。

3. **引导式开服**：端到端流程编排：
   - `POST /api/servers {"name":"..."}` 创建服务器记录。
   - `POST /api/servers/{id}/install` 通过 SteamCMD 安装游戏文件。
   - `PUT /api/servers/{id}/config` 配置（服务器名称、端口、密码、难度等）。
   - （可选）Mod 部署（见 playbook 6）。
   - `POST /api/servers/{id}/start` 启动。
   - 轮询 `GET /api/servers/{id}/rest/status` 确认就绪。

4. **玩家管理**：
   - 列玩家：`GET /api/servers/{id}/rest/players`。
   - 公告：`POST /api/servers/{id}/rest/announce {"message":"..."}` **常规变更，先说明**。
   - 踢人：`POST /api/servers/{id}/rest/kick {"steamid":"..."}` **危险操作，需确认+说明影响（踢出该玩家）**。
   - 封禁/解封：`POST /api/servers/{id}/rest/ban|unban {"steamid":"..."}` **危险操作，需确认**。
   - 白名单：`GET /api/servers/{id}/whitelist`、`POST /api/servers/{id}/whitelist {"steamid":"..."}` **常规变更**。

5. **日志故障排查**：`GET /api/servers/{id}/logs?limit=500` 读最近日志，基于常见模式定位问题：
   - 启动失败：搜索 "failed to bind" / "port already in use" → 端口冲突。
   - 崩溃：搜索 "fatal error" / "exception" / "crash" → 提取堆栈或错误码。
   - Mod 冲突：搜索 "mod" / "plugin" / "load failed" → 识别冲突 Mod。
   - 给出诊断结论与建议（改端口/禁用 Mod/检查配置）。

6. **Mod 管理工作流**：
   - 搜索创意工坊：`GET /api/steam/workshop/search?query=...`（需 Steam 登录，见下）。
   - Steam 登录（如需认证 workshop 下载）：`POST /api/steam/login {"username":"...","password":"..."}` **常规变更，先说明**。
   - 加入全局库：`POST /api/mods {"workshopId":"...","name":"..."}`。
   - 下载：`POST /api/mods/{modId}/download`，SSE 流进度 `GET /api/mods/{modId}/logs/stream`。
   - 链接到服：`POST /api/servers/{id}/mods {"modId":"..."}`。
   - 启用：`PUT /api/servers/{id}/mods/{serverModId}/toggle {"enabled":true}` **常规变更**。
   - 部署（复制到游戏目录）：`POST /api/servers/{id}/mods/deploy` **常规变更，先说明**。

7. **自动化运维模式**：定时任务模式（由 agent host 的调度器驱动，如 cron / Windows Task Scheduler / agent 内置 scheduler；本工具不提供调度）。skill 给出命令编排示例：
   - 定时存档：每小时 `POST /api/servers/{id}/rest/save` **常规变更**。
   - 定时公告：每 2 小时 `POST /api/servers/{id}/rest/announce {"message":"服务器将在 XX:00 重启"}` **常规变更**。
   - 定时重启（低峰维护窗口）：`POST /api/servers/{id}/restart` **危险操作，需确认**；agent 负责在执行前读 `GET /api/servers/{id}/rest/players` 确认影响面。
   - 监控与告警：定时拉 `GET /api/system/stats` + `GET /api/servers/{id}/rest/metrics`，CPU > 90% / 内存不足 / 无在线玩家超 1 小时则通知用户。

### R3.4 依赖声明
skill 内需明确：本 skill 依赖 PalWorld Server Manager **已运行**并可访问（base URL）；API 契约来自 `/swagger/doc.json`，首次使用时 agent 应拉取该 spec 以获得精确端点定义。

## Acceptance Criteria

- [ ] `skills/palworld-ops/SKILL.md` 存在，frontmatter 合法（name/description）。
- [ ] skill 覆盖：接入鉴权流程、三级安全护栏（含危险操作列举）、常用端点速查（指向 live spec）、七个 playbook。
- [ ] 每个危险操作在 playbook 中标注"需用户确认 + 影响面说明"。
- [ ] 端到端可用性验证（手动或 agent 实测）：
  1. 按 skill 指引，agent 能完成登录 → 拉 spec → 列服务器 → 读 system/stats → 产出健康诊断报告的闭环。
  2. 抽查一个常规变更（如创建服务器）与一个危险操作（如重启服务器）的护栏流程：agent 是否在危险操作前向用户确认并说明影响。
- [ ] skill 语言为英文（关键术语可中英并存）；playbook 步骤清晰可执行。

## Out of Scope

- agent host 侧的定时调度实现（由使用者的 agent 平台承担，skill 只给模式）。
- 工具内置 LLM 调用、AI 前端面板。
- 中文版 skill（本期英文为主；后续可基于本期产物翻译）。
- 复杂的 Palworld 游戏机制知识库（如 Pal 配种公式、据点优化）——playbook 聚焦**服务器运维**，游戏玩法由 agent 的通识承担。

## 依赖
- **上游**：子任务 A（Swagger），提供 `/swagger/doc.json` spec；B 在 A 完成后开始编写，可与 A 并行但最终验收需 A 的 spec 产物。
- **无下游**：B 是最后一个子任务。

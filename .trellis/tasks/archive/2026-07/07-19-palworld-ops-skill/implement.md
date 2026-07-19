# Implementation Plan: palworld-ops 运维 skill（子任务 B）

## 前置条件

- 子任务 A（Swagger）已完成或至少 `swag init` 可生成 spec，以便参考端点列表编写 skill。
- 最终端到端验收需子任务 A 完全就绪（`/swagger/doc.json` 可访问）。

## 执行顺序

### 阶段 1：目录与骨架
- [ ] 1.1 创建 `skills/palworld-ops/` 目录（项目根）。
- [ ] 1.2 创建 `skills/palworld-ops/SKILL.md`，写入 frontmatter：
  ```yaml
  ---
  name: palworld-ops
  description: AI-powered operations skill for PalWorld Server Manager. Provides authentication, safety guardrails, and seven operational playbooks (health check, performance tuning, guided setup, player management, troubleshooting, mod workflow, automation patterns).
  ---
  ```
- [ ] 1.3 在正文添加章节骨架（一级标题）：
  ```markdown
  # PalWorld Server Manager Operations Skill
  
  ## Overview
  
  ## Setup & Authentication
  
  ## Safety Guardrails
  
  ## Quick Reference
  
  ## Playbooks
  
  ### 1. Health Check & Diagnostics
  ### 2. Performance Optimization Planning
  ### 3. Guided Server Setup
  ### 4. Player Management
  ### 5. Log-based Troubleshooting
  ### 6. Mod Management Workflow
  ### 7. Automation Patterns
  
  ## Appendix
  ```

### 阶段 2：核心章节（Setup / Guardrails / Quick Reference）
- [ ] 2.1 **Setup & Authentication**：
  - base URL 配置说明（默认 `http://127.0.0.1:8080`）。
  - admin 密码存储建议（agent 安全存储）。
  - 登录流程：`POST /api/auth/login` → token → `Authorization: Bearer` → 401 重登。
  - 首次拉取 `GET /swagger/doc.json` 的指引。
- [ ] 2.2 **Safety Guardrails**：
  - 三级分档说明（只读/常规/危险）。
  - 危险操作清单（9 项，每项标 endpoint + 影响面模板 + 确认要求）：
    - `POST /servers/{id}/stop|restart` / `rest/shutdown|stop` → "will disconnect N online players"
    - `POST /servers/{id}/rest/kick|ban|unban` → "will kick/ban player {steamid}"
    - `DELETE /servers/{id}` → "will delete server record (may include game files)"
    - `DELETE /mods/{modId}` → "will delete mod from global library"
    - `POST /system/update/apply` → "will replace binary and restart process"
  - 强调：agent MUST confirm with user before executing any dangerous operation.
- [ ] 2.3 **Quick Reference**（10 个核心端点，path/method/brief purpose）：
  - `GET /api/servers` - List all servers
  - `POST /api/servers` - Create server
  - `POST /api/servers/{id}/start|stop|restart` - Lifecycle
  - `GET /api/servers/{id}/logs` - Recent logs
  - `GET /api/servers/{id}/logs/stream` - SSE log stream
  - `GET /api/system/stats` - System monitoring
  - `GET /api/servers/{id}/rest/players` - Online players
  - `GET|PUT /api/servers/{id}/config` - Configuration
  - `POST /api/servers/{id}/rest/announce` - Broadcast message
  - 末尾注明："For full API reference, fetch `/swagger/doc.json` at runtime."

### 阶段 3：七个 Playbook（逐个填充）
每个 playbook 按 design 原则：Goal / Steps / Safety Note / Example（可选）。

- [ ] 3.1 **Health Check & Diagnostics**：
  - Goal: 综合诊断服务器状态。
  - Steps:
    1. `GET /api/system/stats` → CPU/memory/disk usage
    2. `GET /api/servers/{id}` → server status (running/stopped)
    3. `GET /api/servers/{id}/rest/metrics` → game metrics (if running)
    4. `GET /api/servers/{id}/rest/players` → online player count
    5. `GET /api/servers/{id}/logs?limit=100` → recent logs, check for errors
    6. Synthesize: Normal / Warning (high CPU/memory) / Error (crash logs)
  - Safety: Read-only, no confirmation needed.

- [ ] 3.2 **Performance Optimization Planning**：
  - Goal: 基于配置与资源给出调优建议。
  - Steps:
    1. `GET /api/config/schema` → available settings
    2. `GET /api/servers/{id}/config` → current PalWorldSettings.ini
    3. `GET /api/system/stats` → hardware capacity
    4. Analyze common tuning dimensions: MaxPlayers, ExpRate, DropItemMaxNum, DayTimeSpeedRate, etc.
    5. Suggest changes with trade-offs (e.g., increase ExpRate → easier progression, reduce DropItemMaxNum → less lag)
  - Safety: Read-only for analysis; if applying changes via `PUT /api/servers/{id}/config`, mark as **regular change** (explain changes before execution).

- [ ] 3.3 **Guided Server Setup**：
  - Goal: 端到端开服流程。
  - Steps:
    1. `POST /api/servers {"name":"..."}` → create server record (**regular**)
    2. `POST /api/servers/{id}/install` → SteamCMD install (**regular, may take minutes**)
    3. `PUT /api/servers/{id}/config` → set name/port/password/difficulty (**regular, explain changes**)
    4. (Optional) Deploy mods (see Playbook 6)
    5. `POST /api/servers/{id}/start` → start server (**regular**)
    6. Poll `GET /api/servers/{id}/rest/status` until ready
  - Safety: All steps are regular or read-only; start may fail if config invalid (check logs).

- [ ] 3.4 **Player Management**：
  - Goal: 玩家列表、公告、踢人/封禁、白名单。
  - Steps:
    1. `GET /api/servers/{id}/rest/players` → list online players (**read-only**)
    2. `POST /api/servers/{id}/rest/announce {"message":"..."}` → broadcast (**regular, explain message**)
    3. `POST /api/servers/{id}/rest/kick {"steamid":"..."}` → kick player (**DANGEROUS, confirm + "will disconnect player {name}"**)
    4. `POST /api/servers/{id}/rest/ban|unban {"steamid":"..."}` → ban/unban (**DANGEROUS, confirm**)
    5. `GET /api/servers/{id}/whitelist` → list whitelist (**read-only**)
    6. `POST /api/servers/{id}/whitelist {"steamid":"..."}` → add to whitelist (**regular**)
  - Safety: Kick/ban/unban require user confirmation.

- [ ] 3.5 **Log-based Troubleshooting**：
  - Goal: 基于日志定位启动失败/崩溃/Mod 冲突。
  - Steps:
    1. `GET /api/servers/{id}/logs?limit=500` → recent logs (**read-only**)
    2. Pattern matching:
       - "failed to bind" / "port already in use" → port conflict
       - "fatal error" / "exception" / "crash" → extract stack trace
       - "mod" / "plugin" / "load failed" → identify conflicting mod
    3. Synthesize diagnosis + recommendation (change port / disable mod / check config)
  - Safety: Read-only analysis.

- [ ] 3.6 **Mod Management Workflow**：
  - Goal: 搜索/下载/部署 Mod 完整流程。
  - Steps:
    1. (Optional) Steam login: `POST /api/steam/login {"username":"...","password":"..."}` (**regular, explain reason: authenticated workshop access**)
    2. `GET /api/steam/workshop/search?query=...` → search mods (**read-only**)
    3. `POST /api/mods {"workshopId":"...","name":"..."}` → add to global library (**regular**)
    4. `POST /api/mods/{modId}/download` → download mod (**regular, may take time**)
       - Monitor progress: `GET /api/mods/{modId}/logs/stream` (SSE)
    5. `POST /api/servers/{id}/mods {"modId":"..."}` → link mod to server (**regular**)
    6. `PUT /api/servers/{id}/mods/{serverModId}/toggle {"enabled":true}` → enable mod (**regular**)
    7. `POST /api/servers/{id}/mods/deploy` → deploy mods to game directory (**regular, explain which mods will be deployed**)
  - Safety: All steps are regular changes (explain before execution); no dangerous operations unless deleting mod (`DELETE /api/mods/{modId}` → **DANGEROUS**).

- [ ] 3.7 **Automation Patterns**：
  - Goal: 定时运维模式（由 agent host 调度驱动）。
  - Patterns:
    - **Auto-save**: Hourly `POST /api/servers/{id}/rest/save` (**regular**)
    - **Scheduled announcements**: Every 2h `POST /api/servers/{id}/rest/announce` (**regular**)
    - **Maintenance restart**: Off-peak `POST /api/servers/{id}/restart` (**DANGEROUS**):
      1. Before execution, fetch `GET /api/servers/{id}/rest/players` → count online players
      2. Confirm with user: "will disconnect N players"
      3. Execute restart
    - **Monitoring & alerts**: Periodic `GET /api/system/stats` + `/rest/metrics`:
      - If CPU > 90% / memory < 10% / no players for 1h → notify user
  - Note: The tool does NOT provide built-in scheduling. Agents should use their host's scheduler (cron / Windows Task Scheduler / agent-internal timer).
  - Safety: Restart requires confirmation; others are regular (explain before execution).

### 阶段 4：收尾与验收
- [ ] 4.1 **Overview** 节：简述 skill 用途、覆盖的七个 playbook、依赖（需工具已运行并可访问）。
- [ ] 4.2 **Appendix** 节：常见问题（如"如何处理 401"、"SSE 流式端点怎么消费"、"Steam 登录失败怎么办"）。
- [ ] 4.3 通读全文，确保：
  - 每个危险操作都在 Safety Guardrails 与对应 playbook 中标注。
  - Quick Reference 指向 `/swagger/doc.json`。
  - 英文为主，关键术语中英并存。
- [ ] 4.4 端到端可用性测试（需子任务 A 完成）：
  - 用真实运行实例或 curl 模拟 agent 行为：
    1. 登录 → 拉 `/swagger/doc.json` → 列服务器 → 读 system/stats → 产出健康诊断。
    2. 抽查护栏：尝试"重启服务器"流程，验证 agent 是否会在执行前向用户确认。
  - 如无真实 agent 可测，手动按 skill 步骤 curl 一遍，验证流程可行。
- [ ] 4.5 （可选）若 `SKILL.md` > 800 行，考虑拆 `playbooks.md`；否则保持单文件。

## 风险缓解

- **风险**：playbook 步骤与实际 API 行为不符。**缓解**：阶段 3 每写完一个 playbook 后，对照 `router.go` 或已生成的 swagger spec 核对端点 path/method；阶段 4.4 端到端测试覆盖至少两个 playbook（健康体检 + 一个变更类）。
- **风险**：危险操作遗漏。**缓解**：阶段 2.2 完成后，与父 PRD R5 列表逐项对照；阶段 4.3 通读时再次检查。
- **风险**：语言风格不一致。**缓解**：统一用英文祈使句（"Fetch ..."、"Confirm with user ..."）；阶段 4.3 通读时修正。

## 验收清单（再次确认）

与 PRD Acceptance Criteria 一致：
- [ ] `skills/palworld-ops/SKILL.md` 存在，frontmatter 合法。
- [ ] skill 覆盖：接入鉴权、三级护栏（含危险操作清单）、常用端点速查（指向 live spec）、七个 playbook。
- [ ] 每个危险操作在 playbook 中标注"需确认+影响面"。
- [ ] 端到端可用性验证：登录→拉 spec→列服务器→读 stats→诊断闭环；抽查护栏（重启服务器需确认）。

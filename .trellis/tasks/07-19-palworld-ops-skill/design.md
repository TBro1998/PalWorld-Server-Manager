# Design: palworld-ops 运维 skill（子任务 B）

## 格式与结构

采用 **Anthropic Agent Skill** 标准格式（参考仓库现有 `.claude/skills/*/SKILL.md`）：
- YAML frontmatter（`name` / `description`）
- Markdown 正文，分节组织
- 附属文件（可选）：`reference.md`、`playbooks.md`

skill 作为"知识文档"被 agent 加载后，agent 根据其指引执行 HTTP 调用 + 遵守护栏 + 应用 playbook 方法论。

## 内容分层（渐进式披露）

### 核心层（SKILL.md 必含）
1. **Setup（接入与鉴权）**：如何连接工具、如何登录换 token、如何处理 401。
2. **Safety Guardrails（安全护栏）**：三级分档，危险操作列表 + 确认要求。
3. **Quick Reference（速查）**：5-10 个最常用端点的 path/method/用途，指向 `/swagger/doc.json` 求精确契约。
4. **Playbooks（运维方法论）**：七个 playbook 的概要（目标 + 关键步骤 + 涉及端点）。

### 扩展层（可拆到附属文件）
- `reference.md`：完整常用端点表（若 SKILL.md 速查不够用）。
- `playbooks.md`：每个 playbook 的详细步骤、示例命令、常见坑。

**本期选择**：优先单文件 `SKILL.md` 包含全部内容（预计 < 800 行，可控）；若实际过长再拆 `playbooks.md`。

## 语言与风格

- **主语言：英文**（agent 模型对英文理解最佳，skill 可移植性最强）。
- 关键术语保留原文或中英并存，如：
  - "PalWorldSettings.ini configuration"
  - "服务器 (server)"——首次出现时标注，后续用英文。
- 风格：指令式 + 结构化（用 `###` 分节、bullet list、code block）。

## API 契约分工

- skill **不手抄完整 API 表**（避免与 spec 漂移）。
- Quick Reference 只列核心端点（约 10 个）的 path/method/简要用途。
- 每节开头注明："For precise request/response schemas, agents should fetch `/swagger/doc.json` at runtime."

## playbook 设计原则

每个 playbook 遵循：
1. **Goal**（目标）：一句话说明该 playbook 解决什么问题。
2. **Steps**（步骤）：编号列表，每步一个 API 调用或判断逻辑。
3. **Safety Note**（安全提示）：该流程中哪些步骤属于危险操作、需如何确认。
4. **Example**（示例，可选）：典型场景的伪代码或命令序列。

## 危险操作清单（护栏核心）

需在 Safety Guardrails 节明确列出，每项标明：
- 端点 path/method
- 影响面描述模板（如"will disconnect N online players"）
- 确认要求（"MUST confirm with user before execution"）

清单（来自父 R5）：
- `POST /api/servers/{id}/stop` / `restart` / `rest/shutdown` / `rest/stop` → 影响在线玩家
- `POST /api/servers/{id}/rest/kick` → 踢出指定玩家
- `POST /api/servers/{id}/rest/ban` / `unban` → 封禁/解封玩家
- `DELETE /api/servers/{id}` → 删除服务器记录（需确认是否含游戏文件）
- `DELETE /api/mods/{modId}` → 删除全局 Mod
- `POST /api/system/update/apply` → 替换二进制并重启进程

## 依赖子任务 A 的产物

skill 内多处写 "fetch `/swagger/doc.json`"，该端点由子任务 A 提供。B 的编写可与 A 并行，但最终验收（端到端可用性测试）需在 A 完成、`/swagger/doc.json` 可访问后进行。

## 兼容与维护

- skill 是纯文档，不依赖编译。变更时直接编辑 markdown。
- 若工具 API 变更，只需更新 playbook 中受影响的步骤 + Quick Reference（若有），精确契约仍由 live spec 保证。

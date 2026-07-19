# palworld-ops Skill 安装说明（面向 AI Agent）

> [English](./INSTALL.en.md) | 中文 | [日本語](./INSTALL.ja.md)

> **本文件是写给 AI Agent 的安装指令。**
> 用户只需把这份文档（或它的链接）交给自己的 Agent（Claude Code、Claude Desktop、Codex、OpenClaw、Hermes Agent 等），
> Agent 按下面的步骤执行即可完成安装，用户无需手动操作。

---

## 你的任务

你正在为用户安装 **`palworld-ops`** 技能 —— 一个让你能够通过 REST API 操作
**PalWorld Server Manager（幻兽帕鲁服务器开服工具）** 的 Agent Skill。

安装完成后，你将具备：

- 身份认证流程（JWT）
- 三级风险安全防护
- 七套运维 playbook（健康检查、性能调优、引导建服、玩家管理、故障排查、模组流程、自动化）

技能就是一个包含 `SKILL.md` 的文件夹。安装 = 把该文件夹放进你的技能目录，
**无需编译或注册**，重启会话后你会自动发现它。

---

## 步骤 1 · 确认运行环境

确认你运行在支持 Agent Skills 的环境（Claude Code / Claude Desktop / Codex / OpenClaw / Hermes Agent 或兼容 `SKILL.md`
格式的 Agent 运行时）。若不支持技能机制，请告知用户本技能无法以“技能”形式安装，
但仍可将 `SKILL.md` 作为普通上下文文档手动提供给你。

## 步骤 2 · 确定安装目录

询问用户希望的安装范围，默认**个人级**：

| 范围 | 目标目录 | 适用场景 |
|------|----------|----------|
| **个人级**（推荐） | `~/.claude/skills/palworld-ops/` | 在任何项目中都能用 |
| **项目级** | `<当前项目根>/.claude/skills/palworld-ops/` | 仅本仓库，可随 git 分享给协作者 |

最终必须形成结构：`<技能目录>/palworld-ops/SKILL.md`。

> Windows 上 `~` 通常为 `C:\Users\<用户名>`，`~/.claude/skills` 即
> `C:\Users\<用户名>\.claude\skills`。

## 步骤 3 · 获取技能文件

根据你能访问到的来源，二选一：

### 情况 A：你已在本仓库的工作目录中

直接从仓库内复制整个技能文件夹（源路径 `skills/palworld-ops/`）：

```bash
# 个人级
mkdir -p ~/.claude/skills
cp -r skills/palworld-ops ~/.claude/skills/

# 或项目级
mkdir -p .claude/skills
cp -r skills/palworld-ops .claude/skills/
```

### 情况 B：你没有本地仓库，需要从 GitHub 拉取

拉取 `SKILL.md`（安装运行时只需要它；`INSTALL.md` 本身不必安装）：

```bash
# 个人级
mkdir -p ~/.claude/skills/palworld-ops
curl -fsSL \
  https://raw.githubusercontent.com/TBro1998/PalWorld-Server-Manager/main/skills/palworld-ops/SKILL.md \
  -o ~/.claude/skills/palworld-ops/SKILL.md
```

若 `curl` 不可用，也可用你自带的网络抓取能力下载同一 URL，写入相同路径。

## 步骤 4 · 验证安装

1. 确认文件存在：`<技能目录>/palworld-ops/SKILL.md`。
2. 确认 `SKILL.md` 顶部含 frontmatter：`name: palworld-ops` 与 `description:`。
3. 提示用户**重启 Agent 会话**（或重新加载技能），使新技能被扫描到。
4. 重启后，用户输入 `/` 应能看到 `palworld-ops`；或在提出帕鲁服务器相关请求时，
   你应能自动触发该技能。

## 步骤 5 · 告知用户使用前提

安装成功后，提醒用户：使用本技能前需满足以下条件（详见 `SKILL.md`）：

- **PalWorld Server Manager 正在运行且可访问**（默认 `http://127.0.0.1:8080`；
  远程/Docker 部署为 `http://<主机IP>:8080`）。
- 用户需提供**管理员密码**，你将用它调用 `POST /api/auth/login` 换取 JWT token。
- 首次使用可先调 `GET /api/auth/status` 判断系统是否已初始化，未初始化则用
  `POST /api/auth/setup` 设置初始密码。
- 需要精确 API schema 时，运行时抓取公开的 `GET /swagger/doc.json`。

---

## 更新已安装的技能

当工具升级、接口或 `SKILL.md` 内容更新后，请区分两种情况：

**1. 仅接口（API schema）变化 —— 通常无需更新技能。**
`SKILL.md` 要求你在运行时抓取 `/swagger/doc.json` 获取精确 schema，因此工具升级、
Swagger 文档更新后，你下次调用拿到的即是最新接口，用户无需任何操作。

**2. `SKILL.md` 内容变化（新增/修改 playbook、快速参考、安全分级规则）—— 需要覆盖更新。**
这些内容写在 `SKILL.md` 里，用户安装的是当时那份文件的副本，源更新后副本不会自动跟随。
用最新文件覆盖旧文件即可：

```bash
# 情况 A：已克隆本仓库 —— 先更新仓库再覆盖
git pull
cp -r skills/palworld-ops ~/.claude/skills/        # 或 .claude/skills/（项目级）

# 情况 B：从 GitHub 覆盖已安装的 SKILL.md
curl -fsSL \
  https://raw.githubusercontent.com/TBro1998/PalWorld-Server-Manager/main/skills/palworld-ops/SKILL.md \
  -o ~/.claude/skills/palworld-ops/SKILL.md
```

覆盖后，提示用户**重启 Agent 会话**，使更新后的技能内容被重新加载。

---

**安装到此结束。** 后续的认证、安全分级与 playbook 均以 `SKILL.md` 为准。

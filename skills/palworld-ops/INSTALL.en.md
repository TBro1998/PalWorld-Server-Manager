# palworld-ops Skill Installation Guide (for AI Agents)

> English | [中文](./INSTALL.zh.md) | [日本語](./INSTALL.ja.md)

> **This document is a set of installation instructions written for an AI Agent.**
> A user just needs to hand this document (or its link) to their agent (Claude Code,
> Claude Desktop, etc.). The agent follows the steps below to complete the install —
> no manual work required from the user.

---

## Your task

You are installing the **`palworld-ops`** skill for the user — an Agent Skill that
lets you operate **PalWorld Server Manager** through its REST API.

Once installed, you will have:

- An authentication flow (JWT)
- Three-tier risk safety guardrails
- Seven operational playbooks (health check, performance tuning, guided setup,
  player management, troubleshooting, mod workflow, automation)

A skill is simply a folder containing a `SKILL.md`. Installing = placing that folder
into your skills directory. **No compilation or registration is needed** — you will
discover it automatically after restarting the session.

---

## Step 1 · Confirm the runtime

Confirm you are running in an environment that supports Agent Skills (Claude Code /
Claude Desktop, or an agent runtime compatible with the `SKILL.md` format). If the
skill mechanism is not supported, tell the user this skill cannot be installed as a
"skill", but `SKILL.md` can still be supplied to you manually as a plain context
document.

## Step 2 · Decide the install directory

Ask the user for the preferred scope; default to **personal**:

| Scope | Target directory | When to use |
|-------|------------------|-------------|
| **Personal** (recommended) | `~/.claude/skills/palworld-ops/` | Usable in any project |
| **Project** | `<project root>/.claude/skills/palworld-ops/` | This repo only; shareable via git |

The final structure must be: `<skills dir>/palworld-ops/SKILL.md`.

> On Windows, `~` is usually `C:\Users\<username>`, so `~/.claude/skills` is
> `C:\Users\<username>\.claude\skills`.

## Step 3 · Obtain the skill file

Pick one based on the source you can access:

### Case A: You are already in this repo's working directory

Copy the entire skill folder from the repo (source path `skills/palworld-ops/`):

```bash
# Personal
mkdir -p ~/.claude/skills
cp -r skills/palworld-ops ~/.claude/skills/

# Or project
mkdir -p .claude/skills
cp -r skills/palworld-ops .claude/skills/
```

### Case B: You have no local repo and need to pull from GitHub

Fetch `SKILL.md` (only this file is needed at runtime; `INSTALL.md` itself does not
need to be installed):

```bash
# Personal
mkdir -p ~/.claude/skills/palworld-ops
curl -fsSL \
  https://raw.githubusercontent.com/TBro1998/PalWorld-Server-Manager/main/skills/palworld-ops/SKILL.md \
  -o ~/.claude/skills/palworld-ops/SKILL.md
```

If `curl` is unavailable, use your own web-fetch capability to download the same URL
and write it to the same path.

## Step 4 · Verify the install

1. Confirm the file exists: `<skills dir>/palworld-ops/SKILL.md`.
2. Confirm `SKILL.md` has frontmatter at the top: `name: palworld-ops` and
   `description:`.
3. Tell the user to **restart the agent session** (or reload skills) so the new skill
   is scanned.
4. After restart, typing `/` should list `palworld-ops`; or you should be able to
   trigger the skill automatically when the user makes a Palworld-server-related
   request.

## Step 5 · Tell the user the prerequisites

After a successful install, remind the user that using this skill requires (see
`SKILL.md` for details):

- **PalWorld Server Manager is running and reachable** (default
  `http://127.0.0.1:8080`; for remote/Docker deployments, `http://<host-ip>:8080`).
- The user must provide the **admin password**, which you use to call
  `POST /api/auth/login` and obtain a JWT token.
- On first use, call `GET /api/auth/status` to check whether the system is
  initialized; if not, use `POST /api/auth/setup` to set the initial password.
- When you need precise API schemas, fetch the public `GET /swagger/doc.json` at
  runtime.

---

## Updating an installed skill

When the tool is upgraded and the API or `SKILL.md` content changes, distinguish two
cases:

**1. Only the API schema changed — usually no skill update needed.**
`SKILL.md` instructs you to fetch `/swagger/doc.json` at runtime for precise schemas,
so after the tool is upgraded and the Swagger docs are updated, your next call already
gets the latest interface. The user does not need to do anything.

**2. `SKILL.md` content changed (new/modified playbooks, quick reference, risk tiers)
— an overwrite is needed.**
This content lives inside `SKILL.md`, and the user installed a copy of that file as it
was at install time; updating the source does not update the copy. Overwrite the old
file with the latest one:

```bash
# Case A: repo already cloned — update the repo, then overwrite
git pull
cp -r skills/palworld-ops ~/.claude/skills/        # or .claude/skills/ (project scope)

# Case B: overwrite the installed SKILL.md from GitHub
curl -fsSL \
  https://raw.githubusercontent.com/TBro1998/PalWorld-Server-Manager/main/skills/palworld-ops/SKILL.md \
  -o ~/.claude/skills/palworld-ops/SKILL.md
```

After overwriting, tell the user to **restart the agent session** so the updated skill
content is reloaded.

---

**Installation ends here.** For authentication, risk tiers, and playbooks, `SKILL.md`
is the source of truth.

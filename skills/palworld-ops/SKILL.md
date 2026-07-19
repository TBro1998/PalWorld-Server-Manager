---
name: palworld-ops
description: AI-powered operations skill for PalWorld Server Manager. Provides authentication, safety guardrails, and seven operational playbooks (health check, performance tuning, guided setup, player management, troubleshooting, mod workflow, automation patterns).
---

# PalWorld Server Manager Operations Skill

## Overview

This skill enables AI agents to operate PalWorld Server Manager through its REST API. It provides:

- **Authentication flow** - How to connect and maintain JWT tokens
- **Safety guardrails** - Three-tier risk classification to prevent destructive actions
- **Quick reference** - Core endpoints for common operations
- **Seven operational playbooks** - Proven workflows for server management

**Requirements:**
- PalWorld Server Manager must be running and accessible
- Agent must have admin password
- For precise API schemas, fetch `/swagger/doc.json` at runtime

## Setup & Authentication

### Base URL Configuration

Default: `http://127.0.0.1:8080`

The agent should prompt the user for the base URL if different, or auto-detect from environment.

### Authentication Flow

1. **Login**: `POST /api/auth/login`
   ```json
   Request: {"password": "<admin-password>"}
   Response: {"token": "<jwt-token>"}
   ```

2. **Use token in all subsequent requests**:
   ```
   Authorization: Bearer <jwt-token>
   ```

3. **Token lifetime**: 7 days. If you receive `401 Unauthorized`, re-login with the password.

4. **First-time setup**: If the system is not configured yet (check `GET /api/auth/status`), use `POST /api/auth/setup` instead of login to set the initial password.

### API Contract

After authenticating, fetch the live OpenAPI specification:

**`GET /swagger/doc.json`** (public, no auth required)

This provides the precise schema for all ~40 endpoints. The Quick Reference below lists the most common ones; for full details, always consult the live spec.

## Safety Guardrails

Agents MUST classify every operation into one of three risk levels and follow the corresponding protocol:

### Level 1: Read-Only / Diagnostic (No Confirmation Needed)

Safe operations that only query state. Execute freely:

- List/get endpoints: `GET /api/servers`, `GET /api/servers/{id}`, `GET /api/mods`
- Logs: `GET /api/servers/{id}/logs`, `GET /api/servers/{id}/logs/stream`
- Monitoring: `GET /api/system/stats`, `GET /api/servers/{id}/rest/metrics`, `GET /api/servers/{id}/rest/players`
- Save parsing: `GET /api/servers/{id}/save/players|guilds|pals|inventory`
- Config read: `GET /api/servers/{id}/config`, `GET /api/config/schema`
- Version check: `GET /api/system/version`, `GET /api/system/update/status`
- Workshop search: `GET /api/steam/workshop/search`
- OpenAPI spec: `GET /swagger/doc.json`

### Level 2: Regular Changes (Explain Before Execution)

Operations that modify state but are generally safe and reversible. Before executing, **briefly explain to the user what will change**:

- Create server: `POST /api/servers`
- Config updates: `PUT /api/servers/{id}/config`
- Mod operations: `POST /api/mods` (add to library), `POST /api/servers/{id}/mods` (link), `PUT /api/servers/{id}/mods/{serverModId}/toggle`, `POST /api/servers/{id}/mods/deploy`
- Announcements: `POST /api/servers/{id}/rest/announce`
- Manual save: `POST /api/servers/{id}/rest/save`
- Whitelist add: `POST /api/servers/{id}/whitelist`
- Steam login: `POST /api/steam/login` (explain: "logging in to enable authenticated workshop access")

### Level 3: Dangerous / Irreversible (MUST Confirm + Explain Impact)

Operations that disconnect players, delete data, or replace binaries. **MUST**:
1. Describe what will happen
2. Quantify the impact (e.g., "will disconnect 3 online players")
3. Wait for explicit user confirmation

**Dangerous operations:**

- **Stop/restart server**: `POST /api/servers/{id}/stop|restart`, `POST /api/servers/{id}/rest/shutdown`, `POST /api/servers/{id}/rest/stop`
  - Impact: "Will disconnect N online players" (fetch player count first via `GET /api/servers/{id}/rest/players`)
  
- **Kick/ban player**: `POST /api/servers/{id}/rest/kick|ban|unban`
  - Impact: "Will kick/ban player {name} (Steam ID: {steamid})"
  
- **Delete server**: `DELETE /api/servers/{id}`
  - Impact: "Will delete server record (game files on disk are NOT removed)"
  
- **Delete mod**: `DELETE /api/mods/{modId}`
  - Impact: "Will remove mod from global library; servers linking it will be affected"
  
- **Apply system update**: `POST /api/system/update/apply`
  - Impact: "Will replace the tool binary and restart the process; all running servers will be stopped during restart"

## Quick Reference

Core endpoints for common operations. For full details, fetch `/swagger/doc.json`.

| Operation | Method | Path | Auth |
|-----------|--------|------|------|
| **Auth** |
| Check status | GET | `/api/auth/status` | No |
| First-time setup | POST | `/api/auth/setup` | No |
| Login | POST | `/api/auth/login` | No |
| **Servers** |
| List servers | GET | `/api/servers` | Yes |
| Create server | POST | `/api/servers` | Yes |
| Get server | GET | `/api/servers/{id}` | Yes |
| Install (SteamCMD) | POST | `/api/servers/{id}/install` | Yes |
| Start | POST | `/api/servers/{id}/start` | Yes |
| Stop | POST | `/api/servers/{id}/stop` | Yes |
| Restart | POST | `/api/servers/{id}/restart` | Yes |
| Get logs | GET | `/api/servers/{id}/logs?limit=N` | Yes |
| Stream logs (SSE) | GET | `/api/servers/{id}/logs/stream?token=<jwt>` | Yes |
| **Config** |
| Get config | GET | `/api/servers/{id}/config` | Yes |
| Update config | PUT | `/api/servers/{id}/config` | Yes |
| Get schema | GET | `/api/config/schema` | Yes |
| **Palworld REST Proxy** |
| Online players | GET | `/api/servers/{id}/rest/players` | Yes |
| Game metrics | GET | `/api/servers/{id}/rest/metrics` | Yes |
| Announce | POST | `/api/servers/{id}/rest/announce` | Yes |
| Kick player | POST | `/api/servers/{id}/rest/kick` | Yes |
| Ban player | POST | `/api/servers/{id}/rest/ban` | Yes |
| **System** |
| System stats | GET | `/api/system/stats` | Yes |
| Version info | GET | `/api/system/version` | No |
| Check for updates | GET | `/api/system/update/check` | Yes |

**SSE Authentication**: SSE endpoints (`/logs/stream`, `/steam/logs/stream`, `/mods/{modId}/logs/stream`, `/system/update/stream`) accept JWT via query parameter: `?token=<jwt>`

## Playbooks

### 1. Health Check & Diagnostics

**Goal**: Assess overall server health and identify issues.

**Steps**:

1. `GET /api/system/stats` → Check system resource usage (CPU, memory, disk)
2. `GET /api/servers/{id}` → Check server status (running/stopped/error)
3. If running:
   - `GET /api/servers/{id}/rest/metrics` → Game performance metrics
   - `GET /api/servers/{id}/rest/players` → Online player count
4. `GET /api/servers/{id}/logs?limit=100` → Recent logs, scan for errors
5. **Synthesize diagnosis**:
   - **Normal**: Server running, resources healthy, no errors
   - **Warning**: High CPU (>80%) or low memory (<10%), or few recent warnings in logs
   - **Error**: Server stopped/crashed, or critical errors in logs

**Safety**: Read-only (Level 1).

### 2. Performance Optimization Planning

**Goal**: Analyze current configuration and suggest tuning based on hardware and player count.

**Steps**:

1. `GET /api/config/schema` → Available settings and their descriptions
2. `GET /api/servers/{id}/config` → Current `PalWorldSettings.ini` values
3. `GET /api/system/stats` → Hardware capacity
4. `GET /api/servers/{id}/rest/players` → Current player count (if running)
5. **Analyze common tuning dimensions**:
   - `MaxPlayers`: Reduce if hardware constrained
   - `ExpRate`, `PalCaptureRate`: Increase for easier progression
   - `DayTimeSpeedRate`, `NightTimeSpeedRate`: Adjust day/night cycle speed
   - `DropItemMaxNum`, `DropItemMaxNum_UNKO`: Reduce to save resources
6. **Present recommendations** with trade-offs (e.g., "Increasing ExpRate to 2.0 will speed up leveling but may reduce long-term engagement")

**Safety**: Read-only analysis (Level 1). If applying changes via `PUT /api/servers/{id}/config`, becomes Level 2 (explain changes first).

### 3. Guided Server Setup

**Goal**: Complete end-to-end setup of a new Palworld server.

**Steps**:

1. `POST /api/servers {"name": "MyServer"}` → Create server record (auto-assigns ports, generates admin password)
2. `POST /api/servers/{id}/install` → Download game files via SteamCMD (may take 5-10 minutes; poll status or monitor logs)
3. `PUT /api/servers/{id}/config` → Configure server:
   - Set `ServerName`, `ServerDescription`
   - Set `PublicPort`, `PublicIP` (if port forwarding)
   - Set `ServerPassword` (if private)
   - Adjust difficulty, rates, etc. (see Playbook 2)
4. (Optional) Deploy mods (see Playbook 6)
5. `POST /api/servers/{id}/start` → Start the server
6. Poll `GET /api/servers/{id}/rest/status` until `reachable: true` (may take 30-60s)
7. Provide connection info to user: `<PublicIP>:<PublicPort>`

**Safety**: Steps 1-3, 5 are Level 2 (explain each action). Step 6 is Level 1 (read-only).

### 4. Player Management

**Goal**: View, announce to, and moderate players.

**Steps**:

1. **List online players**: `GET /api/servers/{id}/rest/players` (Level 1)
   - Returns array of `{name, playerId, userId, ip, ping, locationX, locationY, level}`

2. **Broadcast announcement**: `POST /api/servers/{id}/rest/announce {"message": "..."}` (Level 2)
   - Explain: "Will broadcast message to all online players"

3. **Kick player**: `POST /api/servers/{id}/rest/kick {"steamid": "..."}` (Level 3)
   - Fetch player name first
   - Confirm: "Will disconnect player {name} (Steam ID: {steamid})"

4. **Ban player**: `POST /api/servers/{id}/rest/ban {"steamid": "..."}` (Level 3)
   - Confirm: "Will ban player {name}; they cannot rejoin until unbanned"

5. **Unban player**: `POST /api/servers/{id}/rest/unban {"steamid": "..."}` (Level 3)
   - Confirm: "Will remove ban for Steam ID {steamid}"

6. **Whitelist management**:
   - View: `GET /api/servers/{id}/whitelist` (Level 1)
   - Add: `POST /api/servers/{id}/whitelist {"steamid": "..."}` (Level 2, explain)
   - Remove: `DELETE /api/servers/{id}/whitelist?steamid=...` (Level 2, explain)

### 5. Log-Based Troubleshooting

**Goal**: Diagnose startup failures, crashes, or mod conflicts from logs.

**Steps**:

1. `GET /api/servers/{id}/logs?limit=500` → Fetch recent logs (Level 1)

2. **Pattern matching** (common issues):
   - **Port conflict**: Search for `"failed to bind"`, `"port already in use"`, `"address already in use"`
     - Diagnosis: Port conflict
     - Recommendation: Change `PublicPort` in config or kill conflicting process
   
   - **Crash**: Search for `"fatal error"`, `"exception"`, `"crash"`, `"segmentation fault"`
     - Diagnosis: Game server crash
     - Recommendation: Extract stack trace/error code, check for known issues, verify game files
   
   - **Mod conflict**: Search for `"mod"`, `"plugin"`, `"load failed"`, `"incompatible"`
     - Diagnosis: Mod loading failure
     - Recommendation: Identify conflicting mod, disable it via `PUT /api/servers/{id}/mods/{serverModId}/toggle {"enabled": false}`
   
   - **Permission error**: Search for `"permission denied"`, `"access denied"`
     - Diagnosis: File system permissions
     - Recommendation: Verify install path is writable

3. **Present diagnosis** with recommended next steps

**Safety**: Level 1 (read-only analysis).

### 6. Mod Management Workflow

**Goal**: Search, download, and deploy Steam Workshop mods.

**Steps**:

1. **(Optional) Steam login** (required for authenticated workshop access):
   - `POST /api/steam/login {"username": "...", "password": "...", "guardCode": "..."}` (Level 2)
   - Explain: "Logging in to Steam to access workshop"
   - Monitor: `GET /api/steam/logs/stream?token=<jwt>` (SSE, Level 1)

2. **Search workshop**: `GET /api/steam/workshop/search?q=<query>&num=20` (Level 1)
   - Returns mods with `workshopId`, `title`, `previewUrl`, `subscriptions`, `favoritesCount`

3. **Add to global library**: `POST /api/mods {"workshopId": "...", "name": "..."}` (Level 2, explain)

4. **Download mod files**: `POST /api/mods/{modId}/download` (Level 2, explain: "Downloading mod via SteamCMD")
   - Monitor: `GET /api/mods/{modId}/logs/stream?token=<jwt>` (SSE, Level 1)

5. **Link to server**: `POST /api/servers/{id}/mods {"modId": "..."}` (Level 2, explain)

6. **Enable mod**: `PUT /api/servers/{id}/mods/{serverModId}/toggle {"enabled": true}` (Level 2, explain)

7. **Deploy to game directory**: `POST /api/servers/{id}/mods/deploy` (Level 2, explain: "Copying enabled mods to server install directory")

8. **Restart server** to load mods: `POST /api/servers/{id}/restart` (Level 3, confirm + check online players)

**Dangerous operation**: Deleting a mod from global library: `DELETE /api/mods/{modId}` (Level 3, confirm + explain impact on linked servers).

### 7. Automation Patterns

**Goal**: Define recurring maintenance tasks for agent schedulers (cron, Windows Task Scheduler, agent-internal timers).

**Note**: This tool does NOT provide built-in scheduling. Agents must use their host platform's scheduler.

**Patterns**:

1. **Auto-save** (hourly):
   - Command: `POST /api/servers/{id}/rest/save` (Level 2)
   - Explain before first run: "Will trigger manual save every hour"

2. **Scheduled announcements** (e.g., every 2 hours):
   - Command: `POST /api/servers/{id}/rest/announce {"message": "Server restart in 2 hours for maintenance"}` (Level 2)
   - Explain before first run

3. **Maintenance restart** (off-peak, e.g., 4 AM daily):
   - Before executing:
     1. Fetch player count: `GET /api/servers/{id}/rest/players` (Level 1)
     2. Confirm with user: "Will restart server, disconnecting N online players"
   - Command: `POST /api/servers/{id}/restart` (Level 3)

4. **Monitoring & alerts** (e.g., every 5 minutes):
   - Fetch: `GET /api/system/stats`, `GET /api/servers/{id}/rest/metrics` (Level 1)
   - Alert user if:
     - CPU > 90% for 3+ consecutive checks
     - Memory < 10%
     - Server stopped unexpectedly
     - No online players for > 1 hour (may indicate crash)

**Implementation**: Agents should use their host scheduler (e.g., `cron`, `schtasks`, agent-internal timers) to call these endpoints at the specified intervals. For dangerous operations (restart), always fetch current state and confirm impact before execution.

## Appendix

### Common Issues

**Q: I receive `401 Unauthorized` after some time.**  
A: JWT tokens expire after 7 days. Re-login via `POST /api/auth/login` with the admin password.

**Q: How do I consume SSE streams?**  
A: SSE endpoints (`/logs/stream`, `/steam/logs/stream`, `/mods/{modId}/logs/stream`, `/system/update/stream`) return `text/event-stream`. Pass JWT via query param: `?token=<jwt>`. Each event has `data:` lines; parse and display incrementally.

**Q: Steam login fails with "incorrect guard code".**  
A: Steam Guard requires a 2FA code. If the user has email or mobile authenticator enabled, they must provide the current code in the `guardCode` field. Monitor `GET /api/steam/logs/stream` for prompts.

**Q: `POST /api/servers/{id}/start` returns 409 "Not installed".**  
A: The server must be installed first via `POST /api/servers/{id}/install`. Poll or check logs until installation completes.

**Q: Config changes don't take effect.**  
A: Most config changes require a server restart. Stop the server, update config via `PUT /api/servers/{id}/config`, then start again.

### PalWorldSettings.ini Key Fields

Common settings agents may tune (see `GET /api/config/schema` for full list with descriptions):

- **ServerName**: Display name in server browser
- **ServerDescription**: Description shown in browser
- **ServerPassword**: Password to join (empty = public)
- **PublicPort**: Game port (default 8211)
- **PublicIP**: Public IP for NAT traversal (optional)
- **MaxPlayers**: Max concurrent players (1-32)
- **Difficulty**: `None`, `Normal`, `Hard`
- **DayTimeSpeedRate**: Daytime speed multiplier (0.5 = slower, 2.0 = faster)
- **NightTimeSpeedRate**: Nighttime speed multiplier
- **ExpRate**: Experience gain multiplier (2.0 = double XP)
- **PalCaptureRate**: Pal capture success rate multiplier
- **PalSpawnNumRate**: Pal spawn density multiplier
- **DropItemMaxNum**, `DropItemMaxNum_UNKO`: Max dropped items (reduce to save resources)
- **bEnablePlayerToPlayerDamage**: PvP enabled (true/false)
- **bEnableFriendlyFire**: Friendly fire enabled (true/false)
- **bEnableInvaderEnemy**: Enable raids (true/false)
- **RaidBossEnemyInterval**: Time between raids (minutes)

---

**End of Skill**

For live API reference, always fetch `/swagger/doc.json` at runtime.

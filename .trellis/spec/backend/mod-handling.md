# Mod-Handling Guidelines

> 创意工坊 Mod 的下载 / 部署 / 加载配置约定。SteamCMD 下载见 `internal/steamcmd/workshop.go`，纯文件/INI 逻辑见 `internal/palmod`，编排见 `internal/process` 的 `Manager.UpdateMods` / `RewriteModSettings`，API 见 `internal/api` 的 mod handlers。

---

## Overview

- Palworld 创意工坊内容属于 **game client App ID `1623730`**（不是专用服务器 App ID `2394010`）；下载落地在 `<steamcmdPath>/steamapps/workshop/content/1623730/<workshopID>/`。
- 本项目采用**复制部署**：把下载内容复制到 `<installPath>/Mods/Workshop/<workshopID>/`（不使用 `WorkshopRootDir` 共享目录），每台服务器自包含。
- 服务器端 mod 仅 Windows 支持；路径一律用 `path/filepath`。
- mod 变更**仅在服务器重启后生效**；工具**不自动重启**（只在 UI 提示）。

---

## Gotcha（真机证实，最重要）: 匿名登录下不了 Palworld 工坊内容，必须用拥有游戏的账号

**What**: Palworld 是付费游戏，`workshop_download_item 1623730 <id> +login anonymous` **下不到内容**——SteamCMD 只自更新后静默返回、工坊目录为空（2026-07-15 Windows 真机确认）。必须 `+login <拥有 Palworld 的账号>`。

**方案（复用预登录会话）**: 全局配置 `steam_username`（`config.Config.SteamUsername`，`yaml:"steam_username"` / `env:"STEAM_USERNAME"`）。`steamcmd.DownloadWorkshopItem(steamcmdPath, steamUsername, workshopID, out)` 只用**用户名**做 `+login <user>`，**不传密码、不处理 Steam Guard**。前置一次性设置：用户在终端手动跑一次 `steamcmd +login <user>`（输入 Guard 码），SteamCMD 缓存 sentry/会话，之后本工具复用该缓存会话。`steam_username` 为空 → 回退 `anonymous`（对 Palworld 必失败），此时错误文案（`loginHint`）指引用户配置。

**Why**: 避免在本工具内存储密码或实现交互式 Guard 码喂入（安全 + 复杂度）。用户名单向传入、会话由 SteamCMD 自己管。

**Rule**: 任何工坊下载都走 `DownloadWorkshopItem` 并透传 `steam_username`；下载后**必须校验落地目录存在**（把静默下空转成显式错误 + 可读指引），不可假定 `cmd.Run()` 成功即下到内容。

---

## Convention: 下载 / 部署 / 元数据的纯逻辑集中在 `internal/palmod`，无 DB 依赖

**What**:
- `palmod.Deploy(installPath, workshopID, srcDir) → dstDir`：**先清目标再递归复制**到 `<installPath>/Mods/Workshop/<workshopID>/`；`palmod.Remove(installPath, workshopID)` 删该目录。
- `palmod.ParseInfo(dir) → *Info{PackageName, Version, InstallRules[].IsServer}`：读 `<dir>/Info.json`，**容忍式**（缺字段/大小写/Version 为字符串或数字都不 panic），`IsServer` 缺省 false。
- `palmod.WriteModSettings(installPath, enabled)`：写 `<installPath>/Mods/PalModSettings.ini`。

**Why**: 与 `palsave`/`palconfig` 同构——纯逻辑可单测、不依赖 models/api/db。编排层（`process.Manager`）负责下载→部署→回填→写配置的串联与 DB 落库。

**Rule**: 新增 mod 相关文件/INI 逻辑放 `palmod` 并配单测（`t.TempDir()`，无需真实 mod）；不要在 handler 或 manager 里手拼 Mods 路径。

---

## Gotcha: `ActiveModList` 用 Info.json 的 PackageName，且是重复键 → 面向行读写 PalModSettings.ini

**What**: `PalModSettings.ini` 的 `[PalModSettings]` 段里，`bGlobalEnableMod=true` 开总开关，每个启用 mod 一行 `ActiveModList=<PackageName>`。`PackageName` 取自各 mod 的 `Info.json`（**不是**文件夹名 / Workshop ID）。`ActiveModList` 是**重复键**（数组式），标准 INI 库会折叠成一条 → `WriteModSettings` 用**面向行的自定义读写**：解析行、识别段、剔除旧 `ActiveModList` 行、按启用集重建，保留段内其它键与文件其它段/注释。**幂等**（重复写不产生重复行）。文件缺失（首次启动服务器前）则新建 + `MkdirAll <installPath>/Mods`。

**Why**: 只有解析出 `PackageName` 的 mod 才能进 `ActiveModList`；下载失败/未回填的 mod 不写入，避免半成品配置行。

**Rule**: 只为 `Enabled && strings.TrimSpace(PackageName) != ""` 的 mod 写 `ActiveModList`；改这段逻辑要保住幂等单测。

---

## Convention: `UpdateMods` 复用 `InstallServer` 的 async + capture/SSE 管线（KindSteamCMD）

**What**: `POST /servers/:id/mods/update` 处理器完全照搬 `InstallServer` 模式：`logger.ResetLog(KindSteamCMD)` → `go func`{ `NewCapture`+`NewBroadcastWriter` 组 `io.MultiWriter` → `process.UpdateMods` → `defer capture.Close()` }，返回 202。前端经现有 `/servers/:id/logs/stream`（KindSteamCMD）观察，不新建通道。单个 mod 失败聚合进 `last_error`（多行可读），处理继续（部分成功）。

**Why**: 安装与 mod 更新都是 SteamCMD 长任务，同一日志通道/并发/错误持久化模式，避免重复造轮子。

**Rule**: mod 更新错误复用 server 级 `last_error`（MVP 取舍，与 install/start 共享，成功清除）；不要为 mod 单独加错误列，除非产品需要区分域。

---

## Gotcha: 并发——`updatingMods` 与 `installing` 互斥；运行中不拦截；INI 写有独立锁

**What**:
- `Manager` 的 `updatingMods` 集合受 `m.mu` 保护，与 `installing` **互斥**（安装中/更新中拒绝再次更新）。
- 服务器**运行中不拦截** mod 更新——复制到 `Mods/Workshop` 不触碰在跑的进程，只在下次重启生效（UI 提示重启）。
- `Manager.iniMu`（单锁）串行化所有 `palmod.WriteModSettings` 调用：`UpdateMods`（后台 goroutine 尾部写）与 `RewriteModSettings`（toggle/delete 的 HTTP goroutine 同步写）都用非原子 `os.WriteFile`，无锁会 torn write。写操作亚毫秒且稀，单锁足够。

**Why**: 下载可持续数秒；期间 toggle/delete 会并发改同一 ini 文件。

**Rule**: 新增任何写 `PalModSettings.ini` 的路径都必须持 `iniMu`。

---

## 跨层字段一致性

`Mod` 的 DB 列（`internal/models/server.go`，显式 `gorm:"column:..."`，`PackageName`/`Version` 为 D5 新增）→ Go `json` tag（snake_case）→ 前端 `ui/src/types/server.ts` 的 `Mod` 接口（snake_case）→ i18n `serverConfig.mods.*`（zh/en/ja 三语齐全）。改任一侧四处同步。UI 的 Mods tab 在 `ServerSettingsDialog` 的 `tabs` 数组追加 `'mods'`，`ModsSection` 顶部常驻 `mods.loginHint`（配 `steam_username` + 一次性手动登录说明）。

---

## 待真机复核（截至 2026-07-15）

- `Info.json` 的确切键名/结构（`PackageName` / `Version` / `InstallRule[].IsServer`）依官方文档实现，**尚未用真实 mod 核对**；`ParseInfo` 已容忍式，真机拿到真实 mod 后须比对键名，不符则微调。
- 真实工坊下载（配 `steam_username` + 预登录后）端到端成功与否，以 Windows 真机为准。

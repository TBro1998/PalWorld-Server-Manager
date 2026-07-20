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

**方案(应用内登录 + 复用会话,D7)**:
- **下载**始终 `steamcmd.DownloadWorkshopItem(steamcmdPath, steamUsername, workshopID, out)` 只用**用户名** `+login <user>`,复用 SteamCMD 缓存会话,**下载路径不涉密码**。落地必须校验目录存在(静默下空 → 显式错误)。
- **用户名来源**:DB 键值 `settings`(`internal/settings`,键 `steam_username`)**优先**,空则回退 `config.Config.SteamUsername`。`Manager.resolveSteamUsername()` 做此解析,`UpdateMods` 用它。config 字段仅作初始默认/回退。
- **会话如何建立**:经**应用内登录**(不再要求终端)。`steamcmd.Login(ctx, path, user, pass, guardCode, out)` 跑 `+login <user> <pass> [<code>] +quit`——**Guard 码作第三位置参**,无需伪终端;`classifyLogin` 解析 stdout 得 success/needGuard/badCredentials/error。API `POST /api/steam/login` **同步返回结果**,成功后 `Set(steam_username)`+`Set(steam_session_ready,"true")`。前端两步:账号密码 → 需要则补 Guard 码。
- **实时日志(SSE)**:登录期间 steamcmd 输出 tee 到 `logger.NewBroadcastWriter(streams, steamLogStreamID=0, KindSteamCMD)`,前端经 `GET /api/steam/logs/stream`(复用 `StreamManager`,与安装/更新同机制)实时逐行看。sentinel serverID=0(真实 server ≥1 不冲突);登录只广播不落盘。前端弹窗打开即订阅(早于 POST)以免漏行。**结果走同步 POST 响应,日志走 SSE**——两条通道各司其职,避免 async 结果信令的复杂度。

**Why**: 用户不必开终端;会话由 SteamCMD 自管(登录成功即缓存 sentry),后续下载只需用户名。

**SECURITY(强约束)**: Steam **密码只临时用于本次登录**——只进 `Login` 的子进程 argv,**绝不**写入 `out`/返回错误/DB/日志/last_error/HTTP 响应;调用后立即清空 `req.Password`;`models.Setting` 只有 Key/Value,永不存密码;调用方**禁止**记录 argv。改任何登录相关代码必须守住这条。
  - steamcmd 的**登录输出经 SSE 实时广播**到前端(供用户看登录进度,通道见"实时日志"),但**密码不在 steamcmd 输出里**(只在 argv,steamcmd 不回显),故广播不泄露密码;登录 `out` 是 broadcastWriter,只接 steamcmd stdout/stderr,不落盘、HTTP 响应也不带 log。Guard 码同理(argv,不回显,单次有效)。

**Gotcha(真机核对后固化,2026-07-16)**: `classifyLogin` 必须 **success 优先**。Steam Guard **手机验证器(mobile authenticator)** 账号登录**成功**时,输出仍含 "Steam Guard" / "authenticator"(说明文案 "This account is protected by a Steam Guard mobile authenticator. Please confirm the login in the Steam Mobile app...")。旧的 guard-keyword-first 顺序把成功误判成 needGuard。修正后顺序:success(`waiting for user info...ok`/`to steam public...ok`/`logged in ok`)→ `invalid password`(bad)→ **仅"要码"措辞** `steam guard code`/`two-factor code`/`account logon denied`(needGuard,**不含**裸 `steam guard`/`authenticator`)→ `login failure`(bad)→ error。手机确认流无需喂码:steamcmd 阻塞轮询 "Waiting for confirmation..." 直到用户手机批准→success,故 `steamLoginTimeout=180s` 给批准时间。

**Rule**: 工坊下载走 `DownloadWorkshopItem` 并透传解析后的用户名;登录走 `steamcmd.Login`;改 `classifyLogin` 必须保 success 优先、needGuard 只匹配要码措辞;新 steamcmd 版本/语言若文案不同,加关键字而非改回 guard-first。

---

## Convention: 下载 / 部署 / 元数据的纯逻辑集中在 `internal/palmod`，无 DB 依赖

**What**:
- `palmod.Deploy(installPath, workshopID, srcDir) → dstDir`：**先清目标再递归复制**到 `<installPath>/Mods/Workshop/<workshopID>/`；`palmod.Remove(installPath, workshopID)` 删该目录。
- `palmod.ParseInfo(dir) → *Info{PackageName, ModName, Version, Tags, InstallRules[].IsServer}`：读 `<dir>/Info.json`，**容忍式**（缺字段/大小写/Version 为字符串或数字都不 panic），`IsServer` 缺省 false，`ModName`/`Tags` 缺失为零值（`""`/`nil`）。`PackageName`/`Version`/`IsServer` 参与加载逻辑；`ModName`/`Tags` 仅展示用。
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

## Convention: 异步完成走 SSE `done` 事件，前端不轮询

**What**: `UpdateMods` 后台 goroutine 结束后，handler 依 `process.UpdateMods` 返回值广播一个终止事件：`r.streams.BroadcastEvent(id, KindSteamCMD, "done", "ok"|"error")`（nil 返回=全成功→`ok`，否则 `error`）。前端 `ServerLogsDialog` 监听 `done` 事件（SSE 具名事件），`ModsSection` 的 `onDone(success)` 据此**刷新 mod 列表**并在**全成功时关闭日志弹窗**；失败保持打开以便看错误。**不轮询、不读库、不匹配日志文本**——完成/成功信号直接来自函数返回值。

**How（SSE 通道升级）**: `logger.StreamManager` 的通道从 `chan string` 升级为 `chan logger.Msg{Event,Data}`。`Broadcast(id,kind,line)` 仍是唯一日志入口（内部包成 `Msg{Event:"log"}`，故 `broadcastWriter` 与所有既有调用零改动）；新增 `BroadcastEvent(id,kind,event,data)` 发具名控制事件。两个 SSE handler（`StreamLogs`/`SteamLogStream`）读 `Msg` 后 `c.SSEvent(msg.Event, msg.Data)`。前端用 `es.addEventListener('<event>')` 分别接 `log`/`done`。

**Why**: 安装/更新是异步 202，此前前端只能靠 `setTimeout` 猜测完成时机。函数返回值是最可靠的成功判据（避免 last_error 竞态与日志文案匹配的脆弱性，呼应 `classifyLogin` 的教训）。

**Rule**: 新增异步任务的完成/进度信号走 `BroadcastEvent` 具名事件，不要往 `log` 文本里塞控制标记，也不要加轮询端点；前端 `onDone` 回调用 ref 承接，避免进 SSE effect 依赖导致重连。当前仅 mod 更新广播 `done`；install 复用同管线但未发 `done`（如需可照此追加）。

---

## Convention: 全局 mod 下载的下载态 + 日志可在刷新后重挂（history + live）

**What**: 全局 mod 库下载（`POST /api/mods/:modId/download`）跑在后台 goroutine，与前端页面生命周期无关。前端刷新后要能重新识别"下载中"并续看日志，靠两个渠道把后端状态捞回：
- **下载态**：`GET /api/mods` 的 `ModWithStatus` 带运行时字段 `downloading`（`json:"downloading"`，**不落库**），由 `r.process.IsDownloadingGlobalMod(mod.WorkshopID)` 逐条计算。前端 `/mods` 页据此把 `downloading===true` 的 mod id **并入**（非覆盖）本地 `downloadingIds`，刷新后保持 spinner/日志面板/轮询。
- **历史日志**：`GET /api/mods/:modId/logs`（复用 `logger.ReadLogs(LogDir, modID, KindSteamCMD, lines)`，形状对齐服务器 `GetLogs`）回填刷新前落盘的行；随后 `GET /api/mods/:modId/logs/stream`（SSE）接实时行。这就是本项目既有的"history-then-stream"模式（服务器运行日志 `GetLogs`+`StreamLogs` / `LogsSection` 同构）。
- mod 下载日志的落盘/流 key 用 **modID** 当 `serverID` 位（真实 server ≥1 不冲突，同 steam 登录用 sentinel 0 的思路），kind 固定 `KindSteamCMD`，故 `GetModLogs` 不解析 `kind` query。

**Gotcha（回填竞态，2026-07-20 review 捕获）**: **不能无条件回填** `getLogs`。本会话内点"下载/重新下载"会经 `downloadMutation` 触发 `POST /download`，后端 `logger.ResetLog` 清空日志；若前端同时无脑 `getLogs`，可能读到**上一次**下载的残留行并与新 SSE 行混排。修正：前端用一个"挂载即在下载中"的 ref（`reattachedRef`）区分——**仅刷新重挂场景**（mount 时已 `downloading`）才 `getLogs` 回填；**本会话内新发起**的下载 `setLogLines([])` 从空开始，让 SSE 从头补。

**Why**: 后端状态本就完整（`IsDownloadingGlobalMod` + 落盘日志），丢的只是前端内存态；补齐两个只读渠道即可，无需新机制、不碰落盘/广播路径。

**Rule**: 全局 mod 下载态走 `GET /api/mods` 的 `downloading` 运行时字段（不加 DB 列、不加独立 status 端点，复用列表轮询）；历史日志走独立只读端点 + SSE 续流，别把历史塞进 SSE 建流逻辑；回填只在刷新重挂时做，本会话内新下载从空开始以避开 `ResetLog` 竞态。服务器 mod 部署弹窗（`ModsSection` deploy）尚未享有此重挂能力（同类刷新问题，未在本次范围）。

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

`Mod` 的 DB 列（`internal/models/server.go`，显式 `gorm:"column:..."`，`PackageName`/`Version` 为 D5 新增，`ModName`/`Tags` 为展示元信息新增）→ Go `json` tag（snake_case）→ 前端 `ui/src/types/server.ts` 的 `Mod` 接口（snake_case）→ i18n `serverConfig.mods.*`（zh/en/ja 三语齐全）。改任一侧四处同步。UI 的 Mods tab 在 `ServerSettingsDialog` 的 `tabs` 数组追加 `'mods'`，`ModsSection` 顶部常驻 `mods.loginHint`（配 `steam_username` + 一次性手动登录说明）。

**展示层级（Mods 列表条目）**：主标题优先 `mod_name`（Info.json ModName），回退用户 `name`，再回退 `workshop_id`；次级 mono 小字行拼 `package_name`+`workshop_id`+`v{version}`；`tags` 有值时以 badge 展示（`tags ?? []` 容错——见下方 serializer 陷阱，DB 空值反序列化为 `null`）。纯展示字段不新增 i18n 文案（tag 文本/包名直显）。

## Gotcha: `Tags []string` 用 `serializer:json`，回填必须 `Select`+结构体 `Updates`（禁用 `Updates(map)`）

**What**: `models.Mod.Tags` 为 `[]string` + `gorm:"column:tags;serializer:json"`（SQLite 存 JSON 文本，API 直接输出数组）。回填**必须**用 `db.Model(&Mod{}).Where("id=?",id).Select("package_name","mod_name","version","tags","install_path").Updates(models.Mod{...})`。**不能**用 `Updates(map[string]any{"tags": info.Tags, ...})`——map 路径不走列的 serializer，把 `[]string` 当 SQL 行值元组发出，SQLite 报 `SQL logic error: row value misused (1)`（2026-07-18 单测证实）。

**Why**: `Select` 强制写入所有列名（含空 `PackageName`/`ModName`/`Version`、`nil` Tags 等零值——struct Updates 默认跳零值），同时结构体路径对 `tags` 列应用 json serializer；map 路径二者皆失。

**Rule**: 任何写 serializer 列的更新都用 `Select`+结构体 `Updates`，别用 map；`internal/database` 有 `TestModTagsSerializerRoundTrip` 守此 round-trip，改回填逻辑要保它绿。前端读 `tags` 用 `?? []`（未下载/缺字段时后端返回 `null`）。

---

## Info.json 真机键名（2026-07-18 真实 mod 样本确认）

- 真实样本（工作区 `steamcmd/.../content/1623730/<id>/Info.json`）确认键名：`ModName`、`PackageName`、`Thumbnail`、`Version`（字符串如 `"1.7.2"`）、`Tags`（字符串数组如 `["Gameplay"]`）、`Author`、`Dependencies`、`MinRevision`、`InstallRule[]`（`Type`/`IsServer`/`Targets`）。此前"依官方文档、未真机核对"的疑虑已消除；`ParseInfo` 键名与之一致。
- 本项目现用字段：`PackageName`/`Version`/`InstallRule[].IsServer`（加载逻辑）+ `ModName`/`Tags`（展示）。`Thumbnail`（缩略图文件名，如 `thumbnail.png`）**已知但故意不展示**（产品决策，避免额外图片服务端点）；`Author`/`Dependencies`/`MinRevision` 暂未使用。

## 待真机复核

- 真实工坊下载（配 `steam_username` + 预登录后）端到端成功与否，以 Windows 真机为准。

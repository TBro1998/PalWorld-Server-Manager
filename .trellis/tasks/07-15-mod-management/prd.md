# Mod 管理与 SteamCMD 更新

## Goal

让用户在服务器设置中手动维护一个 mod 列表(输入 Steam 创意工坊 ID),点击「更新 mod」后,工具通过 SteamCMD 自动下载/安装/更新这些 mod,复制部署到对应 Palworld 专用服务器并写入加载配置,使其在服务器**重启后**加载。对应技术方案 FR-4 / NFR-8。

## Background / Confirmed Facts

### 代码现状
- `Mod` 模型已存在 [internal/models/server.go:25-34](internal/models/server.go#L25-L34):`ServerID`、`WorkshopID`、`Name`、`Enabled`、`InstallPath`、时间戳。
- mods 表由 GORM AutoMigrate 建立 [internal/database/database.go:38](internal/database/database.go#L38);已有 `dropLegacyModsIfEmpty` 处理旧表。
- mod 处理器全是 stub [internal/api/handlers.go:607-629](internal/api/handlers.go#L607-L629):`ListMods` / `InstallMod` / `UninstallMod` / `ToggleMod`。
- mod 路由已注册 [internal/api/router.go:102-108](internal/api/router.go#L102-L108):`GET/POST /servers/:id/mods`、`DELETE /:modId`、`PUT /:modId/toggle`。
- SteamCMD 包已有服务器安装 [internal/steamcmd/server.go:19-71](internal/steamcmd/server.go#L19-L71) 与自身安装 [internal/steamcmd/steamcmd.go](internal/steamcmd/steamcmd.go),但**无 workshop 下载**函数。
- 安装采用成熟的异步 + 日志/SSE 模式:`InstallServer` 处理器 [internal/api/handlers.go:261-314](internal/api/handlers.go#L261-L314) 用 `logger.NewCapture(id, KindSteamCMD, ...)` + `NewBroadcastWriter`;进程侧 `Manager.InstallServer` [internal/process/manager.go:275-314](internal/process/manager.go#L275-L314) 用 `installing` 内存集合跟踪、写 `last_error`/`installed`。
- 前端设置弹窗 [ui/src/components/ServerSettingsDialog.tsx](ui/src/components/ServerSettingsDialog.tsx) 以 `tabs` 数组驱动 tab 栏(basics / 分类 / launch),条件渲染;新增 mods tab 成本低。
- 前端无任何 mod 管理 UI。

### Palworld 创意工坊事实(2026,权威来源 docs.palworldgame.com/settings-and-operation/mod/ v1.0.0 © Pocketpair)
- 创意工坊内容属于 **game client App ID `1623730`**(不是专用服务器 App ID `2394010`)。
- SteamCMD 下载:`+login <拥有 Palworld 的账号> +workshop_download_item 1623730 <workshopId> +quit`,落地到 `<steamcmd>/steamapps/workshop/content/1623730/<workshopId>/`。
- 服务器端 mod **仅 Windows 专用服务器支持**(本项目 Windows-only,契合)。
- **不存在 "moddable 分支" / 特殊 beta**(官方文档无此说法;此前 BisectHosting 搜索摘要有误,已作废)。标准 app `2394010` 直接支持。
- 专用服务器加载 mod(官方三选一;本项目采用第一种——复制到服务器目录):
  - 默认读取服务器可执行文件(`PalServer.exe`,即 installPath 根)同目录下的 `Mods/Workshop/<任意文件夹名>/`(Info.json 直接放该文件夹下);或
  - 在 `<installPath>/Mods/PalModSettings.ini` 的 `[PalModSettings]` 下设 `WorkshopRootDir=<绝对路径>`;或
  - 启动参数 `-workshopdir="<绝对路径>"`。
- 启用:`PalModSettings.ini`(首次启动服务器后自动生成)设 `bGlobalEnableMod=true`,每个要加载的 mod 一行 `ActiveModList=<PackageName>`。`PackageName` 取自每个 mod 的 `Info.json`(**不是**文件夹名/Workshop ID)。`-NoMods` 可强制全禁用。
- 生效:**必须重启专用服务器**。重启时自动创建 `Mods/ManagedMods/<PackageName>/InstallManifest.json` 并按 Info.json 的 InstallRules 部署(Paks→`Pal/Content/Paks/~WorkshopMods/<PackageName>` 等)。Info.json 的 `Version` 变化后重启会自动卸旧装新。
- 移除:从 ActiveModList 删除 PackageName,必要时删 `Mods/Workshop/<WorkshopId>/`,重启。
- 服务器兼容前提:Info.json 的 InstallRule 含 `"IsServer": true`,否则不是为服务器设计。
- **已证实(真机验证 2026-07-15)**:`workshop_download_item 1623730` **匿名登录下不到内容**——Palworld 是付费游戏,其工坊内容仅**拥有该游戏的 Steam 账号**可下;匿名时 SteamCMD 仅自更新后静默返回、工坊目录为空。故必须 `+login <账号>`。Steam Guard 两步验证经**预登录会话**规避:用户先在终端手动 `steamcmd +login <账号>` 一次(输入 Guard 码,SteamCMD 缓存 sentry/会话),之后工具只传用户名(不传、不存密码)复用缓存会话。来源:Valve SteamCMD wiki、多个社区报告(见会话记录)。

## Resolved Decisions(收敛结论,原 Open Questions)

- **D1(原 Q1)**:~~moddable 分支~~ 作废——官方无此说法,标准分支 `2394010` 直接支持。
- **D2(原 Q2)部署机制**:**复制**下载内容到 `<installPath>/Mods/Workshop/<workshopId>/`。每台服务器自包含、互不影响,不依赖绝对路径配置(不采用 WorkshopRootDir 共享方案)。
- **D3(原 Q3)生效方式**:**不自动重启**。更新完成后仅下载 + 写配置,UI 提示「需重启服务器生效」;运行中的服务器由用户自行决定何时重启。
- **D4(原 Q4)UI**:在 `ServerSettingsDialog` 新增 **Mods tab**,mod 列表即时 CRUD(增删/启停),「更新 mod」为该 tab 内的动作按钮。
- **D5(原 Q5)数据模型**:新增 `Mod.PackageName` 与 `Mod.Version` 字段。`ActiveModList` 必须用 `PackageName`(非文件夹名/Workshop ID);`Version` 用于更新检测与展示。二者在下载后解析 `Info.json` 回填。
- **D6(真机验证后新增)Steam 登录**:匿名不可用(见 Background)。采用**复用预登录会话**方案——新增全局配置 `steam_username`/`STEAM_USERNAME`(与 `steamcmd_path` 同层,一个拥有 Palworld 的账号即可);下载用 `+login <steam_username>`(仅用户名,不涉密码)。前置一次性设置:用户手动 `steamcmd +login <账号>` 完成 Guard 验证并缓存会话。`steam_username` 为空时回退匿名(对 Palworld 会失败,UI 给出配置指引)。**不**在本工具内处理密码或 Guard 码。

## Requirements

- **FR1 mod 列表 CRUD**:用户在服务器设置的 Mods tab 手动添加 mod(输入 Workshop ID,可选备注名),即时持久化到 mods 表(按 serverID 隔离);可删除条目;可切换启用/禁用。
- **FR2 更新下载**:点击「更新 mod」后,对该服务器列表内**全部** mod 逐个执行 SteamCMD `+login <steam_username> +workshop_download_item 1623730 <workshopId> +quit`(`steam_username` 空则回退 `anonymous`),下载到 SteamCMD 工坊目录后**复制**到 `<installPath>/Mods/Workshop/<workshopId>/`。
- **FR2b Steam 账号配置**:新增全局配置 `steam_username`(config.yaml / `STEAM_USERNAME` / 默认空)。下载复用 SteamCMD 预登录会话(见 D6),工具不处理密码/Guard 码。当下载因匿名/未登录失败时,错误信息须指引用户配置 `steam_username` 并完成一次性手动登录。
- **FR3 元数据回填**:每个 mod 复制后解析其 `Info.json`,回填 `PackageName`、`Version`;并读取 InstallRule 的 `IsServer` 用于兼容性判断。
- **FR4 加载配置写入**:写/更新 `<installPath>/Mods/PalModSettings.ini` 的 `[PalModSettings]`:`bGlobalEnableMod=true`,并为每个**启用**的 mod 写一行 `ActiveModList=<PackageName>`;禁用的 mod 不出现在 ActiveModList。写入保持幂等(重复更新不产生重复行)。
- **FR5 移除**:删除 mod 条目时,从 ActiveModList 移除其 PackageName 并删除 `<installPath>/Mods/Workshop/<workshopId>/`;数据库记录删除。
- **FR6 启停切换**:toggle 仅改 `Enabled` 并重写 ActiveModList,不重新下载、不删文件。
- **FR7 异步进度与日志**:更新为异步操作,复用现有 `logger` capture + SSE 广播模式,实时暴露 SteamCMD 输出;失败(如匿名登录失败、Workshop ID 无效、Info.json 缺失)写入可读错误并可在 UI 查看。
- **FR8 生效提示**:更新/启停/移除完成后,若目标服务器正在运行,前端显示「需重启服务器生效」提示;不自动重启。
- **FR9 兼容性提示**:当某 mod 的 Info.json `IsServer` 非 true 时,给出「该 mod 可能非为服务器设计」的警告,但不阻止用户启用(仅提示)。

## Acceptance Criteria

- [ ] Mods tab 中添加一个合法 Workshop ID 后,记录即时出现在列表并持久化(刷新后仍在),按 serverID 隔离(切换服务器互不串)。
- [ ] 点击「更新 mod」触发异步下载;SteamCMD 输出经 SSE 实时显示;完成后 `<installPath>/Mods/Workshop/<workshopId>/` 存在下载内容。
- [ ] 更新成功的 mod 的 `PackageName`、`Version` 已从 Info.json 回填并在列表展示。
- [ ] `<installPath>/Mods/PalModSettings.ini` 含 `bGlobalEnableMod=true`,且每个启用 mod 恰有一行 `ActiveModList=<PackageName>`;禁用/删除的 mod 不在其中;重复「更新」不产生重复行。
- [ ] 删除 mod 后:数据库记录消失、`Mods/Workshop/<workshopId>/` 被删除、ActiveModList 不再含其 PackageName。
- [ ] toggle 启用/禁用不触发下载,仅重写 ActiveModList。
- [ ] 配置 `steam_username` 且完成一次性手动登录后,下载真实 Palworld mod 成功(工坊目录有内容)。未配置/未登录时下载失败并给出配置指引。
- [ ] 下载失败(无效 ID / 未登录 / 无 Info.json)时,UI 展示可读错误,不产生半成品配置行,不 panic。
- [ ] 任一 mod 变更后,若服务器运行中,UI 显示「需重启生效」提示;工具不自动重启。
- [ ] `go build .`、`go vet ./...`、`go test ./...`、前端 `bun run lint` 与 `bun run build`、`go build .`(嵌入)全部通过。

## Out of Scope

- mod 搜索 / 浏览(NFR-8 明确不做,用户手动输入 ID)。
- 需要 Steam API Key 的能力。
- 「更新 mod」自动重启运行中的服务器(D3:仅提示)。
- WorkshopRootDir / `-workshopdir` 共享目录加载方式(D2:采用复制)。
- 非 Windows(Linux)服务器端 mod(官方仅 Windows 支持;Linux 代码路径保留但不在本任务验证)。
- 在本工具内输入/存储 Steam 密码或处理 Steam Guard 验证码(D6:仅复用用户手动建立的预登录会话,只传用户名)。

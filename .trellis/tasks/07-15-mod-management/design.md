# Design — Mod 管理与 SteamCMD 更新

> 依据 prd.md 的 Requirements 与 Resolved Decisions(D2 复制部署 / D3 不自动重启 / D4 Mods tab / D5 加字段)。

## 1. 边界与分层

复用现有「异步 + logger capture + SSE 广播」管线(与 `InstallServer` 同构),不新建通道:

```
前端 ServerSettingsDialog(Mods tab)
  │  CRUD / 更新触发 (REST)                日志观察复用 KindSteamCMD SSE
  ▼
API handlers.go (mod handlers)  ──────────────► logger capture+broadcast (KindSteamCMD)
  │                                               ▲
  ▼ 同步 CRUD(DB)           ▼ 异步更新            │
process.Manager.UpdateMods ── steamcmd.DownloadWorkshopItem ┘
  │
  ▼ 部署 + 配置写入
palmod (新包): Info.json 解析 / 复制部署 / PalModSettings.ini 读写
```

职责划分:
- **API 层**:参数校验、DB 的同步 CRUD、启动异步更新、组织日志 sink;不含文件/INI 逻辑。
- **process.Manager**:并发跟踪(`updatingMods` 集合)、编排下载→部署→回填→写配置,持久化 `last_error`(复用现有 setError/clearError 或 mod 级错误字段,见 §4)。
- **steamcmd 包**:仅负责调用 SteamCMD 下载 workshop item,返回落地目录。
- **palmod 新包**:纯文件/INI 逻辑(Info.json 解析、复制、PalModSettings.ini 增量读写),无 DB 依赖,可单测。

## 2. 数据模型(D5)

`internal/models/server.go` 的 `Mod` 追加两列(GORM AutoMigrate 增列,幂等、无破坏):

```go
type Mod struct {
    ...existing...
    PackageName string `json:"package_name" gorm:"column:package_name;default:''"` // 取自 Info.json,ActiveModList 用
    Version     string `json:"version" gorm:"column:version;default:''"`           // 取自 Info.json,更新检测/展示
    // InstallPath 复用:存 <installPath>/Mods/Workshop/<workshopId>
}
```

- `WorkshopID` 是用户输入的唯一业务键;`PackageName`/`Version` 下载解析后回填,更新前可能为空。
- AutoMigrate 已在 [database.go:38](internal/database/database.go#L38) 覆盖 `&models.Mod{}`,加列自动生效,无需手写迁移。

## 3. steamcmd:workshop 下载(含登录,D6)

新增 `internal/steamcmd/workshop.go`:

```go
const palworldClientAppID = "1623730"

// DownloadWorkshopItem 下载单个创意工坊 item,返回落地目录:
//   <steamcmdPath>/steamapps/workshop/content/1623730/<workshopID>
// steamUsername 为空时回退 "anonymous"(对 Palworld 付费工坊内容会失败)。
// 仅传用户名——复用用户预先手动 `steamcmd +login <user>` 建立的缓存会话,
// 本函数不接触密码/Steam Guard。out 语义同 InstallPalworldServer(nil→discard,禁止 os.Stdout)。
func DownloadWorkshopItem(steamcmdPath, steamUsername, workshopID string, out io.Writer) (string, error)
```

- 命令:`+login <user|anonymous> +workshop_download_item 1623730 <workshopID> +quit`(无 `+force_install_dir`;workshop 内容固定落在 steamcmd 自身目录下)。
- 复用 `getExecutablePath`(同包私有)定位可执行文件,校验存在性,`cmd.Stdout=cmd.Stderr=out`。
- 下载后校验落地目录存在;不存在则返回错误——错误文案区分「匿名(未配置 steam_username)」与「已配置但仍失败(会话未预登录 / ID 无效)」,指引用户完成一次性手动登录。

**config 贯通**:`config.Config` 加 `SteamUsername`(`yaml:"steam_username" env:"STEAM_USERNAME"`,默认空)→ `process.NewManager` 增 `steamUsername` 参数(router.go 传 `cfg.SteamUsername`)→ `Manager.UpdateMods` 调用 `DownloadWorkshopItem(m.steamcmdPath, m.steamUsername, ...)`。**已证实匿名对 Palworld 不可用**(prd Background),`steam_username` 是功能前提而非可选增强。

## 4. process.Manager:UpdateMods

`internal/process/manager.go` 新增:

```go
updatingMods map[int64]struct{}  // 与 installing/running 同受 m.mu 保护

func (m *Manager) UpdateMods(serverID int64, out io.Writer) error
func (m *Manager) IsUpdatingMods(serverID int64) bool
```

编排(不持 m.mu 跑 SteamCMD/IO):
1. 并发闸:若 `installing` 或 `updatingMods` 已含该 id → 拒绝;运行中**不拦截**(复制到 Mods/Workshop 不影响运行进程,D3 仅提示重启)。置位 `updatingMods`,defer 清除。
2. 读 server.install_path;查该 server 全部 mods。
3. 逐个 mod:`steamcmd.DownloadWorkshopItem` → `palmod.Deploy(installPath, workshopID, downloadedDir)` 复制到 `<installPath>/Mods/Workshop/<workshopID>/` → `palmod.ParseInfo(deployedDir)` 得 PackageName/Version/IsServer → 回填 DB 行(package_name/version/install_path)。单个 mod 失败记录到聚合错误并继续其余(部分成功)。
4. 全部处理后:`palmod.WriteModSettings(installPath, enabledMods)` 写 `<installPath>/Mods/PalModSettings.ini`。
5. 结果:成功清 last_error;有失败写聚合 last_error(经 setError)。日志已通过 out 实时可见。

> 错误呈现:MVP 复用 server 级 `last_error`(与安装一致),聚合成可读多行文本;不新增 mod 级错误列(YAGNI)。

## 5. palmod 新包(纯逻辑,可单测)

`internal/palmod/`:

- `info.go` — `type Info struct { PackageName, Version string; InstallRules []InstallRule }`,`ParseInfo(dir) (*Info, error)` 读取 `<dir>/Info.json`。
  - **风险(须实现期核对)**:Info.json 的确切键名/结构(`PackageName`/`Version`/`InstallRule[].IsServer`)以官方文档为准,但未见真实样本;parser 用容忍式(大小写/缺失字段不 panic),`IsServer` 缺省视为 false 并触发 FR9 警告。
- `deploy.go` — `Deploy(installPath, workshopID, srcDir) (dstDir string, err error)`:递归复制 `srcDir` → `<installPath>/Mods/Workshop/<workshopID>/`(先清目标再复制,保证与源一致);`Remove(installPath, workshopID)` 删除该目录(FR5)。
- `modsettings.go` — PalModSettings.ini 增量读写:
  - `WriteModSettings(installPath string, enabled []EnabledMod) error`:读现有 `<installPath>/Mods/PalModSettings.ini`(不存在则新建 + `MkdirAll` Mods 目录),在 `[PalModSettings]` 段设 `bGlobalEnableMod=true`,**重建**全部 `ActiveModList=<PackageName>` 行(每个启用 mod 一行),保留段内其它键与文件内其它段/注释。幂等:重复调用不产生重复行。
  - **风险**:`ActiveModList` 是重复键(数组式),标准 INI 库会折叠;采用**面向行的自定义读写**(解析行、识别段、剔除旧 ActiveModList 行、插入新行),不引第三方 INI 依赖。

## 6. API 契约(handlers.go + router.go)

保留现有 handler 名与 4 条路由,新增 1 条「更新」路由:

| 方法/路径 | handler | 语义 | 响应 |
|---|---|---|---|
| GET `/servers/:id/mods` | `ListMods` | 按 server_id 列出 | 200 `{mods:[...]}` |
| POST `/servers/:id/mods` | `InstallMod` | **仅新增列表条目**(body: `{workshopId, name?}`),不下载 | 201 mod |
| POST `/servers/:id/mods/update` | `UpdateMods`(新) | 异步下载+部署+写配置(全部 mod) | 202 `{status:"updating"}` |
| DELETE `/servers/:id/mods/:modId` | `UninstallMod` | 删行 + 删 Workshop/<id> + 重写 ini | 204 |
| PUT `/servers/:id/mods/:modId/toggle` | `ToggleMod` | 翻转 enabled + 重写 ini(不下载) | 200 mod |

- `UpdateMods` 处理器复用 [InstallServer 模式](internal/api/handlers.go#L294-L307):`ResetLog(KindSteamCMD)` → `go func`{ capture+broadcast MultiWriter → `process.UpdateMods` }。前端经现有 `/servers/:id/logs/stream`(KindSteamCMD)观察。
- 校验:server 存在;`:modId` 属于该 server(防跨服操作);workshopId 非空。
- toggle/delete 后同步重写 ini(轻量、无下载),在 handler 内直接调 `palmod`(不走异步)。

## 7. 前端(D4)

- `ui/src/types/server.ts`:新增 `Mod` 接口(id/serverId/workshopId/name/enabled/packageName/version/installPath)。
- `ui/src/lib/api.ts`:`modsApi` = { list(id) / add(id,{workshopId,name}) / remove(id,modId) / toggle(id,modId) / update(id) };日志复用 serversApi 现有 stream。
- `ServerSettingsDialog.tsx`:`tabs` 数组([:198](ui/src/components/ServerSettingsDialog.tsx#L198))追加 `'mods'`;新增内联 `ModsSection`:
  - mod 列表(WorkshopID / Name / Version / 启停开关 / 删除),空态提示。
  - 「添加」输入 WorkshopID(+可选名)→ `add`。
  - 「更新 mod」按钮 → `update`,禁用态 during 更新;内嵌复用 SteamCMD 日志查看(现有 SSE 组件/逻辑)。
  - FR8:`server.status === 'running'` 且本 tab 有变更时显示「需重启生效」提示条。
  - FR9:某 mod `IsServer` 非 true 时行内警告图标(后端在更新结果里带该标记,或前端据缺 PackageName 提示——MVP 用后端 last_error 文本 + 行级 version 空判断,精细化留后续)。
- Mods tab 顶部常驻一条**下载前置说明**:需在 config 配 `steam_username`(拥有 Palworld 的账号)并先在终端手动 `steamcmd +login <账号>` 一次(D6);更新失败时错误区展示后端返回的可读指引。
- i18n:`messages/{zh,en,ja}.json` 补 `serverConfig.tabs.mods` 与 `serverConfig.mods.*`(add/update/enabled/remove/restartNeeded/incompatible/empty/error/loginHint)。

## 8. 兼容性 / 回滚

- **加法式变更**:DB 增两列(AutoMigrate)、新增 1 路由与 palmod 包、前端新增 tab —— 均不改动既有行为。
- **回滚**:代码回退即可;残留的 `package_name`/`version` 列与磁盘上的 `Mods/Workshop/*` 内容无害(不被旧代码读取)。
- **Windows-only**:部署/INI 路径以 `filepath` 组装,Linux 分支不在本任务验证(prd Out of Scope);不主动破坏 Linux 编译(`go build` 双平台仍需过)。

## 9. 已知风险(实现期须落地验证)

1. **Info.json 结构未见真实样本** → 容忍式解析 + 实现期用一个真实 mod 核对键名;失败不 panic。
2. ~~匿名下载可能失败~~ **已在真机证实为必然失败**(2026-07-15)→ 解决:D6 复用预登录会话 + `steam_username` 配置(见 §3)。失败文案指引用户配置并完成一次性手动登录。
3. **PalModSettings.ini 重复键** → 自定义面向行读写,不用会折叠重复键的 INI 库。
4. **ini 首次由服务器生成** → 本工具在缺失时主动创建(含 `MkdirAll <installPath>/Mods`)。

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

**用户名来源(D7 后)**:`steam_username` 以 **DB settings 为准**(运行时可改),`config.Config.SteamUsername`(`yaml:"steam_username" env:"STEAM_USERNAME"`)仅作**回退默认**。`Manager.UpdateMods` 下载前调用 `resolveSteamUsername()`:先读 DB settings,空则用 config 值,再传给 `DownloadWorkshopItem`。**已证实匿名对 Palworld 不可用**,用户名 + 已缓存会话是下载前提。详见 §10。

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
- Mods tab 顶部 **Steam 账号区块**(详见 §10 前端部分),取代原静态 loginHint。
- i18n:`messages/{zh,en,ja}.json` 补 `serverConfig.tabs.mods`、`serverConfig.mods.*`(add/update/enabled/remove/restartNeeded/incompatible/empty/error)与 `serverConfig.steam.*`(见 §10)。

## 8. 兼容性 / 回滚

- **加法式变更**:DB 增两列(AutoMigrate)、新增 1 路由与 palmod 包、前端新增 tab —— 均不改动既有行为。
- **回滚**:代码回退即可;残留的 `package_name`/`version` 列与磁盘上的 `Mods/Workshop/*` 内容无害(不被旧代码读取)。
- **Windows-only**:部署/INI 路径以 `filepath` 组装,Linux 分支不在本任务验证(prd Out of Scope);不主动破坏 Linux 编译(`go build` 双平台仍需过)。

## 9. 已知风险(实现期须落地验证)

1. **Info.json 结构未见真实样本** → 容忍式解析 + 实现期用一个真实 mod 核对键名;失败不 panic。
2. ~~匿名下载可能失败~~ **已在真机证实为必然失败**(2026-07-15)→ 解决:D6 复用预登录会话 + `steam_username` 配置(见 §3)。失败文案指引用户配置并完成一次性手动登录。
3. **PalModSettings.ini 重复键** → 自定义面向行读写,不用会折叠重复键的 INI 库。
4. **ini 首次由服务器生成** → 本工具在缺失时主动创建(含 `MkdirAll <installPath>/Mods`)。
5. **SteamCMD 登录输出解析(D7 新增,最高风险)** → 判定 success/needGuard/badCredentials 依赖解析 steamcmd stdout 文案,不同版本/语言可能变化;实现用**多关键字容错匹配** + 落地校验(登录后能否真正下载才是终判),真机核对文案。见 §10。

---

## 10. Steam 应用内登录(D7)

> 取代 D6 的"手动终端建立会话"。下载路径(§3,`+login <username>` 复用缓存会话)不变;本节只新增"如何在前端建立那个会话 + 运行时配置用户名"。

### 10.1 存储(运行时可改,不存密码)

- 新增 KV 表:`models.Setting{ Key string gorm:"column:key;primaryKey"; Value string gorm:"column:value" }`,加入 AutoMigrate。
- 新增 `internal/settings`(或 api 内小 helper):`Get(db, key) (string, error)` / `Set(db, key, val) error`。键:`steam_username`、`steam_session_ready`(`"true"`/空)。
- **密码永不入库**:登录 handler 拿到 password 仅传给 steamcmd 子进程,函数返回即丢弃;不写 DB、不写日志、不进 last_error、不回显响应。
- 下载用户名解析:`Manager` 需 db 句柄(已有 `m.db`)→ `resolveSteamUsername()`:DB `steam_username` 优先,空则 `m.steamUsername`(config 回退)。

### 10.2 后端登录(无需伪终端)

`internal/steamcmd/login.go`:

```go
type LoginResult int
const ( LoginSuccess LoginResult = iota; LoginNeedGuard; LoginBadCredentials; LoginError )

// Login 跑 `steamcmd +login <user> <pass> [<guardCode>] +quit` 并解析结果。
// guardCode 为空则不传第三参。stdin 接 /dev/null(空)避免交互阻塞;带
// context 超时防挂。out 收集 steamcmd 输出(不含密码——密码在 args,不写入 out;
// 调用方也不得记录 args)。
func Login(ctx, steamcmdPath, username, password, guardCode string, out io.Writer) (LoginResult, error)
```

- **Guard 码作 `+login` 第三参**,免 pty/prompt 侦测。
- 输出判定(多关键字容错,真机核对):
  - success:`Waiting for user info...OK` / `Logged in OK`;
  - needGuard:`Steam Guard`、`Two-factor code`、`Account Logon Denied`(邮件码此时已发出);
  - badCredentials:`Invalid Password`、`Login Failure`(且非 Guard)。
- 匿名下载已知失败,故 Login 只用于真实账号。

### 10.3 API(全局,非 per-server)

新增 `protected` 组下 `/steam`:

| 方法/路径 | 语义 | 响应 |
|---|---|---|
| GET `/api/steam/status` | 读 DB | 200 `{username, sessionReady}` |
| POST `/api/steam/login` | body `{username, password, guardCode?}` → `steamcmd.Login`(**同步**,登录短;context 超时 ~60s) | 200 `{result:"success"|"needGuard"|"badCredentials"|"error", message}` |

- 成功:`Set(steam_username)` + `Set(steam_session_ready,"true")`,返回 success。
- needGuard:返回 needGuard(前端展示验证码输入)。
- **响应与日志绝不含 password**;登录 steamcmd 输出可选返回(须确认不含敏感信息)或仅返回归一 message。
- 登录同步执行(几秒),不复用 SSE;失败/超时归一为 error + 可读 message。

### 10.4 前端(Mods tab 内 Steam 账号区块)

- `lib/api.ts`:`steamApi = { status(), login({username,password,guardCode?}) }`。
- `ModsSection` 顶部 `SteamAccountSection`:
  - `useQuery` 拉 `status` → 显示:未配置 / 已登录(username + 绿标)/ 需登录。
  - 「登录 / 重新登录」按钮开弹窗:username、password;提交后若 `needGuard` → 显示 Guard 码输入(提示查邮箱或手机验证器)+ 「密码不会被保存」说明 → 带码重提交;`badCredentials`/`error` → 行内报错。成功关闭弹窗、刷新 status。
  - 「更新 mod」在 `sessionReady=false` 时禁用并提示先登录。
- i18n `serverConfig.steam.*`:title/status(notConfigured/loggedIn/needLogin)/login/relogin/username/password/guardCode/guardHint/passwordNotStored/success/badCredentials/error。

### 10.5 安全说明

- 现有 `authMiddleware` 处于注释关闭状态,`/api` 实际未鉴权;向未鉴权本地端点提交 Steam 密码有风险,但与现有姿态一致且默认绑 `127.0.0.1`。**不在本任务打开鉴权**(超范围),仅确保密码不落盘/不日志。
- 若将来开启鉴权,登录端点自动受益。

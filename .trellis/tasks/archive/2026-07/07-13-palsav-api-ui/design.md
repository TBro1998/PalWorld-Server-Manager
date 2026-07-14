# 设计:存档数据 REST 接口与前端展示

> 面向 `prd.md`。契约以子任务 A 的 `internal/palsave` 公开 API 与现有 `internal/api` 约定为准。

## 1. 边界与分层

```
前端 PlayersSection (guilds/pals/inventory tabs)
        │  apiClient (/api/servers/:id/save/*)
        ▼
internal/api/save_handlers.go   ← 新增:HTTP handler + DTO 映射
        │
        ├─ save_cache.go        ← 新增:按 serverID+mtime 缓存 *palsave.Level
        └─ internal/palsave     ← 子任务 A(只读复用,不改)
                 └─ locate.go   ← 新增:World 目录 / 玩家 sav 路径定位
```

- `internal/palsave` 保持"纯解析"职责;**文件系统定位**(World 目录、玩家文件名映射)属存档领域知识,新增到 `palsave/locate.go`,但不引入对 `models`/`api` 的依赖。
- 缓存与 DTO 映射属 API 层职责,放 `internal/api`。

## 2. 存档路径定位(`internal/palsave/locate.go`,R2)

```go
// LocateWorld 在 installPath 下定位存档 world 目录,返回 Level.sav 路径与 Players 目录。
// 布局:<installPath>/Pal/Saved/SaveGames/0/<worldid>/{Level.sav, Players/}
func LocateWorld(installPath string) (levelPath, playersDir string, err error)
// ErrNoSave:SaveGames/0 不存在或无任何含 Level.sav 的子目录。
var ErrNoSave = errors.New("palsave: no save found")

// PlayerSaveFile 由 PlayerUId 推导玩家存档文件名。
// PlayerUId 形如 "AABBCCDD-0000-0000-0000-000000000000";
// 文件名规则:取 UID 去连字符后大写十六进制 + ".sav"(与游戏一致)。
func PlayerSaveFile(uid string) string

// ResolvePlayerSave 在 playersDir 下解析某 uid 的 .sav 路径:
// 先按 PlayerSaveFile 精确匹配;失败则扫描目录做大小写不敏感回退;仍无则 ErrNoSave。
func ResolvePlayerSave(playersDir, uid string) (string, error)
```

- world 目录选择:遍历 `SaveGames/0/*`,选第一个存在 `Level.sav` 的子目录(通常唯一)。
- 全部用 `path/filepath`,Windows 优先;不写文件。
- 单元测试用 `internal/palsave/testdata`(已有 `Level.sav`/`Player.sav`)构造临时目录布局验证。

## 3. 缓存(`internal/api/save_cache.go`,R3)

```go
type saveCache struct {
    mu      sync.Mutex
    entries map[int64]*saveEntry // key: serverID
}
type saveEntry struct {
    levelPath string
    modTime   time.Time
    size      int64
    level     *palsave.Level
}

// Level 返回缓存的 *palsave.Level;当 levelPath 的 mtime/size 变化或无缓存时重解析。
func (c *saveCache) Level(serverID int64, levelPath string) (*palsave.Level, error)
```

- 失效判据:`os.Stat(levelPath)` 的 `ModTime` 且 `Size` 与缓存一致 → 命中;否则 `palsave.LoadLevel` 重建。
- 玩家 `.sav`:体量小,**不进长期缓存**,handler 内 `palsave.LoadPlayer` 按需解析。
- `saveCache` 挂到 `Router` 上(与 `process`/`streams` 并列),`NewRouter` 初始化 `entries: map{}`。
- 命中判定用于 AC3:测试可通过"改 mtime 前后指针/解析次数"验证(见 implement 校验)。

## 4. REST 端点(`internal/api/router.go` + `save_handlers.go`,R1/R5)

在 `servers` 组内新增(紧邻 `rest` 组):

```go
save := servers.Group("/:id/save")
{
    save.GET("/players", r.SavePlayers)
    save.GET("/guilds", r.SaveGuilds)
    save.GET("/players/:uid/pals", r.SavePlayerPals)
    save.GET("/players/:uid/inventory", r.SavePlayerInventory)
}
```

- Gin 参数命名:第一段沿用 `:id`;`:uid` 仅出现在 `/save/players/` 下,与 `/mods/:modId` 不冲突。

### 4.1 通用解析 helper

```go
// saveResolve 解析 :id → server → world 路径 → 缓存的 *Level。
// 失败时已写好响应,返回 ok=false 由 caller 直接 return。
func (r *Router) saveResolve(c *gin.Context) (server models.Server, levelPath, playersDir string, level *palsave.Level, ok bool)
```

错误映射(结构化 `gin.H{"error": msg}`,前端据 HTTP 码区分):

| 情况 | HTTP | 说明 |
|------|------|------|
| `:id` 非法 | 400 | Invalid server ID |
| server 不存在 | 404 | Server not found |
| 未安装 / `LocateWorld` 返回 `ErrNoSave` | 404 | Save not found |
| `LoadLevel`/`LoadPlayer` 解析失败 | 500 | Failed to parse save |
| `:uid` 无对应玩家/存档 | 404 | Player save not found |

### 4.2 响应 DTO(显式 json 标签,lowerCamel,匹配前端约定)

palsave 结构体无 json tag(会序列化成 PascalCase),故 handler 层显式映射:

```go
type savePlayerDTO struct {
    UID       string `json:"uid"`
    InstanceID string `json:"instanceId"`
    Name      string `json:"name"`
    Level     int    `json:"level"`
    Exp       int64  `json:"exp"`
    GuildID   string `json:"guildId"`
    GuildName string `json:"guildName,omitempty"` // 由 Level.Guilds 反查
}
type savePalDTO struct {
    InstanceID string   `json:"instanceId"`
    OwnerUID   string   `json:"ownerUid"`
    Species    string   `json:"species"`     // CharacterID
    Name       string   `json:"name"`
    Level      int      `json:"level"`
    Gender     string   `json:"gender"`
    Rank       int      `json:"rank"`
    Talent     struct{ HP, Melee, Shot, Defense int } `json:"talent"`
    Passives   []string `json:"passives"`
}
type saveGuildMemberDTO struct {
    UID string `json:"uid"`; Name string `json:"name"`
    Role int `json:"role"`; LastOnline int64 `json:"lastOnline"`
}
type saveGuildDTO struct {
    GuildID string `json:"guildId"`; Name string `json:"name"`
    BaseCampLevel int `json:"baseCampLevel"`; AdminUID string `json:"adminUid"`
    Members []saveGuildMemberDTO `json:"members"`
}
type saveItemDTO struct {
    Container string `json:"container"`; Slot int `json:"slot"`
    Count int `json:"count"`; StaticID string `json:"staticId"`
    ItemType string `json:"itemType,omitempty"`; Durability float64 `json:"durability,omitempty"`
    Passives []string `json:"passives,omitempty"`
}
// 端点顶层包一层对象(便于扩展 meta),如 {"players": [...]} / {"guilds": [...]}
// {"pals": [...]} / {"inventory": {"<container>": [saveItemDTO...]}}
```

- `SavePlayers`:`level.Players` → DTO;`GuildName` 由 `level.Guilds` 的 `GroupID→GuildName` map 反查。
- `SaveGuilds`:`level.Guilds` → DTO。
- `SavePlayerPals`:`level.Pals` 中 `OwnerUID == uid` 过滤 → DTO(帕鲁全在 Level.sav,无需读玩家文件)。
- `SavePlayerInventory`:`ResolvePlayerSave`→`LoadPlayer`→`level.AttachInventories([]*Player{pl})`→ `pl.Inventory.Items` → DTO(按 container 分组)。

## 5. 前端(`ui/src`,R4)

### 5.1 API 与类型
- `lib/api.ts` `serversApi` 增:
  ```ts
  savePlayers: (id) => apiClient.get<SavePlayers>(`/api/servers/${id}/save/players`),
  saveGuilds:  (id) => apiClient.get<SaveGuilds>(`/api/servers/${id}/save/guilds`),
  savePals:    (id, uid) => apiClient.get<SavePals>(`/api/servers/${id}/save/players/${uid}/pals`),
  saveInventory:(id, uid) => apiClient.get<SaveInventory>(`/api/servers/${id}/save/players/${uid}/inventory`),
  ```
- `types/server.ts` 增 `SavePlayer/SavePal/SaveGuild/SaveItem` 及包裹类型,与后端 DTO 字段一一对应。

### 5.2 组件(改 `PlayersSection.tsx`,不新增 section)
- `guilds` tab:react-query 拉 `saveGuilds`;渲染公会卡片/表格 + 成员子表(角色、最后在线时间格式化)。
- `pals` tab:先拉 `savePlayers` 填玩家下拉(shadcn `Select`);选中 uid 后拉 `savePals(id, uid)`;表格展示物种/等级/性别/评级/天赋/被动。
- `inventory` tab:同样玩家下拉;选中后拉 `saveInventory`;按 container 分组展示物品。
- 玩家下拉数据(`savePlayers`)在 `pals`/`inventory` tab 间可共享同一 query key `['save-players', serverId]`。
- 加载/空/错误:沿用 `Placeholder`、`getApiErrorMessage`;区分 404「无存档」提示与 500 错误提示。
- 存档解析与 REST 在线状态无关 → **不依赖 `useRestStatus`**;这三个 tab 只要服务器有存档即可用(离线也能看)。

### 5.3 i18n
- `messages/{zh,en,ja}.json` 的 `serverManage.players` 下补:`guilds.*`、`pals.*`、`inventory.*`(表头、选择器占位、空态、无存档提示、错误)。tab 标签键已存在。

## 6. 兼容性与回滚

- 纯新增:新增文件 + `router.go` 加一个子组 + 前端补 tab 内容与三语键。不改 A 的 `palsave` 现有代码、不改数据库、不改现有端点。
- 回滚:移除 `/save` 子组注册即禁用后端;前端 tab 回退占位。互不影响既有功能。

## 7. 关键风险

- **玩家文件名映射**(R2):UID→文件名规则若不完全一致会 404 → 用"精确匹配 + 目录扫描回退"双保险,并加单测。
- **DTO 与 A 结构体漂移**:A 若后续改字段,DTO 映射需同步 → 映射集中在 `save_handlers.go` 一处,便于维护。
- **大存档解析耗时**:首次请求可能慢 → 缓存兜底;必要时前端加 loading 态(已含)。

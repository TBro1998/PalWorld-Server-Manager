# Save-File Handling Guidelines

> 解析 Palworld 存档（Level.sav / Players/*.sav）并经 REST 暴露的约定。核心库见 `internal/palsave`，API 层见 `internal/api/save_*.go`。

---

## Overview

- 纯 Go 解析库 `internal/palsave`：`LoadLevel(path) → *Level{Players,Pals,Guilds}`、`LoadPlayer(path) → *Player`、`(*Level).AttachInventories([]*Player)` 交叉引用物品容器/动态物品填充背包。库**只解析，不做文件系统定位、不依赖 models/api**。
- 存档**只读**：任何路径操作都不得写入存档文件。
- Windows 优先：路径一律用 `path/filepath`。

---

## Convention: 存档定位集中在 `palsave.LocateWorld`，不要在 handler 里手拼路径

**What**: 由服务器 `InstallPath` 定位存档用 `palsave.LocateWorld(installPath) (levelPath, playersDir, err)`。布局固定为：

```
<InstallPath>/Pal/Saved/SaveGames/0/<worldid>/{Level.sav, Players/*.sav}
```

`<worldid>` 通常单一子目录；`LocateWorld` 遍历 `SaveGames/0/*` 挑第一个含 `Level.sav` 的子目录。缺失时返回 `palsave.ErrNoSave`。

**Why**: 现有代码只有 Config 目录 helper（`palconfig` 的 `<install>/Pal/Saved/Config/<OS>Server`），World 存档目录此前无定位逻辑（服务器运行日志现改为直接接管进程 stdout/stderr，不再有 `Pal.log` 路径 helper）。集中一处避免各 handler 各拼一遍、口径漂移。

**Rule**: 新增任何读存档的功能（备份、地图、统计…）都走 `LocateWorld`，不要再手写 `SaveGames/0` 路径。

---

## Gotcha（已固化）: 玩家 .sav 文件名 = PlayerUId 去连字符大写十六进制

**What**: `Players/` 下的玩家存档文件名由 PlayerUId 派生：`palsave.PlayerSaveFile(uid)` = 去掉连字符 + 全大写 + `.sav`。GVAS reader 产出的 UID 是**小写带连字符**（`formatGUID`，见 `gvas/reader.go`），例：

```
"aabbccdd-0000-0000-0000-000000000000" -> "AABBCCDD000000000000000000000000.sav"
```

`palsave.ResolvePlayerSave(playersDir, uid)` 先按 `PlayerSaveFile` 精确匹配，失败再**大小写不敏感扫描目录**回退（防大小写/格式漂移），仍无则 `ErrNoSave`。

**Why**: UID 字符串形态（小写带连字符）与磁盘文件名形态（大写无连字符）不一致，直接用 UID 拼文件名会 404。双保险（精确 + 扫描回退）覆盖大小写差异。

**Rule**: 需要按 UID 读玩家文件时用 `ResolvePlayerSave`，不要用 `uid + ".sav"` 直接拼。

---

## Convention: `Level.sav` 解析结果按 serverID + mtime + size 缓存

**What**: `internal/api/save_cache.go` 的 `saveCache.Level(serverID, levelPath)` 缓存 `*palsave.Level`；命中判据为 `os.Stat` 的 `ModTime` **且** `Size` 均未变。玩家 `.sav` 体量小，**不缓存**，按需 `LoadPlayer`。缓存挂在 `Router.saves`（与 `process`/`streams` 并列），`NewRouter` 初始化。

**Why**: `Level.sav` 可达数百 KB，全量 GVAS 解析有成本；每请求重解析不可接受（PRD R3）。mtime+size 双条件避免"改后大小相同"的漏判。

**Concurrency Gotcha**: 缓存返回**共享** `*Level`。`AttachInventories` 只**读** Level 的 `itemContainers`/`dynamicItems`（`LoadLevel` 一次性建好、之后不改），只写**每请求**的 `Player`，故并发 inventory 请求无竞态。若将来给 `Level` 增加请求期可变状态，需重新评估共享安全性。

---

## Convention: palsave 结构体无 json tag → API 层显式 DTO 映射

**What**: `palsave` 的 `Player/Pal/Guild/Item` 无 `json` tag（直接序列化会得到 PascalCase）。`internal/api/save_handlers.go` 显式定义 lowerCamel 的 `savePlayerDTO/savePalDTO/saveGuildDTO/saveItemDTO` 并逐字段映射，端点顶层包一层对象（`{"players":[...]}` / `{"guilds":[...]}` / `{"pals":[...]}` / `{"inventory":{"<container>":[...]}}`）。

**Why**: 与前端既有约定（如 `PalPlayers` 的 lowerCamel）一致；映射集中一处，隔离 palsave 字段改名对前端契约的冲击。前端 `ui/src/types/server.ts` 的 `Save*` 类型必须与这些 DTO 逐字段对齐。

**Rule**: 改任一侧字段（palsave 结构体 / DTO / 前端 `Save*` 类型）时三处同步；`lastOnline` 是 Unreal FDateTime ticks（100ns since 0001-01-01），前端 `formatTicks`（`ticks/10000 - 62135596800000`）转 Unix ms。

---

## Convention: `/save` 端点独立于实时 REST，服务器停机也可用

**What**: 存档端点 `GET /api/servers/:id/save/{players,guilds,players/:uid/pals,players/:uid/inventory}`（`protected` 组，参数沿用 `:id`）解析磁盘存档，**不**依赖游戏进程或官方 REST API 在线。错误码：400（id 非法）/ 404（server 不存在、`ErrNoSave` 无存档、玩家存档缺失）/ 500（解析失败），均为结构化 `gin.H{"error": msg}`。

**Why**: 离线也能查玩家/公会/帕鲁/背包。前端 `PlayersSection` 的 `guilds/pals/inventory` 三 tab 因此**不**挂 `useRestStatus`（只有实时 `players` tab 依赖在线 REST）；前端据 `AxiosError.response.status === 404` 区分"无存档"与真错误。

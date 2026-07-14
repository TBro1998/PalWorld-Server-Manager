# 存档数据 REST 接口与前端展示

> 父任务:`07-13-palworld-sav-reader-go`。**依赖子任务 A `palsav-core-lib`**(已完成:提供公开结构体 `Player/Pal/Guild/Item/Inventory` 与 `LoadLevel/LoadPlayer/AttachInventories`)。本 PRD 已在 A 稳定后细化。

## Goal

把子任务 A(`internal/palsave`)解析出的玩家/公会/背包/帕鲁数据,通过 REST API 暴露,并在前端"服务器管理"界面展示。

## 现状对接点(A 完成后确认)

- **核心库能力**(`internal/palsave`):
  - `LoadLevel(path)` → `*Level{Players, Pals, Guilds}`(全量:含离线玩家、所有帕鲁、所有公会)。
  - `LoadPlayer(path)` → `*Player`(填充 `Inventory` 各容器 ID,尚无物品内容)。
  - `(*Level).AttachInventories([]*Player)` → 交叉引用 Level 的物品容器与动态物品,填充每个玩家 `Inventory.Items`。
- **后端 API 约定**(`internal/api/router.go`):路由参数统一为 `:id`(非 `:serverId`);`protected` 组;handler 内 `strconv.ParseInt(c.Param("id"))` → GORM 查 `models.Server` → 结构化 `gin.H{"error": ...}`。参考 `rest_handlers.go` 的 resolve/错误映射风格。
- **服务器安装路径**:`models.Server.InstallPath`。存档位于 `<InstallPath>/Pal/Saved/SaveGames/0/<worldid>/{Level.sav, Players/*.sav}`(现有代码仅有 Config/Logs 目录 helper,World 目录定位需新增)。
- **前端接入点**:`ui/src/components/server-manage/PlayersSection.tsx` 已预留 `players | guilds | pals | inventory` 四个 tab,仅 `players`(实时 REST 在线玩家)已实现,其余为 "coming soon" 占位。**本任务填充 `guilds/pals/inventory` 三个 tab 的存档数据视图**,不新增顶层 section。

## Requirements

- **R1 REST 端点**(Gin `protected` 组,新增 `/:id/save` 子组):
  - `GET /api/servers/:id/save/players` — 存档中全部玩家(UID、昵称、等级、经验、所属公会 ID/名)。用于列表与"选择玩家"下拉。
  - `GET /api/servers/:id/save/guilds` — 公会列表(名称、据点等级、会长、成员及其角色/最后在线)。
  - `GET /api/servers/:id/save/players/:uid/pals` — 指定玩家拥有的帕鲁列表(物种、昵称、等级、性别、评级、词条、被动技能)。
  - `GET /api/servers/:id/save/players/:uid/inventory` — 指定玩家背包(按容器分组的物品:静态 ID、数量、耐久、动态词条等)。
- **R2 服务端定位存档路径**:由 `:id` → `Server.InstallPath` → 定位 `SaveGames/0/<worldid>/`。`<worldid>` 通常单一子目录;需容错(挑含 `Level.sav` 的子目录)。玩家 `.sav` 文件名由 PlayerUId 派生(去连字符、大写十六进制),精确匹配失败时回退到 `Players/` 目录扫描。
- **R3 解析缓存**:`Level.sav` 大、解析有成本 → 按 `serverID + Level.sav mtime` 缓存 `*palsave.Level`;mtime 未变则复用,避免每请求全量解析。玩家 `.sav` 小,可按需解析。
- **R4 前端页面**:在 `PlayersSection` 的 `guilds/pals/inventory` tab 渲染存档数据。`pals`/`inventory` 为按玩家维度 → 提供玩家选择器(数据来自 `/save/players`)。走 `apiClient`;i18n 三语(zh/en/ja);shadcn/ui;沿用现有表格/占位/错误展示风格。
- **R5 错误与边界**:服务器不存在(404)、未安装/无存档(404,可提示"未找到存档")、解析失败(500)。返回结构化错误,前端可区分"无存档"与"真错误"并给出提示。

## Acceptance Criteria

- [ ] AC1 四个端点针对真实存档(`internal/palsave/testdata` 或真实服务器)返回正确 JSON,字段来源与 A 结构体一致。
- [ ] AC2 前端 `guilds` tab 列出公会+成员;`pals`/`inventory` tab 选择玩家后展示其帕鲁与背包。
- [ ] AC3 存档未变时二次请求命中缓存(不重复全量解析 `Level.sav`);存档文件更新(mtime 变化)后自动失效重解析。
- [ ] AC4 后端 `go build ./...` / `go vet ./...` / 相关 `go test` 通过;前端 `cd ui && bun run build` 通过并可嵌入后端;i18n 三语键齐全。
- [ ] AC5 无存档/损坏存档时端点返回结构化错误,前端展示友好提示而非崩溃。

## 约束

- 遵循 `CLAUDE.md`:前端静态导出 + `embed.FS` 嵌入;API 相对路径;`protected` 组(JWT 中间件当前注释,沿用现状);Windows 优先(路径用 `filepath`)。
- **只读**:不修改任何存档文件。
- 复用现有约定:handler 的 `:id` 解析/DB 查询/错误 JSON 风格;前端 `apiClient` + react-query + `useServerId()` + `SectionShell`。

## Notes

- 端点未合并为单一 overview:帕鲁/背包是按玩家维度且体量大,分端点更符合前端分 tab + 按需加载。`/save/players` 同时充当列表与选择器数据源。
- 玩家 `.sav` 文件名 → PlayerUId 的映射是已知易错点(见 R2),design 中固化转换规则并保留扫描回退。

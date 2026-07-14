# 执行计划:存档数据 REST 接口与前端展示

> 依据 `design.md`。顺序执行;每阶段末给校验命令与评审点。后端先行(前端依赖其契约)。

## 阶段 1 — 存档定位(palsave/locate.go)

- [ ] 1.1 新增 `internal/palsave/locate.go`:`ErrNoSave`、`LocateWorld`、`PlayerSaveFile`、`ResolvePlayerSave`(见 design §2)。
- [ ] 1.2 新增 `internal/palsave/locate_test.go`:用 `testdata` 在临时目录搭 `Pal/Saved/SaveGames/0/<world>/{Level.sav,Players/<file>.sav}` 布局,验证:
  - `LocateWorld` 命中含 Level.sav 的子目录;无存档返回 `ErrNoSave`。
  - `PlayerSaveFile` 对样例 UID 产出去连字符大写十六进制文件名。
  - `ResolvePlayerSave` 精确命中 + 大小写回退。
- **校验**:`go test ./internal/palsave/... -run 'Locate|PlayerSave|ResolvePlayer'`
- **评审点 A**:定位规则(尤其玩家文件名映射)确认无误后再继续。

## 阶段 2 — 缓存(api/save_cache.go)

- [ ] 2.1 新增 `internal/api/save_cache.go`:`saveCache`/`saveEntry` + `Level(serverID, levelPath)`(mtime+size 失效,见 design §3)。
- [ ] 2.2 `router.go` `Router` 加 `saves *saveCache` 字段;`NewRouter` 初始化。
- **校验**:`go build ./internal/api/...`

## 阶段 3 — REST handler(api/save_handlers.go + router.go)

- [ ] 3.1 新增 `internal/api/save_handlers.go`:DTO 类型 + `saveResolve` helper + 四个 handler(见 design §4)。
- [ ] 3.2 `router.go` 在 `servers` 组注册 `/:id/save` 子组四端点。
- [ ] 3.3 错误码按 design §4.1 表映射;`inventory` 走 `LoadPlayer`+`AttachInventories`。
- **校验**:`go build ./...` && `go vet ./...`
- **评审点 B**:端点契约(路径/DTO 字段/错误码)确认后再动前端。

## 阶段 4 — 后端联调校验

- [ ] 4.1(可选)新增 `internal/api/save_handlers_test.go`:用 testdata 存档起 gin 测试路由,断言四端点 JSON 结构与关键字段;断言无存档→404。
- [ ] 4.2 若 4.1 成本高,则改为手动:临时把 testdata 摆成 world 布局,`go run .` 后 curl 四端点核对(记录到 journal)。
- **校验**:`go test ./internal/... `(至少 palsave + 新增测试)

## 阶段 5 — 前端 API 与类型

- [ ] 5.1 `ui/src/types/server.ts` 增 `SavePlayer/SavePal/SaveGuild/SaveGuildMember/SaveItem` 及包裹类型(对齐后端 DTO)。
- [ ] 5.2 `ui/src/lib/api.ts` `serversApi` 增 `savePlayers/saveGuilds/savePals/saveInventory`。

## 阶段 6 — 前端视图(PlayersSection 三 tab)

- [ ] 6.1 `guilds` tab:`saveGuilds` query → 公会 + 成员表(角色/最后在线格式化)。
- [ ] 6.2 `pals` tab:玩家 `Select`(`savePlayers`)→ `savePals(uid)` 表格。
- [ ] 6.3 `inventory` tab:玩家 `Select` → `saveInventory(uid)` 按容器分组。
- [ ] 6.4 三 tab 去掉 `RestUnavailableNotice` 依赖(存档不依赖在线 REST);loading/空/404/错误态用 `Placeholder`+`getApiErrorMessage`。
- **校验**:`cd ui && bun run lint`

## 阶段 7 — i18n 三语

- [ ] 7.1 `messages/zh.json`、`en.json`、`ja.json` 的 `serverManage.players` 下补 `guilds/pals/inventory` 明细键(表头、选择器、空态、无存档、错误)。
- **校验**:三文件键结构一致(可 `python` 比对 keys)。

## 阶段 8 — 构建与整合验证

- [ ] 8.1 `cd ui && bun run build`(静态导出)。
- [ ] 8.2 回项目根 `go build .`(嵌入前端)。
- [ ] 8.3 端到端手验(有真实/临时存档时):打开服务器管理 → guilds/pals/inventory 三 tab 数据正确;二次进入命中缓存;无存档时友好提示。
- **校验**:`go build ./...` && `go vet ./...` && `cd ui && bun run build`

## 回滚点

- 阶段 3 后若端点方案有问题:仅回退 `router.go` 子组注册。
- 阶段 6 后若前端有问题:tab 内容回退占位,不影响后端端点与其他 tab。

## 完成判据(对齐 prd AC)

- AC1 四端点 JSON 正确(阶段 4)。
- AC2 三 tab 展示正确(阶段 8.3)。
- AC3 缓存命中/失效(阶段 2 + 8.3)。
- AC4 go build/vet/test + bun build 通过 + 三语齐全(阶段 7/8)。
- AC5 无存档/损坏结构化错误 + 前端提示(阶段 3/6/8.3)。

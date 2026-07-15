# Implement — Mod 管理与 SteamCMD 更新

> 执行顺序:自底向上(纯逻辑 → steamcmd → manager → api → 前端 → i18n),每层可独立编译/测试。契约见 design.md。

## 阶段 0 — 数据模型(加字段)

- [ ] `internal/models/server.go`:`Mod` 追加 `PackageName`、`Version` 列(design §2)。
- **验证**:`go build ./...`;启动一次让 AutoMigrate 增列(或 `go test ./internal/database/`)。
- **门禁 G0**:编译通过,mods 表含新列。

## 阶段 1 — palmod 新包(纯逻辑 + 单测)

- [ ] `internal/palmod/info.go`:`Info` 结构 + `ParseInfo(dir)`,容忍式(缺字段/大小写不 panic),`IsServer` 缺省 false。
- [ ] `internal/palmod/deploy.go`:`Deploy(installPath, workshopID, srcDir)`(先清目标后递归复制)+ `Remove(installPath, workshopID)`。
- [ ] `internal/palmod/modsettings.go`:`WriteModSettings(installPath, enabled)` —— 面向行读写 `[PalModSettings]`,置 `bGlobalEnableMod=true`,重建 `ActiveModList=` 行,保留其它内容;缺文件则新建 + `MkdirAll`。
- [ ] `internal/palmod/*_test.go`:INI 幂等(重复写不重复行)、启停子集、Remove、ParseInfo 容忍缺字段。用 `t.TempDir()`,无需真实 mod。
- **验证**:`go test ./internal/palmod/`、`go vet ./internal/palmod/`、`gofmt`。
- **门禁 G1**:palmod 单测全绿;INI 幂等性用例通过。

## 阶段 2 — steamcmd workshop 下载(含登录,D6)

- [ ] `internal/config/config.go`:加 `SteamUsername`(`yaml:"steam_username" env:"STEAM_USERNAME"`,默认空);`config.example.yaml` 补示例 + 注释(需拥有 Palworld 的账号 + 一次性手动登录)。
- [ ] `internal/steamcmd/workshop.go`:`DownloadWorkshopItem(steamcmdPath, steamUsername, workshopID, out)`(design §3),`+login <user|anonymous>`,返回落地目录并校验存在;失败文案区分「未配置 steam_username」与「已配置但会话未登录/ID 无效」。
- [ ] 贯通:`process.NewManager` 加 `steamUsername` 参数;`router.go` 的 `NewManager(...)` 传 `cfg.SteamUsername`;`Manager` 存字段。
- **验证**:`go build ./...`、`go vet`。(真实下载在阶段 6 真机验证。)
- **门禁 G2**:编译通过,签名与 InstallPalworldServer 的 out 约定一致;config 三层加载含新字段。

> **回滚记录(2026-07-15)**:真机验证证实匿名下载对 Palworld 必然失败;本阶段由「匿名」改为「预登录会话 + steam_username」(prd D6)。

## 阶段 3 — process.Manager.UpdateMods

- [ ] `internal/process/manager.go`:加 `updatingMods` 集合 + `IsUpdatingMods` + `UpdateMods`(design §4:并发闸→逐 mod 下载/部署/回填→写 ini→last_error 聚合)。运行中不拦截。
- **验证**:`go build ./...`、`go vet ./internal/process/`。
- **门禁 G3**:编译通过;并发闸逻辑与 InstallServer 对齐(installing/updatingMods 互斥)。

## 阶段 4 — API 层

- [ ] `internal/api/handlers.go`:实现 `ListMods`(按 server_id 查)、`InstallMod`(仅建行,body `{workshopId,name?}`,校验非空)、`UninstallMod`(删行+`palmod.Remove`+重写 ini)、`ToggleMod`(翻转+重写 ini)、新增 `UpdateMods`(复用 InstallServer 异步+日志模式,`ResetLog`+capture+broadcast → `process.UpdateMods`,202)。
- [ ] 校验 `:modId` 归属该 server(防跨服)。
- [ ] `internal/api/router.go`:mods 组新增 `POST("/update", r.UpdateMods)`([router.go:102-108](internal/api/router.go#L102-L108))。
- **验证**:`go build ./...`、`go vet ./internal/api/`;可加 handler 级测试(建/列/删/toggle 走内存 sqlite)。
- **门禁 G4**:5 端点编译通过并按契约返回码;跨服操作被拒。

## 阶段 5 — 前端

- [ ] `ui/src/types/server.ts`:`Mod` 接口。
- [ ] `ui/src/lib/api.ts`:`modsApi`(list/add/remove/toggle/update)。
- [ ] `ui/src/components/ServerSettingsDialog.tsx`:`tabs` 追加 `'mods'`;`ModsSection`(列表/添加/启停/删除/「更新 mod」按钮 + 复用 SteamCMD SSE 日志 + running 时「需重启生效」提示)。
- [ ] `ui/messages/{zh,en,ja}.json`:`serverConfig.tabs.mods` + `serverConfig.mods.*` 三语(含 `loginHint`:配 steam_username + 一次性手动登录说明)。
- [ ] Mods tab 顶部常驻下载前置说明(loginHint)。
- **验证**:`cd ui && bun run lint`(0 warn)、`bun run build`(含 TS 检查)。
- **门禁 G5**:前端 lint/build 通过;三语键齐全(无缺 key 警告)。

## 阶段 6 — 集成 + 嵌入 + 真机验证

- [ ] 根目录 `go build .`(嵌入前端 out)。
- [ ] Windows 真机:创建/选一个已安装服务器 → Mods tab 加一个真实 Workshop ID → 「更新 mod」→ 观察 SSE 日志 → 核对 `<installPath>/Mods/Workshop/<id>/` 有内容、Info.json 解析出 PackageName/Version、`PalModSettings.ini` 含 `bGlobalEnableMod=true` 与 `ActiveModList=<pkg>`。
- [ ] 用真实 Info.json **核对字段名**(design 风险 1),必要时修正 `ParseInfo`。
- [ ] 删除 mod → 目录与 ActiveModList 行消失;toggle 关闭 → ActiveModList 去掉该行(不重新下载)。
- [ ] 失败路径:填一个无效 ID → UI 见可读错误,无半成品 ini 行,不 panic。
- **门禁 G6(全量)**:prd Acceptance Criteria 全部勾选;`go build .`/`go vet ./...`/`go test ./...`/`bun run lint`/`bun run build` 全绿。

## 验证命令汇总

```bash
# 后端
go build ./... && go vet ./... && go test ./internal/palmod/ ./internal/api/ ./internal/process/
# 前端
cd ui && bun run lint && bun run build
# 嵌入
cd .. && go build .
```

## 回滚点

- 每个阶段独立编译,门禁不过则停在该阶段修复,不进入下一层。
- 整体回滚 = git 回退;残留 DB 列与磁盘 Mods/Workshop 内容无害(design §8)。

## 评审门(进入 Execute 前)

- 待用户 review 本三件产物 + jsonl → `task.py start`(status→in_progress)后方可实现。

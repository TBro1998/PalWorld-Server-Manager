# Mod 依赖显示与已下载状态标注

## Goal

Mod 下载完成后，其 `Info.json` 里的 `Dependencies` 字段记录了该 mod 依赖的其他 mod 的
**PackageName**（例：PalSchema 依赖 `UE4SSExperimentalPW`）。当前工具解析 Info.json 时忽略了
这个字段。本任务要把依赖列表展示在 UI 上，并标注每个依赖是否**已在全局库中下载**，帮助用户
在部署前发现"缺依赖"的情况。

两个页面都要显示：
- 全局 Mod 库页（`/mods`）
- 服务器管理的 Mod 页（`ServerSettingsDialog` 的 Mods tab / `ModsSection`）

## Requirements

- R1 后端 `palmod.Info` 增加 `Dependencies []string`；`ParseInfo` 从 Info.json 的
  `Dependencies` 键读取（容忍缺失 → `nil`，与 `Tags` 同构）。
- R2 `models.Mod` 增加 `Dependencies []string` 列（`gorm:"column:dependencies;serializer:json"`，
  与 `Tags` 同构，AutoMigrate 加列）。下载回填元数据时（`process.Manager.DownloadGlobalMod`）
  一并写入 `dependencies`。
- R3 依赖"是否已下载"的判定口径（用户已确认）：**按 PackageName 匹配全局库中 `downloaded=true`
  的 mod**。即构造"已下载包名集合" = 全局库中所有 `downloaded && package_name != ""` 的
  `package_name`；某依赖名在此集合中 → 已满足（✓），否则未满足（⚠）。二态，不区分"在库未下载"。
- R4 API 在两个列表端点各返回每个 mod 的依赖满足情况，避免前端二次请求：
  - `GET /api/mods`（`ListGlobalMods` / `ModWithStatus`）
  - `GET /api/servers/:id/mods`（`ListServerMods` / `ServerModDetail`）
  - 形状：`dependencies: {name: string, satisfied: bool}[]`（运行时计算，不落库该结构；
    仅 `dependencies` 原始名数组落库）。
- R5 前端两个页面渲染依赖列表：每个依赖名一个小徽标，已满足显示 ✓（success 色），
  未满足显示 ⚠（warning 色）+ tooltip 提示"依赖未下载"。无依赖时不渲染该区域。
- R6 跨层字段一致性：DB 列 → Go json tag（snake_case）→ 前端 `ui/src/types/server.ts` →
  i18n（zh/en/ja 三语齐全）四处同步。

## Constraints

- 遵守 `Tags []string` serializer 陷阱：回填 `dependencies` 必须走 `Select`+结构体 `Updates`，
  禁用 `Updates(map)`（见 mod-handling.md）。前端读 `dependencies` 用 `?? []` 容错（后端可能返回 null）。
- 本任务的 `Dependencies` 是 Info.json 内记录的**依赖 PackageName**，与已有的
  `steamworkshop.ResolveDependencies`（下载前按 Workshop ID 走 Steam Web API 递归解析）是
  **两个独立机制**，互不影响、不复用。
- 依赖满足度是运行时派生值，不落库；只有 `dependencies` 原始名数组落库（回填自 Info.json）。
- Windows 为准；纯逻辑（ParseInfo）配单测（`t.TempDir()`）。

## Acceptance Criteria

- [x] AC1 `ParseInfo` 解析 `Dependencies`：有值时正确读入，缺失时为 `nil` 不报错（`TestParseInfoTolerant` + `TestParseInfoNumericVersionAndMissingFields` 覆盖，绿）。
- [x] AC2 下载完成后 `mods.dependencies` 列被回填（`DownloadGlobalMod` Select+结构体 Updates）；`Info.json` 无 `Dependencies` 时为 null。
- [x] AC3 `GET /api/mods`（`ModWithStatus.Dependencies`）与 `GET /api/servers/:id/mods`（`ServerModDetail.Dependencies`）返回满足情况数组；满足度按 `downloadedPackageSet`（`downloaded && package_name != ''`）计算。
- [x] AC4 `/mods` 页 `ModRow` 与 `ModsSection` 的 `ServerModRow` 经共享组件 `ModDependencies` 渲染徽标，已下载 ✓ / 未满足 ⚠，无依赖不渲染。
- [x] AC5 i18n 三语齐全；前端类型对齐（`bun run build` TS 通过）；`go build .` 通过、`palmod` 单测绿、`bun run build` 通过；lint 无新增错误（2 个 error 为 HEAD 既存，不在本次文件）。

## 收尾：修复既存 stale 测试 + 暴露的迁移 bug（用户追加"顺手修/收尾"）

- `internal/api` 测试包在 HEAD 即编译失败：`newTestRouter` 调 2 参 `NewRouter`（缺 `update.BuildInfo`），且受保护路由已加 JWT 但测试不带 token。已修：`newTestRouter` 传 `BuildInfo{}` + `JWTSecret`，新增 `testToken`/`testJWTSecret`，`doJSON`/`doGET` 附 `Authorization: Bearer`。
- `TestModsCRUD` 测的是已废弃的"mod 属于服务器 + Enabled"旧契约。已重写为 `TestServerModsLinkToggleUnlink`（全局库 `POST /api/mods` + 服务器引用 `POST /api/servers/:id/mods` link/toggle/unlink），并新增 `TestGlobalModDependencies` 覆盖满足度计算。
- `database_test.go` 的 `models.Mod{ServerID/Enabled/InstallPath}` 引用旧字段。已改为全局库 Mod 直建，`TestModTagsSerializerRoundTrip` 对齐现回填列并加 `dependencies` round-trip 断言。
- **暴露的生产 bug**：测试恢复编译后，`TestInitializeLegacyModsFK` 首次真正运行，发现 `collectLegacyMods`（`internal/database/database.go`）无条件 SELECT `package_name/mod_name/version`——最老的 per-server `mods` 表没有这些列，导致迁移报 `no such column`。已修：按 `hasRawColumn` 动态拼列，只 SELECT 存在的列。
- 结果：`go build ./...`、`go test ./...` 全绿（首次全量通过）。
- ja.json 原先缺失整个 `modLibrary` 命名空间（`/mods` 页在日语下本就回退为点分 key）；本次为依赖徽标补了 `modLibrary.dependencies*` 两个 key，其余 `modLibrary.*` 仍缺，属既存 i18n 空缺（未在本次扩大范围）。

## Notes

- 真机样本：`steamcmd/steamapps/workshop/content/1623730/3625280368/Info.json`（PalSchema，
  `Dependencies: ["UE4SSExperimentalPW"]`）已确认字段存在。

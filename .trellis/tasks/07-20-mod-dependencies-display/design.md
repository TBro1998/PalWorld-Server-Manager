# 技术设计 — Mod 依赖显示与已下载状态标注

## 数据流

```
Info.json.Dependencies (PackageName[])
  → palmod.ParseInfo → Info.Dependencies []string        [解析层, R1]
  → Manager.DownloadGlobalMod 回填 → mods.dependencies    [编排层, R2]
  → ListGlobalMods / ListServerMods 计算满足度            [API 层, R3/R4]
      satisfiedSet = { package_name | downloaded && package_name != "" }
  → 前端 ModRow / ServerModRow 渲染徽标                    [UI 层, R5]
```

## 分层改动

### 1. 解析层 `internal/palmod/info.go`
- `Info` 结构增加 `Dependencies []string`。
- `rawInfo` 增加 `Dependencies []string \`json:"Dependencies"\``。
- `ParseInfo` 拷贝 `raw.Dependencies` 到 `info.Dependencies`（缺失 → nil，容忍式，与 Tags 同构）。

### 2. 模型层 `internal/models/server.go`
- `Mod` 增加：`Dependencies []string \`json:"dependencies" gorm:"column:dependencies;serializer:json"\``。
- AutoMigrate 加列即可（`internal/database/database.go` 已用 `db.AutoMigrate(&models.Mod{}, ...)`，
  additive，无需手写迁移）。

### 3. 编排层 `internal/process/manager.go`（DownloadGlobalMod）
- 元数据回填的 `Select(...)` 增加 `"dependencies"`，结构体 `Updates(models.Mod{...})` 增加
  `Dependencies: info.Dependencies`。**必须** Select + 结构体路径（serializer 列陷阱）。
- 无 Info.json 的降级分支（map 更新 downloaded/download_path）不涉及 dependencies，保持原样。

### 4. API 层 `internal/api/mod_handlers.go`

新增共享类型与 helper：
```go
// ModDependency 是一个依赖项及其在全局库中的满足状态（运行时派生，不落库）。
type ModDependency struct {
    Name      string `json:"name"`       // 依赖的 PackageName
    Satisfied bool   `json:"satisfied"`  // 全局库中已有同名且 downloaded 的 mod
}
```

- helper `downloadedPackageSet(db) map[string]struct{}`：一次查询全局库
  `SELECT package_name FROM mods WHERE downloaded = 1 AND package_name != ''`，返回集合。
  两个列表端点复用（各自一次，避免 N+1）。
- `ModWithStatus` 增加 `Dependencies []ModDependency \`json:"dependencies"\``。
  `ListGlobalMods`：先取 `downloadedPackageSet`，对每个 mod 把 `m.Dependencies`（原始名）
  映射为 `[]ModDependency`（name + satisfied）。
- `ServerModDetail` 增加 `Dependencies []ModDependency \`json:"dependencies"\``。
  `ListServerMods`：同样先取 `downloadedPackageSet`，对每个 server mod 用其全局 mod 的
  `gm.Dependencies` 映射。
- 命名注意：`ServerModDetail` 已嵌入 `models.ServerMod`，而 `ServerMod` 无 Dependencies 字段，
  但为避免与将来可能的字段冲突并保持"扁平化 join 字段"风格，显式声明 `Dependencies` 于 detail 上，
  由 `gm.Dependencies`（全局 mod）填充。

### 5. 前端类型 `ui/src/types/server.ts`
```ts
export interface ModDependency {
  name: string
  satisfied: boolean
}
```
- `Mod` 增加 `dependencies?: ModDependency[]`（列表端点计算返回；`?` 因其他来源如 AddGlobalMod
  的响应可能不含）。
- `ServerMod` 增加 `dependencies: ModDependency[]`（ListServerMods 恒返回，可能为空数组）。

### 6. 前端渲染
共享一个小组件避免重复：
- 新建 `ui/src/components/server-manage/ModDependencies.tsx`（或就近内联）：
  入参 `deps: ModDependency[] | null | undefined`；空则返回 null；否则渲染一行小徽标，
  满足 ✓（CheckCircle2, text-success）/ 未满足 ⚠（AlertTriangle, text-warning）+ title。
  文案用 i18n key `mods.dependencies` 标题 + `mods.dependencyMissing` tooltip。
- `/mods` 页 `ModRow`：在 tags 行下方插入依赖行（`mod.dependencies`）。
- `ModsSection` 的 `ServerModRow`：在 tags 行下方插入依赖行（`sm.dependencies`）。

### 7. i18n
- 复用 `serverConfig.mods.*` 命名空间（两处都用 `useTranslations('serverConfig')` / `'modLibrary'`）。
  实际两页命名空间不同（`/mods` 用 `modLibrary` + `serverConfig`，ModsSection 用 `serverConfig`），
  共享徽标组件接收翻译函数或用两页都可访问的 key。**决定**：把新增 key 放入 **两个** 命名空间
  （`modLibrary.dependencies*` 和 `serverConfig.mods.dependencies*`），或让共享组件接收 `t` 与 key。
  简化实现：共享组件接收已解析好的 `label` / `missingLabel` 字符串作为 props，各页自行传入。
  新增 i18n key（三语）：
  - `serverConfig.mods.dependencies` = "依赖" / "Dependencies" / "依存"
  - `serverConfig.mods.dependencyMissing` = "该依赖尚未下载" / "This dependency is not downloaded" / "この依存はまだダウンロードされていません"
  - `modLibrary` 侧同样加 `dependencies` / `dependencyMissing`（若 `/mods` 页徽标标题走 modLibrary 命名空间）。

## 兼容性 / 回滚

- AutoMigrate 加列 additive，旧库升级即得空列；旧 mod 行 `dependencies` 为 null，前端 `?? []` 容错。
- 满足度纯运行时计算，无落库结构变化风险。
- 回滚：还原文件即可；已写入的 `dependencies` 列留存无害（前端忽略即可）。

## 风险

1. **依赖名大小写/空格**：PackageName 匹配用精确相等（Info.json 内 PackageName 与 Dependencies
   约定一致）。如遇大小写差异，MVP 不做归一化（真机若出现再固化 spec）。
2. **自依赖 / 重复名**：Dependencies 直接来自 Info.json，可能含重复；渲染层按原样展示（去重可选，
   MVP 不强制）。

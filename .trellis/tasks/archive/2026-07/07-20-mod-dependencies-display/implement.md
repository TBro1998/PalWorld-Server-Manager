# 执行计划 — Mod 依赖显示与已下载状态标注

## 顺序清单

### 后端
- [ ] 1. `internal/palmod/info.go`
  - `Info` 加 `Dependencies []string`。
  - `rawInfo` 加 `Dependencies []string \`json:"Dependencies"\``。
  - `ParseInfo` 拷贝 `raw.Dependencies`。
- [ ] 2. `internal/palmod/palmod_test.go`
  - `TestParseInfoTolerant` 断言 `Dependencies` 被解析（样本加 `"Dependencies":["Dep1","Dep2"]`）。
  - 缺失用例（`TestParseInfoNumericVersionAndMissingFields`）断言 `Dependencies == nil`。
- [ ] 3. `internal/models/server.go`
  - `Mod` 加 `Dependencies []string` + `gorm:"column:dependencies;serializer:json"`，注释同 Tags。
- [ ] 4. `internal/process/manager.go`（`DownloadGlobalMod` 元数据回填）
  - `Select(...)` 加 `"dependencies"`；`Updates(models.Mod{...})` 加 `Dependencies: info.Dependencies`。
- [ ] 5. `internal/api/mod_handlers.go`
  - 新增 `ModDependency{Name, Satisfied}` 类型。
  - 新增 helper `downloadedPackageSet(db) map[string]struct{}`。
  - 新增 helper `buildModDependencies(names []string, satisfied map[string]struct{}) []ModDependency`。
  - `ModWithStatus` 加 `Dependencies []ModDependency`；`ListGlobalMods` 填充。
  - `ServerModDetail` 加 `Dependencies []ModDependency`；`ListServerMods` 填充。

### 前端
- [ ] 6. `ui/src/types/server.ts`
  - `ModDependency` 接口；`Mod.dependencies?`；`ServerMod.dependencies`。
- [ ] 7. 共享徽标组件（`ui/src/components/server-manage/ModDependencies.tsx`）
  - props：`deps`, `label`, `missingLabel`；空返回 null。
- [ ] 8. `ui/src/app/mods/page.tsx` `ModRow` 接入（label 走 `modLibrary` 命名空间新 key）。
- [ ] 9. `ui/src/components/server-manage/ModsSection.tsx` `ServerModRow` 接入（label 走 `serverConfig.mods.*`）。
- [ ] 10. i18n `ui/messages/{zh,en,ja}.json`
  - `serverConfig.mods.dependencies` / `dependencyMissing`
  - `modLibrary.dependencies` / `dependencyMissing`

## 验证命令

```bash
# 后端
go build ./...
go test ./internal/palmod/... ./internal/api/... ./internal/database/... ./internal/process/...

# 前端
cd ui && bun run lint && bun run build
```

## 审查关口

- serializer 列陷阱：dependencies 回填走 Select+结构体 Updates，不用 map。
- 前端 `dependencies ?? []` 容错。
- i18n 三语齐全，无缺 key。
- 满足度计算：`downloaded && package_name != ""` 才计入满足集合。

## 回滚点

- 后端改动集中在 4 个 .go 文件 + 1 测试；前端 5 处。git 还原即可回滚。
- dependencies 列 additive，回滚后残留无害。

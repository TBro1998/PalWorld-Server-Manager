# Design — mod 元信息展示(ModName/PackageName/Tags)

## 架构与边界

改动沿现有分层，无新增服务/接口，无破坏性迁移：

```
palmod.ParseInfo (解析层)  →  Manager.UpdateMods (回填层)  →  DB(mods 表新列)
        ↑ Info.json                                              ↓ GET /servers/:id/mods
                                                          前端 Mod 类型 + ModsSection UI
```

## 契约 / 数据流

### 1. palmod.Info（[internal/palmod/info.go](../../../internal/palmod/info.go)）

`Info` 结构体新增字段：

```go
type Info struct {
    PackageName string
    ModName     string   // 新增：Info.json 的 ModName
    Version     string
    Tags        []string // 新增：Info.json 的 Tags
    IsServer    bool
}
```

`rawInfo` 新增 `ModName string json:"ModName"` 与 `Tags []string json:"Tags"`；
`ParseInfo` 直接把 `raw.ModName`、`raw.Tags` 拷入 `Info`。保持宽容：字段缺失时
`ModName=""`、`Tags=nil`（不报错）。Tags 用原生 `[]string` 解析即可，JSON 数组直接映射。

### 2. models.Mod（[internal/models/server.go](../../../internal/models/server.go)）

新增两列：

```go
ModName string   `json:"mod_name" gorm:"column:mod_name;default:''"`
Tags    []string `json:"tags" gorm:"column:tags;serializer:json"`
```

- `Tags` 用 GORM `serializer:json`：SQLite 存 JSON 文本，`c.JSON` 输出为数组。
  空/未下载时数据库为空 → GORM 反序列化为 `nil` → JSON 输出 `null`。前端需容错
  （`tags ?? []`）。
- `serializer:json` 是 GORM 层能力，与 glebarez/sqlite 纯 Go 驱动无关，可用。

### 3. 回填（[internal/process/manager.go:450-462](../../../internal/process/manager.go#L450-L462)）

`UpdateMods` 成功解析分支的 `Updates(map[string]any{...})` 增加：

```go
"mod_name": info.ModName,
"tags":     info.Tags,
```

并把本地 `mod.ModName = info.ModName`、`mod.Tags = info.Tags` 一并回填（本地副本仅用于
后续 ActiveModList 计算，tags/modname 不参与，但保持一致性）。partial-success 语义不变：
解析失败仍走既有 install_path-only 分支，不写 mod_name/tags。

> 注意：`Updates` 用 `map[string]any` 且 value 为 `[]string` 时，GORM 会对带
> `serializer:json` 的列自动序列化（按列名匹配 schema）。若实测 map 形式不触发
> serializer，则回退为结构体 `Updates(models.Mod{...})` + `Select` 指定列。实现时以
> 单测验证 round-trip（见 implement.md 校验项）。

### 4. 前端（[ui/src/types/server.ts](../../../ui/src/types/server.ts) / [ModsSection.tsx](../../../ui/src/components/server-manage/ModsSection.tsx)）

`Mod` 接口新增 `mod_name: string`、`tags: string[]`（后端可能返回 `null`，用可选或
读取处 `?? []` 容错）。

列表条目渲染（替换 L198-213 的标题/副标题块）：
- 主标题：`m.mod_name || m.name || m.workshop_id`。
- 未下载告警图标条件不变（`m.package_name === ''`）。
- 次级行：`m.package_name`（有值）、`m.workshop_id`、`v{version}`（有值）以 `·` 连接。
- Tags：`(m.tags ?? []).length > 0` 时渲染一排 badge。复用现有 UI 原子（若无 Badge
  组件则用 Tailwind `rounded` 小 span，与项目既有风格一致）。

## 兼容性 / 迁移

- 仅新增列，`AutoMigrate` 自动补列，旧库无需手动迁移（AC4）。
- 旧 mod 行 `mod_name` 空、`tags` null；下次 UpdateMods 后回填。
- API 响应新增字段，旧前端忽略即可；无破坏。

## 取舍

- Tags 用 `serializer:json` 而非逗号拼接：tag 内容虽简单，但 JSON 保真、前端零解析、
  与“数组语义”一致，成本相同。
- 不新增缩略图端点：用户明确排除，避免文件服务/路径穿越等额外风险面。

## 回滚

改动均为加法（新字段/新 UI 分支），回滚 = 还原这几处编辑；DB 残留空列无害（与项目既有
“AutoMigrate 不删列，残列无害”约定一致）。

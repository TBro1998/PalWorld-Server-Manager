# Design: 后端数据库层迁移到 GORM

## 架构与边界

保持现有分层不变，仅把数据访问的底层从 `database/sql` 换成 GORM：

```
main.go → database.Initialize() *gorm.DB
        → server.New(cfg, *gorm.DB) → api.NewRouter(*gorm.DB) → Router.db
                                    → process.NewManager(*gorm.DB) → Manager.db
```

不新增 repository/DAO 层——直接在现有 handler/manager 里用 `r.db.*` / `m.db.*` 的 GORM 链式 API，保持改动最小、与现有代码风格一致。

## 依赖

- 新增：`gorm.io/gorm`、`github.com/glebarez/sqlite`。
- `modernc.org/sqlite` 从直接依赖降为 glebarez 的间接依赖（仍在 go.sum，无 CGO）。
- `database/sql` 仅在需要 `sqlDB, _ := db.DB(); sqlDB.Close()` 时间接出现。

## 数据模型（models 包补 GORM tag）

关键约束：**不要用 `gorm.Model`**（它内嵌 `DeletedAt` 会触发软删除，改变 DeleteServer 语义）。保留显式字段，仅加 tag。

```go
type Server struct {
    ID          int64     `json:"id" gorm:"primaryKey"`
    Name        string    `json:"name" gorm:"not null"`
    InstallPath string    `json:"install_path" gorm:"not null"`
    Status      string    `json:"status" gorm:"-"`            // 派生，不持久化
    PID         int       `json:"pid" gorm:"default:0"`
    LaunchArgs  string    `json:"launch_args" gorm:"default:''"`
    Installed   bool      `json:"installed" gorm:"default:false"`
    LastError   string    `json:"last_error,omitempty" gorm:"default:''"`
    CreatedAt   time.Time `json:"created_at"`                 // GORM 自动维护
    UpdatedAt   time.Time `json:"updated_at"`                 // GORM 自动维护
}
```

- `Mod`/`User` 同样补 `primaryKey` 等 tag；`Mod.ServerID` 保留（FK 约束非关键，mods 逻辑仍 stub，不强制建 FK）。
- 表名：GORM 默认复数化 `servers`/`mods`/`users`，与现有表名一致，无需 `TableName()`。
- `db:"..."` tag 可保留或移除（GORM 不用它），本次一并清理为 gorm tag。

## 迁移（database.Initialize）

```go
func Initialize(dbPath string) (*gorm.DB, error) {
    db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
    if err != nil { return nil, ... }
    if err := db.AutoMigrate(&models.Server{}, &models.Mod{}, &models.User{}); err != nil { return nil, ... }
    return db, nil
}
```

- 删除 `migrate`/`addColumnIfMissing`/`dropColumnIfExists`（AutoMigrate 覆盖加列；删列按 Q1 不做）。
- 旧库：AutoMigrate 幂等，残留 `status`/弃用列不受影响。
- 注意 import cycle：`database` 包将 import `models`。确认 `models` 不反向 import `database`（当前 models 仅 import `time`，安全）。

## 查询改写对照

| 场景 | 现写法 | GORM |
|---|---|---|
| 列表 | `db.Query(SELECT...)` + scan 循环 | `db.Order("created_at DESC").Find(&servers)` |
| 单条 | `db.QueryRow(...).Scan(...)` | `db.First(&s, id)` |
| not-found | `err.Error()=="sql: no rows..."` | `errors.Is(err, gorm.ErrRecordNotFound)` |
| 插入 | `db.Exec(INSERT...)` + `LastInsertId()` | `db.Create(&s)`（自动回填 ID/时间戳） |
| 更新指定列 | `db.Exec(UPDATE ... SET x=?)` | `db.Model(&Server{ID:id}).Updates(map[string]any{...})` |
| 删除 | `db.Exec(DELETE...)` | `db.Delete(&Server{}, id)`（无 DeletedAt → 硬删） |
| 存在性 | `SELECT 1 ... Scan` | `db.Select("id").First(&s, id)` 判 not-found |
| 局部字段 | `SELECT install_path,... Scan(&a,&b)` | `db.Select(...).First(&s, id)` 后取字段 |

要点：
- **部分列更新用 `map[string]any` 或 `Select(...).Updates(...)`**，避免结构体零值被跳过/误写。`pid=0`、`last_error=""`、`installed=false` 都是合法目标值，必须用 map 明确写入。
- 时间戳交给 GORM：移除手写 `time.Now()` / `CURRENT_TIMESTAMP`；`Create`/`Updates`（经 `Model`）会自动维护 `updated_at`。用 map 更新时 GORM 仍会自动补 `updated_at`。
- `CreateServer` 两步（insert 后回填默认 install_path）保留逻辑：`Create` 后若 path 为空则 `Updates` 一次。

## 派生状态语义（不变）

- `Status` `gorm:"-"`，永不入库/出库。
- `DeriveStatus` 仍读 `pid`（内存 running/installing 优先）+ `last_error`。
- handler 查询后仍调用 `hydrateStatus`。

## 兼容与回滚

- 兼容：AutoMigrate 对新旧库均幂等；API 响应结构体（json tag）不变 → 前端零改动。
- 回滚点：改动集中在 db 层与调用点，`git revert` 单次提交即可回退；无数据破坏（不删列、不改数据）。

## 权衡

- 不建 repository 层：项目规模小（3 表、~28 调用点），额外抽象无收益，违背 first-principles 最小机制原则。
- 保留显式模型字段而非 `gorm.Model`：避免软删除语义漂移，且与现有 json 契约一致。

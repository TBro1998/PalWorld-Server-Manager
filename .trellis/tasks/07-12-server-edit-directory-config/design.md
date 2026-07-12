# 技术设计：服务器目录与配置编辑

## 边界与总体结构

新增一个后端包 `internal/palconfig` 作为"参数注册表 + INI 读写 + 启动参数"的单一真源。API 层新增配置读写端点，前端新增两个对话框（基础编辑 / 配置编辑）。持久化分工：

- **配置参数（OptionSettings）** → 直接读写磁盘 `PalWorldSettings.ini`（用户决策）。
- **启动参数** → 无 INI 归属，存数据库 `servers.launch_args`（JSON）。
- **install_path** → 数据库（沿用现有列）。
- **installed** → **持久化到 `servers.installed` 列**（不再每次请求探测）。仅在三个时机重算并落库：① 后端启动校正；② 修改服务器目录后；③ 安装完成后。`ListServers`/`GetServer` 直接读列。

## 数据模型

### DB 迁移（`internal/database/database.go`）
- `servers` 增列 `launch_args TEXT DEFAULT ''` 与 `installed BOOLEAN DEFAULT 0`。
- `migrate()` 增加幂等 `ALTER TABLE`（先查 `PRAGMA table_info(servers)`，缺列才加），兼容既有 DB。

### models（`internal/models/server.go`）
```go
LaunchArgs string `json:"launch_args" db:"launch_args"` // JSON 字符串
Installed  bool   `json:"installed"    db:"installed"`  // 持久化，按需更新
```

## 参数注册表（`internal/palconfig/schema.go`）

以官方文档四大类为准，定义 `ParamDef`：
```go
type ParamType string // "bool" | "int" | "float" | "string" | "enum" | "raw"
type ParamDef struct {
    Key      string
    Type     ParamType
    Default  string     // INI 原始形态（float 带小数、string 带引号在序列化层处理）
    Category string     // performances|serverManagement|features|gameBalances
    Options  []string   // enum 可选值
}
var Params = []ParamDef{ ... 全量 ... }
```
- enum：`Difficulty`、`DeathPenalty`(None/Item/ItemAndEquipment/All)、`RandomizerType`(None/Region/All)、`LogFormatType`(Text/Json)。
- raw（不结构化，原样透传）：`DenyTechnologyList`、`CrossplayPlatforms`、`AllowConnectPlatform`、`RandomizerSeed` 等含括号/元组/保留项。
- 其余按 bool/int/float/string。
- 该数组即：① 播种默认值来源；② `/config/schema` 响应来源；③ 读写时类型基准。

## INI 读写（`internal/palconfig/ini.go`）

### 路径解析
```
configDir(installPath) =
  <installPath>/Pal/Saved/Config/WindowsServer  (GOOS=windows)
  <installPath>/Pal/Saved/Config/LinuxServer    (其它)
file = configDir/PalWorldSettings.ini
```

### 读取 `LoadSettings(installPath) (map[string]string, error)`
1. 若 `file` 不存在：
   - 若 `<installPath>/DefaultPalWorldSettings.ini` 存在 → 复制其内容为种子；
   - 否则用内置 `embed` 默认模板（`default_settings.ini`）为种子；
   - 写入 `file`（`MkdirAll` 配置目录）。
2. 读 `file`，定位 `[/Script/Pal.PalGameWorldSettings]` 段的 `OptionSettings=(...)` 行。
3. `parseOptionSettings(inner)`：按顶层逗号切分（状态机：跟踪引号内与括号深度），每段 `key=value`，value 去外层引号后入 map；解析不了的键以 raw 原值入 map。
4. 与注册表求并集：注册表有但文件缺的键，用默认补齐（保证前端全量展示）。

### 写入 `SaveSettings(installPath, values map[string]string) error`
1. 依注册表顺序 + 文件中出现的额外键，序列化 `key=value`：string/enum 加引号，raw 原样，数值原样。
2. 组装 `OptionSettings=(...)`，替换/新建该行，保留文件其余内容。
3. 原子写（临时文件 + rename）。

### 原始文本兜底
- `LoadRaw(installPath) string` 返回当前 `OptionSettings=(...)` 整行。
- `SaveRaw(installPath, line string)` 校验以 `OptionSettings=(` 开头、括号配平后写回。

## 启动参数（`internal/palconfig/launchargs.go`）

```go
type LaunchArgs struct {
    Port                    *int
    Players                 *int
    UsePerfThreads          bool
    NoAsyncLoadingThread    bool
    UseMultithreadForDS     bool
    NumberOfWorkerThreads   *int
    PublicLobby             bool
    PublicIP                string
    PublicPort              *int
    LogFormat               string // "" | "text" | "json"
}
func (a LaunchArgs) ToArgs() []string  // 生成 exec 参数切片
```
- DB 存 JSON（`ToArgs` 只在启动时调用）。
- `Build`/`Parse` 在 `palconfig` 内做 JSON <-> struct。

## API 层（`internal/api`）

### installed 的持久化与更新（按需，不频繁探测）
- 探测助手下沉到 process 包并导出：`process.IsInstalled(installPath) bool`（内部即 `serverExecutable` 判定），供多处复用。
- **启动校正**：`Manager.ReconcileInstalled()` 遍历所有服务器，`UPDATE servers SET installed = ?`；在 `server.setupRoutes` 中紧随 `ReconcileOnStartup()` 调用。
- **改目录后**：`UpdateServer` 内若 `install_path` 变化，重算并写 `installed`。
- **安装完成后**：`InstallServer` 后台 goroutine 成功分支追加 `installed = 1`。
- `ListServers` / `GetServer` 的 SELECT 增加 `installed` 列并直接返回，不做 stat。

### UpdateServer 扩展（PUT `/servers/:id`）
- 请求体新增 `installPath *string`、`launchArgs *json.RawMessage`。
- 若传 `installPath` 且与原值不同：要求当前 `status == stopped`，否则 400；更新列并重算 `installed` 落库。
- 若传 `launchArgs`：校验能 `Parse` 后原样存库。
- 响应带最新 `installed`。
- 兼容旧字段（name/ports/rcon）保持不变。

### 配置端点（新增）
- `GET /servers/:id/config` → `{ settings: {k:v}, launchArgs: {...}, raw: "OptionSettings=(...)", installed: bool }`。
  - 未安装（无 exe 也无 Default.ini 且模板可用）仍返回模板播种值，附 `installed:false`，前端可编辑但提示未安装。
- `PUT /servers/:id/config` → 二选一：
  - `{ settings: {...}, launchArgs: {...} }`（结构化保存：写 INI + 更新 launch_args）；或
  - `{ raw: "OptionSettings=(...)", launchArgs: {...} }`（原始文本保存）。
  - 要求 `status == stopped`。
- `GET /config/schema` → 返回 `palconfig.Params`（含 category / type / default / options）驱动前端表单。

### 路由（router.go）
```
servers.GET("/:id/config", r.GetServerConfig)
servers.PUT("/:id/config", r.UpdateServerConfig)
protected.GET("/config/schema", r.GetConfigSchema)
```

## 进程管理（`internal/process`）

- `StartServer` 载入 `launch_args`，`palconfig.ParseLaunchArgs(json).ToArgs()` 追加到 `exec.Command(exe, args...)`。
- `serverRow` 增 `launchArgs string` 字段并在 `loadServer` 查询。
- 导出 `IsInstalled(installPath) bool`（复用 `serverExecutable` 内部逻辑）。
- 新增 `(m *Manager) ReconcileInstalled() error`：遍历 servers，按 `IsInstalled` 回写 `installed` 列；`server.setupRoutes` 中在 `ReconcileOnStartup()` 之后调用。
- 其余生命周期逻辑不动。

## 前端（`ui/src`）

### types/server.ts
- `Server` 增 `launch_args: string`、`installed: boolean`。
- 新增 `ServerConfig`、`ConfigParamDef`、`LaunchArgs`、`UpdateServerData`(增 installPath/launchArgs) 类型。

### lib/api.ts
- `serversApi.update` 请求体扩展 installPath/launchArgs。
- 新增 `getConfig(id)`、`updateConfig(id, body)`、`configSchema()`。

### 组件
- `ServerCard.tsx`：stopped 时增"编辑""配置"按钮；`!installed` 时状态区显示"需要安装"提示并高亮安装按钮。
- `EditServerDialog.tsx`：编辑 name / install_path / 端口 / 启动参数（结构化开关+输入）。保存走 `serversApi.update`。
- `ServerConfigDialog.tsx`：Tabs 按四大类渲染 schema 驱动的表单控件（bool→Switch, enum→Select, int/float→Input number, string/raw→Input）；额外一个"原始文本"Tab（Textarea 编辑 OptionSettings 行）+ 启动参数区。保存走 `serversApi.updateConfig`。
- 复用现有 `ui/dialog`、`input`、`label`、`button`、`badge`、`card`；如缺 `switch`/`select`/`tabs`/`textarea` 则补最小 shadcn 组件。

### i18n（全量参数说明三语）
- `ui/messages/{zh,en,ja}.json` 增：
  - `editServer`、`serverConfig` 段（分类标题、按钮、提示、"需要安装"）。
  - `serverConfig.params.<Key>` 段：**每个参数**含 `label` 与 `desc`，三语齐全（Performances/Server management/Features/Game balances 全量 + 启动参数）。
- 英文 `desc` 直接采用官方文档描述；zh/ja 翻译。
- 前端表单：控件旁显示 `label`，问号/悬浮显示 `desc`；enum 选项值保持原文（如 `ItemAndEquipment`）。
- 参数 Key 本身（写入 INI 的键名）保持英文原样，仅展示层做三语。

## 兼容性与回滚

- DB 仅新增列，向后兼容；回滚只需忽略新列。
- INI 写入原子化，失败不破坏原文件；raw 校验失败直接 400 不落盘。
- 前端为增量组件，不改动现有卡片核心动作。
- 内置默认模板确保无 Default.ini 时也可编辑。

## 风险与取舍

- **单行 OptionSettings 解析**：用状态机而非正则，重点保证 round-trip；无法结构化的键走 raw，避免破坏。
- **全量翻译成本**：按用户要求，全部参数的 label+desc 做三语（英文取官方文档，zh/ja 翻译）。集中放在 `serverConfig.params`，一次性录入。
- **运行时编辑**：明确禁止（stopped 才可），避免与 PalServer 启动时覆盖 INI 冲突。

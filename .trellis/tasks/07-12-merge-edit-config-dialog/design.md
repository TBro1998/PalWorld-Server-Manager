# 技术设计：合并编辑与配置界面并清理 RCON

## 架构与边界

改动横跨前后端，但**运行时行为不变**（launch_args + INI 仍是唯一生效源）。核心是"收敛编辑入口 + 删除惰性/弃用字段"。

```
前端
  ServerCard.tsx        两按钮(编辑/配置) → 单按钮"设置"(齿轮)；port 展示改从 launch_args 解析
  ServerSettingsDialog  新组件：合并 EditServerDialog + ServerConfigDialog（7 标签）
  EditServerDialog.tsx  删除
  ServerConfigDialog.tsx删除（逻辑并入新组件）
  types/server.ts       Server / UpdateServerData 去掉 port/query_port/rcon_port/rcon_enabled
  lib/api.ts            无需新端点；保存时按序调用 update + updateConfig
  messages/{zh,en,ja}   合并/删除/新增 i18n 键

后端
  palconfig/schema.go   Params 移除 RCONEnabled / RCONPort（RESTAPI* 保留）
  models/server.go      去掉 Port/QueryPort/RCONPort/RCONEnabled 字段
  api/handlers.go       serverColumns / scanServer / CreateServer INSERT / UpdateServer / UpdateServerRequest 去掉四列
  database/database.go  CREATE 语句去掉四列；新增 dropColumnIfExists 迁移移除旧库四列
```

## 数据流

### 打开弹窗（仅 idle/stopped 时可入口）
1. `GET /servers/:id/config` → { settings, launchArgs, raw, installed }
2. `GET /config/schema` → params（已不含 RCON）
3. 名称/安装目录来自列表里的 `server`（servers.name / install_path）。

### 保存（R7 编排）
单个"保存"按钮触发两步（顺序执行，第一步成功再第二步；集中错误处理）：

1. **元数据** `PUT /servers/:id`（UpdateServerRequest）：`{ name, installPath, launchArgs }`
   - launchArgs 含 `-port`（基础标签的游戏端口）+ 启动参数标签其余项。
2. **配置** `PUT /servers/:id/config`：`{ settings | raw }`
   - 结构化模式：保存前把 `settings.ServerName = name`（R3 同步），仅当 `installed` 时才有意义（未安装时 config 端点会拒绝，见约束）。
   - raw 模式沿用现状（raw 优先，不注入 ServerName）。

失败处理：任一步失败 → 在弹窗内展示错误、保留用户输入、不关闭。成功 → 失效 `['servers']` 与 `['serverConfig', id]` 查询并关闭。

> 说明：保持两个端点、前端编排，避免后端合并端点的额外契约变更（YAGNI）。UpdateServer 对 launch_args 校验已存在（ParseLaunchArgs）。

## 关键契约变更

### 后端 `Server` 模型 / 响应
移除 JSON 字段 `port` `query_port` `rcon_port` `rcon_enabled`。`serverColumns` 与 `scanServer` 同步（否则 Scan 目标与列不匹配会 panic/错位）。`CreateServer` INSERT 不再写这四列；返回的 `models.Server` 字面量去掉对应字段。

### `UpdateServerRequest`
```go
type UpdateServerRequest struct {
    Name        string           `json:"name"`
    InstallPath *string          `json:"installPath,omitempty"`
    LaunchArgs  *json.RawMessage `json:"launchArgs,omitempty"`
}
```
UpdateServer 的 UPDATE 语句改为 `SET name=?, updated_at=? WHERE id=?`（不再含端口/RCON）。install_path 与 launch_args 分支逻辑保持不变。

### 前端类型
`Server` 去掉 port/query_port/rcon_port/rcon_enabled（保留 `launch_args: string`）。`UpdateServerData` 去掉 port/queryPort/rconPort/rconEnabled，保留 `name/installPath/launchArgs`。

### schema.Params
删除 `{Key:"RCONEnabled",...}` 与 `{Key:"RCONPort",...}` 两行。其余不动。`RESTAPIEnabled/RESTAPIPort` 保留。

## DB 迁移

`CREATE TABLE IF NOT EXISTS servers` 去掉 `port/query_port/rcon_port/rcon_enabled` 四行（影响全新库）。

对既有库新增幂等迁移，仿 `addColumnIfMissing` 写 `dropColumnIfExists(db, "servers", col)`：先 `PRAGMA table_info` 确认列存在，再 `ALTER TABLE servers DROP COLUMN <col>`。四列各调一次。

- SQLite ≥3.46（modernc v1.33.1）支持 DROP COLUMN，且这些列无索引/约束依赖，安全。
- 失败策略：DROP 失败记 warning 但**不致命**（列残留无害，运行时不读）。与现有 `addColumnIfMissing` 的致命返回不同——这里降级为告警，避免破坏启动。

## UI 组件设计（ServerSettingsDialog）

- 复用 ServerConfigDialog 的标签栏/滚动区/renderControl/LaunchToggle/LaunchNumber。
- 新增 `基础` 标签置于最前：
  - 服务器名称：`Input`（绑定本地 name，来自 server.name）。
  - 安装目录：`Input`（绑定 installPath，来自 server.install_path）+ 目录变更提示（沿用 EditServerDialog 的 pathChanged 逻辑）。
  - 游戏端口：`LaunchNumber`（-port，绑定 launchArgs.port）。
- 标签集：`基础 / 性能(performances) / 服务器管理(serverManagement) / 玩法(features) / 平衡(gameBalances) / 启动参数(launch) / 原始文本(raw)`。
- 启动参数标签删除 publicIP/publicPort/logFormat 三个控件。
- Save 按钮 disabled 条件：进行中，或（结构化/raw 需要）`!installed`——沿用 ServerConfigDialog 现有 `!server?.installed` 限制；名称/launchArgs 部分即便未安装也可存（UpdateServer 无 installed 限制）。
  - 决策：未安装时仍允许保存"基础"里的名称/目录/端口（走 update），但 INI 保存会被后端拒绝。为简单起见：未安装时只调用 update，跳过 config；已安装时两步都走。用 `installed` 判定。

## ServerCard 端口展示

`server.port` 不再存在。改为：解析 `server.launch_args`(JSON) 取 `.port`，无则显示 `8211`。用小工具函数就地解析（try/catch，失败回退默认）。

## i18n 变更清单（zh/en/ja 对称）

- `servers`：新增 `settings`（齿轮按钮文案）；`edit`/`config` 键可保留或删除（若不再引用则删）。
- 新增 `serverSettings.*`（或复用 `serverConfig` + 加 `tabs.basics`、`basics.name/path/port/pathChangedHint`）。倾向：合并到 `serverConfig`，新增 `tabs.basics` 与基础字段键，减少新命名空间。
- 删除 `editServer` 命名空间（或保留 title/cancel/save 复用）。删除 `serverConfig.launch.publicIP/publicPort/logFormat`、`serverConfig.params.RCONEnabled`、`serverConfig.params.RCONPort`。

## 兼容性 / 回滚

- 运行时零行为变更（生效源不变）。
- DB 迁移不可逆（列删除），但删除的是惰性列，无数据价值损失。回滚代码后旧库缺列会导致老代码 INSERT 报错——回滚需同时恢复列（低概率，记录于 implement rollback）。
- 前端为静态嵌入：改动需 `bun run build` 后再 `go build`。

## 权衡

- 保留双端点 + 前端编排 vs 新建合并端点：选前者，改动小、契约稳定。
- 主动清理 INI 里 RCON vs 透传：选透传，非破坏、值 False 无害。
- DROP COLUMN 致命 vs 告警：选告警，保证启动韧性。

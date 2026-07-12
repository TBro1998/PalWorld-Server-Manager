# 执行计划：合并编辑与配置界面并清理 RCON

按后端→前端顺序，每组结束跑对应校验。前端最后统一 build/lint。

## 阶段 A：后端字段与 schema 清理

- [ ] A1. `internal/palconfig/schema.go`：删除 `RCONEnabled`、`RCONPort` 两条 ParamDef。
- [ ] A2. `internal/models/server.go`：删除 `Port/QueryPort/RCONPort/RCONEnabled` 字段。
- [ ] A3. `internal/api/handlers.go`：
  - `serverColumns` 去掉 `port, query_port, rcon_port, rcon_enabled`。
  - `scanServer` 的 Scan 目标同步去掉四个字段（顺序对齐新列表）。
  - `CreateServer`：INSERT 去掉四列及对应值；返回的 `models.Server` 字面量去掉四字段。
  - `UpdateServerRequest`：改为 `{ Name, InstallPath *string, LaunchArgs *json.RawMessage }`。
  - `UpdateServer`：基础 UPDATE 改为 `SET name=?, updated_at=? WHERE id=?`；install_path / launch_args 分支保持。
- [ ] A4. 校验：`go build ./...`

## 阶段 B：DB 迁移

- [ ] B1. `internal/database/database.go`：CREATE TABLE servers 去掉 `port/query_port/rcon_port/rcon_enabled` 四行。
- [ ] B2. 新增 `dropColumnIfExists(db, table, column)`（仿 addColumnIfMissing；列存在才 DROP；失败记 warning 不致命）。
- [ ] B3. 在 migrate() 末尾对四列各调用一次。
- [ ] B4. 校验：`go build ./...` && `go test ./...`（palconfig ini_test 应仍通过）。

## 阶段 C：前端类型与 API

- [ ] C1. `ui/src/types/server.ts`：`Server` 去掉 port/query_port/rcon_port/rcon_enabled；`UpdateServerData` 去掉 port/queryPort/rconPort/rconEnabled（保留 name/installPath/launchArgs）。
- [ ] C2. `ui/src/lib/api.ts`：无端点变更，确认 update/updateConfig 签名可用。

## 阶段 D：合并弹窗组件

- [ ] D1. 新建 `ui/src/components/ServerSettingsDialog.tsx`：以 ServerConfigDialog 为基底，新增 `基础` 标签（名称 / 安装目录 + pathChanged 提示 / 游戏端口=launchArgs.port）。
- [ ] D2. 标签集 `[basics, ...CATEGORIES, launch, raw]`；启动参数标签删除 publicIP/publicPort/logFormat 控件。
- [ ] D3. 保存编排：结构化模式下先 `settings.ServerName = name`；`installed` 时两步（update 元数据+launchArgs、updateConfig），未安装时仅 update；错误集中处理、失败不关闭。
- [ ] D4. 删除 `EditServerDialog.tsx` 与 `ServerConfigDialog.tsx`。

## 阶段 E：卡片与页面接线

- [ ] E1. `ui/src/components/ServerCard.tsx`：`idle` 区两个按钮（编辑/配置）替换为单个"设置"齿轮按钮 → `onSettings(server)`；端口展示改从 `launch_args` 解析 `-port`（回退 8211）。
- [ ] E2. `ui/src/app/servers/page.tsx`：用单一 `settingsServer` state + `<ServerSettingsDialog>` 取代 editingServer/configServer 两套 state 与两个弹窗；移除 EditServerDialog/ServerConfigDialog import 与 updateServerMutation 冗余（更新逻辑并入弹窗）。

## 阶段 F：i18n

- [ ] F1. `ui/messages/zh.json`、`en.json`、`ja.json` 对称修改：
  - 新增 `servers.settings`；`serverConfig.tabs.basics` 与 `serverConfig.basics.{name,path,port,pathChangedHint}`（或等价命名）。
  - 删除 `serverConfig.launch.{publicIP,publicPort,logFormat}`、`serverConfig.params.RCONEnabled`、`serverConfig.params.RCONPort`。
  - 处理 `editServer` 命名空间（不再引用则删；若复用 save/cancel 则保留）。
- [ ] F2. 确认无组件引用已删除的键。

## 阶段 G：整体校验

- [ ] G1. `cd ui && bun run lint`
- [ ] G2. `cd ui && bun run build`
- [ ] G3. 项目根 `go build .`（确认嵌入新前端）
- [ ] G4. 手动/构建验证：卡片单按钮、弹窗 7 标签、保存两步、无 RCON、卡片端口正确。

## 校验命令汇总

```bash
go build ./... && go test ./...
cd ui && bun run lint && bun run build && cd ..
go build .
```

## 风险文件 / 回滚点

- `internal/api/handlers.go`：`serverColumns` 与 `scanServer` **必须严格对齐**，否则 Scan 列错位。改完立即 `go build` + 起服 `GET /servers` 冒烟。
- `internal/database/database.go`：DROP COLUMN 不可逆；回滚代码需同时恢复 CREATE 四列，否则老代码 INSERT 失败。回滚点：阶段 B 提交前。
- 前端删除两个弹窗后如页面仍 import 会编译失败——阶段 D4/E2 一并处理。
- 每阶段后单独可编译；出问题回退到上一阶段提交。

## 备注

- 平台以 Windows 为准；ConfigDir 的 Windows/Linux 分支不动。
- 运行时行为不应改变（生效源仍是 launch_args + INI）。

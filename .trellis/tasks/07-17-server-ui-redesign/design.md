# Design — 重设计服务器管理界面（设置内联）

## 架构总览

保持单一二进制 / 静态导出约束，**不改后端 API**。全部改动在 `ui/src` 前端层。

核心变化：把"设置弹窗（`ServerSettingsDialog`）"解构为管理页内的多个内联导航项，并引入一个**页面级设置草稿状态 + 全局粘性保存栏**。

### 新二级导航结构（均衡拆分 + 分组表头）

```
监控 (group: monitor)
  • 概览        overview   (existing OverviewSection)
  • 玩家与存档  players    (existing PlayersSection)
  • 运维        operations (existing OperationsSection)
  • 日志        logs       (NEW — inline runtime logs, SSE)
配置 (group: config)      ← 均由 SettingsDraft 驱动，共享保存栏
  • 基础设置    basics     (name/path/port/passwords/description/REST)
  • 游戏配置    game       (内部子标签 性能/管理/玩法/平衡)
  • 启动参数    launch
  • Mods        mods       (mod CRUD + Steam 登录；独立自保存，不入保存栏)
  • 原始配置    raw
更多 (group: more)
  • 地图        map        (existing, comingSoon 占位)
  • 备份        backup     (existing, comingSoon 占位)
```

`SECTIONS` 由平铺数组改为分组数组：`{ group, items: [{key, icon}] }[]`。左导航按组渲染，组间加轻量表头（`serverManage.groups.*`）。

### 设置草稿状态（跨页脏状态的核心）

新增 `SettingsDraftProvider`（React Context）包裹 `ManagePanel`，承载原 `ServerSettingsDialog` 的本地编辑态并提升到页面级：

- 数据：`name, installPath, settings(Record), launchArgs, rawText, rawMode` + 各自 baseline。
- 来源：`serversApi.getConfig(id)` + server 元数据；到达后 seed draft 与 baseline。
- 计算：`isDirty`（draft vs baseline 深比较；rawMode 单独判定）、`dirtyCount`（变更字段数，用于"⚠ N 项未保存"）。
- 动作：
  - `save()`：复刻现有原子语义——`serversApi.update(id,{name,installPath,launchArgs})`；若 `server.installed` 再 `serversApi.updateConfig(id, rawMode?{raw}:{settings:{...,ServerName:name},launchArgs})`。成功后失效 `['serverConfig',id]`、`['servers']`、`['server',id]`，并把 baseline 重置为当前 draft。
  - `discard()`：draft 重置回 baseline。
- setter：`setField / setSetting(key,val) / setLaunch(patch) / setRaw(text)`；任何 setter 触发脏状态；`setSetting`/结构化编辑会清除 rawMode（保持现有互斥语义）。

各配置子页是**纯受控视图**，只从 context 读值、调 setter；不再各自持有保存逻辑。

### 全局粘性保存栏

- 位置：`ManagePanel` 底部，`sticky bottom-0`（内容列滚动容器内），仅在 `isDirty` 时出现（滑入动画）。
- 内容：左“⚠ N 项未保存 / 未安装时提示仅元数据可存”，右 `[放弃] [保存]`（saving 态禁用 + loading 文案）。
- 作用域：**所有配置类子页共享一个保存栏**。切换配置子页不清空 draft、不丢脏状态。
- 非配置页（监控/更多）：不显示保存栏（`isDirty` 仍可能为真——保留栏可见，避免用户误以为改动丢失；实现时二选一，见 implement，默认：只要 dirty 就常驻，跨所有子页可见，切回配置页可继续编辑）。

### Mods 与 Steam 登录

- `ModsSection` 从 `ServerSettingsDialog` 抽出为独立文件 `server-manage/ModsSection.tsx`，成为 `mods` 导航项内容。其 CRUD/更新维持**独立自保存**（不进保存栏）。
- `SteamAccountSection` 一并抽出；其内部登录仍是子弹窗（属"账户登录动作"，非配置弹窗，保留合理）。
- `PasswordInput` 抽到 `server-manage/shared.tsx` 供 basics 与 steam 复用。

### 日志内联

- 新增 `server-manage/LogsSection.tsx`：把 `ServerLogsDialog kind="server"` 的运行日志能力内联（复用其 SSE 逻辑，抽出为可复用的日志视图组件 `LogView`）。
- 列表页 `ServerCard` 的"日志"按钮：改为跳管理页 `?id=&section=logs`（或保留卡片弹窗——见 implement 决策：默认保留卡片上的运行日志弹窗入口以便不进管理页也能看，但管理页内提供内联版）。
- **SteamCMD 安装/更新日志弹窗保留**：仍由列表页安装动作 `setInstallLogsServer` 触发（`ServerLogsDialog kind="steamcmd"`）。Mods 更新触发的 steamcmd 日志同样保留弹窗。

### 列表页精修（卡片网格）

- 保留 `ServerCard` 网格，重整信息层级：
  - 主操作按状态收敛为单一显性主按钮（未安装→安装更新；已停止→启动；运行中→停止），主按钮突出。
  - 次级操作（重启/日志/删除等）收进"更多"菜单或次级按钮区，降低视觉噪声。
  - 状态徽章 + 关键磁贴（端口/REST）保留；"管理"作为进入详情的清晰入口。
- 不改数据获取（`serversApi.list` 5s 轮询 + keepPreviousData）与 mutation 逻辑。

## 契约 / 数据流

- 无新增后端端点；沿用 `serversApi.{list,get,getConfig,configSchema,update,updateConfig,install,start,stop,restart,delete}`、`modsApi.*`、`steamApi.*`。
- 配置读取：管理页挂载即 `getConfig`（不再 gated on dialog open）。
- 保存原子性：与现状一致（先 update 后 updateConfig）。未安装时仅写元数据，UI 明示。

## 兼容 / 迁移

- `ServerSettingsDialog.tsx` 拆解后删除；`SettingsSection.tsx`（按钮入口）删除。
- 管理页移除 `settingsOpen` 状态与弹窗挂载。
- i18n：`serverConfig.*` 键基本复用；新增 `serverManage.groups.*`、新 `serverManage.sections.{basics,game,launch,mods,raw,logs}`、保存栏 `serverManage.save.*`（zh/en/ja 同步）。旧 `serverManage.settings.*`（title/desc/hint/open）废弃删除。

## 取舍

- 选内联多导航项 + 单一保存栏：兼顾"设置直达"与"原子保存"，避免自动保存对需重启配置的误操作风险。
- 游戏 INI 四类合一（内部子标签）：控制导航长度，扫视高效（用户已选均衡）。
- 不动后端：降低风险、维持单二进制约束。

## 回滚

- 纯前端改动，按文件粒度可回退。风险文件：`manage/page.tsx`（导航与 provider 装配）、新 `SettingsDraftProvider`、抽离的 `ModsSection/SteamAccountSection`。保留 git 提交点在"抽离设置组件（无行为变化）"与"接入保存栏"两处，便于二分回退。

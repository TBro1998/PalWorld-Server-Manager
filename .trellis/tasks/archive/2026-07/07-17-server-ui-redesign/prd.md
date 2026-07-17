# 重设计服务器管理界面（设置内联）

## Goal

从人机交互角度重新设计服务器管理相关界面，核心是**消除"点击按钮弹出大弹窗"的设置交互**，让服务器配置直接内联展示在管理页中；同时优化管理页整体布局与信息层级，并顺带优化服务器列表页。让"查看状态 → 调整配置 → 保存"成为一条不被弹窗打断的连续流程。

## Background / 现状（confirmed facts）

全局壳层：`AppShell` = 17rem 应用侧栏(`Sidebar`) + 内容区；管理页在内容区内再套一个 13rem 二级左导航。

- 列表页 [ui/src/app/servers/page.tsx](ui/src/app/servers/page.tsx)：`ServerCard` 网格 + `AddServerDialog`(弹窗) + `ServerLogsDialog`(运行日志/SteamCMD 日志弹窗)。
- 卡片 [ui/src/components/ServerCard.tsx](ui/src/components/ServerCard.tsx)：状态徽章、端口/REST 磁贴、描述/密码信息行；操作按钮 启动/停止/重启/安装更新/管理(跳 `/servers/manage?id=`)/日志/删除。
- 管理页 [ui/src/app/servers/manage/page.tsx](ui/src/app/servers/manage/page.tsx)：二级左导航 `SECTIONS = [overview, players, operations, map, backup, settings]`；`settings` 目前只是 [SettingsSection.tsx:20](ui/src/components/server-manage/SettingsSection.tsx#L20) 的一个按钮，点击 `setSettingsOpen(true)` 打开 `ServerSettingsDialog` 大弹窗。**这是本次要消除的交互。**
- 设置弹窗 [ui/src/components/ServerSettingsDialog.tsx](ui/src/components/ServerSettingsDialog.tsx)：内部 tab = `basics / mods / performances / serverManagement / features / gameBalances / launch / raw`，外加 Mods 页内的 Steam 账号登录子弹窗。
  - 保存语义（关键）：单个 Save 按钮做**原子保存**——先 `serversApi.update(id,{name,installPath,launchArgs})`（元数据，任何状态可存），再（仅 `server.installed` 时）`serversApi.updateConfig(id,{settings,launchArgs} | {raw})`（INI，未安装会被后端拒绝）。structured 模式下把外显 name 同步进 INI `ServerName`。
  - Mods 页 CRUD/更新是**独立自保存**动作，不走全局 Save；依赖 Steam 会话就绪(`steamApi.status().sessionReady`)。
- 已有可复用布局原语 [ui/src/components/server-manage/shared.tsx](ui/src/components/server-manage/shared.tsx)：`SectionShell / PanelCard / Placeholder / useServerId`。
- 其它 section 现状：overview/players/operations 为功能页（依赖 REST，手动刷新不自动轮询）；map/backup 为 `comingSoon` 占位。players/operations 内部已有自己的确认弹窗（踢人/封禁、关服/停止）——这些是**动作确认**弹窗，不在"消除弹窗"范围内。
- i18n：`serverManage.sections.*` 驱动导航；`serverConfig.tabs.* / params.*` 驱动设置字段；文案在 `ui/messages/{zh,en,ja}.json`。
- 平台：Windows 优先；静态导出（`output:'export'`），页面靠 `?id=` + Suspense 包 `useSearchParams`。

## User Decisions（已确认）

- 范围：**最大范围**——含服务器列表页一起重设计。
- 设置组织：**把设置拆成独立的二级导航项**（Mods、启动参数、游戏配置等从"设置弹窗"里提升为左导航条目，各自内联）。
- 走 Trellis：创建任务并规划。

## Requirements

- **R1 设置内联**：管理页不再用弹窗承载服务器配置；`ServerSettingsDialog` 拆解删除，配置以内联页面直接呈现在内容区。进入配置类导航项即见可编辑表单。
- **R2 导航重构（均衡拆分 + 分组表头）**：二级左导航按分组渲染——
  - 监控：概览 / 玩家与存档 / 运维 / 日志(新增)
  - 配置：基础设置 / 游戏配置(内部子标签 性能·管理·玩法·平衡) / 启动参数 / Mods / 原始配置
  - 更多：地图(占位) / 备份(占位)
- **R3 保存模型（全局粘性保存栏 + 跨页脏状态）**：设置草稿态提升到页面级（`SettingsDraftProvider`）；任一配置页有改动即在底部呈现粘性保存栏（显示未保存计数、放弃/保存），一次**原子提交**（复刻现有 `update` → 若已安装再 `updateConfig` 语义）。跨配置子页切换不丢草稿、不丢脏状态。未安装时仅元数据可存并明示。Mods 的 CRUD/更新维持独立自保存，不进保存栏。
- **R4 管理页布局与信息层级优化**：状态头、分组导航、各 section 视觉一致性；复用 `shared.tsx` 原语。
- **R5 列表页精修（卡片网格）**：保留卡片式，重整信息层级——主操作按状态收敛为单一显性主按钮，次操作降噪；不改数据获取与 mutation。
- **R6 日志内联**：管理页新增"日志"导航项内联实时运行日志（复用 `ServerLogsDialog` 的 SSE 逻辑抽出 `LogView`）；**SteamCMD 安装/更新日志弹窗保留**（由列表页安装动作与 Mods 更新触发）。
- **R7 i18n**：新增/调整的 `serverManage.*` 与保存栏文案三语(zh/en/ja)同步，无裸键。
- **R8 复用与边界**：复用现有 `serversApi/modsApi/steamApi` 与 REST/mutation 逻辑，**不改后端 API**；维持静态导出约束（`useSearchParams` 保持 Suspense 包裹）。

## Acceptance Criteria

- [x] 管理页设置全程无"打开设置"弹窗；进入配置类导航项即见可编辑表单。（`ServerSettingsDialog`/`SettingsSection` 已删除）
- [x] 原 `ServerSettingsDialog` 的编辑能力（basics/性能/管理/玩法/平衡/启动/raw/mods/steam 登录）在新内联结构中无功能回退。（逻辑逐一迁移复刻）
- [x] 任一配置页改动触发底部粘性保存栏并显示未保存计数；跨配置子页切换草稿不丢；保存原子提交后栏消失并刷新 `servers`/`serverConfig`/`server` 查询。（`SettingsDraftContext` + `SettingsSaveBar`；交互行为需后端实机验证）
- [x] 未安装服务器仅元数据可存且 UI 明示；已安装 INI 正常写入；rawMode 保存走 `{raw}` 分支。（复刻原 update→updateConfig 语义）
- [x] 管理页"日志"导航项可内联实时查看运行日志；SteamCMD 安装/更新日志弹窗仍正常。
- [x] 列表页主操作按状态正确、次操作可达。
- [x] 追加：运维页新增服务器启动/停止/重启/更新控制（进程级，不受 REST 限制）。
- [x] 三语文案完整，无裸 i18n 键（176 键 zh/en/ja 对齐）；`bun run lint` 通过；`bun run build` 静态导出成功。

> 验证范围：静态校验（lint / TypeScript / 构建预渲染 / i18n 对齐）已通过。保存脏状态、原子提交、SSE 日志等交互行为需 Go 后端 + 已配置服务器方能端到端验证，本次未实机运行。

## Out of Scope

- 后端 API 改动。
- players/operations 的动作确认弹窗（踢人/封禁/关服属动作确认，保留）。
- map/backup 占位区的实际功能实现。
- SteamCMD 安装/更新日志弹窗改内联（本次保留弹窗）。

# Implement — 重设计服务器管理界面（设置内联）

## 执行顺序（每步可独立验证；★=建议提交点）

### 阶段 A：抽离设置组件（行为不变，纯搬运）
1. 新建 `server-manage/shared.tsx` 增补：把 `PasswordInput`、`LaunchToggle`、`LaunchNumber` 从 `ServerSettingsDialog.tsx` 抽到 shared，供内联页与 steam 复用。
2. 新建 `server-manage/ModsSection.tsx`：搬运 `ServerSettingsDialog` 内的 `ModsSection` + `SteamAccountSection`（保持 CRUD/更新自保存、steam 登录子弹窗、steamcmd 日志弹窗不变）。
3. 校验：`bun run lint` 通过；此时 `ServerSettingsDialog` 仍引用旧内部实现即可（下一步替换）。★

### 阶段 B：设置草稿状态 + 保存栏
4. 新建 `server-manage/SettingsDraftContext.tsx`：`SettingsDraftProvider` + `useSettingsDraft()`。
   - 内部 `useQuery(['serverConfig',id])`、`useQuery(['configSchema'])`；seed draft/baseline。
   - 暴露 `name/installPath/settings/launchArgs/rawText/rawMode`、setter、`isDirty`、`dirtyCount`、`save()`、`discard()`、`saving`、`error`、`installed`、`schema`。
   - `save()` 复刻原子语义（update → 若 installed updateConfig；structured 注入 ServerName）；成功失效 `['serverConfig',id] ['servers'] ['server',id]` 并重置 baseline=draft。
5. 新建 `server-manage/SettingsSaveBar.tsx`：读 context，`isDirty` 时 `sticky bottom-0` 呈现；`[放弃][保存]`；未安装提示。
6. 校验：单元层面不易测，靠阶段 D 端到端验证。★

### 阶段 C：内联设置子页（受控视图）
7. `server-manage/settings/BasicsSettings.tsx`：name/path/port/ServerPassword/AdminPassword/ServerDescription/RESTAPIEnabled/RESTAPIPort（复刻原 basics tab，改为读写 context）。
8. `server-manage/settings/GameSettings.tsx`：内部子标签 性能/管理/玩法/平衡（复刻 CATEGORIES 渲染，过滤 `ServerName` 与 `BASICS_INI_KEYS`，`renderControl` 迁入）。
9. `server-manage/settings/LaunchSettings.tsx`：复刻 launch tab。
10. `server-manage/settings/RawSettings.tsx`：复刻 raw tab（编辑即 `setRaw` 且置 rawMode）。
11. 校验：`bun run lint`。★

### 阶段 D：管理页装配
12. 改 `manage/page.tsx`：
    - `SECTIONS` 改为分组结构；左导航按组渲染 + 组表头。
    - 用 `SettingsDraftProvider` 包裹面板；移除 `settingsOpen` 与 `ServerSettingsDialog` 挂载。
    - section 路由新增 `basics/game/launch/mods/raw/logs`；配置类页共享保存栏（`<SettingsSaveBar/>` 放面板底部）。
    - 支持 `?section=` 初始定位（供列表页"日志/设置"直达；缺省 overview）。
13. 新建 `server-manage/LogsSection.tsx` + 抽 `LogView`（复用 `ServerLogsDialog` 的 SSE 逻辑）内联运行日志。
14. 删除 `ServerSettingsDialog.tsx`、`SettingsSection.tsx`；清理 import。
15. 校验：`bun run lint`。★

### 阶段 E：列表页精修
16. 改 `ServerCard.tsx`：主操作单显性主按钮（按状态）、次操作收敛、信息层级重整；"日志"入口指向 `manage?id=&section=logs`（或保留卡片弹窗，二选一，实现时取风险低者）。
17. 校验：`bun run lint`。★

### 阶段 F：i18n + 收尾
18. `messages/{zh,en,ja}.json`：新增 `serverManage.groups.{monitor,config,more}`、`serverManage.sections.{basics,game,launch,mods,raw,logs}`、`serverManage.save.{unsaved,unsavedCount,metaOnly,save,saving,discard,saved}`、`serverManage.game.tabs.*`；删除废弃 `serverManage.settings.*`。三语齐全，无裸键。
19. 全量校验（见下）。★（提交由主流程 Phase 3 统一处理）

## 验证命令

```bash
cd ui
bun run lint
bun run build   # 静态导出必须成功（embed 前置）
```

端到端（verify skill / 手动）：
- 进入管理页 → 各配置子页可编辑；改动任一字段底部保存栏出现，显示未保存计数。
- 跨配置子页切换 draft 不丢；保存后栏消失、卡片/概览刷新。
- 未安装服务器：仅元数据可存，UI 明示；已安装：INI 正常写入。
- rawMode：编辑 raw 后保存走 `{raw}` 分支。
- Mods 增删/开关/更新独立生效，不触发保存栏；Steam 登录子弹窗正常。
- 运行日志内联可实时滚动；SteamCMD 安装/更新日志弹窗仍正常。
- 列表页主操作按状态正确、次操作可达。
- 切换 zh/en/ja 无裸键。

## 风险文件 / 回滚点

- `manage/page.tsx`（装配核心）、`SettingsDraftContext.tsx`（状态与保存原子性）、抽离的 `ModsSection.tsx`（steam/steamcmd 流）。
- 回滚：阶段 A 提交点（纯抽离，无行为变化）与阶段 D 提交点（接入保存栏）之间可二分。

## 备注

- 不改后端；沿用现有 `serversApi/modsApi/steamApi`。
- 保持 Windows 优先、静态导出约束；页面 `useSearchParams` 仍需 Suspense 边界（沿用现有包裹）。

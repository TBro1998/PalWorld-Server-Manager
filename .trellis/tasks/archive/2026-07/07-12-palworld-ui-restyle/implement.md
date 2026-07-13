# Implement — 帕鲁风格全局界面改造

按序执行，每步可独立验证。改动集中在 `ui/`。

## 执行清单

### 步骤 1 — 设计 token 重铸（核心杠杆）
- [ ] 编辑 `ui/src/app/globals.css`：按 design.md 替换 `:root` 与 `.dark` 全部 token 值，`--radius` 改为 `1rem`。
- [ ] 保持 `@theme inline` 映射结构不变。
- **验证**：`bun run dev`，首页/服务器页所有 shadcn 组件（卡片、按钮、Badge）应自动变为青绿主色 + 大圆角。

### 步骤 2 — 氛围层工具类
- [ ] 在 `globals.css` `@layer utilities` 新增 `.bg-sky`、`.shadow-pal`（及可选 `.bg-clouds`）。
- **验证**：类可被引用，无 CSS 编译错误。

### 步骤 3 — 全局字体
- [ ] `ui/src/app/layout.tsx` 引入 `next/font/google` 的 Nunito，设 `variable`/`subsets`，挂到 `<body>` className。
- [ ] `globals.css` 设 `--font-sans` + CJK 回退链，确保 `font-sans` 生效。
- **验证**：三语言(en/zh/ja)切换，拉丁字形圆润、中日文正常无缺字。

### 步骤 4 — App Shell + 侧边栏（架构变更 / 门 A′）
- [ ] 新增 `components/Sidebar.tsx`（帕鲁卡片风：品牌胶囊 + 导航项 + 激活态高亮 + 底部语言/暗色切换）。
- [ ] 新增 `components/AppShell.tsx`（侧边栏 + 主区 topbar + `.bg-sky` 内容区；窄屏侧边栏可收起）。
- [ ] `layout.tsx` 用 `AppShell` 包裹 `children`，根背景改 `.bg-sky`；`Navbar.tsx` 被取代（删除或降级为 topbar），`LanguageSwitcher` 迁入侧边栏。
- [ ] 新增暗色模式切换（写 `.dark` class 到 `<html>`，localStorage 持久化）。
- **验证**：两路由在 Shell 下正常导航，侧边栏激活态正确，明/暗切换生效，窄屏可收起。

### 步骤 5 — 首页 → 仪表盘概览 Dashboard（视觉+结构标杆 / 门 B）
- [ ] 复用 `['servers']` 查询，`reduce` 派生统计（总数/运行/停止/错误·未安装）。
- [ ] 布局：欢迎条 + 主 CTA、统计卡片行、服务器速览列表（跳转 `/servers`）、次要特性带；友好空态。
- [ ] 全部帕鲁调色 + 圆润卡片 + `.shadow-pal`，无蓝紫粉，`text-gray-*` → token。
- **验证**：仪表盘统计与 servers 数据一致、跳转正常；作为全站基调标杆确认后再推进。

### 步骤 6 — 服务器页 + ServerCard 重构（结构 + 交互 + 语义色 / 门 C）
- [ ] 结构：顶部工具区（标题 + 新增按钮 + 计数/筛选）；卡片信息分层（状态徽标显著、信息分块、操作按语义分组）。
- [ ] 交互态：首次加载骨架卡片、友好空态（CSS/emoji + 引导 + CTA）、错误态可读 + 重试；轮询刷新不闪烁（TanStack Query `placeholderData`/keepPreviousData）。
- [ ] 语义色：状态 Badge 对齐（运行绿/停止灰/安装蓝/错误红/警告琥珀），`text-gray-*` → token。
- **验证**：功能回归（增/装/启/停/重启/删/设置/日志入口全部可用）+ 三态呈现 + 风格统一。

### 步骤 7 — 弹窗重构（Add / Settings / Logs）
- [ ] 先读三个 Dialog 现状。
- [ ] 统一圆角/留白/分区标题/帕鲁配色；Settings 各 Tab 分组与信息密度优化；Logs 终端保留深色但容器/live 点协调。
- **验证**：三个弹窗打开、表单校验、设置保存、日志 SSE 实时流均正常，风格不突兀。

### 步骤 8 — 全量收尾核对
- [ ] grep 复查残留：`from-blue`、`via-purple`、`to-pink`、`text-gray-`、`bg-purple` 应基本清零（终端/刻意保留处除外并注明）。
- [ ] 明/暗、三语言全页面目检。
- [ ] **功能回归全量走查**：对照 prd.md 验收标准 7（功能零回退）逐项确认。

## 验证命令

```bash
cd ui
bun run lint          # 无新增报错
bun run build         # 静态导出成功产出 ui/out/
```

- 视觉验证：`bun run dev` → http://localhost:3000，逐页 + 明暗 + 三语言目检；对照 prd.md 验收标准 1–6。

## 审查门 / 回滚点

- **门 A（步骤1后）**：token 生效、primitive 自动继承正确，再继续。
- **门 A′（步骤4后）**：App Shell 导航/明暗/收起可用，再进入页面级重构。
- **门 B（步骤5后）**：仪表盘作为视觉+结构标杆确认基调，再推进其余页面。
- **门 C（步骤6后）**：服务器页功能回归通过（重构不破坏能力）后再动弹窗。
- **回滚**：改动集中于 `ui/src/app/{globals.css,layout.tsx,page.tsx,servers/page.tsx}` 与 `ui/src/components/{Navbar,ServerCard,ServerLogsDialog,AddServerDialog,ServerSettingsDialog}.tsx`；`git checkout -- <files>` 可整体回滚，无数据/结构迁移。

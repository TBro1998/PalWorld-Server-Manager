# Design — 帕鲁风格全局界面改造

## 设计语言

「明亮清新卡通」＝ 户外晴空 + 圆润厚实的游戏 UI 面板。三条支柱：

- **配色**：晴空蓝背景 / 青绿(teal)主色 / 暖阳橙点缀 / 深蓝墨字。
- **形状**：大圆角(1rem)、略厚边框、柔和但有存在感的"厚"投影，悬停轻微上浮。
- **字体**：圆润无衬线（Nunito）承拉丁字形，CJK 系统回退。

## 调色板（shadcn HSL token）

在 `ui/src/app/globals.css` 内重铸。结构不变（`:root` + `.dark` + `@theme inline` 映射不动），只改值与 `--radius`。

### 亮色 `:root`
```
--background: 200 65% 97%;      /* 晴空白蓝 */
--foreground: 210 35% 22%;      /* 深蓝墨字 */
--card: 0 0% 100%;              /* 纯白面板 */
--card-foreground: 210 35% 22%;
--popover: 0 0% 100%;
--popover-foreground: 210 35% 22%;
--primary: 183 58% 45%;         /* 帕鲁青绿 teal */
--primary-foreground: 0 0% 100%;
--secondary: 42 55% 93%;        /* 暖奶油 */
--secondary-foreground: 210 35% 25%;
--muted: 200 35% 93%;
--muted-foreground: 210 16% 45%;
--accent: 38 92% 55%;           /* 暖阳橙 */
--accent-foreground: 25 60% 18%;
--destructive: 0 78% 58%;
--destructive-foreground: 0 0% 100%;
--border: 200 30% 84%;
--input: 200 30% 86%;
--ring: 183 58% 45%;
--radius: 1rem;                 /* 卡通大圆角 */
```

### 暗色 `.dark`（深青蓝夜幕 + 发光点缀）
```
--background: 205 40% 12%;
--foreground: 195 30% 92%;
--card: 205 35% 16%;
--card-foreground: 195 30% 92%;
--popover: 205 35% 16%;
--popover-foreground: 195 30% 92%;
--primary: 183 62% 52%;         /* 提亮的青绿，暗底更亮 */
--primary-foreground: 205 45% 10%;
--secondary: 205 25% 24%;
--secondary-foreground: 195 30% 92%;
--muted: 205 25% 22%;
--muted-foreground: 200 15% 65%;
--accent: 40 90% 58%;
--accent-foreground: 205 45% 10%;
--destructive: 0 65% 45%;
--destructive-foreground: 0 0% 100%;
--border: 205 25% 26%;
--input: 205 25% 26%;
--ring: 183 62% 52%;
```

## 氛围层（纯 CSS，无图片资源）

在 `globals.css` 的 `@layer utilities` / base 中新增，静态导出安全：

- **`.bg-sky`**：晴空竖向渐变 `linear-gradient(180deg, sky-top → sky-bottom)`，用于 `layout.tsx` 根容器，替换现有灰白渐变；暗色下切深青蓝。
- **`.bg-clouds`**（可选叠加）：2–3 层 `radial-gradient` 模拟远处云团/柔光，低透明度，`pointer-events:none` 覆盖层。
- **`.shadow-pal`**：卡通厚投影，如 `0 6px 0 rgba(...)` 或柔和大范围 `0 10px 30px -10px`（择一，倾向柔和大范围以免过于扁平贴纸感）。
- 保留 `.bg-grid-pattern`（不再首页使用则删其调用，工具类可留）。

## 字体

`ui/src/app/layout.tsx` 引入 `next/font/google` 的 **Nunito**（圆润、字重全、自托管、离线可用），暴露为 CSS 变量并挂到 `<body>`。在 `globals.css` `@theme` 设 `--font-sans` 使 Tailwind `font-sans` 生效，`font-family` 回退链接系统 CJK（`"Microsoft YaHei","PingFang SC","Hiragino Sans",sans-serif`）保证中日文。

> 注：Nunito 不含 CJK 字形，靠回退链渲染中日文——这是刻意取舍（避免打包数 MB 的 CJK 字体、破坏轻量静态导出）。

## 语义状态色映射

`ServerCard` / `Badge` 状态：

| 状态 | 语义 | 实现 |
|------|------|------|
| running 运行 | 绿 | 新增 `success` 语义 or 直接 `bg-primary`（青绿即"活"）→ 用绿点 + primary |
| stopped 停止 | 灰 | `secondary` / `muted-foreground` |
| installing 安装中 | 信息蓝 | `outline` + 蓝色调 |
| error 错误 | 红 | `destructive` |
| needsInstall 警告 | 琥珀 | 保留 amber，或统一到 `accent` 暖橙 |

`ServerLogsDialog` 终端面板保留深色（终端惯例），但边框/圆角/live 点与新体系协调（live 点用 primary 青绿或保留 emerald）。

## 硬编码清扫策略

原则：**能用 token 就用 token**，需要渐变/品牌感处用帕鲁双色渐变（青绿→暖橙 或 青绿→天蓝），集中定义为工具类避免散落。

- `layout.tsx` 背景 → `.bg-sky`。
- `Navbar.tsx` logo/brand 渐变 → 青绿系；active/hover 灰 → `bg-secondary`/`text-foreground`/`muted`。
- `page.tsx` hero/CTA/feature 六色渐变 → 帕鲁调色（主青绿 + 暖橙 + 天蓝点缀，收敛为 2–3 个和谐渐变，不再蓝紫粉）。
- `ServerCard.tsx` `text-gray-*` → `text-muted-foreground`；`text-amber/red` → 语义色。
- `servers/page.tsx` 空态/文字灰 → token。

## UI/UX 结构改造（大胆版：App Shell 重构）

原则：**保功能、保 API 契约、保静态导出**；结构/交互/数据流的调整为提升清晰度与体验。每个界面在实现时先在对应审查门确认结构方案，再落地。

### App Shell（核心架构变更）
将「顶部 Navbar + 页面各自撑满」改为**常驻侧边栏应用外壳**：

- 新增 `components/AppShell.tsx` + `components/Sidebar.tsx`，在 `layout.tsx` 包裹所有页面。
- **侧边栏**（帕鲁卡片风，圆角/奶油底/青绿高亮，桌面常驻、窄屏可收起为抽屉/图标条）：
  - 顶部：品牌 logo（青绿胶囊）+ 应用名。
  - 导航项：仪表盘(概览) `/`、服务器 `/servers`（图标 + 标签 + 激活态高亮胶囊）。
  - 底部：语言切换 + 暗色模式切换。
- **主区**：顶部细 topbar（当前页标题 + 页面级操作按钮，如"新增服务器"）+ 内容区（`.bg-sky` 晴空背景）。
- `Navbar.tsx` 原顶栏被 Shell 取代（删除或重构为 topbar），`LanguageSwitcher` 迁入侧边栏底部。

### 首页 `page.tsx` → 仪表盘概览 Dashboard
由营销落地页改为**状态概览仪表盘**（数据全部由现有 `serversApi.list()` 的 `['servers']` 查询派生，**不加后端**）：

- 顶部欢迎条 + 主 CTA（新增服务器 / 前往管理）。
- **统计卡片行**：服务器总数、运行中、已停止、错误/未安装——均从 servers 列表 `reduce` 得出。
- **服务器速览**：运行中/需关注服务器的紧凑列表（点击跳转 `/servers`），无服务器时显示友好空态引导。
- 保留原六项能力介绍为次要区块（卡通卡片），或收敛为一条特性带，避免与仪表盘信息重复。

### 服务器页 `servers/page.tsx` + `ServerCard`
- 顶部工具区：标题 + "新增服务器"主按钮 + （如信息够）服务器计数/筛选；空态改为友好插画式（纯 CSS/emoji + 引导文案 + CTA）。
- 卡片信息分层：状态徽标显著化，基础信息（路径/端口/密码）与操作区分块，操作按钮按语义分组（主操作 vs 危险操作），减少一排按钮的拥挤。
- 加载态：首次加载显示骨架卡片而非空白；轮询刷新不打断视图。

### 弹窗（Add / Settings / Logs）
- 统一圆角、留白、分区标题与帕鲁配色。
- Settings 多 Tab：保持标签，但优化每 Tab 的分组标题与信息密度、控件对齐。
- Logs：终端面板保留深色，容器与 live 指示与新体系协调。

### 交互与数据流
- 统一 加载/空/错误 三态呈现（骨架、友好空态、可读错误 + 重试）。
- 操作反馈：启停/安装/删除等给出即时状态与（如已有）toast/内联反馈；破坏性操作确认。
- 数据流仅在服务上述体验时微调（如 TanStack Query 的 `placeholderData`/`keepPreviousData` 让轮询不闪烁、局部 loading），**不改后端接口与返回结构**。

## 影响面 / 兼容

- 前端 UI/UX 改造（视觉 + 结构 + 交互 + 前端数据流）；**不触后端 API、路由契约、数据库、i18n 文案**。
- token 结构不变 → 所有 shadcn primitive 自动继承视觉；结构/交互改动风险集中在手工重构的页面与组件，需逐面在审查门确认 + 功能回归验证。
- 暗色需与亮色同步验证；功能零回退为硬性门槛。

## 取舍

- **不引入图片资源**：氛围全 CSS，换取轻量与静态导出零风险；代价是装饰精细度有限（可接受，风格靠配色+圆角+字体已足够）。
- **CJK 靠系统回退**：不打包 CJK 字体，换取包体小；代价是不同 OS 下中日文字形略有差异（可接受）。

## 回滚

改动集中在 `globals.css`、`layout.tsx` 及 6 个组件；`git checkout -- <files>` 即可整体回滚，无数据/结构迁移。

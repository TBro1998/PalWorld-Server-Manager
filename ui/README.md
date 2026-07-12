# Palworld 服务器管理器 - 前端

> [English](./README.en.md) | 中文 | [日本語](./README.ja.md)

Next.js 16 前端，使用 App Router 和静态导出以嵌入 Go 二进制文件。

## 技术栈

- **框架**: Next.js 16 (App Router)
- **UI 组件**: shadcn/ui + Radix UI
- **样式**: Tailwind CSS v4
- **状态管理**: Zustand
- **数据请求**: TanStack Query + axios
- **表单**: react-hook-form + zod
- **国际化**: next-intl (支持中文、英文、日文)

## 目录结构

```
ui/
├── src/
│   ├── app/[locale]/  # App Router 页面（国际化）
│   ├── components/    # React 组件
│   ├── lib/          # 工具函数和 API 客户端
│   ├── hooks/        # 自定义 React Hooks
│   ├── stores/       # Zustand 状态管理
│   ├── types/        # TypeScript 类型定义
│   └── i18n/         # 国际化配置
└── messages/         # 翻译文件 (en.json, zh.json, ja.json)
```

## 开发

```bash
npm install
npm run dev
```

打开 [http://localhost:3000](http://localhost:3000)

## 生产构建

```bash
npm run build
```

这将在 `out/` 目录中创建静态导出，该目录将被嵌入到 Go 二进制文件中。

## 国际化

应用支持三种语言：
- 英文 (en)
- 中文 (zh)
- 日文 (ja)

在 `messages/{locale}.json` 中添加翻译，它们将自动被识别。

## API 集成

前端通过 `/api` 连接到 Go 后端 API。`src/lib/api.ts` 中的 API 客户端处理：
- JWT 令牌管理
- 请求/响应拦截器
- 自动令牌刷新
- 错误处理

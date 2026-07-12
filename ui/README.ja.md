# Palworld サーバーマネージャー - フロントエンド

> [English](./README.en.md) | [中文](./README.md) | 日本語

Goバイナリに埋め込むためのApp Routerと静的エクスポートを備えたNext.js 16フロントエンド。

## 技術スタック

- **フレームワーク**: Next.js 16 (App Router)
- **UIコンポーネント**: shadcn/ui + Radix UI
- **スタイリング**: Tailwind CSS v4
- **状態管理**: Zustand
- **データフェッチ**: TanStack Query + axios
- **フォーム**: react-hook-form + zod
- **国際化**: next-intl (英語、中国語、日本語をサポート)

## ディレクトリ構造

```
ui/
├── src/
│   ├── app/[locale]/  # App Router ページ（国際化対応）
│   ├── components/    # React コンポーネント
│   ├── lib/          # ユーティリティとAPIクライアント
│   ├── hooks/        # カスタム React Hooks
│   ├── stores/       # Zustand ストア
│   ├── types/        # TypeScript 型定義
│   └── i18n/         # 国際化設定
└── messages/         # 翻訳ファイル (en.json, zh.json, ja.json)
```

## 開発

```bash
pnpm install
pnpm run dev
```

[http://localhost:3000](http://localhost:3000) を開く

## 本番ビルド

```bash
pnpm run build
```

これにより`out/`ディレクトリに静的エクスポートが作成され、Goバイナリに埋め込まれます。

## 国際化

アプリは3つの言語をサポートしています：
- 英語 (en)
- 中国語 (zh)
- 日本語 (ja)

`messages/{locale}.json`に翻訳を追加すると、自動的に認識されます。

## API統合

フロントエンドは`/api`でGoバックエンドAPIに接続します。`src/lib/api.ts`のAPIクライアントは以下を処理します：
- JWTトークン管理
- リクエスト/レスポンスインターセプター
- 自動トークン更新
- エラーハンドリング

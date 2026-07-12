# Palworld サーバーマネージャー - バックエンド

> [English](./README.en.md) | [中文](./README.md) | 日本語

Palworld専用サーバーを管理するためのGoバックエンドサーバー。

## 技術スタック

- **フレームワーク**: Gin
- **データベース**: SQLite (modernc.org/sqlite - 純粋なGo、CGOなし)
- **認証**: JWT
- **リアルタイムログ**: Server-Sent Events (SSE)

## ディレクトリ構造

```
server/
├── main.go         # アプリケーションエントリーポイント
├── internal/
│   ├── api/           # HTTPハンドラーとルーティング
│   ├── auth/          # 認証ロジック
│   ├── config/        # 設定管理
│   ├── database/      # データベース初期化とマイグレーション
│   ├── models/        # データモデル
│   ├── server/        # HTTPサーバー設定
│   ├── steamcmd/      # SteamCMD統合
│   └── i18n/          # 国際化
└── pkg/
    ├── logger/        # ロギングユーティリティ
    └── utils/         # 共通ユーティリティ
```

## ビルド

```bash
go mod download
go build .
```

## 実行

```bash
./bin/palworld-server-manager
```

## 環境変数

- `HOST` - サーバーホスト（デフォルト: 127.0.0.1）
- `PORT` - サーバーポート（デフォルト: 8080）
- `DATABASE_PATH` - SQLiteデータベースパス（デフォルト: ./data/palworld.db）
- `JWT_SECRET` - JWT署名シークレット（本番環境では変更してください！）
- `STEAMCMD_PATH` - SteamCMDインストールパス
- `PALWORLD_BASE_PATH` - Palworldサーバーベースディレクトリ

## フロントエンドの埋め込み

フロントエンドはGoの`embed`パッケージを使用して埋め込まれます。まずNext.jsフロントエンドをビルドします：

```bash
cd ../ui
bun run build
```

次にGoバイナリをビルドします - フロントエンドが自動的に含まれます。

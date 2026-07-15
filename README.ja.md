# Palworld サーバーマネージャー

> [English](./README.en.md) | [中文](./README.md) | 日本語

Palworld専用サーバーのための包括的な管理ツール。MODサポート、マルチサーバー管理、モダンなWebUIを備えています。

> ⚠️ **プロジェクトの状態：開発中**
>
> 本プロジェクトはまだ正式にリリースされておらず、現在も開発が進行中です。機能は不安定であったり、変更・未実装の可能性があるため、本番環境での使用は推奨されません。試用とフィードバックは歓迎です。正式リリースをお待ちください。

## 機能

### 実装済み

- 🚀 **ワンクリックサーバー管理** - サーバーの起動、停止、再起動が簡単に
- 📥 **ワンクリックサーバーインストール** - SteamCMD経由でPalworld専用サーバーをダウンロード・インストール
- 🎮 **マルチサーバー対応** - 独立した設定・セーブ・ポートを持つ複数のサーバーを管理
- ⚙️ **ビジュアル設定編集** - 起動引数と `PalWorldSettings.ini` をGUIで編集
- 📝 **リアルタイムログ** - SSE経由でサーバーログをリアルタイム表示（履歴ログを含む）
- 🎛️ **REST APIコマンド** - RCONの代替となる broadcast / save / shutdown / kick / ban
- 🌍 **多言語対応** - 中国語、英語、日本語をサポート
- 📦 **単一バイナリ** - Goバイナリにフロントエンドを埋め込み（約15-25MB）
- 🖥️ **クロスプラットフォームアーキテクチャ** - コードレベルではクロスプラットフォーム。MODの都合により、現在は正式にはWindowsのみサポート

### 開発予定

- 🔧 **MOD管理** - ワークショップIDを入力し、SteamCMD経由でMODを自動ダウンロード・インストール・有効/無効化
- 🔒 **認証** - JWT認証とユーザー管理によるリモートアクセスの保護
- 📊 **リアルタイム監視** - CPU、メモリ、オンラインプレイヤー統計
- ⬆️ **自動更新** - GitHub Releasesに基づく更新検出とワンクリック更新

## インストールと使用方法

### ダウンロードとインストール

1. [Releases](https://github.com/TBro1998/PalWorld-Server-Manager/releases) ページから最新バージョンをダウンロード
2. ダウンロードしたアーカイブを任意のディレクトリに解凍
3. `palworld-server-manager.exe`（Windows）をダブルクリックして実行

### 初回セットアップ

1. プログラム起動後、ブラウザで `http://127.0.0.1:8080` にアクセス（Docker/リモートデプロイの場合は `http://<ホストIP>:8080`）
2. 初回アクセス時に管理者アカウントを作成
3. ログイン後、Palworldサーバーの管理を開始できます

### Dockerデプロイ（Linux推奨）

マネージャーとPalworldゲームサーバーは**同一のコンテナ**内で動作し、データはボリュームに永続化されるため、コンテナを再構築してもセーブは失われません。

```bash
# 1. コードを取得
git clone https://github.com/TBro1998/PalWorld-Server-Manager.git
cd PalWorld-Server-Manager

# 2. ビルドして起動（初回ビルドはフロントエンド+バックエンドをコンパイルするため数分かかります）
docker compose up -d --build

# 3. ブラウザで http://<ホストIP>:8080 にアクセスし、管理者アカウントを作成後、サーバーをインストール・管理
```

要点：

- 本番環境で使用する前に、**必ず `docker-compose.yml` の `JWT_SECRET` を変更してください**。
- SteamCMDとPalworldサーバーは、プログラムがコンテナ内で**初回実行時に自動的に** `/data` ボリュームへダウンロードします。手動での事前インストールは不要です。
- デフォルトのポートマッピング：`8080/tcp`（管理画面）、`8211/udp`（ゲーム）、`27015/udp`（クエリ）。
  UIでサーバーの `-port` / `-QueryPort` を変更した場合は、composeのポートマッピングも合わせて調整してください。
- データボリューム `psm-data` はコンテナの `/data` にマウントされ、データベース、SteamCMD、セーブ、ログを含みます。このボリュームをバックアップすれば全データをバックアップできます。
- イメージはDebian（glibc）ベースで、SteamCMDとPalworld Linuxサーバーに必要なランタイムライブラリを内蔵しています。コンテナは非rootユーザー `steam` で実行されます。

### Linuxネイティブデプロイ

Dockerを使わずに、Linuxホスト上で直接実行することもできます（x86_64、glibcが必要）：

```bash
# 依存関係（Debian/Ubuntuの例）：SteamCMDは32ビットプログラムです
sudo dpkg --add-architecture i386 && sudo apt-get update
sudo apt-get install -y ca-certificates lib32gcc-s1 libstdc++6 libstdc++6:i386

# 実行（外部アクセス用に HOST=0.0.0.0 を設定）
HOST=0.0.0.0 PORT=8080 JWT_SECRET=your-secret ./palworld-server-manager
```

プログラムはPalworldに必要な `steamclient.so` のシンボリックリンクを `~/.steam/sdk64` に自動作成します。インストール完了後、サーバーを起動できます。

## 主な機能

### サーバー管理（実装済み）
- Palworld専用サーバーをワンクリックでインストール
- サーバーの起動、停止、再起動
- サーバーパラメータ、起動引数、ポートをGUIで編集
- サーバーの稼働状況の確認
- 複数サーバーの独立管理

### ログ（実装済み）
- リアルタイムでサーバーログを表示（SSEプッシュ）
- 履歴ログの表示

### REST APIコマンド（実装済み）
- broadcast / save / shutdown / kick / ban（RCONの代替）

### 開発予定
- **MOD管理**：ワークショップIDを入力してMODを自動ダウンロード・インストール、ワンクリックで有効/無効化、インストール済みリストの管理
- **認証**：ユーザー名・パスワードによるログインとJWTセッション保護
- **システム監視**：サーバーのCPU、メモリ使用状況、オンラインプレイヤー数
- **自動更新**：GitHub Releasesに基づく更新検出とワンクリック更新

## 設定

プログラムは2つの設定方法をサポートしており、優先順位は：**設定ファイル > 環境変数 > デフォルト値**

### 方法1：設定ファイル（推奨）

プログラムディレクトリに `config.yaml` ファイルを作成します：

```yaml
# Webインターフェース設定
host: "127.0.0.1"  # リッスンアドレス
port: 8080          # ポート

# パス設定
steamcmd_path: "./steamcmd"        # SteamCMDインストールパス
palworld_base_path: "./palworld"   # Palworldサーバーディレクトリ

# データベース
database_path: "./palworld.db"

# JWTシークレット（本番環境では変更してください）
jwt_secret: "your-secure-secret-key"
```

完全な設定例については `config.example.yaml` を参照してください。

### 方法2：環境変数

`config.yaml` ファイルが存在しない場合、プログラムは環境変数を使用します：

- `HOST` - Webインターフェースのリッスンアドレス
- `PORT` - Webインターフェースのポート
- `STEAMCMD_PATH` - SteamCMDインストールパス
- `PALWORLD_BASE_PATH` - Palworldサーバーインストールディレクトリ

## 開発者向けドキュメント

開発に参加したい方、または技術的な詳細を知りたい方は以下をご参照ください：

- [技術提案書](./PalWorld_TECHNICAL_PROPOSAL.md) - 詳細な技術設計
- [バックエンド開発ガイド](./server/README.md) - Goバックエンド開発ガイド
- [フロントエンド開発ガイド](./ui/README.md) - Next.jsフロントエンド開発ガイド

## ライセンス

[GNU Affero General Public License v3.0](./LICENSE)

## コントリビュート

プルリクエストを歓迎します！お気軽にご投稿ください。

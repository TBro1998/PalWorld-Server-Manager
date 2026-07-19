# Palworld サーバーマネージャー

> [English](./README.en.md) | [中文](./README.md) | 日本語

Palworld専用サーバーのための包括的な管理ツール。MODサポート、マルチサーバー管理、モダンなWebUIを備えています。

> ⚠️ **プロジェクトの状態：開発中**
>
> 本プロジェクトはまだ正式にリリースされておらず、現在も開発が進行中です。機能は不安定であったり、変更・未実装の可能性があるため、本番環境での使用は推奨されません。試用とフィードバックは歓迎です。正式リリースをお待ちください。

## 機能

### 🚀 サーバー管理（実装済み）
- SteamCMD経由でPalworld専用サーバーをワンクリックインストール
- サーバーの起動、停止、再起動
- マルチサーバー対応 — 独立した設定・セーブ・ポート
- サーバーパラメータ、起動引数、ポートをGUIで編集

### 📝 ログ（実装済み）
- SSEプッシュでサーバーログをリアルタイム表示
- 履歴ログの表示

### 🎛️ REST APIコマンド（実装済み）
- broadcast / save / shutdown / kick / ban（RCONの代替）

### 🔧 MOD管理（実装済み）
- ワークショップIDを入力し、SteamCMD経由でMODを自動ダウンロード・インストール・有効/無効化

### 🔒 認証（実装済み）
- ユーザー名・パスワードによるログインとJWTセッション保護（リモートアクセス向け）

### ⬆️ 自動更新（実装済み）
- GitHub Releasesに基づく更新検出とワンクリック更新

### 🌍 その他（実装済み）
- 多言語対応 — 中国語、英語、日本語
- 単一バイナリ — Goバイナリにフロントエンドを埋め込み（約15-25MB）
- クロスプラットフォームアーキテクチャ — コードレベルではクロスプラットフォーム。MODの都合により、現在は正式にはWindowsのみサポート

### 📋 開発予定
- 📊 **リアルタイム監視**：サーバーのCPU、メモリ使用状況、オンラインプレイヤー数
- ⏰ **スケジュールタスク**：定期再起動・定期保存などのスケジュール設定
- 💾 **バックアップ管理**：セーブデータの自動バックアップとワンクリック復元
- 🔄 **クラッシュ自動復旧**：サーバー異常終了時の自動再起動とアラート通知
- 🌐 **プレイヤーポータル**：ゲームプレイヤー向けの専用ログインページ。サーバー状態確認や申請などの自助操作が可能
- 🐳 **ゲームサーバーのコンテナ化**：Palworldゲームサーバーを独立したDockerコンテナ内で実行し、より優れた分離とリソース管理を実現
- 🛡️ **PalDefender サポート**：PalDefenderアンチチートシステムの統合、設定管理とステータス監視を提供

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

Docker Hubにイメージを公開しています：[`tbro98/palsm`](https://hub.docker.com/r/tbro98/palsm)

```bash
# 1. docker-compose.yml をダウンロード
curl -O https://raw.githubusercontent.com/TBro1998/PalWorld-Server-Manager/main/docker-compose.yml

# 2. イメージを取得して起動（Docker Hubから自動取得、ローカルビルド不要）
docker compose up -d

# 3. ブラウザで http://<ホストIP>:8080 にアクセスし、管理者アカウントを作成後、サーバーをインストール・管理
```

要点：

- SteamCMDとPalworldサーバーは、プログラムがコンテナ内で**初回実行時に自動的に** `/data` ボリュームへダウンロードします。手動での事前インストールは不要です。
- デフォルトのポートマッピング：`8080/tcp`（管理画面）、`8211/udp`（ゲーム）、`27015/udp`（クエリ）。
  UIでサーバーの `-port` / `-QueryPort` を変更した場合は、composeのポートマッピングも合わせて調整してください。
- `./psm-data` ディレクトリはコンテナの `/data` にマウントされ、データベース、SteamCMD、セーブ、ログを含みます。このディレクトリをバックアップすれば全データをバックアップできます。
- イメージはDebian（glibc）ベースで、SteamCMDとPalworld Linuxサーバーに必要なランタイムライブラリを内蔵しています。コンテナは非rootユーザー `steam` で実行されます。
- 最新版へ更新：`docker compose pull && docker compose up -d`

### Linuxネイティブデプロイ

Dockerを使わずに、Linuxホスト上で直接実行することもできます（x86_64、glibcが必要）：

```bash
# 依存関係（Debian/Ubuntuの例）：SteamCMDは32ビットプログラムです
sudo dpkg --add-architecture i386 && sudo apt-get update
sudo apt-get install -y ca-certificates lib32gcc-s1 libstdc++6 libstdc++6:i386

# 実行（外部アクセス用に HOST=0.0.0.0 を設定）
HOST=0.0.0.0 PORT=8080 ./palworld-server-manager
```

プログラムはPalworldに必要な `steamclient.so` のシンボリックリンクを `~/.steam/sdk64` に自動作成します。インストール完了後、サーバーを起動できます。

## AIエージェント運用スキル（palworld-ops）

本プロジェクトはAIエージェント用スキル [`skills/palworld-ops`](./skills/palworld-ops/) を提供しています。Claude Code、Claude Desktop、Codex、OpenClaw、Hermes Agent などのエージェントが、REST API を通じてサーバーを直接運用できます（ヘルスチェック、パフォーマンス調整、ガイド付きセットアップ、プレイヤー管理、トラブルシューティング、Mod ワークフロー、自動化）。

**インストール方法**：手動での設定は不要です。エージェント向けのインストール手順書をあなたのエージェントに渡し、エージェント自身にインストールさせてください。

次の文（またはドキュメントのリンク）をエージェントに送ってください：

> https://raw.githubusercontent.com/TBro1998/PalWorld-Server-Manager/main/skills/palworld-ops/INSTALL.ja.md を読んで、その手順に従って palworld-ops スキルをインストールしてください。

すでに本リポジトリをクローンしている場合は、ローカルファイル [`skills/palworld-ops/INSTALL.ja.md`](./skills/palworld-ops/INSTALL.ja.md) をエージェントに読ませて実行させることもできます。

インストール後、エージェントのセッションを再起動すれば、エージェントに Palworld サーバーの管理を依頼できます。使用前に、マネージャーが起動していることを確認し、管理者パスワードをエージェントに提供してください。

**更新方法**：ツールのアップグレード後、API の変更は自動的に反映されます（エージェントが実行時に最新の API ドキュメントを読み込みます）ので、操作は不要です。スキルの内容自体が更新された場合のみ、エージェントに最新のスキルを再取得させて旧版を上書きし、セッションを再起動してください。詳細は [INSTALL.ja.md](./skills/palworld-ops/INSTALL.ja.md) の「インストール済みスキルの更新」セクションを参照してください。

## 設定

プログラムは2つの設定方法をサポートしており、優先順位は：**環境変数 > 設定ファイル > デフォルト値**

> **注意**：環境変数は常に `config.yaml` より優先されます。DockerでファイルをマウントしつつHOST等だけ環境変数で上書きするといった混在も可能です。

### 初回起動の流れ

設定ファイルを事前に用意する必要はありません。プログラム起動後、初めてWeb UIにアクセスすると管理者パスワードの設定が案内されます。設定完了後、プログラムが**JWTシークレットを自動生成**し、すべての設定を **`config.yaml` に書き込みます**。以降の起動はそのファイルから読み込まれるため、再設定は不要です。

### 設定ファイル

プログラムディレクトリに `config.yaml` を作成します（初回起動**前**にポートやパスをカスタマイズしたい場合）：

```yaml
# Webインターフェース
host: "127.0.0.1"  # リッスンアドレス；外部アクセスやDockerの場合は 0.0.0.0 に変更
port: 8080

# パス
steamcmd_path: "./steamcmd"    # SteamCMDインストールパス
database_path: "./palworld.db" # SQLiteデータベースパス
log_dir: "./logs"              # ログディレクトリ

# 自動更新：本プロジェクトをフォークした場合のみ変更
github_repo: "TBro1998/PalWorld-Server-Manager"
```

> `jwt_secret` と `password_hash` は初回Web設定時にプログラムが自動生成して書き込みます — **手動で設定する必要はありません**。

### 環境変数（Docker / 設定ファイルなしの場合）

プログラムディレクトリに `config.yaml` が**存在しない**場合、以下の環境変数から設定を読み込みます：

| 環境変数 | 説明 | デフォルト値 |
|---|---|---|
| `HOST` | Webインターフェースのリッスンアドレス | `127.0.0.1` |
| `PORT` | Webインターフェースのポート | `8080` |
| `DATABASE_PATH` | SQLiteデータベースファイルパス | `./palworld.db` |
| `STEAMCMD_PATH` | SteamCMDインストールパス | `./steamcmd` |
| `LOG_DIR` | ログディレクトリ | `./logs` |
| `GITHUB_REPO` | 自動更新ソースリポジトリ | `TBro1998/PalWorld-Server-Manager` |

`jwt_secret` と `password_hash` は初回Web設定時に自動生成されて `config.yaml` に書き込まれるため、環境変数で提供する必要はありません。

## ライセンス

[GNU Affero General Public License v3.0](./LICENSE)

## コントリビュート

プルリクエストを歓迎します！お気軽にご投稿ください。

## 謝辞

本プロジェクトは以下のオープンソースプロジェクトにインスピレーションを受けています。感謝申し上げます：

- [PalworldSaveTools](https://github.com/deafdudecomputers/PalworldSaveTools) - Palworldセーブファイルの解析とツールライブラリ
- [palworld-save-pal](https://github.com/oMaN-Rod/palworld-save-pal) - Palworldセーブ管理ツール

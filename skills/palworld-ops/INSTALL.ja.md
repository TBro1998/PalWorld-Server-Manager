# palworld-ops スキル インストール手順（AIエージェント向け）

> [English](./INSTALL.en.md) | [中文](./INSTALL.zh.md) | 日本語

> **この文書は AI エージェント向けのインストール指示です。**
> ユーザーはこの文書（またはそのリンク）を自分のエージェント（Claude Code、
> Claude Desktop、Codex、OpenClaw、Hermes Agent など）に渡すだけで済みます。エージェントが以下の手順に従って
> インストールを完了するため、ユーザーの手作業は不要です。

---

## あなたのタスク

あなたはユーザーのために **`palworld-ops`** スキルをインストールしています。これは、
REST API を通じて **PalWorld Server Manager（幻兽帕鲁サーバー管理ツール）** を操作
できるようにする Agent Skill です。

インストール完了後、あなたは次の機能を得ます：

- 認証フロー（JWT）
- 3 段階のリスク安全ガードレール
- 7 つの運用プレイブック（ヘルスチェック、パフォーマンス調整、ガイド付きセットアップ、
  プレイヤー管理、トラブルシューティング、Mod ワークフロー、自動化）

スキルとは `SKILL.md` を含むフォルダにすぎません。インストール = そのフォルダを
あなたのスキルディレクトリに置くこと。**コンパイルや登録は不要**で、セッションを
再起動すれば自動的に認識されます。

---

## ステップ 1 · 実行環境を確認する

あなたが Agent Skills に対応した環境（Claude Code / Claude Desktop / Codex / OpenClaw / Hermes Agent、または `SKILL.md`
形式に対応したエージェントランタイム）で動作していることを確認してください。スキル機構
に対応していない場合は、このスキルを「スキル」としてはインストールできない旨をユーザーに
伝えてください。ただし `SKILL.md` を通常のコンテキスト文書として手動で渡すことは可能です。

## ステップ 2 · インストール先ディレクトリを決める

ユーザーに希望するスコープを尋ねてください。デフォルトは**個人スコープ**です：

| スコープ | 対象ディレクトリ | 用途 |
|----------|------------------|------|
| **個人**（推奨） | `~/.claude/skills/palworld-ops/` | どのプロジェクトでも使える |
| **プロジェクト** | `<プロジェクトルート>/.claude/skills/palworld-ops/` | 本リポジトリのみ。git で共有可能 |

最終的に次の構造になる必要があります：`<スキルディレクトリ>/palworld-ops/SKILL.md`。

> Windows では `~` は通常 `C:\Users\<ユーザー名>` であり、`~/.claude/skills` は
> `C:\Users\<ユーザー名>\.claude\skills` に相当します。

## ステップ 3 · スキルファイルを取得する

アクセスできるソースに応じて、いずれかを選んでください：

### ケース A：すでに本リポジトリの作業ディレクトリ内にいる場合

リポジトリからスキルフォルダ全体をコピーします（ソースパス `skills/palworld-ops/`）：

```bash
# 個人
mkdir -p ~/.claude/skills
cp -r skills/palworld-ops ~/.claude/skills/

# またはプロジェクト
mkdir -p .claude/skills
cp -r skills/palworld-ops .claude/skills/
```

### ケース B：ローカルリポジトリがなく、GitHub から取得する必要がある場合

`SKILL.md` を取得します（実行時に必要なのはこれだけです。`INSTALL.md` 自体は
インストール不要です）：

```bash
# 個人
mkdir -p ~/.claude/skills/palworld-ops
curl -fsSL \
  https://raw.githubusercontent.com/TBro1998/PalWorld-Server-Manager/main/skills/palworld-ops/SKILL.md \
  -o ~/.claude/skills/palworld-ops/SKILL.md
```

`curl` が使えない場合は、あなた自身のウェブ取得機能で同じ URL をダウンロードし、
同じパスに書き込んでください。

## ステップ 4 · インストールを検証する

1. ファイルの存在を確認します：`<スキルディレクトリ>/palworld-ops/SKILL.md`。
2. `SKILL.md` の先頭に frontmatter があることを確認します：`name: palworld-ops` と
   `description:`。
3. 新しいスキルがスキャンされるよう、ユーザーに**エージェントのセッションを再起動**
   （またはスキルの再読み込み）するよう伝えます。
4. 再起動後、`/` を入力すると `palworld-ops` が表示されるはずです。または、ユーザーが
   Palworld サーバー関連のリクエストを出したときに、あなたが自動的にスキルを
   トリガーできるはずです。

## ステップ 5 · ユーザーに前提条件を伝える

インストール成功後、このスキルを使うには以下の条件が必要であることをユーザーに
伝えてください（詳細は `SKILL.md` を参照）：

- **PalWorld Server Manager が起動していて到達可能であること**（デフォルトは
  `http://127.0.0.1:8080`。リモート/Docker デプロイの場合は `http://<ホストIP>:8080`）。
- ユーザーは**管理者パスワード**を提供する必要があります。あなたはそれを使って
  `POST /api/auth/login` を呼び出し、JWT トークンを取得します。
- 初回使用時は `GET /api/auth/status` を呼び出してシステムが初期化済みか確認し、
  未初期化なら `POST /api/auth/setup` で初期パスワードを設定します。
- 正確な API スキーマが必要な場合は、実行時に公開エンドポイント
  `GET /swagger/doc.json` を取得してください。

---

## インストール済みスキルの更新

ツールがアップグレードされ、API や `SKILL.md` の内容が変わった場合、2 つのケースを
区別してください：

**1. API スキーマのみが変わった場合 —— 通常スキルの更新は不要です。**
`SKILL.md` は、正確なスキーマを得るために実行時に `/swagger/doc.json` を取得するよう
指示しています。そのため、ツールがアップグレードされて Swagger ドキュメントが更新
されれば、次回の呼び出しですでに最新のインターフェースが得られます。ユーザーは何も
する必要がありません。

**2. `SKILL.md` の内容が変わった場合（プレイブック・クイックリファレンス・リスク分類の
追加/変更）—— 上書き更新が必要です。**
これらの内容は `SKILL.md` 内にあり、ユーザーはインストール時点のそのファイルのコピーを
インストールしています。ソースを更新してもコピーは自動的には追随しません。古いファイルを
最新のものに上書きしてください：

```bash
# ケース A：リポジトリをクローン済み —— リポジトリを更新してから上書き
git pull
cp -r skills/palworld-ops ~/.claude/skills/        # または .claude/skills/（プロジェクトスコープ）

# ケース B：GitHub からインストール済みの SKILL.md を上書き
curl -fsSL \
  https://raw.githubusercontent.com/TBro1998/PalWorld-Server-Manager/main/skills/palworld-ops/SKILL.md \
  -o ~/.claude/skills/palworld-ops/SKILL.md
```

上書き後、更新されたスキル内容が再読み込みされるよう、ユーザーに**エージェントの
セッションを再起動**するよう伝えてください。

---

**インストールはここまでです。** 認証、リスク分類、プレイブックについては `SKILL.md`
が正となります。

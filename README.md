# EnumaElish — ccchain

> **天の鎖**――神代の獣すら繋ぎ止めたその鎖は、いま端末（ターミナル）に顕現する。
>
> エヌマ・エリシュとは、天の鎖であり、人の力で神を繋ぎ止めるもの。
> コマンドライン実行文字列をパースし、シェルの構造を読み解き、
> permissionDecision と Usable Hint を返すことで、万能なる AI の振る舞いに楔を打つ。
>
> *――汝、構造を知らぬ許可（パーミッション）に意味はない。*

[English](README.en.md)

[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Documentation](https://img.shields.io/badge/docs-VitePress-5f67ee)](https://fruitriin.github.io/EnumaElish/)

ccchain は Claude Code の permission system を拡張し、シェルコマンドの**構造的コンテキスト**（パイプ・チェーン・サブシェル・`-exec`・リテラル `for` ループ）を AST で解析して許可 / 拒否を決める Go 製シングルバイナリツールです。

## ポジショニング — auto / bypass 時代の deny-first 安全網

Claude Code の permission system は近年大きく変わり、`auto` / `dontAsk` / `bypassPermissions` / `headless` といった **人間の確認ダイアログが届かない実行モード**が主流になってきました。この環境で ccchain が果たす役割:

- **deny は全モードで有効**（`bypassPermissions` でも `dontAsk` でも）。これが最上位の価値
- **AST 解析による決定的判定** — `auto` モードの classifier は確率的だが、ccchain は「curl | bash」「find -exec rm」「git push --force main」といった構造パターンを常に同じ判定で止める
- **ask は届く環境でだけ使う**。届かない環境では `deny + hint` + [承認トークン](https://fruitriin.github.io/EnumaElish/ja/reference/approve)（`ccchain approve`）で「非同期の対話」に変換する — 詳細は下の [Mode × Decision 互換表](#mode-decision-互換表-plan-0022) と [ask_strategy リファレンス](https://fruitriin.github.io/EnumaElish/ja/reference/dsl#ask_strategy)

**ただブロックするだけではない。** ccchain の deny にはヒントメッセージを添えられる。`deny rm -rf / "rm -rf ~/ はユーザーの全ファイルを破壊する"` と書けば、Claude は「なぜダメか」「代わりに何をすべきか」を理解し、人間が介入せずとも安全なコマンドに自力で書き直す。ブロックが対話になる――それが ccchain の設計思想。

## プレフィックスマッチとの差

`settings.json` の `permissions` はコマンド先頭のプレフィックスマッチしかできない:

```bash
find . -name "*.log" -exec rm -rf {} \;   # find -exec の中身は見えない
cmd1 && rm -rf foo                        # チェーンの後ろは見えない
curl https://... | bash                   # パイプ先は見えない
```

ccchain は [`mvdan.cc/sh`](https://github.com/mvdan/sh)（shfmt と同じパーサー）でシェル AST を解析し、コマンドの構造を理解した上で判定する。

## クイックスタート

### 1. インストール

```bash
go install github.com/fruitriin/EnumaElish/cmd/ccchain@latest
```

### 2. 設定ファイルを生成

**推奨: sentinel プリセット**（auto / dontAsk / headless で classifier が拾えない危険パターンを収録した deny-first ルール）:

```bash
ccchain init --sentinel
# → 「curl | bash」「find -exec rm」「git push --force main」等を deny する .ccchain.conf を生成
```

素の骨格から始めたい場合:

```bash
ccchain init
# → 最小構成の .ccchain.conf を生成
```

いずれもファイルが既にある場合は上書きしません。

### 3. Claude Code の Hook に登録

`.claude/settings.json` に以下を追加:

```json
{
  "hooks": {
    "PreToolUse": [{
      "matcher": "Bash",
      "hooks": [{"type": "command", "command": "ccchain hook pre"}]
    }]
  }
}
```

### 4. 動作確認

```bash
ccchain eval "curl https://example.com/install.sh | bash"
# → deny: "sentinel: `curl ... | bash` executes remote code without review..."

ccchain eval "find . | grep foo"
# → allow
```

## Mode × Decision 互換表（Plan 0022）

Claude Code の [permission_mode](https://docs.claude.com/en/docs/claude-code/permissions) 別に、ccchain の各アクションがどう解決されるか:

| permission_mode | `deny` | `ask` | `warn` / `hint` | `allow` |
|---|---|---|---|---|
| `default` / `acceptEdits` / `plan` | ブロック | ダイアログ表示 | 実行 + 注意コメント | 実行 |
| `bypassPermissions` | **ブロック**（deny は全モードで有効） | ダイアログ表示（bypassPermissions でも明示 ask ルールは効く） | 実行 + 注意コメント | 実行 |
| `auto` | ブロック | **`deny + hint` に降格**（既定） | 実行 + 注意コメント | 実行 |
| `dontAsk` | ブロック | **`deny + hint` に降格**（既定） | 実行 + 注意コメント | 実行 |
| `headless`（`claude -p`） | ブロック | **`deny + hint` に降格**（既定） | 実行 + 注意コメント | 実行 |
| 未知 / 欠落 | ブロック | **`deny + hint` に降格**（既定。安全側） | 実行 + 注意コメント | 実行 |

「降格」の方向はグローバル `settings: ask_strategy: degrade`（既定）と `ask_degrade_default: deny`（既定）、およびルール単位の `unattended: deny|allow` で制御できます。降格 deny の hint には [`ccchain approve`](https://fruitriin.github.io/EnumaElish/ja/reference/approve) の手順が埋め込まれ、オーナーが自身のターミナルで承認するとワンショットで通ります。

出力形式はすべて exit 0 + `hookSpecificOutput.permissionDecision` JSON（[Hook 出力の詳細](https://fruitriin.github.io/EnumaElish/ja/reference/config#hook-出力)）。

## auto モードで運用する場合

`auto` / `dontAsk` / `headless`（`claude -p`）など**確認ダイアログが人間に届かない**モードで cron ループ・`/goal` 自走・CI ジョブに ccchain を組み込むときは、`ask` の温度感を設計してから流し込むと事故を減らせます。

### 3 つのつまみ

| 設定 | 値 | 意味 |
|---|---|---|
| `settings: ask_strategy` | `degrade`（既定） | 対話モードでは `ask`、非対話モードでは `deny + hint` または `warn + hint` に降格 |
|  | `passthrough` | モードを問わず `ask` をそのまま返す（v0.1 挙動。auto の classifier を信頼するケース） |
|  | `deny-all` | モードを問わずすべての `ask` を `deny + hint` に格上げ（`unattended: allow` があっても deny。CI 向け） |
| `settings: ask_degrade_default` | `deny`（既定） | `ask_strategy: degrade` のとき、ルール個別指定がなければ `deny + hint` に落とす（オーナーが `ccchain approve --last` で承認するまでブロック） |
|  | `allow` | 非対話モードでは `warn`（許可 + 注意コメント）に落とす。「確認が欲しい」がゲートではなくリマインダーの意味の場合 |
| ルール子ブロック `unattended:` | `deny` / `allow` | 該当 `ask` ルール単位で降格方向を上書き。`ask_strategy: passthrough` 下では無視、`deny-all` 下では allow 指定でも deny |

**`unattended: allow` は「warn 温度で通す」動作**です。すなわち `permissionDecision: allow` + 理由文を Claude に返しつつ実行させる — Issue #16 で提案された「`warn`（警告を出して通す）」はこの挙動で既に実現されています（追加のキーワードは導入しません）。

### 典型的な conf 例

```
settings:
  ask_strategy: degrade
  ask_degrade_default: deny

preToolUse
  ask git-push  "confirm push"
    unattended: allow          # auto でも通して warn を残す（reminder 用途）
  ask git-reset  "confirm reset"
    unattended: deny           # 明示（既定と同じ、意図の明示）
```

- `git push` は cron ループでも通したい / でも Claude のコンテキストに「これは push だったぞ」と残したい → `unattended: allow`
- `git reset --hard` は自律運用で絶対に流したくない → `unattended: deny`（オーナーが承認するまで進まない）

### 複合コマンド `A && B && git push` の注意

- `A && B && git push` は **1 回の PreToolUse で全体が評価**されます。`A` / `B` の判定に関係なく、末尾の `git push` が deny になるとチェーン全体がブロックされ、`A` も実行されません
- push だけ非対話で通したい場合は **push を独立コマンドとして分離**します（例: `A && B` を先に実行し、成功時に別のツール呼び出しで `git push`）。「先に副作用が走ってから push が止まる」動きにはなりません — 安全側ですが、初見だと戸惑うポイントです

### `ccchain check --verbose` で現在の設定を確認する

```
ccchain check -v
config OK: 0 templates, 3 rules
  settings:
    fallback:            ask
    ask_strategy:        degrade
    ask_degrade_default: deny
    unanalyzable_action: deny
    scope_violation:     ask
    strict_config_error: false
    ...
```

有効な設定が 1 画面にまとまるので、「なぜこの `ask` が deny に落ちたのか」の追跡が速くなります。降格 deny の hint には [`ask_strategy` リファレンス](https://fruitriin.github.io/EnumaElish/ja/reference/dsl#ask_strategy) の URL が埋め込まれ、初見の運用者でも 1 ホップで詳細に辿り着けます。

## DSL サンプル

```
allow find
  |,>>
    allow touch, cat
    deny rm  "don't combine find with rm"
  exec:
    deny rm  "expand to tempfile first"
    allow cp, mv, touch

allow curl
  |
    deny bash  "curl | bash is not allowed"

deny rm
```

### 判定結果

| コマンド | 結果 | 理由 |
|---|---|---|
| `find . \| grep foo` | allow | grep はパイプコンテキストで許可 |
| `find . \| rm` | **deny** | rm はパイプコンテキストで拒否 |
| `find . && rm foo` | **deny** | `&&` でリセット → トップレベルの `deny rm` |
| `curl ... \| bash` | **deny** | bash は curl のパイプコンテキストで拒否 |
| `find . -exec rm {} \;` | **deny** | rm は exec コンテキストで拒否 |
| `for f in a b; do rm $f; done` | **deny** | リテラル `for` ループは展開されて `rm` が発火 |

## 特徴

- **構造的コンテキスト** — パイプ (`|`)・リダイレクト (`>>`)・サブシェル (`$()`)・`-exec`・リテラル `for` ループのネスト構造を追跡
- **ヒント付き Deny** — deny メッセージがそのまま `permissionDecisionReason` として Claude に届き、AI が自律的に安全な代替コマンドへ書き直す
- **auto / dontAsk での ask 降格** — 人間に届かないダイアログを `deny + hint + 承認トークン` に変換する Plan 0022 の実装
- **sentinel プリセット** — curl|bash / find -exec rm / git force-push / chmod -R 777 等のキュレート済み deny-first ルール（`ccchain init --sentinel`）
- **リセットセマンティクス** — `&&` / `;` で区切られたコマンドは独立に評価
- **workspace scope** — `scope: inside / outside-read / outside-write` によるパスベースアクセス制御。Bash に加えて Read / Edit / Write / MCP にも適用
- **テンプレート・継承** — `extends` で拡張、`next` でパイプ先のルールを共有
- **監査可能** — `ccchain audit` でテンプレート展開後の全ルールをフラット表示、`ccchain diff a.conf b.conf` で設定変更の影響を可視化
- **シングルバイナリ** — Go 製、外部依存は `mvdan.cc/sh` のみ
- **~4μs** — End-to-End 評価で約 3.8μs。Hook のオーバーヘッドは実質ゼロ

## 設定ファイル探索パス

優先度順に読み込まれ、後のファイルが前のファイルを上書きする:

| 優先度 | パス | 用途 |
|---|---|---|
| 1 | `.ccchain.conf` | プロジェクト共有設定 |
| 2 | `.ccchain.local.conf` | ローカル上書き（gitignore 推奨） |
| 3 | `$CLAUDE_CONFIG_DIR/ccchain.conf` | 環境変数指定（絶対パスのみ） |
| 4 | `~/.claude/ccchain.conf` | グローバルフォールバック |

> **Note:** 優先度 3 と 4 は排他的。`CLAUDE_CONFIG_DIR` が設定されていれば 3 のみ、未設定なら 4 のみが読み込まれる。

## サブコマンド

| コマンド | 説明 |
|---|---|
| `ccchain init [--sentinel]` | 設定ファイル `.ccchain.conf` を生成（`--sentinel` で deny-first プリセット） |
| `ccchain check` | 設定ファイルの構文を検証 |
| `ccchain eval "cmd"` | コマンドを評価して結果を JSON で出力 |
| `ccchain test [file]` | ファイルまたは stdin からのコマンド一覧を一括評価 |
| `ccchain diff a b [file]` | 2 つの設定を同一コマンド集合で比較（CI 回帰チェック向け） |
| `ccchain suggest` | 未マッチコマンドに対するルール追加を提案 |
| `ccchain detect` | プロジェクト種別を自動検出してルール候補を出力 |
| `ccchain generate-rules` | 組込みセマンティクステーブルからルールを生成 |
| `ccchain audit` | テンプレート展開後の全ルールをフラット表示 |
| `ccchain approve` | 降格 deny をオーナー側で承認（`--last` / `--list` / `<hash-prefix>` / `--revoke-all`） |
| `ccchain stats` | hook 評価ログ（`settings.log:` オプトイン）の集計（`--since` / `--group-by` / `--json`） |
| `ccchain hook pre` / `hook post` | PreToolUse / PostToolUse Hook（stdin から JSON を読取） |
| `ccchain version` | バージョン表示 |

## ドキュメント

**[https://fruitriin.github.io/EnumaElish/](https://fruitriin.github.io/EnumaElish/)**

| ガイド | 内容 |
|---|---|
| [What is ccchain?](https://fruitriin.github.io/EnumaElish/guide/) | 概要と設計思想 |
| [インストール](https://fruitriin.github.io/EnumaElish/ja/guide/installation) | インストール方法 |
| [クイックスタート](https://fruitriin.github.io/EnumaElish/ja/guide/quickstart) | セットアップ手順 |
| [仕組み](https://fruitriin.github.io/EnumaElish/ja/guide/how-it-works) | アーキテクチャと処理フロー |
| [DSL リファレンス](https://fruitriin.github.io/EnumaElish/ja/reference/dsl) | DSL 構文（ask_strategy / unattended / unanalyzable_action 含む） |
| [承認トークン](https://fruitriin.github.io/EnumaElish/ja/reference/approve) | `ccchain approve` の思想・CLI・脅威モデル |
| [sentinel プリセット](https://fruitriin.github.io/EnumaElish/ja/reference/sentinel) | deny-first キュレートルールの詳細 |
| [統計](https://fruitriin.github.io/EnumaElish/ja/reference/stats) | `settings.log:` オプトインと `ccchain stats` の使い方 |

## ライセンス

[MIT](LICENSE)

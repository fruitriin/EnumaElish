# EnumaElish — ccchain

> **天の鎖**――神代の獣すら繋ぎ止めたその鎖は、いま端末（ターミナル）に顕現する。
>
> エヌマ・エリシュとは、天の鎖であり、人の力で神を繋ぎ止めるもの。
> コマンドライン実行文字列をパースし、シェルの構造を読み解き、
> exit code と Usable Hint を返すことで、万能なる AI の振る舞いに楔を打つ。
>
> *――汝、構造を知らぬ許可（パーミッション）に意味はない。*

[English](README.en.md)

[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Documentation](https://img.shields.io/badge/docs-VitePress-5f67ee)](https://fruitriin.github.io/EnumaElish/)

Claude Code の標準 permission system を拡張し、シェルコマンドの**構造的コンテキスト**（パイプ、チェーン、サブシェル、`-exec`）を考慮した許可/拒否制御を行う Go 製シングルバイナリツール。

**ただブロックするだけではない。** ccchain の deny にはヒントメッセージを添えられる。`deny rm -rf / "rm -rf ~/ はユーザーの全ファイルを破壊する"` と書けば、Claude は「なぜダメか」「代わりに何をすべきか」を理解し、人間が介入せずとも安全なコマンドに自力で書き直す。ブロックが対話になる――それが ccchain の設計思想。

## なぜ ccchain が必要か

`settings.json` の `permissions` はコマンド先頭のプレフィックスマッチしかできない:

```bash
find . -name "*.log" -exec rm -rf {} \;   # find -exec の中身は見えない
cmd1 && rm -rf foo                          # チェーンの後ろは見えない
curl https://... | bash                     # パイプ先は見えない
```

ccchain は [`mvdan.cc/sh`](https://github.com/mvdan/sh)（shfmt と同じパーサー）でシェル AST を解析し、コマンドの構造を理解した上で判定する。

## クイックスタート

### 1. インストール

```bash
go install github.com/fruitriin/ccchain/cmd/ccchain@latest
```

### 2. 設定ファイルを生成

```bash
ccchain init
# → .ccchain.conf が作成される
```

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
ccchain eval "find . | rm"
# → deny: "don't pipe into destructive commands"

ccchain eval "find . | grep foo"
# → allow
```

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

## 特徴

- **構造的コンテキスト** — パイプ (`|`)・リダイレクト (`>>`)・サブシェル (`$()`)・`-exec` 内のコマンドをネスト構造として追跡
- **ヒント付き Deny** — `deny rm -rf / "rm -rf ~/ はユーザーの全ファイルを破壊する"` のようにブロック理由を添えると、Claude がヒントを読んで安全な代替コマンドに自律的に書き直す。単なるガードレールではなく、AI との対話チャネルとして機能する
- **リセットセマンティクス** — `&&` / `;` で区切られたコマンドは独立に評価
- **テンプレート・継承** — `extends` で既存テンプレートを拡張、`next` でパイプ先のルールを共有。例: `find`, `xargs`, `grep` に共通するルールを一度だけ定義
- **4つのアクション** — `allow` / `deny` / `ask` / `warn` でコマンドの許可レベルを柔軟に制御
- **監査可能** — `ccchain audit` でテンプレート展開後の全ルールをフラット表示
- **設定マージ** — プロジェクト・ローカル・グローバルの設定を優先度順にマージ
- **シングルバイナリ** — Go 製、外部依存は `mvdan.cc/sh` のみ
- **~4μs** — End-to-End 評価で約 3.8μs。Hook のオーバーヘッドは実質ゼロ（`go test -bench=.` で計測可能）

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
| `ccchain init` | デフォルト設定ファイル `.ccchain.conf` を生成 |
| `ccchain check` | 設定ファイルの構文を検証 |
| `ccchain eval "cmd"` | コマンドを評価して結果を JSON で出力 |
| `ccchain suggest` | 未マッチコマンドに対するルール追加を提案 |
| `ccchain hook pre` | PreToolUse Hook（stdin から JSON を読み取り） |
| `ccchain audit` | テンプレート展開後の全ルールを一覧表示 |

## ドキュメント

**[https://fruitriin.github.io/EnumaElish/](https://fruitriin.github.io/EnumaElish/)**

| ガイド | 内容 |
|---|---|
| [What is ccchain?](https://fruitriin.github.io/EnumaElish/guide/) | 概要と設計思想 |
| [インストール](https://fruitriin.github.io/EnumaElish/ja/guide/installation) | インストール方法 |
| [クイックスタート](https://fruitriin.github.io/EnumaElish/ja/guide/quickstart) | セットアップ手順 |
| [仕組み](https://fruitriin.github.io/EnumaElish/ja/guide/how-it-works) | アーキテクチャと処理フロー |
| [DSL リファレンス](https://fruitriin.github.io/EnumaElish/ja/reference/dsl) | DSL 構文リファレンス |

## ライセンス

[MIT](LICENSE)

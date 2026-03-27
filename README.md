# EnumaElish — ccchain

> Claude Code Chain: 構造的パーミッション制御ツール

[![Documentation](https://img.shields.io/badge/docs-VitePress-5f67ee)](https://fruitriin.github.io/EnumaElish/)

Claude Code の標準 permission system を拡張し、シェルコマンドの構造的コンテキスト（パイプ、チェーン、サブシェル）を考慮した許可/拒否制御を行う Go 製シングルバイナリツール。

## クイックスタート

```bash
# インストール
go install github.com/fruitriin/ccchain/cmd/ccchain@latest

# デフォルト設定を生成
ccchain init

# .claude/settings.json に Hook を登録
# → ドキュメント参照

# 動作確認
ccchain eval "find . | rm"
# → {"action":"deny","message":"don't pipe into destructive commands",...}
```

## なぜ ccchain が必要か

`settings.json` の `permissions` はコマンド先頭のプレフィックスマッチしかできない:

```bash
find . -name "*.log" -exec rm -rf {} \;   # find -exec の中身は見えない
cmd1 && rm -rf foo                          # チェーンの後ろは見えない
curl https://... | bash                     # パイプ先は見えない
```

ccchain は `mvdan.cc/sh`（shfmt と同じパーサー）でシェル AST を解析し、構造を理解した上で判定する。

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

| コマンド | 結果 | 理由 |
|---|---|---|
| `find . \| grep foo` | allow | grep はパイプコンテキストで許可 |
| `find . \| rm` | **deny** | rm はパイプコンテキストで拒否 |
| `find . && rm foo` | **deny** | && でリセット → トップレベルの deny rm |
| `curl ... \| bash` | **deny** | bash はcurl のパイプコンテキストで拒否 |
| `find . -exec rm {} \;` | **deny** | rm は exec コンテキストで拒否 |

## 特徴

- **構造的コンテキスト** — パイプ・リダイレクト・サブシェル内のコマンドをネストとして追跡
- **リセットセマンティクス** — `&&` / `;` で区切られたコマンドは独立に評価
- **テンプレート・継承** — `find`, `xargs`, `grep` に共通するルールを `extends` / `next` で共有
- **監査可能** — `ccchain audit` で全ルールのフラット展開を表示
- **deny で AI を誘導** — ブロック理由を Claude に伝え、自律的な書き直しを可能にする
- **シングルバイナリ** — Go 製、外部依存は `mvdan.cc/sh` のみ
- **~5μs** — Hook のオーバーヘッドは実質ゼロ

## ドキュメント

**[https://fruitriin.github.io/EnumaElish/](https://fruitriin.github.io/EnumaElish/)**

| ガイド | 内容 |
|---|---|
| [What is ccchain?](https://fruitriin.github.io/EnumaElish/guide/) | 概要と設計思想 |
| [Installation](https://fruitriin.github.io/EnumaElish/guide/installation) | インストール方法 |
| [Quick Start](https://fruitriin.github.io/EnumaElish/guide/quickstart) | セットアップ手順 |
| [How It Works](https://fruitriin.github.io/EnumaElish/guide/how-it-works) | アーキテクチャと処理フロー |
| [DSL Reference](https://fruitriin.github.io/EnumaElish/reference/dsl) | DSL 構文リファレンス |

## ライセンス

MIT

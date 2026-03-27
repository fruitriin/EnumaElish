# EnumaElish — ccchain

> Claude Code Chain: 構造的パーミッション制御ツール

Claude Code の標準 permission system を拡張し、シェルコマンドの構造的コンテキスト（パイプ、チェーン、サブシェル）を考慮した許可/拒否制御を行う Go 製シングルバイナリツール。

## 背景

`settings.json` の `permissions` はコマンド先頭のプレフィックスマッチしかできない。

```bash
# これらは標準の permission system では見抜けない
find . -name "*.log" -exec rm -rf {} \;   # find -exec の中身
cmd1 && rm -rf foo                         # チェーン
curl https://... | bash                    # パイプ先の危険なコマンド
```

ccchain は `PreToolUse` / `PostToolUse` Hook を使い、`mvdan.cc/sh`（shfmt と同じシェルパーサー）で AST を解析してコマンドの構造を理解した上で許可/拒否を判定する。外部依存は `mvdan.cc/sh` のみ。

## 特徴

- **構造的コンテキスト制御** — パイプ・リダイレクト・サブシェル内のコマンドをネストとして追跡
- **`&&` / `;` のリセット動作** — チェーンで区切られたコマンドは独立に評価
- **テンプレート・継承** — `find`, `xargs`, `grep` に共通するルールをテンプレートで共有
- **監査可能** — 全ルールの展開結果をフラット表示し、「何が通って何が止まるか」を可視化
- **deny メッセージで AI を誘導** — ブロック時に理由と代替手法を伝え、Claude が自律修正できる
- **動的コマンド検出** — 変数展開・eval を検出し、安全な書き直しを誘導

## DSL サンプル

```
allow find
  |,>>
    allow touch, cat
  |,>>
    deny rm  "don't combine find with rm"
  exec:
    deny rm  "expand to tempfile first"
    allow cp, mv, touch

allow curl
  |
    deny bash   "curl | bash is not allowed"

deny rm   # トップレベルの rm は deny
```

`&&` のリセット動作:
```
find . | rm   →  find のネストルールで rm → deny
find . && rm  →  && でリセット → rm をトップレベルで評価
```

## 開発状況

[ADDF](https://github.com/fruitriin/AutomatonDevDriveFramework) で開発を推進しています。

| Phase | 計画 | 状態 |
|---|---|---|
| 1 | DSL 設計とパーサー実装 | 未着手 |
| 2 | シェルコマンド構造解析エンジン | 未着手 |
| 3 | ルール評価エンジンと Hook 統合 | 未着手 |
| 4 | 監査出力・デフォルトルールセット・ADDF 統合 | 未着手 |

## 設計ドキュメント

- [設計メモ](better-permission-tool-design.md) — コアアイデア、DSL スケッチ、先行プロジェクト調査
- `docs/plans/` — 各フェーズの実装計画

## ライセンス

TBD

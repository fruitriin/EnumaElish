# デフォルトルール

`ccchain init` で生成されるデフォルトルールセットの詳細です。

## テンプレート

### `primitive`
安全な出力先コマンド（読み取り専用）:
```
template primitive
  |,>>
    allow cat, echo, head, tail, wc, sort, uniq
```

### `safeRead`
読み取り指向の処理コマンド。primitive を継承:
```
template safeRead
  next: primitive
  |,>>
    allow grep, awk, sed
```

### `bulkExec`
一括処理コマンド。破壊的コマンドからの保護付き:
```
template bulkExec
  extends: safeRead
  |,>>
    deny rm    "don't pipe into destructive commands"
  exec:
    deny rm    "expand to tempfile first"
    allow cp, mv, touch
```

## コマンドルール

| コマンド | アクション | テンプレート | 説明 |
|---|---|---|---|
| `ls` | allow | primitive | ディレクトリ一覧。パイプ先は cat, head 等のみ |
| `find` | allow | bulkExec | パイプで rm に流すのを防止、-exec rm も防止 |
| `xargs` | allow | bulkExec | find と同じ保護 |
| `grep` | allow | safeRead | 読み取り専用のフィルター処理 |
| `rm` | **ask** | — | ユーザーに確認を求める |
| `curl \| bash` | **deny** | — | リモートコード実行を防止 |
| `curl \| sh` | **deny** | — | リモートコード実行を防止 |
| `eval` | **deny** | — | 静的解析不能なため拒否 |

## 設定

```
settings:
  max_context_depth: 2    # audit 展開の最大深度
  max_rules_per_cmd: 5    # audit でのコマンドあたりのルール数上限
  fallback: ask           # ルールにマッチしないコマンドのデフォルト動作
```

### `fallback` の選択肢

| 値 | 効果 |
|---|---|
| `ask` | ユーザーに確認（デフォルト、推奨） |
| `allow` | 全て許可（テスト用） |
| `deny` | 全て拒否（厳格モード） |

## カスタマイズ

デフォルトルールの後にルールを追加すると、last-rule-wins で上書きできます:

```
# .ccchain.conf に追記

# プロジェクトでは rm を許可
allow rm

# プロジェクト固有のビルドツール
allow gradle
  next: bulkExec
```

個人用の上書きは `.ccchain.local.conf` に:

```
# .ccchain.local.conf（.gitignore 対象）

# 自分の環境では全て許可
allow rm
allow eval
```

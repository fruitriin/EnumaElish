# 構造的コンテキスト

ccchain の核心は**構造的コンテキスト** — コマンドが「何であるか」だけでなく、シェル式の「どこに現れるか」に依存するルールを書ける能力です。

## パイプはコンテキストを構築する

```
allow find
  |,>>
    deny rm  "find と rm をパイプで繋がないでください"
```

| コマンド | 結果 |
|---|---|
| `find .` | allow |
| `find . \| rm` | **deny** |
| `find . \| grep foo` | allow |

## チェーンはコンテキストをリセットする

`&&`、`||`、`;` はリセットポイントです:

| コマンド | 評価 | 結果 |
|---|---|---|
| `find . \| rm` | find のパイプコンテキスト → deny | **deny** |
| `find . && rm` | `&&` でリセット → rm をトップレベルで評価 | トップレベルの rm ルールに従う |

## exec コンテキスト

`find -exec`、`xargs`、`bash -c`、`eval` のネストコマンドを検出:

```
allow find
  exec:
    deny rm  "一時ファイルに展開してください"
    allow cp, mv
```

## 解析不能パターン

変数展開や動的 eval は自動的に **deny** されます:

```bash
$cmd foo              # → deny
$(generate_cmd) foo   # → deny
eval "$dynamic"       # → deny
```

詳細は[英語版](/guide/structural-context)を参照してください。

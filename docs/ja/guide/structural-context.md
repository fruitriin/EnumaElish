# 構造的コンテキスト

ccchain の核心は**構造的コンテキスト** — コマンドが「何であるか」だけでなく、シェル式の「どこに現れるか」に依存するルールを書ける能力です。

## パイプはコンテキストを構築する

コマンドが `|` で接続されると、後続のコマンドは親のコンテキストルールで評価されます:

```
allow find
  |,>>
    allow grep, sort, head
    deny rm  "find と rm をパイプで繋がないでください"
```

| コマンド | コンテキスト | 結果 |
|---|---|---|
| `find .` | (トップレベル) | allow |
| `find . \| grep foo` | find → pipe → grep | allow |
| `find . \| rm` | find → pipe → rm | **deny** |
| `find . \| sort \| head` | find → pipe → sort, head | allow |

`|,>>` はパイプ (`|`) とリダイレクト (`>>`) の両方にマッチします。`|` のみ、`>>` のみも指定できます。

## チェーンはコンテキストをリセットする

`&&`、`||`、`;` は**リセットポイント**です。チェーン演算子の後のコマンドはトップレベルから再評価されます:

```
deny rm
allow find
  |,>>
    deny rm  "find と rm をパイプで繋がないでください"
```

| コマンド | 評価過程 | 結果 |
|---|---|---|
| `find . \| rm` | find のパイプコンテキスト → deny rm | **deny** (パイプルール) |
| `find . && rm foo` | `&&` でリセット → rm をトップレベルで評価 → deny rm | **deny** (トップレベルルール) |
| `find . && ls` | `&&` でリセット → ls をトップレベルで評価 | allow |

これはシェルの実行セマンティクスに一致します: `&&` は「前のコマンドが成功したら、次のコマンドを独立に実行する」という意味です。

## exec コンテキスト

一部のコマンドは引数として他のコマンドを実行します。ccchain はこれらのパターンを検出し、ネストされたコマンドを `exec:` コンテキストで評価します:

```
allow find
  exec:
    deny rm  "一時ファイルに展開してください"
    allow cp, mv
```

| コマンド | 検出 | 結果 |
|---|---|---|
| `find . -exec rm {} \;` | `-exec` で exec コンテキスト発動 | **deny** |
| `find . -exec cp {} /tmp/ \;` | `-exec` で exec コンテキスト発動 | allow |

### 対応するネストパターン

| パターン | 検出方法 |
|---|---|
| `find -exec CMD {} \;` | `-exec` / `-execdir` 引数 |
| `xargs CMD` | 最初の非フラグ引数 |
| `bash -c "CMD"` | `-c` 引数（再帰パース） |
| `sh -c "CMD"` | `-c` 引数（再帰パース） |
| `eval "CMD"` | 引数（静的文字列のみ） |

## 解析不能パターン

動的展開を含むコマンドは静的に解析できません:

```bash
$cmd foo              # 変数がコマンド名
$(generate_cmd) foo   # コマンド置換がコマンド名
eval "$dynamic"       # 動的な eval 引数
```

これらは自動的に **deny** されます:

```json
{
  "action": "deny",
  "message": "dynamic command detected: static analysis not possible"
}
```

ccchain は「分からないものは拒否する」方針です。ただし ccchain 自身のエラー（パース失敗等）は Fail-Open で許可します — この区別が重要です。動的コマンドは「分析した結果、安全性を判断できなかった」のであり、「分析自体に失敗した」のではありません。

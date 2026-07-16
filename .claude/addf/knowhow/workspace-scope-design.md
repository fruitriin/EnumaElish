# workspace スコープの設計思想と制限

## 知見

### ツール種別ごとの適用状況

| ツール | スコープ適用 | 実装箇所 |
|---|---|---|
| Read / Edit / Write / WebFetch | 適用済み | `eval/tool.go` の `EvaluateTool` |
| Bash コマンドの引数パス | Plan 0017 で適用 | `eval/evaluate.go` の `matchCommand` 末尾 |
| MCP ツール | best-effort（file_path/path/url キー） | `hook.go` の `extractMCPArg` |

Read/Edit/Write では最初から動作していたが、**Bash コマンドの引数パスには Plan 0017 まで未適用だった**。`cat /etc/passwd` はスコープをすり抜けていた。

### 複数パスによるホワイトリスト的運用

```
settings:
  workspace: ~/workspace, /tmp/hogehoge
```

この設定で:

| パス | 判定 | 理由 |
|---|---|---|
| `~/workspace/README.md` | inside → allow | workspace 内 |
| `/tmp/hogehoge/data.txt` | inside → allow | workspace 内 |
| `/tmp/other/file` | outside → ask | workspace 外 |
| `/etc/passwd` | outside → ask | workspace 外 |

`/` は deny だが `/tmp/hogehoge` は OK、のような制御ができる。

### outside は ask 止まり（deny ではない）

スコープ外アクセスが検出されても、`allow` → `ask` にエスカレーションされるだけで `deny` にはならない。ユーザーが承認すれば workspace 外のファイルも読み書きできる。

**設計上の理由**: スコープは「確認（ask）」であり「禁止（deny）」ではない。完全なブロックは `args:` ルールで行う（`.ssh/`, `.env` 等にデフォルト deny がある）。

**セキュリティレビューの指摘**: ユーザーが独自ルールで `allow Read` のみ設定し `args:` 保護を省略した場合、スコープ外は ask で止まるだけ。`scope_violation: ask|deny` の設定化が検討候補。

### parseKeyValue の1トークン制限

`workspace: ~/a, /tmp/b` の設定で `parseKeyValue` が `~/a` しか返さなかった問題。デバッグ過程:

1. `ClassifyPath("/tmp/hogehoge/data.txt", ["/tmp/hogehoge"])` → 単体テストでは正しく inside
2. `Evaluate` 内で outside になる → パースの問題に絞り込み
3. `parseKeyValue` がコロン直後の1トークンしか返さない → workspace 設定では独自パース追加

パーサーの内部動作を知らないと見つけにくいバグ。**結合テストが重要**。

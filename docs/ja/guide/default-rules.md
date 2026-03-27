# デフォルトルール

`ccchain init` で生成されるデフォルトルールセットです。

| コマンド | アクション | テンプレート | 備考 |
|---|---|---|---|
| `ls` | allow | primitive | ディレクトリ一覧 |
| `find` | allow | bulkExec | pipe-to-rm と exec-rm を防止 |
| `xargs` | allow | bulkExec | find と同じ保護 |
| `grep` | allow | safeRead | 読み取り専用 |
| `rm` | **ask** | — | ユーザーに確認 |
| `curl \| bash` | **deny** | — | リモートコード実行を防止 |
| `eval` | **deny** | — | 静的解析不能 |

詳細は[英語版](/guide/default-rules)を参照してください。

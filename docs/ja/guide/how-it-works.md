# 仕組み

## アーキテクチャ

ccchain は Claude Code の [PreToolUse hook](https://docs.anthropic.com/en/docs/build-with-claude/claude-code/hooks) として動作します。

```
Claude がコマンドを実行しようとする
         │
         ▼
┌──────────────────────┐
│  ccchain hook pre    │
│                      │
│  1. シェル AST 解析  │  ← mvdan.cc/sh (bash モード)
│  2. トポロジー構築   │  ← パイプ、チェーン、サブシェル
│  3. ルール評価       │  ← .ccchain.conf
│  4. 判定結果を返す   │
└──────────────────────┘
         │
    ┌────┴────┐
    │         │
  exit 0    exit 2
  (許可)    (拒否 + 理由メッセージ)
```

## パフォーマンス

| 処理 | 所要時間 |
|---|---|
| シェル AST パース | ~2 μs |
| トポロジー構築 | ~9 μs |
| ルール評価 | ~3 μs |
| **合計** | **~5 μs** |

Hook のオーバーヘッドは実質ゼロです。

詳細は[英語版](/guide/how-it-works)を参照してください。

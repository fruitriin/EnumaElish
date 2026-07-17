# アクション リファレンス

## アクション種別

### `allow`

コマンドの実行を許可します。出力なし。

```
allow ls
allow find
  next: bulkExec
```

**Hook 動作:** exit 0、出力なし。

### `deny`

コマンドをブロックします。メッセージは `permissionDecisionReason` として Claude に届き（[設定リファレンス / Hook 出力](./config.md#hook-出力) 参照）、自律的な書き直しが可能になります。

```
deny rm  "trash コマンドを使ってください"
deny eval  "eval は静的解析不能です。コマンドを直接記述してください"
```

**Hook 動作:** exit 0、JSON に `permissionDecision: "deny"` + `permissionDecisionReason` を出力。

**設計原則:** deny メッセージには「なぜブロックされたか」と「代わりに何をすべきか」を書く。これにより ccchain は単なるブロッカーではなく、AI への教育ツールになります。

### `warn`

コマンドを許可しますが、Claude に注意メッセージを送信します。

```
warn curl  "WebFetch の使用を検討してください"
```

**Hook 動作:** exit 0、`permissionDecision: "allow"` + `permissionDecisionReason`（注意文は Claude のコンテキストに残るが実行はブロックされない）。

**注意:** Claude が警告に従うかはモデル依存です。ccchain は出力フォーマットを保証しますが、Claude の振る舞いは制御しません。

### `ask`

Claude Code の組み込みパーミッションダイアログに委譲します。非対話モード（`auto` / `dontAsk` 等）では [`ask_strategy`](./dsl.md#ask_strategy) と ask ルールの [`unattended:`](./dsl.md#unattended) 指定に従って `deny + hint`（既定）または `warn` に降格します。

```
ask rm
  message: "ファイル削除を確認してください"
  unattended: deny   # 既定: 非対話モードでは deny+hint に降格
```

**Hook 動作:** exit 0、JSON に `permissionDecision: "ask"`（対話モードのみ）。

### `hint`

> **注意:** `ccchain hook post` は現在パススルーです。`postToolUse` ルール内での `hint` アクション、および PostToolUse のルール評価は将来のリリースで対応予定です。

PreToolUse 層での `hint` は「よりソフトな warn」として振る舞います — 実行はブロックされず、メッセージが Claude のコンテキストに届きます。

**Hook 動作:** exit 0、`permissionDecision: "allow"` + `permissionDecisionReason`。

## 評価順序

### last-rule-wins

複数のルールがマッチした場合、**最後の**マッチルールが優先:

```
allow rm      # 最初のマッチ
deny rm       # 最後のマッチ — こちらが勝つ
```

### 制限レベル

パイプラインや複合コマンドの評価では、全セグメントの中で**最も制限的な**結果が返されます:

| レベル | アクション |
|---|---|
| 0 | allow |
| 1 | hint |
| 2 | warn |
| 3 | ask |
| 4 | deny |

### フォールバック

どのルールにもマッチしないコマンドは `fallback` 設定に従います（デフォルト: `ask`）。

## 動的コマンド

変数展開やコマンド置換を含むコマンドは自動的に deny されます:

```bash
$cmd foo              # → deny（変数がコマンド名）
$(generate_cmd) foo   # → deny（コマンド置換がコマンド名）
eval "$dynamic"       # → deny（動的な eval 引数）
```

メッセージ: `"dynamic command detected: static analysis not possible"`

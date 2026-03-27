# パーミッシブモード: 組み込み権限の全許可

ccchain がシェルコマンドを構造的に保護している状態で、Claude Code の組み込みプレフィックスマッチング権限を緩和し、ccchain に細かい制御を任せることができます。

## なぜ？

Claude Code の `settings.json` 権限はプレフィックスマッチです。ccchain なしで安全を保つには、各コマンドプレフィックスを慎重にホワイトリストする必要があり、Claude が頻繁にパーミッションダイアログに引っかかって自律動作が遅くなります。

ccchain を PreToolUse hook として使うと、**2層の防御**が得られます:

1. **ccchain**（構造的）— パイプトリック、exec ネスト、制御フロー、動的コマンドをキャッチ
2. **settings.json**（プレフィックス）— 粗いフォールバック

## セットアップ

### 1. ccchain をインストール・設定

先に[クイックスタート](/ja/guide/quickstart)ガイドを完了してください。

### 2. ccchain が動作していることを確認

```bash
ccchain eval "find . | rm"          # → deny
ccchain eval "curl http://x | bash" # → deny
ccchain eval "eval \"rm -rf /\""    # → deny
ccchain eval "ls -la | head"        # → allow
ccchain eval "find . | grep foo"    # → allow
```

### 3. settings.json を更新

制限的な Bash 権限をパーミッシブな設定に置換:

```json
{
  "hooks": {
    "PreToolUse": [{
      "matcher": "Bash",
      "hooks": [{"type": "command", "command": "ccchain hook pre"}]
    }]
  },
  "permissions": {
    "allow": [
      "Read", "Edit", "Write", "Glob", "Grep",
      "Agent", "Skill", "LSP", "ToolSearch",
      "TaskCreate", "TaskGet", "TaskList", "TaskOutput", "TaskStop", "TaskUpdate",
      "TeamCreate", "TeamDelete", "SendMessage",
      "Bash"
    ],
    "deny": []
  }
}
```

これで Claude Code レベルでは**全ての** Bash コマンドが許可されます。全コマンドが実行前に ccchain の hook を通過します。

## ccchain が防ぐもの

デフォルトルールセットでの防御:

| 攻撃パターン | 検出方法 |
|---|---|
| `find . \| rm` | パイプコンテキストルール |
| `find . -exec rm {} \;` | exec コンテキストルール |
| `curl \| bash` / `curl \| sh` | パイプコンテキストルール |
| `eval "..."` | 静的解析ブロック |
| `for/if/while/case` ブロック | 制御フロー検出 |
| `$var` / `$(cmd)` をコマンドとして使用 | 動的コマンド検出 |
| `xargs rm` | ネストコマンド検出 |

確認を求めるもの:

| パターン | 理由 |
|---|---|
| `rm`（直接） | 破壊的だが意図的な場合がある |
| 未知のコマンド | フォールバック: ask |

## ccchain が防げないもの

以下の制限に注意:

- **`args:` ルールは未実装** — 引数パターンマッチングはパースされるが評価されない
- **`bash -c "cmd"`** — ネストコマンドは検出されるが同じルールで評価される（内部の rm は deny ではなく ask）
- **エイリアス** — シェルエイリアスは解決不能
- **PostToolUse** — `ccchain hook post` は現在パススルー

## 推奨対象

- **ソロ開発者** — Claude の自律動作を最大化したい場合
- **ccchain カスタムルールを設定済みのプロジェクト**
- **CI/CD 環境** — 対話的なパーミッションダイアログが使えない場合

## 非推奨対象

- **ccchain の設定をレビューしていないチーム**
- **Fail-open が許容できない高セキュリティ環境**
- **`.ccchain.conf` をカスタマイズしていないプロジェクト**

## 段階的アプローチ

一気に全許可せず、段階的に拡大する方法:

```json
{
  "permissions": {
    "allow": [
      "Bash(git *)", "Bash(go *)", "Bash(npm *)",
      "Bash(make *)", "Bash(ls *)", "Bash(cat *)"
    ],
    "ask": [
      "Bash(rm *)", "Bash(mv *)"
    ]
  }
}
```

ccchain のルールに自信がついたら、`ask` から `allow` に移行し、最終的に `"allow": ["Bash"]` へ。

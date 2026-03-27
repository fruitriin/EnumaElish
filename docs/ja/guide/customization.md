# カスタマイズ

## 設定ファイル探索順

ccchain は以下の順で設定ファイルを探します。後のファイルのルールが last-rule-wins で上書きできます:

| 優先度 | パス | 用途 |
|---|---|---|
| 1 | `.ccchain.conf` | プロジェクト共有ルール（git にコミット） |
| 2 | `.ccchain.local.conf` | 個人用上書き（.gitignore 対象） |
| 3 | `$CLAUDE_CONFIG_DIR/ccchain.conf` | Claude Code のグローバル設定 |
| 4 | `~/.claude/ccchain.conf` | フォールバックグローバル設定 |

`--config <path>` で検索をスキップし、特定ファイルを直接指定できます。

## プロジェクトルール (`.ccchain.conf`)

チーム全員で共有するルールを記述します:

```
# プロジェクト固有のビルドツール
allow npm
  |
    deny rm  "npm の出力を rm にパイプしないでください"

allow cargo
  next: bulkExec

# プロジェクトの方針: rm の代わりに trash を使う
deny rm  "trash コマンドを使ってください"
allow trash
```

## 個人用オーバーライド (`.ccchain.local.conf`)

`.gitignore` に追加してから、個人の好みでカスタマイズ:

```
# 上級者なので rm を許可
allow rm

# 自分の環境固有のツール
allow brew
allow mise
```

## カスタムテンプレートの作成

プロジェクト固有のテンプレートを定義:

```
template myProjectSafe
  |,>>
    allow jq, yq, csvkit
    deny curl  "パイプで curl に流さないでください"
  exec:
    allow node, python3

allow my-cli
  next: myProjectSafe
```

## Claude Code パーミッションとの併用

ccchain は **PreToolUse hook** として動作するため、Claude Code の組み込みパーミッションチェックの**前**に実行されます。2つのシステムは補完関係です:

- **Claude Code パーミッション** (`settings.json`): コマンドプレフィックスによる粗い制御
- **ccchain**: 構造的コンテキストによる細かい制御

コマンドは**両方のチェック**を通過する必要があります。

### 推奨ワークフロー

1. `settings.json` で `Bash(find *)` を allow に追加（find 自体は許可）
2. `.ccchain.conf` で find のパイプ/exec コンテキストを制御
3. `find . | grep foo` → 両方通過 → 実行
4. `find . | rm` → ccchain が deny → ブロック

## マージの仕組み

複数の設定ファイルが見つかった場合:
- **テンプレート**: 全ファイルから収集（同名はエラー）
- **ルール**: 検索順に追加（last-rule-wins で上書き可能）
- **Settings**: 最後の `settings:` ブロックが優先

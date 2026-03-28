# ccchain セルフホスティング（dogfooding）のノウハウ

## 知見

### hook 登録のパス

`settings.json` で ccchain を hook に登録する際、パスは `"$CLAUDE_PROJECT_DIR"/ccchain` とする。ビルド済みバイナリがプロジェクトルートにある前提。`go run` は毎回ビルドが走り hook のレイテンシが数秒になるため不可。

### .ccchain.conf と .ccchain.local.conf の使い分け

- `.ccchain.conf` — プロジェクト共有。英語メッセージ。git にコミット
- `.ccchain.local.conf` — 個人用上書き。日本語メッセージ等。.gitignore 対象

last-rule-wins なので `.local.conf` のルールが `.conf` を上書きする。

### parseKeyValue の制限

DSL パーサーの `parseKeyValue` はコロン直後の**1トークンしか**返さない。`workspace: ~/a, /tmp/b` のようなカンマ区切り値はトークンが分割されるため、`workspace` 設定のパースでは独自にコロン以降の全トークンを収集する必要があった（Plan 0017 で修正）。

### テスト駆動のルール調整

1. `~/.claude/projects/` からコマンドを収集
2. `ccchain test commands.txt` で一括評価
3. ask が多すぎる箇所に allow ルールを追加
4. 繰り返し

このフローでデフォルトルール 167 コマンドを allow=147, ask=8 に最適化できた。

### Bash コマンドのスコープ判定

`ExtractPathArgs` でパスっぽい引数を抽出するが、`/` を含む引数は全てパスと判定される。`-o /dev/null` のようなフラグ+パスも含まれる。過剰検出だが安全側に倒れるので許容。

### メッセージテンプレートの {id}

`{id}` は `crypto/rand` で生成される12文字の hex。毎回異なるため一時ファイル名の衝突を防ぐ。deny メッセージに `"find -print > /tmp/targets_{id}.txt"` と書くと、Claude が実際にそのファイル名を使って書き直す。

### Go モジュールパスとリポジトリ名の不一致

`go.mod` のモジュールパスは `github.com/fruitriin/ccchain` だがリポジトリ名は `EnumaElish`。`go install` が混乱する。将来的にリポジトリ名の変更 or リダイレクト設定が必要。

### Makefile タスクランナー

Go プロジェクトでは `Makefile` が最もシンプル。`make check`（品質ゲート全部）、`make test-integration`、`make test-fixture`、`make bench` を定義。Task (go-task) や Mage もあるが、既に Makefile がある場合は追加依存を入れる理由がない。

### .ccchain.conf の gitignore 注意

初期状態で `.ccchain.conf` が `.gitignore` に入っていた。プロジェクト共有設定なのでコミット対象にすべき。`.ccchain.local.conf` を gitignore 対象にする（個人上書き用）。

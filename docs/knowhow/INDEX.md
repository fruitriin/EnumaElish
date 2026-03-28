# Knowhow Index

> 自動生成。`/addf-knowhow-index reindex` で再生成できる。

## ccchain 開発・運用

| ファイル | 要約 | キーワード |
|---|---|---|
| [ccchain-dogfooding.md](ccchain-dogfooding.md) | ccchain セルフホスティング時の知見。hook 登録パス、conf/local.conf の使い分け、parseKeyValue の制限、テスト駆動ルール調整、スコープ判定、メッセージテンプレート、Go モジュールパス不一致、Makefile | dogfooding, hook, settings.json, .ccchain.conf, .ccchain.local.conf, parseKeyValue, workspace, scope, テンプレート, {id}, go install, Makefile |
| [fixture-based-testing.md](fixture-based-testing.md) | フィクスチャベーステスト設計。commands.txt × rules-*.conf の組み合わせテスト、ccchain test サブコマンド、テスト駆動ルール調整、ccconv によるログ収集 | フィクスチャ, testdata, commands.txt, rules-*.conf, ccchain test, 組み合わせテスト, TestFixtureCombination, ccconv, セッションログ |
| [security-review-findings.md](security-review-findings.md) | セキュリティレビューで発見された脆弱性パターン。相対パスのスコープ判定、ask止まり、MCP引数未検査、regex複合サブコマンド、git config保護不完全 | セキュリティ, scope, 相対パス, MCP, extractMCPArg, regex, サブコマンド, git config, fsmonitor, credential.helper |
| [doc-drift-pattern.md](doc-drift-pattern.md) | ドキュメントドリフトのパターンと対策。ロードマップ未更新、printUsage忘れ、README一覧の古さ | ドキュメント, ドリフト, ロードマップ, printUsage, README, addf-doc-review-agent |
| [workspace-scope-design.md](workspace-scope-design.md) | workspace スコープの設計思想と制限。ツール種別ごとの適用状況、複数パスホワイトリスト、ask止まりの設計判断、parseKeyValue デバッグ | workspace, scope, ScopeInside, ScopeOutside, ask, deny, Read, Edit, Bash, MCP, parseKeyValue |
| [dsl-rule-design.md](dsl-rule-design.md) | DSL ルール設計パターン。last-rule-wins の活用、.conf/.local.conf 役割分担、args: 正規表現の罠、複合サブコマンド regex | last-rule-wins, args, 正規表現, .ccchain.conf, .ccchain.local.conf, chmod, 777, サブコマンド, normalizeSubcommands |

## プロセス・運用

| ファイル | 要約 | キーワード |
|---|---|---|
| [process-improvement-patterns.md](process-improvement-patterns.md) | プロセス改善パターン。/loop + /addf-dev 自動消化、ノウハウ蓄積ステップ欠落の発見と修正、Plan 即時作成パターン | /loop, /addf-dev, バックログ, 自動消化, ノウハウ, Progress テンプレート, Plan, TODO |

## Claude Code 設定・運用

| ファイル | 要約 | キーワード |
|---|---|---|
| [ADDF/claude-md-at-mention.md](ADDF/claude-md-at-mention.md) | CLAUDE.md の @FileName メンション展開の仕組みと使い分け | @展開, メンション, クオート, ネスト展開, CLAUDE.md, インライン展開, ファイル参照, ブートシーケンス |
| [ADDF/ignore-file-strategy.md](ADDF/ignore-file-strategy.md) | .gitignore / .claudeignore / .git/info/exclude の役割分けと運用戦略 | .gitignore, .claudeignore, .git/info/exclude, respectGitignore, settings.json, settings.local.json, Glob, Grep, ファイル除外 |

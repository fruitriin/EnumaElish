# Knowhow Index

> 自動生成。`/addf-knowhow-index reindex` で再生成できる。

## ccchain 開発・運用

| 鮮度 | ファイル | 要約 | キーワード |
|---|---|---|---|
| ❓ | [ccchain-dogfooding.md](ccchain-dogfooding.md) | ccchain セルフホスティング時の知見。hook 登録パス、conf/local.conf の使い分け、parseKeyValue の制限、テスト駆動ルール調整、Go モジュールパス不一致、Makefile | dogfooding, hook, settings.json, .ccchain.conf, .ccchain.local.conf, parseKeyValue, workspace, ExtractPathArgs, {id}, go install, Makefile, last-rule-wins |
| ❓ | [fixture-based-testing.md](fixture-based-testing.md) | フィクスチャベーステスト設計。commands.txt × rules-*.conf の組み合わせテスト、ccchain test サブコマンド、テスト駆動ルール調整ワークフロー | フィクスチャ, testdata, commands.txt, rules-*.conf, ccchain test, TestFixtureCombination, TestFixtureDangerousNeverAllow, TestFixtureCompareRulesets, stdin |
| ❓ | [security-review-findings.md](security-review-findings.md) | セキュリティレビューで発見された脆弱性パターン。相対パスのスコープ判定、ask止まり、MCP引数未検査、複合サブコマンド regex、git config 保護不完全、未知フラグ透過の fail-open | セキュリティ, scope, 相対パス, CLAUDE_PROJECT_DIR, MCP, extractMCPArg, normalizeSubcommands, git config, fsmonitor, credential.helper, unknown flag, flagPassthroughCommands, parseCLIArgs |
| ❓ | [workspace-scope-design.md](workspace-scope-design.md) | workspace スコープの設計思想と制限。ツール種別ごとの適用状況、複数パスホワイトリスト、outside は ask 止まりの設計判断、parseKeyValue デバッグ | workspace, scope, ScopeInside, ScopeOutside, ask, deny, Read, Edit, Bash, MCP, parseKeyValue, scope_violation |
| ❓ | [dsl-rule-design.md](dsl-rule-design.md) | DSL ルール設計パターン。last-rule-wins の活用、.conf/.local.conf 役割分担、args: 正規表現の罠、next: primitive、複合サブコマンド regex | last-rule-wins, args, 正規表現, .ccchain.conf, .ccchain.local.conf, chmod, 777, next: primitive, normalizeSubcommands, \s+ |
| ❓ | [doc-drift-pattern.md](doc-drift-pattern.md) | ドキュメントドリフトのパターンと対策。ロードマップ未更新、printUsage 追加忘れ、README 一覧の古さ、DSL リファレンス未記載 | ドキュメント, ドリフト, ロードマップ, printUsage, README, addf-doc-review-agent, 品質ゲート |
| 🟢 2026-07-06 | [vitepress-angle-bracket-placeholders.md](vitepress-angle-bracket-placeholders.md) | VitePress は生の山括弧プレースホルダ（`<番号>` 等）を未クローズ HTML タグ扱いしビルド失敗。エラー行番号のずれ、機械走査 → 一括修正 → 1回ビルドの手順 | VitePress, Vue, Element is missing end tag, 山括弧, プレースホルダ, コードスパン, &lt;, docs:build, srcExclude, CI |
| ❓ | [savanna-smell-detector.md](savanna-smell-detector.md) | savanna-smell-detector Go 導入知見。Go イディオム相性のフィードバックループ、テストスメル修正パターン、smell-allow コメント | savanna, テストスメル, .savanna.toml, smell-allow, Giant Test, Conditional Test Logic, Silent Skip, t.Fatal, assertEqual, cargo install |

## プロセス・運用

| 鮮度 | ファイル | 要約 | キーワード |
|---|---|---|---|
| ❓ | [process-improvement-patterns.md](process-improvement-patterns.md) | プロセス改善パターン。/loop + /addf-dev 自動消化、ノウハウ蓄積ステップ欠落の発見と修正、Plan 即時作成パターン | /loop, /addf-dev, バックログ, 自動消化, CronDelete, Progress テンプレート, /addf-knowhow, Plan 即時作成, TODO |

## Claude Code 設定・Hooks（ADDF 由来）

| 鮮度 | ファイル | 要約 | キーワード |
|---|---|---|---|
| 🟢 2026-06-11 | [ADDF/claude-code-hooks.md](ADDF/claude-code-hooks.md) | Claude Code Hooks の全イベント・exit コードフロー制御・JSON 出力・transcript からのコンテキスト使用量計測 | Hooks, PreToolUse, PostToolUse, UserPromptSubmit, Stop, exit 2, permissionDecision, matcher, transcript_path, isSidechain, CLAUDE_PROJECT_DIR |
| 🟡 2026-03-18 | [ADDF/claude-md-at-mention.md](ADDF/claude-md-at-mention.md) | CLAUDE.md の @FileName メンション展開の仕組みと使い分け（展開/クオート/二重展開回避） | @展開, メンション, クオート, ネスト展開, CLAUDE.md, インライン展開, ブートシーケンス, CLAUDE.repo.md |
| 🟢 2026-07-02 | [ADDF/ignore-file-strategy.md](ADDF/ignore-file-strategy.md) | .gitignore / .claudeignore / .git/info/exclude の役割分けと respectGitignore の挙動 | .gitignore, .claudeignore, .git/info/exclude, respectGitignore, settings.json, Glob, Grep, CLAUDE.local.md, exp.md |
| 🟡 2026-03-19 | [ADDF/permission-settings-pattern.md](ADDF/permission-settings-pattern.md) | 権限を3パターン（アップストリーム/ダウンストリーム/汎用）で分類する配置ルールと、permissions ルール構文の技術仕様 | permissions, settings.json, settings.local.json, allow, ask, deny, ワイルドカード, Bash(git status *), mcp__server__tool, gh api, スキルは権限をネストしない |
| 🟡 2026-03-19 | [ADDF/pretooluse-block-with-rationale.md](ADDF/pretooluse-block-with-rationale.md) | PreToolUse フックで根拠提示型ブロックを行うパターン。/tmp/ 回避・CLAUDE_CODE_TMPDIR・cd 突き抜け防止・sed 退行防止等の横展開 | PreToolUse, decision: block, reason, /tmp/, CLAUDE_CODE_TMPDIR, 根拠提示, 自己ブロック回避, check-tmp.py, addfReplace |

## ADDF フレームワーク設計・運用（ADDF 由来）

| 鮮度 | ファイル | 要約 | キーワード |
|---|---|---|---|
| 🟢 2026-06-10 | [ADDF/upstream-downstream-separation.md](ADDF/upstream-downstream-separation.md) | アップストリーム（ADDF）とダウンストリーム（プロジェクト）のファイル分離パターン3種と新規ファイル配置の判断基準 | .addf.md, ADDF/, addf- プレフィックス, plans-add, INDEX.addf.md, ProgressTemplate, ダウンストリーム, アップストリーム, 計画番号 |
| 🟡 2026-03-21 | [ADDF/existing-project-install-pattern.md](ADDF/existing-project-install-pattern.md) | 既存プロジェクトへの ADDF 導入パターン。鶏と卵問題、CLAUDE.md 退避、干渉チェック3カテゴリ、信頼モデル | addf-init, WebFetch, raw.githubusercontent.com, CLAUDE.md 退避, CLAUDE.repo.md, 干渉チェック, 導入前レビュー, マーカーブロック, 外部起動 |
| 🟡 2026-03-21 | [ADDF/release-skill-separation.md](ADDF/release-skill-separation.md) | リリーススキルの責務分割パターン。スキル=ルーター、設定ファイル=手順定義、exp=プロジェクト戦略 | addf-release, ルーター, ADDF-Release.addf.md, addf-release.exp.md, upstream, downstream, チェンジログ, publish, dry-run |
| 🟡 2026-03-21 | [ADDF/skill-design-patterns.md](ADDF/skill-design-patterns.md) | Anthropic 社内知見に基づくスキル設計パターン。9カテゴリ分類・Gotchas育成・段階的開示・description はトリガー条件 | スキル, 9カテゴリ, Gotchas, Progressive Disclosure, description, config.json, references/, オンデマンドフック, commands/ vs skills/, exp.md |
| 🟢 2026-06-10 | [ADDF/rule-placement-execution-guarantee.md](ADDF/rule-placement-execution-guarantee.md) | ルール配置と実行保証 — 参照では実行されない。実行主体が必ず読むファイルに手順を書く。CLAUDE.local.md をセッション状態の保存先にする応用 | 実行保証, 参照では実行されない, インライン展開, サブエージェント定義, ProgressTemplate, CLAUDE.local.md, addf-mode |
| 🟢 2026-07-03 | [ADDF/sync-lint-design.md](ADDF/sync-lint-design.md) | 同期 lint の設計 — 検出はツール、解釈と修復はエージェント。欠如=SKIP・exit 3値・tomllib ガード・列挙の陳腐化排除・ドリフト注入テスト | lint-template-sync.py, 同期ペア, 正規化テキスト比較, 欠如=SKIP, exit 3値, tomllib, uv run, PEP 723, 参照⇔カバレッジ, ドリフト注入, mktemp サンドボックス |
| 🟢 2026-06-11 | [ADDF/plan-status-drift-check.md](ADDF/plan-status-drift-check.md) | Plan 状態の信用ベース運用 — 疑う仕事は lint（ペア6）に任せ、TODO の状態表記は基本信用する。ヘッダ遡及付与による一括清算 | 実装状況ヘッダ, lint ペア6, TODO 状態, git log 突合, 信用ベース, 遡及付与, ドリフト |
| 🟢 2026-07-02 | [ADDF/checklist-backing-lint.md](ADDF/checklist-backing-lint.md) | チェックリスト裏付け lint — 手順書の「確認」項目に A型（実行チェック）/B型（human-judgment マーカー）の裏付けを要求する | lint-checklist.py, A型/B型, human-judgment, 実行チェック, WARNING, ホワイトリスト, theater 化, ステップ抽出 |
| 🟢 2026-07-02 | [ADDF/optional-skill-optin.md](ADDF/optional-skill-optin.md) | オプトイン式スキルの退避＋有効化コピー設計。原本=真実源・コピー使い捨て・改変コピーは触らない3原則、孤児検出 | .claude/addf/optional/, sync-optional-skills.py, オプトイン, Behavior.toml, enable 型検証, 孤児検出, シンボリックリンク不使用, 番号付きステップ |
| 🟢 2026-07-03 | [ADDF/plan-refinement-pattern.md](ADDF/plan-refinement-pattern.md) | 粗々プランの詰め方 — 未決事項を「決定＋根拠」に1:1変換する。決定間の整合レビュー、完了条件の A型/B型分離 | 粗々プラン, 未決事項, 決定＋根拠, knowhow フィルタ, 実地確認, 決定間の矛盾, 完了条件, human-judgment |
| 🟢 2026-07-05 | [ADDF/speculative-integration-design.md](ADDF/speculative-integration-design.md) | 投機 feature の squash 統合設計 — 破壊的 git 操作を専用 worktree に閉じ込める。commit 非0 の理由判定、使い捨てブランチ再生成 | speculate-integrate.py, squash, 統合ブランチ, reset --hard, commit_failed, diff --cached, pre-commit フック注入, key=value 出力, addf-speculate |
| 🟢 2026-07-05 | [ADDF/speculative-cycle-lessons.md](ADDF/speculative-cycle-lessons.md) | 投機サイクル運用の教訓 — 同一 AST 層の意図衝突、ペルソナ並列レビューが掘り当てる既存バグの分離、兄弟課題の同一ブランチ化 | 投機サイクル, 意図の衝突, AST 層, 兄弟パターン, 既存バグ分離, Dashboard, 3ペルソナ, skeptic, attacker, newcomer |
| 🟢 2026-07-03 | [ADDF/worktree-dotdir-copy.md](ADDF/worktree-dotdir-copy.md) | worktree への .claude 複製 — cp -r の「既存ディレクトリへの入れ子」罠。`cp -r .claude/. dst/.claude/` が正解 | cp -r, .claude/., worktree, 入れ子, gitignore 対象ファイル, .exp.md, 成功して見える失敗 |
| 🟢 2026-07-10 | [ADDF/cron-loop-worktree-race.md](ADDF/cron-loop-worktree-race.md) | cron 経由 `/loop` による `/addf-dev` 再入は CLAUDE.md の並列実装方針（git worktree 隔離）の対象外で、同一 working tree に複数サイクルが並行実装しうる。実例と衝突時の手動差分分離手順、再発防止の選択肢（interval 延長・single-flight ロック・worktree 化）を記載 | cron, CronCreate, /loop, /addf-dev, 並行実行, working tree, 競合, レース, git status, 外部変更検知, single-flight |
| 🟢 2026-07-10 | [ADDF/worktree-isolation-cd-persistence.md](ADDF/worktree-isolation-cd-persistence.md) | worktree 隔離下のエージェント自身が共有チェックアウトへ絶対パス `cd` すると、以降の Bash 呼び出しが隔離を離脱したまま実行され続けるヒヤリハット。Write/Edit にはパス単位のガードがあるが Bash の cd 永続にはない非対称性。1コマンド内で完結させる／`git -C` を使う等の回避策を記載 | worktree, 隔離, cd, 永続, Bash, Write, Edit, 非対称性, isolation, git -C, ヒヤリハット |
| 🟢 2026-07-10 | [ADDF/docs-site-single-source-sync.md](ADDF/docs-site-single-source-sync.md) | VitePress 等のドキュメントサイトで単一ソースを保つ設計。docs/guide/*.md をビルド時生成物（gitignore対象）にし、相対リンクを解決してガイド間相対パス/GitHub blob URL/raw URLへ書き換える。アンカー・画像の考慮、ignoreDeadLinks:falseでドリフト検出、node不在ダウンストリームでのテストSKIP設計 | VitePress, docs, 単一ソース, sync-docs, リンク書き換え, ignoreDeadLinks, gitignore, ビルド生成物, SKIP設計 |
| 🟢 2026-07-10 | [ADDF/measurement-sampling-bias.md](ADDF/measurement-sampling-bias.md) | 実測ベース判断 Plan の探索条件バイアス。メタデータ（ディレクトリ有無等）でなくファイル実在で探索する、「大きい」と「スコープ内で肥大化」を同一視しない、code-review に除外判断の妥当性検証を依頼すると拾える | 実測, 判断Plan, サンプリングバイアス, 探索条件, find, 母集団, YAGNI, code-review, 反証事例 |
| 🟢 2026-07-06 | [ADDF/map-driven-migration-tool.md](ADDF/map-driven-migration-tool.md) | 自分自身を移動させる移行ツールの設計パターン（Plan 0037）。check/apply/rewrite のモード分離と dirty 拒否によるコミット分離の構造的強制、完了メッセージは移動後の新パスで案内、マップ探索は新位置優先、symlink 除外（blob 追跡でリポジトリ外書き込みになる）、NUL＋UTF-8 のバイナリ判定と走査集合の一致、境界判定と逐次置換の不変条件、空中間ディレクトリの自己ロック回避、backup ref の既存確認、rewrite の射程外5類型（相対階層参照・join/連結の分割断片・書き込み先の親 mkdir・Markdown 相対リンク・裸参照/ツリー図の分割文字列。バイナリは再ビルド要） | migrate-paths, paths.toml, lint-residual-paths, マップ駆動, git mv, 参照書き換え, symlink, 境界チェック, backup ref, 自己移動, 空ディレクトリ, one-shot, 移行, Plan 0037 |
| 🟢 2026-07-06 | [ADDF/persona-review-oneshot.md](ADDF/persona-review-oneshot.md) | one-shot 実施 Plan にはペルソナ並列レビュー＋実地攻撃が効く（Plan 0037 実証）。51テスト全パスの実装から Critical 4件を検出。attacker に「実際に動かして攻撃してよい」を明示・skeptic はテストの網羅範囲そのものを疑う・コンセンサス昇格の実例・攻撃再現手順はテストとしてスイートに固定 | ペルソナ, 視点ずらし, attacker, skeptic, newcomer, コンセンサス補正, 実地攻撃, one-shot, レビュー, 品質ゲート, Plan 0020, Plan 0037 |
| 🟢 2026-07-06 | [ADDF/knowhow-obsolescence-patterns.md](ADDF/knowhow-obsolescence-patterns.md) | knowhow が陳腐化しやすい3つの記述パターン（未実装リスト・現物埋め込み例示・閉じた分岐図）と逃がし方。Plan 0032 の棚卸しから抽出 | 陳腐化, obsolescence, 未実装, 例示リスト, 分岐図, 単一ソース化, revise, freshness, 訂正履歴 |
| 🟢 2026-07-06 | [ADDF/context-and-transcript.md](ADDF/context-and-transcript.md) | トランスクリプト JSONL の構造（uuid/parentUuid 連結リスト・tool ペア）とコンテキストの非対称双方向性 — 実行中は読み直されず、resume は外部編集・複製を無検証で受け入れる（実験済み）。能動 compaction の不在、ScheduleWakeup/CronCreate の session-only 性、死蔵アイデア（手術+resume・世代交代・ループの閉じ方3案）を着手トリガー付きで保存 | transcript, JSONL, resume, compaction, parentUuid, tool_result, session-id, スナップショット, ScheduleWakeup, CronCreate, session-only, 世代交代, トランスクリプト手術, 死蔵, headless, claude -p |
| 🟢 2026-07-07 | [ADDF/transcript-archive-restore.md](ADDF/transcript-archive-restore.md) | Plan 0042 の PreCompact アーカイブから resume で復元する具体手順（新 UUID にリネームコピー → claude --resume）・落とし穴（元セッション並走禁止・汚染ごと復元・バージョン差異・tool ペアリング）。復元は例外運用として扱う | resume, 復元, スナップショット, PreCompact, transcript-archive, session-id, UUID, 汚染復元, projects, tool_use, tool_result, Claude Code, Plan 0042 |
| 🟢 2026-07-07 | [ADDF/bash-toml-parse-pitfalls.md](ADDF/bash-toml-parse-pitfalls.md) | bash + awk による簡易 TOML パースの8つの罠と対策（`=` 切り捨て・コメント除去のクオート無視・ヘッダ行末コメント・jq `//` の空文字列素通し・ファイル名サニタイズ不足・秒精度衝突・フォールバック経路のテスト漏れ・macOS bash 3.2 の set -u × basename）。Python 依存を避けたい場面の補完 | TOML, bash, awk, jq, パース, sanitize, パストラバーサル, 命名衝突, フォールバック, macOS, bash 3.2, set -u, Plan 0042 |

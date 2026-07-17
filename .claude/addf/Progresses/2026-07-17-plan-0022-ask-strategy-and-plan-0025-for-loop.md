### 現在のタスク: Plan 0022 — ccchain 再起動（auto モード時代の deny-first 安全網）

#### サブタスクチェックリスト

- [ ] Phase 0: 仕様の再裏取り（WebFetch で hooks/permission-modes/permissions を確認、Plan に検証結果追記、乖離あれば設計修正）
- [ ] Phase 1: hook I/O 現代化
  - [ ] permission_mode 読取（toolInput 拡張 + classifyMode を internal/eval に追加）
  - [ ] permissionDecision 4値 JSON 出力（outputResult 書き換え、deny を exit 0 + JSON へ）
  - [ ] ActionHint の出力整理（allow + reason）
  - [ ] 単体テスト更新（hook_test.go）
- [ ] Phase 2: ask_strategy
  - [ ] settings: ask_strategy / ask_degrade_default パーサー対応
  - [ ] ルール子ブロック unattended: パーサー対応
  - [ ] 降格解決ロジック（unattended: → ask_degrade_default → 組込み既定 deny、deny-all 優先）
  - [ ] 降格 deny メッセージテンプレート（{permission_mode} {approve_command} 変数、切詰め見直し）
  - [ ] 単体・統合テスト
- [ ] Phase 3: 承認トークン（ccchain approve）
  - [ ] AST 正規化ハッシュ（動的要素は承認対象外）
  - [ ] pending/approved JSONL ストア（0600、ファイルロック、TTL、スコープ、ワンショット消費）
  - [ ] approve CLI（--last / --list / <hash-prefix> / --revoke-all）+ printUsage 更新
  - [ ] hook 統合（降格 deny 時の pending 記録、再実行時の承認照合）
  - [ ] 監査ログ統合
  - [ ] 単体テスト（並行アクセス含む）
- [ ] Phase 4: sentinel プリセット（ccchain init --sentinel）
  - [ ] sentinelConfig 定数 + --sentinel フラグ
  - [ ] fixture テスト（rules-sentinel.conf × commands）
  - [ ] 統合テスト期待値追加
- [ ] Phase 5: ドキュメント（README/README.en ポジショニング刷新、docs/ リファレンス追加、CHANGELOG）
- [ ] Stage 1: go test ./... + go vet + go build + ADDF テスト
- [ ] Stage 2: ペルソナ並列コードレビュー + セキュリティレビュー + doc レビュー（ドキュメント変更あり）
- [ ] レビュー指摘対応 → 再テスト
- [ ] ノウハウ記録・Plan 反映・Feedback 記録・アーカイブ・コミット

#### 日記

##### 2026-07-17 — Plan 0022 着手、Phase 0 調査をエージェントに委譲
**やったこと**: /goal で「/addf-dev Plan0022, 0025」を受領。前タスク（2026-07-06 オーナー指示対応）を Progresses/ にアーカイブし、stale な Dashboard.md（PR #11 マージ済みの内容）を提示のうえ削除。knowhow エージェントと Phase 0 仕様裏取りエージェント（claude-code-guide、WebFetch で公式 docs を確認）を並列起動。hook.go / ast.go / evaluate.go の現状を読了 — Plan 記載のギャップ（permission_mode 未読取、旧出力形式、defer なし）は現物と一致。
**今の見立て**: Phase 1 が全ての前提なのでメインで実装し、Phase 2+3 と Phase 4 は worktree 並列委譲が Plan の依存図に合う。Phase 0 の結果次第で classifyMode の区分数と hook_output オプションの要否が変わる。
**次の自分へ**: Phase 0 エージェントの結果を Plan 0022 の「### 検証結果」セクションに追記してから Phase 1 に入ること。0022 完了後に Plan 0025 が控えている（goal に含まれる）。
**気になっていること**: リリースについて Plan は v1.0.0 提案だが、既に v0.1.0 タグ運用が始まっており（2026-07-06 リリース済み）、日記では「0022 が入ったら 0.2.0」がオーナー合意。Plan 記載より日記の合意が新しいので 0.2.0 側で進める。

##### 2026-07-17 — Phase 1・2 完了（hook I/O 現代化 + ask_strategy）
**やったこと**: Phase 1（hookSpecificOutput 4値 JSON、permission_mode/session_id/cwd 読取、ClassifyMode 2区分、outputResult の純関数化）と Phase 2（ask_strategy 3値 + ask_degrade_default + unattended: ルール子ブロック + ResolveAsk 降格解決 + 切詰め 600 字化）を実装、それぞれコミット済み。E2E 実機確認: auto で ask→deny+承認手順 hint、default で ask 維持、unattended: allow で warn 降格、permission_mode 欠落時は保守的に deny。
**今の見立て**: allow は意図的に中立（無出力）にした — ccchain の allow は「他の権限レイヤーを迂回しない」。降格メッセージは既存コードとの一貫性から英語にした（Plan の例文は日本語だが組込みメッセージは全て英語のため）。
**次の自分へ**: Phase 3（approve、Phase 2 の降格 hint が言及する `ccchain approve --last` の実体）と Phase 4（sentinel プリセット）は独立性が高いので worktree 並列委譲する。Phase 3 の脅威モデル（hint にトークン不掲載・approve 自体を deny）を委譲プロンプトに必ず含めること。
**気になっていること**: fallback ask も auto では deny 降格される（安全側だが、既存 conf なしユーザーの auto 運用では体感が変わる）。CHANGELOG と README に明記が必要（Phase 5）。

##### 2026-07-17 — Phase 3 完了報告受領、統合開始（コンテキスト 220k 超）
**やったこと**: Phase 3（approve）の worktree エージェントが完遂。internal/approve 新設（独自 canonical serializer + SHA-256、JSONL ストア、O_EXCL ロック、O_NOFOLLOW、0600/0700、ワンショット消費、並行テスト済み）、hook 統合は「approved 照合 → ResolveAsk → 降格 deny なら pending 記録」の順序。E2E 5 ステップ全通過。Phase 4（sentinel）と Plan 0025（for ループ）はまだ worktree で実行中。
**今の見立て**: Phase 3 の設計判断は妥当（import cycle 回避で hook.go がオーケストレータ、承認消費は warn 経路で可視化）。ADDF テスト 2 スイート失敗は既知の downstream 文脈問題で無関係。
**次の自分へ**: (1) Phase 3 worktree でコミット → main に squash マージ → 全テスト再実行、(2) Phase 4 / 0025 の完了通知を待って同様に統合（Phase 4 は sentinel に ccchain approve の deny ルールを含むはず — Phase 3 との整合を確認）、(3) Phase 5 ドキュメント（CHANGELOG は Phase 1/2 分まで記載済み、Phase 3/4 分の追記必要）、(4) 品質ゲート Stage 2 はペルソナ並列（unattended 自走時ルール）+ セキュリティレビュー + doc レビュー。knowhow 候補4件はエージェント報告に記録済み（syntax.Printer の quote 保持、JSONL lock パターン、hook 統合順序、outer ccchain 下のテスト実行注意）— 完了処理でまとめて記録する。
**気になっていること**: コンテキスト 222k。compaction が来たらこの日記と Progress のチェックリスト、TODO の Agent 3体の状態（Phase3 完了/Phase4 実行中/0025 実行中）から再構築する。worktree パスは .claude/worktrees/agent-*。

##### 2026-07-17 — Plan 0025 統合完了、残るは Phase 4 のみ
**やったこと**: Phase 3（approve）と Plan 0025（for ループ部分解析 + unanalyzable_action）を main に squash マージ。0025 は worktree base が古く（v0.1.0 直後）、ast.go / parser.go で Phase 2 との衝突を手動解消（Settings に AskStrategy/AskDegradeDefault と UnanalyzableAction を併存、パーサー分岐は3ケース並べた）。全テスト・vet 通過。ADDF 実 conf での実地確認も完了（literal for ループが head ルールに従い allow — ドッグフーディングのフィードバックループが閉じた）。make smell は既存4件（scope_test.go の Ignored Test）のみで新規スメルなし。
**今の見立て**: Plan 0025 の実装エージェントが glob 検出（for f in *.log の誤展開防止）を独自に追加していたのは良い判断。Phase 4（sentinel）はまだ実行中。
**次の自分へ**: (1) Phase 4 完了通知 → 統合（sentinel の approve deny ルールと Phase 3 の整合確認、init_cmd.go/main.go の衝突可能性）、(2) Phase 5 ドキュメント: README 互換表 + sentinel クイックスタート + docs/reference（ask_strategy/approve/sentinel/unanalyzable_action、en+ja）+ CHANGELOG に Phase 3/4 と 0025 追記 + roadmap 更新、(3) 品質ゲート Stage 2: ペルソナ並列 + security + doc レビュー（0022 と 0025 まとめて）、(4) 完了処理: knowhow（エージェント報告に候補7件）、Plan status 反映、Feedback、アーカイブ。
**気になっていること**: 既存スメル4件（symlink テストの条件スキップ）は主題外 — 品質ゲート後の観察として Feedback か新 Plan 判断。バージョンは 0.2.0 で進める（v1.0.0 ではなく）。

##### 2026-07-17 — skeptic レビュー到着、Critical 2件を検出
**やったこと**: Stage 1 完了（Go テスト・vet・ビルド全 pass、ADDF テストは既知の downstream 失敗のみ）。Phase 5 docs + skeptic/attacker/security の4体を並列起動。skeptic が Critical 2件を検出。
**skeptic Critical 集約**:
- C1: hook.go の lookupApproval が `ClassifyMode==NonInteractive` でゲートされているため、`ask_strategy: deny-all` で interactive モードでも deny になるケースで承認消費が発火しない → 承認案内が壊れた約束になる。修正: lookupApproval のゲートを ResolveAsk の実際の deny 降格判定と一致させる
- C2: Plan 0025 の for ループ展開由来の ask/deny を hook.go が pending 記録する際、canonical serializer が ForClause 未対応で ErrUnsupported を無言で握りつぶす。案内は出るが pending は無く、`ccchain approve --last` が別の pending を誤消費する。修正: canonicalCommand に ForClause 対応追加、または recordPendingApproval で ErrUnsupported にもユーザー向けメッセージを追加
- Warning: unattended: が args:/scope: に未対応（意図判断のコメントすらない）、for 展開時にクオート情報が失われる（unquoted 空白リテラルで単語分割の乖離）
**今の見立て**: C1/C2 は主題内（Plan 0022 と 0025 の縫い目そのもの）で必修正。他3体（attacker/security/docs）の結果を待って集約し、まとめて修正エージェントに委譲する。C2 の修正は canonicalCommand に ForClause を明示 unsupported として理由付きメッセージを返させるのが最小変更で安全側。
**次の自分へ**: 全レビュー到着後 → 集約 → 主題内修正 → 再テスト → Plan 反映 → knowhow → Feedback → アーカイブ → コミット。C1 の修正は degrade.go の分岐でも hook.go の分岐でもよいが、hook.go 側で「preAskAction==ask && (ResolveAsk 後が deny になる) なら lookupApproval を試す」に統一するのが自然（deny-all 戦略の interactive も自動的にカバーされる）。
**気になっていること**: コンテキスト 280k、実効目安 200k。もし compaction が来ても、この日記と skeptic の主要 Critical/Warning 記録があれば復帰可能。

##### 2026-07-17 — 全レビュー到着（skeptic×2 + security）、Critical 集約
**やったこと**: 3体のレビューが完了（attacker と phase5-docs はまだ実行中）。skeptic は2セッション分投入されて集約結果に厚みが出た。
**Critical 集約（主題内、フェーズ内対応必須）**:
- **skeptic#1 C1**: hook.go の lookupApproval が ClassifyMode==NonInteractive ゲートで、`ask_strategy: deny-all` の interactive モードで承認消費が発火しない → 承認案内が壊れた約束になる
- **skeptic#1 C2**: Plan 0025 の for ループ由来 ask/deny を hook が pending 記録する時、canonical serializer が ForClause 未対応で ErrUnsupported を無言で握りつぶす → 案内は出るが pending は無く、approve --last が別 pending を誤消費
- **skeptic#2 C1**: canonical hash が引数境界を消失（`echo "a b"` と `echo a b` が同一 hash）→ `find "-name x"` の承認で `find -name x` を通せる。実機再現あり
- **skeptic#2 C2**: `unattended:` 指定が args:/scope: サブルール発火時に消失（applyArgsRules で lastMatch を作り直すとき Unattended 未コピー）→ 実機再現あり、ask_degrade_default が滲出
- **skeptic#2 C3**: for ループ展開が word splitting 未適用（`for f in "target dir" x; do cp -t $f file` で argv 個数が実行時と乖離）
- **security C1**: 承認ストア（pending/approved.jsonl）が平文で HMAC 等の認証なし。sentinel は `ccchain approve` サブコマンドを deny するのみで、ストアファイルへの `echo >>` 系書き込みは無防備（workspace_paths 未設定時は scope 判定完全スキップ） → agent が承認を偽造可能
- **security C2**: deny の exit 2 経路を完全撤去 → 新 JSON を解釈しない旧 CC バージョンや将来のリグレッションで全 deny がサイレント fail-open
- **security H1**: approve --list / 確定表示にサニタイズ未適用 → ANSI エスケープ注入で人間の承認判断を欺ける
- **security H2**: CCCHAIN_APPROVE_STORE 環境変数がテスト用の分岐なしに本番で有効 → hook 起動コマンドや .bashrc を書ける状況でストア差し替え
- **security H3**: ロックファイルにステイル検知なし → `touch ~/.claude/ccchain/store.lock` で承認導線を DoS
- **security H4**: init --sentinel の Next steps に settings.json permissions.deny の推奨を書いていない
**Warning 集約**:
- skeptic#2 W: plan モードの Phase 0 検証未クローズ、args:/scope: の Unattended 未対応が Plan 内で判断根拠なし
- security M1: git config 保護が knowhow 既知の findings（core.fsmonitor / filter.*/diff.external / credential.helper）を反映しきれていない
- security M2: pending.jsonl の巨大行で bufio.Scanner が破壊される
**今の見立て**: Critical 全10件 + High 4件はほぼ全部主題内。C1 security（HMAC 認証）は Plan 記載の「依存追加最小」原則との相性判断があり、暗号鍵管理を hook 経路に持ち込むと運用が複雑化 — 現実的な代替は「ストアディレクトリへの書き込みを sentinel と DSL で厚く守る + init --sentinel 時に settings.json permissions.deny を追記案内」。C2 security（exit 2 追記）はコストゼロで多重化できる小修正。それ以外の修正は概ね小規模〜中規模。attacker と docs の結果を早めに待って、集約後に修正エージェント1体に委譲する。
**次の自分へ**: attacker と docs 到着後、(1) 主題内修正を1体の実装エージェントに委譲、(2) HMAC/認証は「Plan 0022 完了後の後続 Plan（Phase 3 強化）」として起票、(3) 修正後に全テスト + E2E 実機、(4) Phase 5 docs マージ、(5) 完了処理。scratch ファイル（`internal/{approve,eval,shell}/zz_skeptic_*_test.go` 等）は skeptic#2 の残骸で untracked。テスト影響なし、承認消費節約のため保留、修正エージェントに削除依頼。
**気になっていること**: コンテキスト 280k で目安 200k 大幅超過。修正エージェント委譲時のプロンプトは長くなるがそれで進むしかない。compaction が来ても、この日記から主要 Critical/High が復元できるように詳述した。

##### 2026-07-17 — attacker 到着（Critical/High 独立新規指摘）、修正エージェントに追加指示検討
**やったこと**: attacker レビューが完了。skeptic#2 C1 と重複する canonical hash 衝突（`rm -- "-rf /tmp"` vs `rm -- -rf /tmp` の実 E2E 攻撃実証つき）に加え、**新規の Critical/High** を独立検出。
**attacker 新規指摘**:
- **C2**: Plan 0025 の bodyRedefinesVar が `*syntax.Assign` と入れ子 for しか検出せず、`read f <<< "/"; rm -rf $f` のようなビルトイン再代入が完全にすり抜ける → 静的評価とシェル実行の argv 乖離、sentinel の強い警告が出ない。実機再現あり
- **H1**: `.ccchain.conf` / `.ccchain.local.conf` 自体を守る Edit/Write ルールが default にも sentinel にも一切ない → 1ファイル編集で `ask_strategy: passthrough` を注入して降格を無効化できる（`ccchain approve` deny は self-fence があるが、DSL 側のフェンスがない）
- **H2**: `\rm -rf /` がコマンド名マッチをすり抜ける → sentinel の rm 保護ルール全てを迂回。既存の topology.go の Lit 保持（Plan 0009 由来）だが Phase 4 の保護主張に直結
- **H3**: acceptEdits × Read/Edit/Write の hook ask 挙動が未検証 → acceptEdits で Edit/Write の ask が届く保証は Phase 0 でも Bash しか確認していない
- **M1**: TTL が壁時計依存でモノトニック時計未使用（time.Now、date 迂回時）
**判断**: attacker の C2/H1/H2/H3 は主題内だが、既発注の修正エージェント（fix-critical）の指示に含めていなかった。C2 は Plan 0025 の主題内なのでフォローアップ指示を追加すべき。H1（conf 保護）と H2（backslash escape）は Plan 0026（承認ストア強化）に含めるより、fix-critical に追加した方が同じフェーズで解決できる。H3 は Phase 0 の再検証が要る（別 Plan 候補）。M1 も別 Plan。
**次の自分へ**: fix-critical エージェントに SendMessage で追加指示（C2/H1/H2 追加）を送るか、完了報告を待って追加サイクルで対応するか判断する。SendMessage が最小変更で確実。指示送付後、fix-critical 完了 → 全 E2E 確認 → Phase 5 docs 統合 → 完了処理。

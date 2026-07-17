# 進捗表

## 運用ルール

### タスク開始時
1. `.claude/addf/Feedback.md` を読み、前回の改善アクションで未対応のものがあれば考慮する
2. 以下の手順で Markdown チェックリストを作成する
   1. 1ショットで作業できる範囲にサブタスクを分割する
   2. 並行作業できる粒度でさらに分割する
   3. 各サブタスクにテスト作成・統合テスト・Lint・ビルドが必要か検討し、必要なら追加する
   4. 必要に応じて 2.1〜2.3 を再帰的に適用する

### 作業中
3. サブタスク着手時に `- [x]` でチェックしていく。並列可能なタスクはコンテナオーケストレーションを利用する
   - Plan の曖昧さで確信が持てないときは CLAUDE.md「迷ったときの作法（7割共有原則）」に従う（閾値割れなら `.claude/addf/Questions.md` に質問を置いてタスクを切り替える）
   - 長大なタスクでは、サブタスク完了時点でブランチ `checkpoint/<phase>-<N>` を切ってよい。別方針を試すときは checkpoint から `alt/` を分岐する
3.5. **日記を書く（代替わり引き継ぎ）**（「3.5」は後続の番号参照を壊さないための意図的な枝番）: resume・compaction・`/loop` の次イテレーションで起きる「小さな代替わり」のたびに、次の代の自分（同僚でもあり、寝て起きたあとの自分でもある）が状況に入れるよう、タスクの「#### 日記」セクションにエントリーを書く
   - **書くタイミング**: サブタスク完了時 / 重要な判断をした直後 / 計画を変更したとき / コンテキストが長くなり compaction を予感したとき
   - **書式（4項目）**（時刻 HH:MM は省略可）:
     ```
     ##### YYYY-MM-DD HH:MM — <出来事の一行>
     **やったこと**: <完了した作業と判断の要約>
     **今の見立て**: <現状認識。確信度があれば記す>
     **次の自分へ**: <次に着手すべきこと・先に確認すべきこと>
     **気になっていること**: <未解決の不確実性・前提・違和感。なければ「なし」>
     ```
   - 「日記」という語彙の意図（「遺書」を使わない理由）は `.claude/addf/guides/development-process.md` 参照
   - ブランチ checkpoint が「何がコミットされたか（事実）」を残すのに対し、日記は「なぜそうしたか・次に何を考えていたか（文脈）」を残す。両方で前任者の靴に履き替えられる
   - 日記の自動生成フックは導入しない。書くこと自体が思考の整理であり、次の自分への手紙として人格を持って書く
   - **コンテキスト満杯時の指針**（Plan 0041 の「満杯時の出口」教義）: コンテキスト残量が少ないことを理由にループを止めない・タスク着手を控えない。auto-compact は harness が上限接近時に自動発動し、`post-compact-recovery.sh` と日記が受け止める。エージェントの仕事は止まらないこと。残量少時は**復帰容易性の高いタスク**（進捗がファイル差分に現れる・サブタスクの刻みが小さい）を優先し、**未コミットの大きな途中状態を長時間抱える one-shot 級タスクは残量少時に着手しない**。進捗の外部化（こまめなコミット・チェックリスト更新・日記）を通常より密に刻む
4. 実装フェーズの最終サブタスク完了時、以下の知見を `/addf-knowhow` で記録する（既存 knowhow の更新も含む）:
   - **コーディング知見**: 実装中に発見した再利用可能なパターン、落とし穴、技術的判断とその根拠
   - **分かれ道の目印**: 差し戻し・やり直し・想定外の判断が発生したサブタスクがあれば、使用したスキルの `.exp.md`「🔀 分かれ道の目印」にも追記する（書式: `.claude/addf/templates/ExperienceTemplate.md`。失敗の告白ではなく、意思決定が枝分かれしたポイントと次に同じ分岐に立ったときの選び方を道標として書く）

### エージェント起動時の共通ルール
- エージェントチーム（TeamCreate）やサブエージェント（Agent）を作成するとき、各エージェントへのプロンプトに **最初に `/addf-knowhow-index` を実行する** よう指示を含めること
- これにより各エージェントがプロジェクトの知見ベースを把握した状態で作業を開始できる

### タスク完了時 — 品質検証

4. プロジェクトのビルド・Lint・テストコマンドを実行する
   - プロジェクトテスト: `go test ./...` + `go vet ./...` + `go build ./cmd/ccchain`
   - ADD フレームワークテスト: `bash .claude/addf/tests/run-all.sh`
   - **失敗した場合 → 実装に差し戻す**。原因分析 → 修正 → 再実行
4.5. 統合テスト品質ゲートを実行する（「4.5」はプロジェクト固有ステップの意図的な枝番）
   - `go test ./internal/eval/ -run TestIntegration -v` で全統合テストを実行
   - 以下の3点を確認: <!-- human-judgment -->（テスト出力の解釈と期待値の追加判断）
     1. **結果の妥当性**: 危険コマンドが allow になっていないか（TestIntegrationDangerousRealWorld）
     2. **改善追跡**: ask → deny への改善が必要なコマンドのトラッキング（TestIntegrationDangerousIdealDeny）
     3. **新規危険パターン**: 実装中に発見した新たな危険コマンドを TestIntegrationDangerousRealWorld に追加
   - 新しいルールやコマンド解析を追加した場合、統合テストの期待値を更新する
5. `addf-code-review-agent` でコードレビューを実施する
   - 通常タスクは単体（ペルソナなし）で起動する
   - **マイルストーン・リリース直前・`mode: critical` 宣言時・unattended 自走時（`/addf-mode unattended`）**は、ペルソナ並列（視点ずらしレビュー）を起動する。起動前に `.claude/agents/addf-code-review-agent.md` を読み、ペルソナ定義に従うこと
   - ペルソナ並列の集約: 同一箇所・同一原因の指摘は1件にまとめてペルソナを列挙する。**2ペルソナ以上が独立に指摘した項目は重要度を1段上げる**（コンセンサス補正）
   - **ドキュメント変更を含むタスクでは `addf-doc-review-agent` も起動する**（ドキュメントドリフト検出）。起動条件: `git diff` に `*.md` 変更・`docs/` 配下変更・`.claude/commands/` や `.claude/agents/` の定義変更のいずれかが含まれる場合。起動判断はメインエージェント側で行い、条件を満たさなければスキップしてよい。エージェントの詳細は `.claude/agents/addf-doc-review-agent.md` を参照。**コードレビューと並列でよい**（両者は変更差分の別観点を見るため独立実行できる。集約は起動側で行う）
6. `addf-contribution-agent` で ADD フレームワークへのコントリビューション候補を検出する
7. レビュー指摘・発見への対応（**一次軸: 主題との関係 / 二次軸: クリティカル度**）:
   - **主題に沿うもの → このフェーズ内で対応する**（クリティカル度は問わない）:
     - Plan の意図の延長にある修正・追加・改善は、修正範囲が広くても同一 Plan 内でやりきる
     - レビュー指摘（Critical/High/Medium/Low/Info いずれも）が Plan の主題内なら、
       Critical/High は必修正・Medium 以下は原則修正の順で対応する
   - **主題から外れるもの → 別 Plan に切り出す**（「ついでに見つけてしまった何か」）:
     - 発見されたバグ・改善余地の関心事が現在の Plan と異なるなら、修正せずに新しい
       Plan（`.claude/addf/plans/`）を書き起こして `TODO.md` に追加し、現在の Plan を完了させる
     - **切り出した Plan の優先度はクリティカル度で決める**（二次軸）: 主題外の Critical/High は
       TODO 優先度最上位に置き、次タスクで即着手する（「フェーズ内先送り禁止」の安全性は
       粒度変更後も維持される）
   - **判定に迷ったら「主題外」に倒し、切り出し先の Plan に主題内で扱えなかった理由を残す**
     （後から統合したくなったら次サイクルで判断すればよい）
   - **切り出した Plan の実装ルート**は次サイクルの `/addf-dev` で
     `変更ルート判断表`（`.claude/addf/guides/speculative-development.md` の「変更ルート判断」節）
     に従う（本ルールは「切り出すか否か」、変更ルート判断表は「どう実装するか」の別軸）
   - 修正後、ビルド・Lint・テストを再実行して通過を確認する
8. 品質ゲートで得た知見を `/addf-knowhow` で記録する:
   - **品質ゲート知見**: レビューエージェントが検出したパターン（セキュリティ、コード品質、分離パターン違反等）のうち、他のタスクでも再発しうるもの

#### ノウハウ蓄積

9. 投入されたタスクのPlanに実装完了状況を反映する
10. タスク全体の総括知見を `/addf-knowhow` で記録する:
    - **タスク総括**: 計画と実装のギャップ、想定外だった点、次回同種タスクへの教訓。コーディング・品質ゲートで既に記録した知見と重複しないこと

#### フィードバック記録

11. `.claude/addf/Feedback.md` にPlan, TODO, Progress推進エンジンの問題の記録・改善アクションを追記する。反映済みの項目は削除する
12. `.claude/addf/Feedback.md` にプロジェクト進行上の問題の記録・改善アクションを追記する。反映済みの項目は削除する
13. Progress 推進エンジン自体に関するフィードバック・ノウハウがあれば、テンプレート（`.claude/addf/templates/ProgressTemplate.md`）の改善案を `.claude/addf/Feedback.md` に記録する

#### アーカイブとコミット

14. `.claude/addf/Progresses/YYYY-MM-DD-プラン名.md` にリネームして移動し、`.claude/addf/templates/ProgressTemplate.md` から新規の Progress.md を作成する
15. コミットする

---

## タスク

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

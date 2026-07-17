# Plan 0022 / 0025 実装で得た知見（2026-07-17）

> Plan 0022（auto モード時代の deny-first 安全網）と Plan 0025（リテラル for ループの
> 部分解析）の並行実装、および Stage 2 ペルソナ並列レビュー（skeptic×2 + attacker +
> security）で得られた再利用可能な知見をまとめる。

## 🟢 鮮度: 2026-07-17（実装完了時）

---

## 1. hook 出力の新旧共存: exit 2 + JSON の二重出力パターン

Claude Code hooks 仕様上、deny は「exit 2 + stderr」または「exit 0 + `hookSpecificOutput.permissionDecision: "deny"` JSON」のどちらでも通る（両サポート）。混在時は「exit 2 なら JSON は無視」が公式ルール。

**教訓**: deny の場合のみ **両方を出力**する多重防御が最適。
- 正しく JSON を解釈するホスト: exit 2 で先にブロック、JSON は無視される
- 古い CC バージョン・別ハーネス: exit 2 でブロック（唯一の砦）
- 実装コストゼロ、互換性コストゼロ

allow/ask/warn は exit 0 のまま（JSON のみ）。

参照: `cmd/ccchain/hook.go emitResponse`

## 2. canonical hash に argSep — argv 境界の消失を防ぐ

シェルコマンドを AST → 文字列に serialize してハッシュ化する際、**引数を空白で join すると argv 境界が消失する**。`echo "a b"` と `echo a b` が同一ハッシュに落ちる。

**教訓**: argv には現れない区切り（ASCII Unit Separator `\x1f` 等）で引数を結合する。または JSON 配列としてシリアライズ。

`rm -- "-rf /tmp"`（1引数のリテラルファイル名）を承認したら、`rm -- -rf /tmp`（2引数の危険コマンド）が同一ハッシュで通る攻撃を防げる。

参照: `internal/approve/normalize.go canonicalCommand`

## 3. Result 構造体のフィールド伝播漏れ

`Result` のような累積構造体でフィールド追加した際、既存の「別の結果を作り直す」経路（args:/scope: サブルール発火、ツール別評価等）で**新規フィールドがコピーされない**バグが起きやすい。

**教訓**: 新フィールド追加時は、その型を新規生成する全箇所を grep で洗い出し、明示的に伝播する。testcase は「親ルールの指定が args: マッチ後も生き残る」等の**貫通シナリオ**を書く。

参照: `internal/eval/evaluate.go` の `applyArgsRules` / `applyScopeToCommand` / `argsTooLongResult` の Unattended 伝播

## 4. AST 種別の明示的 unsupported 化（default 分岐の危険）

`switch v := node.(type)` で新しい AST 種別への対応を追加する際、`default:` に「未対応」の意味を持たせると、将来 AST 拡張時に**無言で拾える型が増える**リスクがある。

**教訓**: 対応する型を明示列挙し、`default:` は明示的に `ErrUnsupported` を返す。カバー対象が広がった時（Plan 0025 で ForClause が analyzable 側に来た等）、既存コードが無言でスキップする代わりに明示的エラーで検出される。

参照: `internal/approve/normalize.go canonicalCommand`

## 5. bodyRedefinesVar の落とし穴 — ビルトインは Assign ノードにならない

シェルの `for VAR in ...; do BODY; done` のループ内で `VAR` が再代入されるか静的検出する場合、`*syntax.Assign`（`VAR=value`）だけ見ると **`read`, `printf -v`, `mapfile`, `readarray`, `declare`, `local`, `let`, `getopts` 等のビルトイン** による代入が全部漏れる（これらは通常の `CallExpr`）。

**教訓**: ループ変数を第1引数に取るビルトイン一覧を保守し、`CallExpr` の Cmd が該当ビルトインで引数がループ変数と一致する場合は再代入とみなす。

参照: `internal/shell/topology.go bodyRedefinesVar`

## 6. for ループの word splitting 乖離

`wordToString` はクォート情報を保持しない設計。`for f in "target dir" x; do cp -t $f file; done` を展開すると、評価時は `cp -t "target dir" file`（3引数）だが、実行時は unquoted `$f` の word-splitting で `cp -t target dir file`（4引数）になる。

**教訓（最小変更）**: WordIter.Items のリテラル値が空白を含む場合は非解析にフォールバックする（精緻な対応は「クォート情報の追跡」を要する — Plan 0025 の後続改善候補）。

参照: `internal/shell/topology.go tryExpandForClause`

## 7. コマンド名位置の `\` エスケープ剥がし

Shell の `\rm` は alias/function を迂回する一般的なイディオムで `rm` として実行される。`Lit` を verbatim に保持すると ccchain は `\rm` と `rm` を別コマンドとして扱い、sentinel の rm 保護が全部素通りする。

**教訓**: `writeWordPart` の Lit 分岐で、**Word の先頭パート**（コマンド名位置）に限り、先頭の `\` を1文字剥がす（`\\` は保持）。引数位置の `\` は変更しない（`rm \foo` の `\foo` は引数 `foo` としてのシェル挙動と一致）。

参照: `internal/shell/topology.go unescapeCommandName`

## 8. 承認ストアの設定自己書換保護

sentinel が `ccchain approve` サブコマンドを deny しても、`.ccchain.conf` / `.ccchain.local.conf` そのものを Write/Edit すれば `ask_strategy: passthrough` を注入して降格を無効化できる。DSL 側の設定自己書換フェンスがないと、CLI の self-fence だけでは守れない。

**教訓**: 設定ファイル本体を Edit/Write ツール向けの deny ルールで守る。default プリセット・sentinel プリセット両方に含める。

参照: `internal/preset/sentinel.conf` の Edit/Write ルール、`cmd/ccchain/init_cmd.go defaultConfig`

## 9. ステイルロック回避のパターン — PID + timestamp + mtime grace period

`O_EXCL` ロックファイルは、プロセス異常終了で残ると恒久 DoS になる（`touch <lock>` で誰でも起こせる）。

**教訓**: ロックファイルに `{PID, timestamp}` を JSON で書き、取得失敗時に (a) PID 生存確認、(b) mtime > 閾値（例 30 秒）を確認し、両方 false なら強制奪取。

**追加の落とし穴**: 初期実装で「空ファイル即座 reap」にすると `O_EXCL` 直後の write 前ウィンドウで race する。mtime grace period が必要（正規オーナーの O_EXCL → write の間隙が保護される）。

参照: `internal/approve/store.go withLock`

## 10. env-var seam の本番副作用

テスト用に `CCCHAIN_APPROVE_STORE` のような環境変数分岐を本番コード内に置くと、hook 起動コマンドや shell rc が書ける状況で agent がストア差し替え可能になる。

**教訓**: 本番 `DefaultDir()` から環境変数分岐を撤去し、テスト用は unexported の setter（`SetDefaultDirForTest`）で置換。同一パッケージ内でしか使えないので production 側からは実質呼び出せない。

参照: `internal/approve/store.go SetDefaultDirForTest`

## 11. 人間側 UI にもサニタイズ — approve --list の ANSI 注入防御

エージェント向けの deny/warn メッセージには制御文字サニタイズを適用しても、`ccchain approve --list` のような**人間がターミナルで直接読む出力**にサニタイズを適用し忘れると、ANSI エスケープ注入で承認判断を欺ける（人間の目には別のコマンドが見えるが実際は別物）。

**教訓**: 人間の防衛線（承認プレビュー・確認表示）にもエージェント同等のサニタイズを適用する。両チャネルで同一のサニタイズ関数を共有する。

参照: `cmd/ccchain/approve.go previewCommand` / `printApprovalGranted`

## 12. 承認 hint にトークン・ハッシュを載せない

deny+hint にトークンやハッシュ値を埋め込むと、エージェントのコンテキストに入り、agent が自分で `ccchain approve <token>` を実行して自己承認できてしまう。

**教訓**: hint には**手順のみ**を書く（「オーナーがターミナルで `ccchain approve --last` を実行」）。deny した要求は pending ファイルに記録し、人間が別ターミナルで pending 一覧を確認・承認するフロー。合わせて `ccchain approve` 自体を sentinel で deny する多重防御。

参照: `internal/eval/degrade.go approveCommand`

## 13. 委譲プロンプトの「実装済み前提」は grep で確認する

worktree 隔離下のサブエージェント委譲では、親エージェントが「Phase 1/2 は main に merge 済み」と書いても、**worktree の base が古ければ実際には未実装**のことがある（複数エージェントの並行実装で発生しやすい）。

**教訓**: worktree 委譲プロンプトの「実装済み前提」は、委譲エージェント側で `grep -r "<新機能名>" internal/ cmd/` で 0 hit ならまだ入っていないと判定できる。委譲プロンプトにこの確認手順を書くと安全性が上がる。

参照: Phase 4 実装エージェントが `unattended:` の未実装を検知して deny 直接記述に切り替えた実例

## 14. 兄弟課題を同一 worktree ブランチに束ねる選定則

複数の投機課題を並列 worktree で実装するとき、**AST 層の同じ場所を触る兄弟課題**（例: topology.go の word 抽出）は別ブランチにすると integration で意図衝突する。

**教訓**: 選定時にファイル集合の重なりだけでなく「意味論の同一 AST 層への干渉」を確認する。同 AST 層の兄弟課題は1本のブランチに束ねる。

参照: Plan 0022 と Plan 0025 が独立と宣言されつつも承認正規化（canonical hash）と for ループ展開で結合していた実例

## 15. ペルソナ並列レビューの実効性

skeptic × 2（前提の疑問と Plan の意図の分離）+ attacker（実 E2E 攻撃再現）+ security（既知パターン照合）の 4 体並列で、単体レビューでは見つからなかった Critical 8 件を検出。

**教訓**:
- skeptic 2 セッションは重複を恐れずに投入する。異なる観点で独立に問題を検出する（今回は同じ「hash 衝突」でも E2E 攻撃再現の切り口 vs Plan 意図と実装の齟齬の切り口）
- attacker は「実機再現コード付き」の指摘を優先する。理論より実証
- コンセンサス補正（2ペルソナ以上が独立に指摘 → 重要度1段上げ）は今回も機能した

**次回の追加観点**: レビュー用 scratch ファイルの掃除。サブエージェントが `ccchain hook` の rm 拒否に阻まれて掃除できず未追跡ファイルを残す（自己ホスト環境の副作用）。委譲プロンプトに scratch は `t.TempDir` を使うよう指示する。

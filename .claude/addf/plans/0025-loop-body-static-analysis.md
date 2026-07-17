# Plan 0025: リテラル `for` ループの部分解析 と `unanalyzable_action` 設定

## 実装状況: 完了（2026-07-17）

Phase 1（WordIter 形式 for ループの部分解析）・Phase 2（unanalyzable_action 設定）
とも実装完了。ADDF 実 conf での実地確認済み（ドッグフーディング元の for ループ問題が
解消）。Stage 2 レビューで検出された空白リテラルの word splitting 乖離（C4）と
ビルトインによるループ変数再代入検出漏れ（C5: read/mapfile/declare/local/let/getopts/
printf -v）は主題内として修正済み。

## 背景

ADDF 本体（AutomatonDevDriveFramework）が Plan 0040 で ccchain をドッグフーディング導入した際、
`for f in a b c; do ...; done` 形式の `for` ループが軒並み `deny`（`"dynamic command detected:
static analysis not possible"`）になることが実運用で2度判明した。

コードベース調査（2026-07-16）で以下を確認済み:

- `internal/shell/topology.go:172-176` の `buildCommandFromStmt` は、AST ノードの種別だけで
  `*syntax.ForClause` / `*syntax.WhileClause` / `*syntax.IfClause` / `*syntax.CaseClause` /
  `*syntax.Block` を無条件に `Analyzable: false` にする（コメント: "Control flow structures
  contain commands that can't be statically evaluated in isolation — deny for safety"）
- `internal/eval/evaluate.go:108-114`（単発コマンド）・`:140-146`（パイプライン先頭）・
  `:161-171`（パイプライン後続）の3箇所で、`!cmd.Analyzable` は無条件に `dsl.ActionDeny` を
  返す。これは `.ccchain.conf` の `settings: fallback:` にも `--default-action` CLI フラグにも
  従わない、独立したハードコード経路である（`./ccchain --default-action ask eval 'for f in a b;
  do echo $f; done'` で deny のまま返ることを実機確認済み）
- `internal/eval/evaluate_test.go` 等（`internal/eval/*_test.go` 全11ファイル）に control-flow
  denial のテストは1件も存在しない — 現状の挙動自体に回帰保護がない

**このハードコード自体は意図的な safety-first 設計であり、バグではない**。しかし
「ワードリストが完全にリテラルな `for` ループ」は、ループ変数への代入を1つずつ展開して
本文（body）を通常の文と同様に解析すれば安全に判定できる — にもかかわらず現状は一律 deny
に倒れており、ADDF 側の実運用では「ループを書かず個別コマンドを列挙する」という回避を
強いられている。

## スコープ

1. **`WordIter` 形式 `for` ループの部分解析**（項目A） — ワードリストが全て静的リテラルな
   場合に限り、ループ変数を各リテラル値に展開した上で本文を通常解析する
2. **`unanalyzable_action` 設定の新設**（項目B） — 構造解析そのものが失敗した場合
   （control-flow・subshell・func-decl・unknown-stmt 等、`Analyzable: false` 全般）の
   アクションを、既定 `deny`（現状維持）のまま `ask` へ明示的に緩められる設定を追加する

**スコープ外**（別 Plan 候補として残す）:

- `CStyleLoop`（`for ((i=0; i<n; i++))`）の解析 — 数値ループカウンタの列挙は本 Plan の
  リテラル文字列展開とは別の実装が要る
- `while`/`if`/`case`/`Block`/`Subshell`/`FuncDecl` の部分解析 — `WordIter` の for ループより
  複雑（実行時条件・関数呼び出し追跡等が絡む）。項目Bの `unanalyzable_action: ask` が
  当面の緩和策として機能する
- ループ変数がワードリスト以外の位置（例: リダイレクト先ファイル名）に使われるケースの
  精密な扱い — 項目Aの実装では通常の `Analyzable` 判定と同じ経路を通すため、置換後に
  依然として動的要素が残っていれば自然と非解析扱いになる想定（フェーズ1実装時に要検証）

## 設計原則

- 既存 DSL・設定ファイルの後方互換を守る（`unanalyzable_action` 未設定時は現行の `deny` 挙動と
  完全に同一）
- 「解析できないものは安全側に倒す」という核となる設計思想は変えない。項目Aは「実は解析できる
  ケースの解析漏れを直す」精度向上であり、「解析できないものを緩める」わけではない
- 項目Bで `unanalyzable_action` に指定できる値は `ask` / `deny` のみとし、`allow` は許可しない
  （`scope_violation` と同じ制約 — 構造解析失敗を無条件 allow にできてしまうと、この設定自体が
  安全機構を丸ごと無効化する抜け道になる）

## Phase 1: `WordIter` 形式 for ループの部分解析

### 実装方針

1. `internal/shell/topology.go` の `buildCommandFromStmt` に `*syntax.ForClause` 用の分岐を
   追加する:
   - `Loop` が `*syntax.WordIter` であり、**`in` 節が省略されていない**（`WordIter.InPos` が
     有効な位置である）こと、かつ `Items` の**全要素**が既存の `isAnalyzable()`
     （`topology.go:307`）で真と判定される場合のみ「展開可能」とする。
     **注意**: `for f; do ...; done`（`in` 省略形）は `Items` が空スライスになるが、これは
     「空リスト＝0回反復」ではなく**シェルの位置パラメータ（`$1 $2 ...`）に対する動的ループ**
     である（mvdan.cc/sh の `WordIter` ドキュメント参照）。「全要素が analyzable」の判定は
     空スライスで空虚に真となるため、`InPos` チェックを欠くとこのケースを誤って展開可能と
     判定し、位置パラメータ越しの危険操作が安全側でない方向（解析可能扱い）に倒れる
   - `CStyleLoop`（`Loop.(*syntax.CStyleLoop)`）は現状どおり `Analyzable: false` に倒す
   - `Select`（`select` 文, POSIX 拡張）は同様に非対応のまま `Analyzable: false` に倒す
2. 展開可能な場合、`Do []*Stmt`（ループ本体）の各文に対して既存の `extractSegments()`
   （`topology.go:69`）を再帰的に適用し、本体のセグメント列を得る
3. `WordIter.Items` の各リテラル値について、本体セグメント中の `wordToString()` 結果に含まれる
   `$<ループ変数名>` / `${<ループ変数名>}` 相当のトークンをその値で文字列置換してから
   通常のルール照合（`matchCommand` 等）にかける。AST 自体を書き換えるのではなく、
   既存の文字列描画（`wordToString`）の出力に対する文字列置換で実現する
   （AST ミューテーションより実装・レビューが容易なため。この方式の限界は
   「置換後に再度シェル引用規則を考慮する必要がある位置」だが、単純な引数展開の範囲では
   影響が小さいと判断。実装時に反証があれば AST ベースへ切り替える）
4. 各リテラル値について展開・評価した結果のうち、最も制限的な結果（既存の
   `isMoreRestrictive` と同じ比較則）をループ全体の結果として採用する
5. **反復回数の上限**を設ける（例: 50件。`max_context_depth`/`max_rules_per_cmd` と同種の
   安全弁）。上限を超えたら従来どおり `Analyzable: false` に倒す（性能・レビュー容易性の
   両面での安全策）
6. ループ変数がボディ内で再代入される（シャドーイング・ネストした同名ループ変数）ケースは
   フェーズ1のスコープ外とし、検出したら安全側（非解析）に倒す

### テスト方針

- `internal/shell/topology_test.go`（新規 or 既存拡張）に以下のケースを追加:
  - リテラルのみの `for f in a.txt b.txt; do cat "$f"; done` → 展開・解析される
  - `for f in $(ls); do ...; done`（コマンド置換を含む）→ 従来どおり非解析
  - `for f in a "$X" b; do ...; done`（一部だけ動的）→ 従来どおり非解析
  - `for f; do echo "$f"; done`（`in` 節省略 = 位置パラメータへのループ）→ 従来どおり非解析
    （`Items` 空スライスの空虚な真で誤展開しないことの確認）
  - 反復回数が上限を超えるケース → 非解析にフォールバック
- `internal/eval/evaluate_test.go` に、展開後の本体が deny ルールに触れるケース
  （例: `for f in a b; do rm -rf "$f"; done` で `rm -rf` が deny 設定なら全体も deny になる）
  を追加し、「解析できるようになったことで危険な操作を正しく検出できる」ことを実証する
- 既存の control-flow denial テストが1件もない現状を踏まえ、本 Plan と同時に
  `for` 以外の control-flow（while/if/case/subshell/func-decl）が引き続き deny になることの
  回帰テストも追加する（フェーズ1の変更が意図せず他の control-flow を緩めていないことの確認）

## Phase 2: `unanalyzable_action` 設定の新設

### 実装方針

1. `internal/dsl/ast.go` の `Settings` 構造体に `UnanalyzableAction Action` フィールドを追加。
   `DefaultSettings()` の既定値は `ActionDeny`（現行挙動を完全維持）
2. `internal/dsl/parser.go` の設定パース（`case "fallback":` 等が並ぶ箇所、`parser.go:455`
   付近）に `case "unanalyzable_action":` を追加。許可値は `ask`/`deny` のみ
   （`scope_violation` と同じバリデーション、`allow` は `ParseError` にする）
3. `internal/dsl/config.go` の `mergeSettings()`（`config.go:121`）に
   `overlay.Explicit["unanalyzable_action"]` の分岐を追加（`scope_violation` と同型）
4. `internal/eval/evaluate.go` の3箇所（108-114行目・140-146行目・161-171行目）で
   ハードコードされている `Action: dsl.ActionDeny` を `config.Settings.UnanalyzableAction`
   参照に置き換える

### テスト方針

- `internal/dsl/parser_test.go` に `unanalyzable_action: ask` のパース成功・
  `unanalyzable_action: allow` のパースエラーを追加
- `internal/dsl/config_test.go`（あれば）に `mergeSettings` の overlay 上書きケースを追加
- `internal/eval/evaluate_test.go` に、`unanalyzable_action: ask` 設定下で
  `(subshell)` 等の非解析コマンドが `deny` ではなく `ask` を返すことを確認するテストを追加
- 既定値（未設定時）で従来どおり `deny` になることの回帰テストを必ず含める

## 完了条件

- [ ] Phase 1: リテラルのみの `for` ループが正しく展開・評価される（`go test ./...` 通過）
- [ ] Phase 1: 反証テスト（動的ワードリスト・上限超過）が引き続き非解析＝deny になる
- [ ] Phase 1: 他の control-flow（while/if/case/subshell/func-decl）の回帰テストを新設し、
      引き続き deny になることを確認（現状これらのテストが皆無だったための追加）
- [ ] Phase 2: `unanalyzable_action: ask`/`deny` の設定が反映される。`allow` 指定時は
      設定エラーになる
- [ ] Phase 2: 未設定時の既定挙動（deny）が変わらないことの回帰テスト
- [ ] `make smell`（savanna-smell-detector）でスメル0件、または `smell-allow` コメント付きで
      意図的許容のみ残る
- [ ] README / docs のルール一覧に `unanalyzable_action` の説明を追記
- [ ] ADDF 側の `.ccchain.conf`（`~/workspace/AutomatonDevDriveFramework/.ccchain.conf`）で
      実際に control-flow を含むコマンドを評価し、意図通りに緩和されることを確認
      （ドッグフーディング元の Plan 0040 側からのフィードバックループを閉じる）

## 関連

- ADDF 側の発端: `AutomatonDevDriveFramework/.claude/addf/knowhow/ADDF/ccchain-dogfooding-phase1.md`
  「`for` ループ等の制御構文は静的解析不能として無条件 deny される」節
- Plan 0022（ask_strategy・deny-first sentinel）とは独立（Phase 0022 は permission_mode 起因の
  ask 到達性の話、本 Plan は構造解析の精度・設定可能性の話）

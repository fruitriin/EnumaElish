# Plan 0022: ccchain 再起動 — auto モード時代の deny-first 安全網

## 実装状況: 未着手

## 背景

開発停止中に Claude Code のパーミッション仕様が大きく変わり、ccchain の設計が環境に追いつかれた。

**2026-07 時点の Claude Code 仕様（オーナー確認済みの転写。実装前に Phase 0 で必ず再裏取りする）:**

- PreToolUse hook の `permissionDecision` は `allow / deny / ask / defer` の4値。**hook は権限モードチェックの前に走る**
- パーミッションモード: `default` / `acceptEdits` / `plan` / `auto`（v2.1.83+、classifier が審査）/ `dontAsk`（v2.1.199+）/ `bypassPermissions`（v2.1.126+）
- 相互作用（最重要）:
  - `deny`: **全モードで実行ブロック**。bypassPermissions でも有効
  - `ask`: default・acceptEdits・**bypassPermissions → ダイアログが出る**。auto → defer 扱い（classifier 行き、人間への強制確認は不可）。dontAsk → プロンプトなしで拒否。headless（`claude -p`）→ defer
  - `allow`: 全モードでプロンプトスキップ
  - `defer`: モードの既定フローに委ねる（非インタラクティブ専用）
- settings.json の `permissions.ask` 配列は存在し、評価順は deny > ask > allow。auto モードでも ask ルールはダイアログを出す
- PermissionRequest hook は非インタラクティブでは実行されない。`--permission-prompt-tool` 相当の仕組みは現存しない

**含意**: 旧構成（bypassPermissions + hook）は現仕様では正規のレイヤリングとして成立する。唯一の穴は **auto モードと headless での ask**（人間に届かず classifier / 既定フローに吸われる）。ccchain の `ask` は「人間に確認してほしい」という意図の表明なので、届かない環境では **deny+hint または allow+hint に明示的に倒す**のが本計画の核心。どちらに倒すかは内容次第でユーザーがルール単位に指定し、既定は deny 側（安全側）とする。

### 現状とのギャップ（コードベース調査 2026-07-04）

計画の前提となる実装済み事実と乖離:

| 項目 | 現状 | ギャップ |
|---|---|---|
| hook 出力形式 | deny = stderr + **exit 2**、ask = `{"decision":"ask"}`、warn = `{"decision":"allow","message":...}`（`cmd/ccchain/hook.go:113` `outputResult`） | 新仕様の `hookSpecificOutput.permissionDecision` / `permissionDecisionReason` JSON 形式に未対応。**4値を返す土台がない** |
| `permission_mode` 読取 | **未読取**（stdin パース構造体 `toolInput` は `tool_name` / `tool_input` のみ。grep 0 hit） | ask_strategy の分岐入力が取れていない |
| `defer` アクション | DSL に存在しない（`ActionAllow/Deny/Warn/Ask/Hint`、`internal/dsl/ast.go:19-25`） | hook 出力としての defer をどう扱うか設計が必要 |
| deny メッセージ | Plan 0012 のテンプレート展開あり（`internal/eval/message.go`、`{command}` 等の変数 + sanitize + 200 字切詰め） | 承認手順の埋め込みで 200 字制限と衝突する可能性 |
| プリセット | `ccchain init` の `defaultConfig` 定数1種のみ（`cmd/ccchain/init_cmd.go:8-131`） | プリセット選択機構がない |
| バージョン | git tag なし、`main.version = "dev"` | 破壊的変更を含むためタグ運用開始が必要 |

## スコープ

1. **hook I/O 現代化** — permissionDecision 4値出力と permission_mode 読取（他の全項目の前提）
2. **ask_strategy** — DSL の ask アクションを実行時モードで解決
3. **承認トークン（`ccchain approve`）** — deny+hint 経由の人間承認をワンショットで通す
4. **deny-first プリセット（`ccchain init --sentinel`）** — キュレート済みルールセット同梱
5. **ポジショニング刷新** — README/docs を「deny-first 安全網」へ書き直し

**スコープ外**（別計画として登録済み）:
- ADDF 統合（/addf-init での hook 自動設営）→ Plan 0023
- Slack/リモート承認連携 → Plan 0024

## 設計原則（オーナー指定の制約）

- 既存 DSL・設定ファイルの後方互換を守る
- シングルバイナリ・依存追加は最小限（承認トークンはファイルベース）
- deny は必ずヒント付き。「ブロックが対話になる」思想を崩さない

---

## Phase 0: 仕様の再裏取り（実装前必須ゲート）

背景セクションの転写を信じず、WebFetch で以下を再確認する。**結果は本 Plan のこのセクションに追記し、乖離があれば設計を修正してから Phase 1 に進む。**

- [x] https://code.claude.com/docs/en/hooks-guide.md — PreToolUse の入出力スキーマ（`permission_mode` フィールドの正確な名前と値集合、`hookSpecificOutput.permissionDecision` の4値、`permissionDecisionReason` の表示先）
- [x] https://code.claude.com/docs/en/permission-modes.md — 6モードの正確な挙動、auto での ask → defer 扱いの記述
- [x] https://code.claude.com/docs/en/permissions.md — deny > ask > allow の評価順、bypassPermissions での deny/ask の有効性
- [x] **headless の検出方法**: hook 入力 JSON から headless（`claude -p`）を判別できるか（permission_mode に現れるのか、別フィールドか、判別不能か）。判別不能なら ask_strategy の「headless」区分は「非インタラクティブ系モードと同一扱い」に設計変更する
- [x] **旧形式との共存**: exit 2 + stderr（現行の deny）と新 JSON 形式のどちらが優先されるか。旧 Claude Code バージョンに新 JSON を渡した場合の挙動（後方互換戦略の決定材料）
- [x] `permissionDecisionReason` の文字数制限・表示のされ方（deny+hint の承認手順埋め込みが読める形で届くか）

検証結果の記録先: 本セクション末尾に「### 検証結果（YYYY-MM-DD）」を追記。.claude/addf/knowhow/ADDF/claude-code-hooks.md との乖離があれば `/addf-knowhow-revise` で更新する。

### 検証結果（2026-07-17）

公式ドキュメント（hooks.md / hooks-guide.md / permission-modes.md / permissions.md）を WebFetch で再確認した。

**確定した事実（出典付き）:**

1. **`permission_mode` フィールドは実在**（hooks.md の PreToolUse 入力スキーマ）。キー名は `permission_mode`、値は `default` / `acceptEdits` / `plan` / `auto` / `dontAsk` / `bypassPermissions` の6値すべて。他フィールド: `session_id`, `cwd`, `tool_name`, `tool_input`, `hook_event_name`, `effort`, `prompt_id`, `transcript_path`。**session_id は来る**（Phase 3 のスコープ判定に併用可）
2. **`hookSpecificOutput.permissionDecision` は4値**（allow / deny / ask / defer）。`defer` は「通常の permissions フローへ委譲。headless `-p` mode でのみ機能」と明記。`permissionDecisionReason` は Claude にフィードバックされる（エージェントのコンテキストに入る）
3. **`permissionDecisionReason` の文字数制限は記載なし** → 降格 deny の承認手順埋め込みは長さの面で問題ないが、自衛的に上限は設ける（下記設計修正）
4. **旧形式（exit 2 + stderr）と新 JSON（exit 0）は両方サポート**。「Don't mix them: Claude Code ignores JSON when you exit 2」（hooks-guide.md）。新 JSON が推奨 → **`hook_output: legacy` 設定オプションは追加しない**（Plan の条項どおり、不要な設定面を増やさない）
5. **auto モードで hook が ask を返した場合の挙動は公式に明記なし**。dontAsk は「prompt する代わりに deny」（permission-modes.md）、bypassPermissions では「explicit ask rules は依然 prompt する」。背景転写の「auto → classifier 行き」は裏取りできなかったが、いずれにせよ auto/dontAsk で ask が人間に確実に届く保証はなく、降格戦略の価値は変わらない
6. **headless は hook 入力 JSON から判別不能**（該当フィールドの記載なし） → **設計変更: ask_strategy の「headless」区分は削除**。分類は permission_mode のみで行う2区分（interactive / nonInteractive）
7. **評価順 deny > ask > allow を確認**（permissions.md）。「hook の決定は permission rules をバイパスしない」— hook が allow を返しても settings の deny/ask ルールは独立に評価される（hooks-guide.md）
8. **PreToolUse hook は permission-mode チェックの前に走る**（hooks-guide.md「PreToolUse hooks fire before any permission-mode check」）

**設計修正（本検証を受けて）:**

- `classifyMode` は2区分で確定: interactive（default / acceptEdits / plan / bypassPermissions）/ nonInteractive（auto / dontAsk / 未知値 / 空文字）。未知・空は保守的に nonInteractive
- `hook_output: legacy` オプションは追加しない。全アクションを exit 0 + `hookSpecificOutput` JSON に統一（deny の exit 2 + stderr を廃止 — 破壊的変更として CHANGELOG に明記）
- Phase 3 のスコープ判定は session_id + cwd の併用で確定
- 降格メッセージは `permissionDecisionReason` に埋め込む。公式の文字数制限はないが、サニタイズ層の切詰め上限を 200 → 600 字に拡大して承認手順が収まるようにする（prompt injection サニタイズ自体は維持）

## Phase 1: hook I/O 現代化

### 入力: permission_mode の読取

`cmd/ccchain/hook.go` の `toolInput` を拡張:

```go
type toolInput struct {
    ToolName       string          `json:"tool_name"`
    Input          json.RawMessage `json:"tool_input"`
    PermissionMode string          `json:"permission_mode"` // Phase 0 で正確なキー名を確認
}
```

- 未知の値・空文字は **最も保守的な非インタラクティブ扱い**にフォールバック（前方互換: 新モードが増えても安全側）
- モード分類関数 `classifyMode(mode string) ModeClass` を `internal/eval` に置く（interactive / nonInteractive の2区分。Phase 0 の headless 調査結果次第で3区分）

### 出力: permissionDecision 4値 JSON

`outputResult` を新形式に書き換える:

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "deny",
    "permissionDecisionReason": "ccchain: curl|bash はパイプ実行のため deny。承認するには…"
  }
}
```

- deny も **exit 0 + JSON** に移行（現行の exit 2 + stderr から変更）
- **後方互換オプション**: `settings:` に `hook_output: legacy | modern`（デフォルト modern）を追加。Phase 0 の共存調査で旧形式しか解さない環境が確認されたら legacy を残す。調査で新形式が全対象バージョンで機能するなら本オプションは**追加しない**（不要な設定面を増やさない）
- `ActionHint` の出力未実装（現行 `default:` で exit 0 に落ちる）もこの機会に整理: hint は `permissionDecision: "allow"` + reason で警告文を届ける

### DSL への影響

- `defer` は **DSL のアクションには追加しない**。defer は「モード既定に委ねる」であり、ルール作者の意図としては fallback 未マッチと等価。hook 出力層でのみ使う（例: ccchain が判定不能なツールへの中立応答）
- 既存 5 アクション（allow/deny/warn/ask/hint）はそのまま。**後方互換維持**

## Phase 2: ask_strategy

### 動作

DSL 評価結果が `ask` のとき、hook 出力層で実行時モードにより解決する。ask が人間に届かないモードでは **deny+hint / allow+hint のどちらかに倒す**。どちらに倒すかは内容次第でユーザーがルール単位に指定でき、既定は deny 側（オーナーレビュー 2026-07-04 反映）:

| permission_mode | 既定の解決 | 根拠 |
|---|---|---|
| default / acceptEdits / bypassPermissions | `ask` をそのまま出力 | ダイアログが人間に届く |
| plan | `ask` をそのまま出力 | 実行されない（read-only 計画中）。Phase 0 で要確認 |
| auto | **降格**: deny+hint（既定）または allow+hint（ルール指定時） | ask は classifier 行きで人間に届かない |
| dontAsk | 同上 | ask は無告知拒否になり対話にならない |
| headless（判別可能なら） | 同上 | ask は defer 扱いで人間に届かない |

**倒す方向の使い分け**:
- **deny+hint**（既定）: 人間の判断なしに実行させたくないもの。hint に承認手順（Phase 3）を埋め込み、「非同期の対話」に変換する
- **allow+hint**: 止めるほどではないが注意喚起したいもの（例: workspace 外の読取り、初回実行のツール）。`permissionDecision: "allow"` + reason で実行を通しつつ、エージェントのコンテキストに注意文を残す。既存の `warn` アクションと同じ出力形態であり、実装は warn への降格として整理できる

### 設定（.ccchain.conf）

グローバル既定 + ルール単位の上書きの2層:

```
settings:
  ask_strategy: degrade        # degrade（既定）| passthrough | deny-all
  ask_degrade_default: deny    # deny（既定）| allow — degrade 時に倒す側の全体既定

preToolUse:
  ask docker "コンテナ操作は確認したい"
    unattended: allow          # このルールの ask は非対話時 allow+hint に倒す

  ask git-branch-delete "ブランチ削除は backup ref を先に"
    unattended: deny           # 明示（既定と同じ）。非対話時 deny+hint + 承認手順
```

- `degrade`（既定）: 上表のとおり非インタラクティブで降格。倒す側は `unattended:`（ルール単位）→ `ask_degrade_default`（グローバル）→ `deny`（組込み既定）の順で解決
- `passthrough`: 常に ask をそのまま返す（旧挙動。classifier を信頼する運用向け）
- `deny-all`: モードを問わず ask を deny+hint に格上げ（最保守。CI 等。`unattended: allow` 指定より優先）
- **後方互換**: `ask_strategy` / `unattended:` 未記載の既存 conf は degrade + deny 側になる。既存動作からの変化は「auto/dontAsk で ask が deny になる」ことだが、これは安全側への変化であり本計画の目的そのもの。CHANGELOG と README に明記する
- パーサー変更: ルール子ブロックに `unattended:` キーワードを追加（`parseRule` の子ブロック分岐、`args:` ブロック内の個別アクションにも同様の指定を許すかは実装時に判断）。sentinel プリセット（Phase 4）の各 ask ルールにも方向を明示的にキュレートして同梱する

### 降格時の deny メッセージ

Plan 0012 のテンプレート機構を拡張し、降格 deny に自動付加する定型文を用意する:

```
ccchain: このコマンドは人間の承認が必要ですが、現在のモード（auto）では
確認ダイアログを表示できません。承認するには対話セッションで実行するか、
オーナーがターミナルで `ccchain approve --last` を実行してください。
```

- 新テンプレート変数 `{permission_mode}` `{approve_command}` を `internal/eval/message.go` に追加
- `sanitizeForMessage` の 200 字切詰めは承認手順が収まるよう見直す（切詰め値を定数化し、降格メッセージは切詰め対象外にするか上限を拡大。prompt injection 対策のサニタイズ自体は維持）

## Phase 3: 承認トークン（`ccchain approve`）

### 脅威モデル（設計の要）

**hint にトークンそのものを埋め込んではならない。** hint はエージェントのコンテキストに入るため、トークンを見たエージェントが自分で `ccchain approve <token>` を実行して自己承認できてしまう。よって:

- hint には**手順のみ**を書く（「オーナーがターミナルで `ccchain approve --last` を実行」）
- deny した要求は pending ファイルに記録し、**人間が別ターミナルで** pending 一覧を確認・承認する
- `ccchain approve` コマンド自体を sentinel プリセットで `deny`（エージェント経由の実行を ccchain 自身が止める。自己言及的だが hook 経由の Bash はすべて ccchain を通るため成立する）。加えて README で settings.json の `permissions.deny` に `Bash(ccchain approve*)` を追加する構成を推奨（二重防御）

### フロー

```
1. auto モードで ask → deny 降格発生
   → ~/.claude/ccchain/pending.jsonl に {正規化コマンド, ハッシュ, cwd, timestamp} を追記
   → hint: 「承認するには: ccchain approve --last（オーナーのターミナルで）」
2. 人間: ccchain approve --last（または ccchain approve --list で選択）
   → ~/.claude/ccchain/approved.jsonl に {ハッシュ, 発行時刻, TTL, スコープ} を追記
3. エージェントが同一コマンドを再実行
   → hook が正規化ハッシュ一致 + TTL 内 + スコープ一致を確認 → allow（ワンショット消費: エントリを消費済みにマーク）
```

### 設計判断（実装時に確定、初期値を提示）

- **正規化**: `mvdan.cc/sh` の AST を printer で再出力した文字列の SHA-256。空白・クォートの揺れを吸収し、意味の同一性で照合する。動的要素（`$VAR`、`$(...)`）を含むコマンドは**承認対象外**（展開結果が変わるため。pending 記録時に reject し hint で理由を伝える）
- **TTL**: 既定 15 分。`ccchain approve --ttl 1h` で上書き
- **スコープ**: 既定はセッション+ディレクトリ限定（`session_id` があれば併用 — Phase 0 で hook 入力に session_id が来るか確認。なければ cwd 一致のみ）。`--global` でマシン全体
- **ワンショット**: 既定は1回消費。`--count N` は最初は実装しない（YAGNI）
- **ストア**: `~/.claude/ccchain/` 配下の JSONL 2 ファイル + ファイルロック（`O_EXCL` の lock ファイル方式。依存追加なし）。ファイルパーミッション 0600
- **監査**: 承認・消費は Plan 0004 の audit ログにも記録する

### CLI 追加

```
ccchain approve --last          # 直近の pending を承認
ccchain approve --list          # pending 一覧（番号選択）
ccchain approve <hash-prefix>   # ハッシュ指定
ccchain approve --revoke-all    # 未消費の承認を全破棄
```

`printUsage`（`cmd/ccchain/main.go`）への追加を忘れない（doc-drift-pattern.md の printUsage 整合チェック対象）。

## Phase 4: deny-first プリセット（`ccchain init --sentinel`）

### 方針

エンジンの価値をルールセットで届ける。「classifier が構造を見ないから拾えないが、AST 解析なら確実に止められる」パターンをキュレートする。

### 収録ルール（初期セット。fixture で全件検証）

| パターン | アクション | 根拠 |
|---|---|---|
| `curl \| bash` / `wget \| sh`（パイプ先が任意シェル） | deny | 未検査コードの実行。プレフィックスマッチでは `curl` 全体を止めるしかなかった代表例 |
| `find … -exec rm` / `-delete` | deny | ネスト実行の破壊操作（`evaluateNested` の主戦場） |
| 保護パスへの `rm -rf`（`~`, `/`, `.git`, workspace 外） | deny | scope 機構 + args: の合わせ技 |
| `git push --force` / `+refs` を main/master/develop へ | deny | args: regex。ブランチ名は `args:` パターンで判定 |
| backup ref なしのブランチ削除（`git branch -D`） | ask（sentinel では deny+hint で「先に backup ref を作る手順」を提示） | my-environment.md の既知の再発ポイントでもある |
| `git reset --hard` + `git clean -fd` のチェーン | deny | 未コミット変更の全損 |
| `chmod -R 777` / `chown -R` の広域適用 | deny | |
| `eval` / `source <(curl …)` | deny | 動的コード実行 |
| dd / mkfs / diskutil の書込み系 | deny | |

- 既存 `defaultConfig`（`init_cmd.go`）とは別定数 `sentinelConfig` として持ち、`--sentinel` フラグで選択。既定の `ccchain init` の出力は**変えない**（後方互換）
- semantics テーブル（`internal/semantics/table.go`）に不足パターンがあれば追補し、`generate-rules` との整合を保つ
- 各 deny ルールに **必ず message を付ける**（なぜ止めたか + 代替手順 + 承認手順）。「ブロックが対話になる」原則の実践
- dsl-rule-design.md の args: regex の罠（範囲パターンの過度マッチ、複合サブコマンドのスペース正規化）に留意

### 検証

- fixture テスト（fixture-based-testing.md 方式）: `testdata/fixtures/rules-sentinel.conf` を追加し、`TestFixtureDangerousNeverAllow` の対象に組み込む
- `TestIntegrationDangerousRealWorld` / `TestIntegrationDangerousIdealDeny` に sentinel 適用時の期待値を追加

## Phase 5: ポジショニング刷新（README / docs）

### 書き直しの軸

「プレフィックスマッチの限界を補う」→ **「auto/bypass 時代に classifier が拾えない構造的パターンを止める deny-first 安全網」**

- ccchain の deny は**全モードで有効**（bypassPermissions でも）— これが最上位の価値
- auto モードの classifier は確率的判定。ccchain は AST 解析による決定的判定。「classifier の誤許可に対する最後の砦」
- ask は届く環境でだけ使い、届かない環境では deny+hint + 承認トークンで「非同期の対話」に変換する

### 成果物

- README.md / README.en.md: ポジショニング書き直し + **モード×判定の互換表**（背景セクションの相互作用表を整形して掲載）+ sentinel プリセットのクイックスタート
- docs/（VitePress）: ask_strategy・approve・sentinel のリファレンスページ追加、既存 DSL リファレンスに settings 追記
- CHANGELOG: 破壊的変更（hook 出力形式）・安全側変更（ask 降格）を明記
- doc-drift-pattern.md の4パターン（ロードマップ・printUsage・README 一覧・DSL リファレンス）をチェックリストとして品質ゲートで確認

## 実装順序と依存

```
Phase 0（再裏取り）
  └→ Phase 1（hook I/O） ← 全ての前提
       ├→ Phase 2（ask_strategy）
       │    └→ Phase 3（承認トークン）… 降格 deny の hint が approve に言及するため
       └→ Phase 4（sentinel プリセット）… Phase 2/3 と並行可能（worktree 分離）
Phase 5（ドキュメント）… Phase 2〜4 の確定後
```

リリース: 完了時に初の git tag（`v1.0.0` を提案 — hook 出力形式の破壊的変更を含む節目。Makefile の `git describe` 運用が始動する）。

## テスト戦略

- 単体: `classifyMode` / ask 解決（`unattended:` → `ask_degrade_default` → 組込み既定の3層解決、deny-all の優先を含む）/ 正規化ハッシュ / TTL・スコープ判定 / pending-approved ストア（並行アクセス含む）
- 統合: 既存 `TestIntegration*` を permission_mode 別にパラメタライズ（interactive/auto で期待値が変わるケースを明示）。`TestIntegrationDangerousRealWorld` に「auto モードで ask が deny に降格していること」の検証を追加
- fixture: rules-sentinel.conf の全収録ルール × commands.txt
- E2E 手動: 実際の Claude Code auto モードで hook を設定し、降格 deny → approve → 再実行 allow の一連フローを確認（品質ゲートの human-judgment 項目）

## セキュリティレビュー観点（addf-security-review-agent への申し送り)

- 承認トークンの自己承認経路（hint にトークンが漏れていないか、approve 自体の deny が効いているか）
- pending/approved ファイルの権限・シンボリックリンク攻撃・ロック競合
- 正規化ハッシュの衝突・バイパス（コメント挿入、エンコーディング、IFS 操作で同一ハッシュの別意味コマンドが作れないか）
- 降格メッセージ経由の prompt injection（sanitize の維持を確認）
- `unattended: allow` の指定範囲の妥当性（sentinel プリセット内に allow 側へ倒す ask ルールを含める場合、その選定根拠。広すぎる allow 降格は安全網の穴になる）
- security-review-findings.md の既知パターン（相対パス、MCP 引数未検査）が新コードパスで再発していないか

## 参照ノウハウ

- .claude/addf/knowhow/dsl-rule-design.md — args: regex の罠、last-rule-wins、conf の役割分担
- .claude/addf/knowhow/security-review-findings.md — 過去の脆弱性5パターン
- .claude/addf/knowhow/ADDF/pretooluse-block-with-rationale.md — deny 時の根拠伝達
- .claude/addf/knowhow/ADDF/claude-code-hooks.md — hook 入出力仕様（Phase 0 で鮮度再検証）
- .claude/addf/knowhow/doc-drift-pattern.md — ドキュメント同期の4パターン
- .claude/addf/knowhow/fixture-based-testing.md — プリセット検証方式

## AI 実装時間の見積もり

- Phase 0: 1セッション枠（WebFetch 調査 + Plan 追記）
- Phase 1–2: 1〜2セッション（hook I/O は影響範囲が広くテスト更新が主コスト）
- Phase 3: 1〜2セッション（ストア設計 + セキュリティレビュー往復）
- Phase 4: 1セッション（ルール設計 + fixture）
- Phase 5: 1セッション（ドキュメント一式）

# Plan 0026: 承認ストア強化とレビュー由来の主題外項目

## 実装状況: 未着手

## 背景

Plan 0022（ask_strategy と deny-first sentinel）と Plan 0025（for ループの部分解析）の
実装完了後に実施したペルソナ並列レビュー（skeptic ×2 + security + attacker）で、
Plan 0022/0025 の主題内は Phase 内で修正しつつも、以下は「主題から外れる or
API/設計変更を要する」ため別 Plan として切り出した項目群。

現状 Phase 3（承認ストア）は認証・完全性検証・時計依存を持たない最小構成で、
リリース前の段階での妥当性判断が必要になる。加えて、Plan 0022 Phase 0
（Claude Code 仕様の裏取り）は Bash 経路のみを検証しており、Read/Edit/Write 経路
での hook `ask` の動作は未確認のまま実装されている。

これらは「動くもの」の追加ではなく「動いてしまうもの」の狭窄・監査の
硬化なので、実装時期は本流機能とは切り離してよい。

## スコープ

7 項目を Phase 単位に整理する。**Plan 0022/0023/0024 とは独立で、いつ着手しても
本流機能に影響しない**（順序も自由）。

### Phase 1 — 承認ストアの完全性強化（security C1）

**問題**: `~/.claude/ccchain/approved.jsonl` は改竄検知手段を持たない。
攻撃者が同ファイルへの書き込みを取れれば、任意コマンドの ApprovedEntry を
偽造できる（ccchain approve を経ずに）。

**方針**（複数案の要検討）:
- **案 A**: HMAC-SHA256 でエントリ毎に MAC を付与し、機械キーを
  `~/.claude/ccchain/mac.key`（0600、TPM/keychain 無しの純ファイル運用）に保管する
- **案 B**: ストアディレクトリ全体に sentinel マーカーファイル（内容固定・書換検出）を
  置き、書換を検知したら CheckApproved が即 false（fail-closed）を返す
- **案 C**: 承認ストアに書き込む主体を「オーナー端末経由の `ccchain approve` 呼び出し」に
  限定し、hook 経由の pending 記録は別ストア（改竄されても実害の少ないもの）に分離する

**決定基準**: ccchain の「依存追加最小」原則との相性（案 A は crypto/hmac のみで
外部依存ゼロ、案 B は仕組みは薄いが検知の完全性がない、案 C は API 変更が大きい）
を Phase 開始時に再評価する。

### Phase 2 — 承認ストア Scope の型安全化（skeptic#2 H1）

**問題**: `ApprovedEntry.Scope == ScopeSession` かつ `SessionID == ""` の場合、
現在の CheckApproved は「SessionID が空なら比較スキップ」で通過させる
（`internal/approve/store.go:483-489`）。これは buildApproved の seed フォールバックで
到達可能な状態で、意図としては「テスト用の空でもマッチさせる」だが実運用では
「セッション制約なしのゆるい scope」に化ける。

**方針**: `ApproveOptions` / `ApprovedEntry` の Scope を型分離する。
たとえば `ScopeSession(sessionID)` / `ScopeCWD(cwd)` / `ScopeGlobal` のような
enum-with-payload 相当を Go で表現し、空文字列を「無制約」と誤解される余地を
排除する。既存 JSONL の後方互換は保つ（読み時は寛容、書き時は新形式）。

### Phase 3 — Plan 0022 Phase 0 の再検証（skeptic#2 H2 + attacker H3）

**問題**: Plan 0022 Phase 0（Claude Code 仕様の裏取り）は Bash tool のみを対象とし、
Read/Edit/Write と各 permission_mode の掛け算での `ask` 挙動が実機確認されていない。
特に:

- `acceptEdits` × Read/Edit/Write の hook `ask` は自動許可されるか？
- `plan` × 任意 tool の hook `ask` は何を返すか？
- `bypassPermissions` × hook `deny` は本当にブロックされるか？

**方針**: Claude Code の実機セッションで matrix テストを組み、
docs/reference/permission-mode-behavior.md に結果を記録する。
発見された差異は個別に Plan 0022 の後続として起票するか、本 Plan Phase 4 に統合する。

### Phase 4 — sentinel の git config 保護補完（security M1）

**問題**: Plan 0018（doc-sync-and-security-fixes）で Medium として起票された
git config 保護の穴が sentinel には未反映。`git config` 系の allow は現行
`^config\s+.*(editor|pager|hook|gpg\.program|core\.sshCommand)` のみで、
以下は素通り:

- `git config --global include.path <path>` — 別ファイルを include
- `git config --global fsmonitor.hook.path <path>` — hook 経路の追加
- `git config core.hooksPath <dir>` — hooks ディレクトリの差し替え

**方針**: sentinel.conf の git config 保護に上記3系統を deny 追加。
`internal/preset/sentinel.conf` の該当行を1本追加＋fixture テストを1件追加で完了する。

### Phase 5 — pending.jsonl の寛容化（security M2）

**問題**: `readJSONL` は `bufio.Scanner`（`Buffer(1<<20, 1<<20)`）を使うため、
1 MB を超える1行（例: 巨大なコマンド文字列や後方に空白詰めしたエントリ）は
scanner.Scan() が false を返し、以降の行を全て読まない。攻撃者が pending.jsonl に
1 MB 超のダミー行を注入すると、以降の pending 全てが「存在しない」ことになる
（ApproveByHashPrefix / ApproveLast が「no pending」を返し始める）。

**方針**: `bufio.Reader.ReadBytes('\n')` に置き換え、行単位で読み進める。
巨大行は個別にスキップ（ログ出力のみ）してストア全体を壊さない。

### Phase 6 — 承認 TTL のモノトニック時計化（attacker M1）

**問題**: `ApprovedEntry.ApprovedAt + TTLSeconds` は unix 時刻の比較で判定される。
`s.now()` を差し替えることで壁時計に依存しているため、システム時計を後戻り
させれば有効期限を延長できる（承認から10分後にシステム時計を1時間戻す →
`now < ApprovedAt+TTLSeconds` が再び真になる）。

**方針**: 保存時刻を unix 時刻のまま維持しつつ、CheckApproved / Purge の判定を
`time.Since(time.Unix(e.ApprovedAt, 0))` ではなく、絶対値の diff で
「経過秒数がマイナスになったら未来の approved」扱いにする防御を加える。
より根本的にはモノトニック時計併記だが、JSONL でシリアライズできないため
最小変更としては「後戻りしたら再検証」で妥協する。

### Phase 7 — sentinel ドキュメントの多層防御追加

**問題**: Security H4 で対応した `ccchain init --sentinel` の Next steps 出力
（settings.json への `Bash(ccchain approve*)` 明示追加）は、実行時案内としては
届くが、`docs/reference/sentinel.md` にも書かれていない。

**方針**: sentinel.md の冒頭に「settings.json での多層防御」節を追加し、
Next steps と同一の推奨手順を文書化する。

## スコープ外

- 承認ストアの分散化（複数マシン間の同期）—— リモート承認は Plan 0024 の主題
- Claude Code の仕様変更追従（v2.1.83+ 以降で変わりうる）—— 継続的な文書更新に留める
- ccchain 自体の暗号署名（バイナリ検証）—— リリース戦略で扱う

## 優先度

- **Phase 3**（仕様再検証）は他 Phase の前提になりうる（挙動が想定と違えば
  Phase 1/2 の設計を見直す必要が出る）ため、着手時は Phase 3 を先に回す
- **Phase 1 / Phase 5** は現行の deny 経路の完全性に直接影響するため、
  次に優先度が高い
- **Phase 4 / Phase 7** はドキュメント + 1ファイル修正で完了できるので、
  スキマ時間で潰せる
- **Phase 2 / Phase 6** は攻撃コスト vs 実装コストの比が微妙 —— 実運用中の
  攻撃観測（audit ログ）で兆候が出てから着手してよい

## 完了条件

- 上記 Phase のうち、着手した Phase が全て `go test ./...` と統合テストを通過する
- Phase 3 の仕様検証結果が docs に記録され、他 Phase の設計判断に反映されている
- Phase 1 の実装案が採択される場合、その脅威モデル差分が `docs/reference/approve.md`
  に明記される

## 参照

- Plan 0022（ask_strategy と deny-first sentinel）: 本 Plan の親
- Plan 0025（for ループの部分解析）: 承認ストアの canonicalCommand 経路に影響
- レビュー由来項目の抽出元コミット: `Plan 0022/0025 主題内修正` のペルソナ
  並列レビュー（skeptic ×2 + security + attacker）

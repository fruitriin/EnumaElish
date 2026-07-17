# Changelog

このプロジェクトの特筆すべき変更を記録する。
形式は [Keep a Changelog](https://keepachangelog.com/ja/1.1.0/) に、バージョニングは [Semantic Versioning](https://semver.org/lang/ja/) に従う。

v1.0.0 までは 0.x 帯で破壊的変更を許容する（後方互換の保証は v1.0.0 から）。

## [Unreleased]

## [0.2.1] - 2026-07-17

Issue [#15](https://github.com/fruitriin/EnumaElish/issues/15) 対応。v0.2.0 の Phase 2（非対話モードでの `ask` 降格）と `fallback: ask` の組み合わせが、明示ルール未カバーのコマンドを auto/dontAsk モードで広域 deny 化する問題への緩和策。仕様変更ではなく、事前警告と default preset 拡張。

### 追加

- `ccchain check` に警告追加: `settings.fallback: ask` かつ `settings.ask_strategy: degrade`（既定）の場合、auto/dontAsk モードで意図しない広域 deny が起きる旨と対処法（`ask_strategy: passthrough` / allow ルール追加）を stderr に表示
- `ccchain init` の default preset に read-only ユーティリティの allow を追加: `uniq`, `sed`, `awk`, `cut`, `tr`, `tee`, `basename`, `dirname`, `date`, `env`, `printf`, `seq`, `test`, `file`, `stat`, `tree`, `readlink`

### マイグレーションガイド（v0.1.x → v0.2.x）

`settings: fallback: ask` を使っている既存 conf は、v0.2.0 以降の auto / dontAsk モードで明示ルール
未カバーのコマンドが実質 deny になる（Phase 2 仕様どおり）。以下のいずれかで回避:

1. **旧挙動維持（推奨せず）**: `settings: ask_strategy: passthrough` を追加 → ask が降格されず v0.1 相当の挙動に戻る
2. **read-only ユーティリティを allow に列挙**: v0.2.1 の default preset を参考に、自身の conf に `allow sed`, `allow awk`, `allow cut` などを追記
3. **fallback: allow に切替**: 未カバーコマンドを許可し、危険なものは deny/ask で個別に列挙する反転設計

いずれの選択でも、`ccchain check` 実行時の警告メッセージに従うのが最短。

[0.2.1]: https://github.com/fruitriin/EnumaElish/releases/tag/v0.2.1

## [0.2.0] - 2026-07-17

Plan 0022（auto モード時代の deny-first 安全網）と Plan 0025（リテラル for ループの部分解析）を投入。
hook 出力形式の刷新を含む破壊的変更あり。詳細は [`docs/reference/approve.md`](docs/reference/approve.md)・[`docs/reference/sentinel.md`](docs/reference/sentinel.md)・[`docs/reference/dsl.md`](docs/reference/dsl.md) を参照。

### 破壊的変更

- **PreToolUse hook の出力形式を現代化**（Plan 0022 Phase 1）: 全アクションを exit 0 + `hookSpecificOutput.permissionDecision` JSON（allow / deny / ask）に統一。deny の旧形式（exit code 2 + stderr）は廃止。Claude Code は両形式をサポートするが混在は不可のため、新 JSON 形式のみを出力する
- **非対話モードで `ask` が既定で deny に降格**（Plan 0022 Phase 2）: `permission_mode` が auto / dontAsk / 未知の環境では ask ダイアログが人間に届かないため、`ask` 評価結果は deny + 承認手順ヒントに降格する（安全側への変更）。旧挙動に戻すには `settings: ask_strategy: passthrough` を指定する
- **リテラル `for` ループの部分展開が既定に**（Plan 0025）: `for f in a b c; do BODY; done` のようにリテラル語リストで書かれた `for` ループを各イテレーションに展開して評価するようになった。従来「解析不能」として扱われていたコマンドの一部が具体的なルール判定を通るようになる。展開できないループ（動的語リスト・C 系 for・select 等）は `unanalyzable_action` の対象で、既定 `deny`。旧挙動（すべての for を解析不能扱い）に戻す手段は用意しない

### 追加

- hook 入力から `permission_mode` / `session_id` / `cwd` を読取（Claude Code hooks 仕様準拠）
- `settings: ask_strategy:`（degrade | passthrough | deny-all、既定 degrade）と `ask_degrade_default:`（deny | allow、既定 deny）
- ask ルールの子ブロック `unattended: deny|allow` — 非対話時にルール単位で降格方向を指定
- `hint` アクションの hook 出力を実装（permissionDecision allow + reason。従来は出力されず無言で通過していた）
- **承認トークン**（Plan 0022 Phase 3、`ccchain approve`）: 降格 deny をオーナーが自身のターミナルで承認するとワンショットで通せる仕組み。`--last` / `--list` / `<hash-prefix>` / `--revoke-all` / `--ttl <duration>` / `--global` フラグ。既定 TTL 15 分、スコープは session_id+cwd。動的コマンド（`$VAR` / `$(...)` / バッククォート）は承認対象外。hint にトークンは埋め込まない（自己承認防止）。ストアは `$CLAUDE_CONFIG_DIR/ccchain/` または `~/.claude/ccchain/`、パーミッション 0600、`O_EXCL` ロック
- **sentinel プリセット**（Plan 0022 Phase 4、`ccchain init --sentinel`）: auto / dontAsk / headless で classifier が拾えない構造パターンを収録した deny-first キュレートルール。curl|<shell>・find -exec rm / -delete・rm 保護パス（`/`, `~`, `$HOME`, `.git`）・git force-push / branch -D / reset --hard / clean -fd / filter-branch / config editor 系・chmod/chown 広域再帰・eval / source <(...)・dd / mkfs / diskutil eraseDisk 系・`ccchain approve` 自己承認フェンス等を収録。正本は `internal/preset/sentinel.conf`
- `settings: unanalyzable_action:`（ask | deny、既定 deny、Plan 0025 Phase 2）: 構造的に解析不能なコマンド（`eval` / 動的サブシェル / 展開できない for / C 系 for / select / 動的制御構造等）のアクションを設定可能に。`allow` は意図的に非対応（安全網を無効化できてしまうため）
- ドキュメント刷新（Plan 0022 Phase 5）: README を「auto / bypass 時代の deny-first 安全網」ポジショニングに書き直し、Mode × Decision 互換表と sentinel クイックスタートを追加。承認トークン・sentinel の専用リファレンスページを追加。DSL リファレンスに `ask_strategy` / `ask_degrade_default` / `unanalyzable_action` / `unattended:` の節を追加

### 変更

- deny/warn メッセージのサニタイズ切詰めを 200 → 600 バイトに拡大（承認手順の埋め込み対応）、UTF-8 境界セーフ化

[0.2.0]: https://github.com/fruitriin/EnumaElish/releases/tag/v0.2.0

## [0.1.0] - 2026-07-06

初回リリース。

### 追加

- 独自テキスト DSL（インデントベース）によるルール定義・テンプレート・args 正規表現ルール
- シェルコマンドの構造的コンテキスト解析（パイプ・チェーン・サブシェル、`mvdan.cc/sh` ベース）
- PreToolUse / PostToolUse hook 統合（allow / deny / warn / ask / hint）
- ワークスペーススコープアクセス制御（read/write 分離・シンボリックリンク解決・Bash 引数スコープ）
- deny メッセージテンプレート・deny リダイレクト・マルチツール制御（Read/Edit）
- コマンドセマンティクステーブルとプロジェクト種別自動検出（`detect` / `generate-rules`）
- CLI: `check` / `eval` / `test` / `diff` / `suggest` / `audit` / `init` / `hook` / `version`
  - `diff` は2つの設定を同一コマンド集合で評価して差分表示（`--changed-only` / `--exit-on-change`、CI 向け exit code 2=CHANGED / 3=評価エラー）
- CI パイプライン（Go テスト + ドキュメントビルド）
- インストール: `go install github.com/fruitriin/EnumaElish/cmd/ccchain@v0.1.0`（コマンド名は `ccchain`）

[0.1.0]: https://github.com/fruitriin/EnumaElish/releases/tag/v0.1.0

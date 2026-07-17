# Changelog

このプロジェクトの特筆すべき変更を記録する。
形式は [Keep a Changelog](https://keepachangelog.com/ja/1.1.0/) に、バージョニングは [Semantic Versioning](https://semver.org/lang/ja/) に従う。

v1.0.0 までは 0.x 帯で破壊的変更を許容する（後方互換の保証は v1.0.0 から）。

## [Unreleased]

### 破壊的変更

- **PreToolUse hook の出力形式を現代化**（Plan 0022 Phase 1）: 全アクションを exit 0 + `hookSpecificOutput.permissionDecision` JSON（allow / deny / ask）に統一。deny の旧形式（exit code 2 + stderr）は廃止。Claude Code は両形式をサポートするが混在は不可のため、新 JSON 形式のみを出力する
- **非対話モードで `ask` が既定で deny に降格**（Plan 0022 Phase 2）: `permission_mode` が auto / dontAsk / 未知の環境では ask ダイアログが人間に届かないため、`ask` 評価結果は deny + 承認手順ヒントに降格する（安全側への変更）。旧挙動に戻すには `settings: ask_strategy: passthrough` を指定する

### 追加

- hook 入力から `permission_mode` / `session_id` / `cwd` を読取（Claude Code hooks 仕様準拠）
- `settings: ask_strategy:`（degrade | passthrough | deny-all、既定 degrade）と `ask_degrade_default:`（deny | allow、既定 deny）
- ask ルールの子ブロック `unattended: deny|allow` — 非対話時にルール単位で降格方向を指定
- `hint` アクションの hook 出力を実装（permissionDecision allow + reason。従来は出力されず無言で通過していた）

### 変更

- deny/warn メッセージのサニタイズ切詰めを 200 → 600 バイトに拡大（承認手順の埋め込み対応）、UTF-8 境界セーフ化

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

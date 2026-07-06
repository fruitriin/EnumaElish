# Changelog

このプロジェクトの特筆すべき変更を記録する。
形式は [Keep a Changelog](https://keepachangelog.com/ja/1.1.0/) に、バージョニングは [Semantic Versioning](https://semver.org/lang/ja/) に従う。

v1.0.0 までは 0.x 帯で破壊的変更を許容する（後方互換の保証は v1.0.0 から）。

## [Unreleased]

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

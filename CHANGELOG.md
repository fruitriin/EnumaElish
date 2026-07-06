# Changelog

このプロジェクトの特筆すべき変更を記録する。
形式は [Keep a Changelog](https://keepachangelog.com/ja/1.1.0/) に、バージョニングは [Semantic Versioning](https://semver.org/lang/ja/) に従う。

v1.0.0 までは 0.x 帯で破壊的変更を許容する（後方互換の保証は v1.0.0 から）。

## [Unreleased]

### 初回リリース（v0.1.0）に向けた収録予定

- 独自テキスト DSL（インデントベース）によるルール定義・テンプレート・args 正規表現ルール
- シェルコマンドの構造的コンテキスト解析（パイプ・チェーン・サブシェル、`mvdan.cc/sh` ベース）
- PreToolUse / PostToolUse hook 統合（allow / deny / warn / ask / hint）
- ワークスペーススコープアクセス制御（read/write 分離・シンボリックリンク解決・Bash 引数スコープ）
- deny メッセージテンプレート・deny リダイレクト・マルチツール制御（Read/Edit）
- コマンドセマンティクステーブルとプロジェクト種別自動検出（`detect` / `generate-rules`）
- CLI: `check` / `eval` / `test` / `suggest` / `audit` / `init` / `hook`
- CI パイプライン（Go テスト + ドキュメントビルド）

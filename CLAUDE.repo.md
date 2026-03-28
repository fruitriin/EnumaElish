# CLAUDE.repo.md

EnumaElish — ccchain (Claude Code Chain)

Claude Code の標準 permission system を拡張し、シェルコマンドの構造的コンテキスト（パイプ、チェーン、サブシェル）を考慮した許可/拒否制御を行う Go 製シングルバイナリツール。

## 技術スタック

- Go
- `mvdan.cc/sh`（シェルパーサー、唯一の外部依存）
- 独自テキスト DSL（インデントベース）
- シェルパースモード: bash（`syntax.LangBash`）
- `args:` パターン: regex
- バージョニング: セマンティックバージョニング

## 設定ファイル探索パス（優先度順）

1. `.ccchain.conf`（プロジェクトルート）
2. `.ccchain.local.conf`（ローカル上書き、gitignore 対象）
3. `$CLAUDE_CONFIG_DIR/ccchain.conf`
4. `~/.claude/ccchain.conf`（`CLAUDE_CONFIG_DIR` 未設定時のフォールバック）

# プロジェクト種別

このリポジトリは **ADDF 利用プロジェクト** です。

# 哲学

AutomatonDevDrive由来のソースコードは .claude 配下に収めてください。
AutomatonDevDrive由来のスキルは addf- プレフィックスを持ちます
プロジェクトルート配下にADD由来のファイルをなるべく置かないべきです。

---

## コミットログ規約

日本語で書く。形式:

```
[領域] 変更内容の要約

詳細説明（必要な場合）
```

---

## テスト

プロジェクト固有テスト:

```bash
go test ./...
go vet ./...
go build ./cmd/ccchain
```

ADD フレームワークテスト:

```bash
bash .claude/tests/run-all.sh
```

品質ゲートの Stage 1 で上記の全コマンドを実行してください。

---

## 知見蓄積（ノウハウ）

タスク完了時に `/addf-knowhow` で実装知見を記録する。以下の観点で振り返る:

- **コーディング**: Go の実装パターン、`mvdan.cc/sh` の使い方、DSL パーサーのハマりポイント、ベンチマーク知見
- **品質ゲート**: コードレビュー・セキュリティレビューエージェントから得た指摘パターン、統合テスト戦略
- **タスク総括**: 計画と実装の乖離、セキュリティレビューの優先度変更（例: Plan 0014 を 0011 より先に実装した判断）

既存ノウハウは `docs/knowhow/INDEX.md` で一覧確認できる。

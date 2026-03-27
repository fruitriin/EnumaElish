# Contributing to ccchain

ccchain へのコントリビューションを歓迎します。

## 開発環境

```bash
# Go 1.25 以上が必要
go version

# ビルド
go build ./cmd/ccchain

# テスト
go test ./...

# Lint
go vet ./...

# ベンチマーク
go test -bench=. ./...
```

## プロジェクト構造

```
cmd/ccchain/          CLI エントリポイント
internal/
  dsl/                DSL レキサー・パーサー・テンプレート解決
  shell/              シェル AST 解析・トポロジー構築
  eval/               ルール評価エンジン
  audit/              監査出力
docs/                 VitePress ドキュメント
```

## コミットメッセージ

日本語で、以下の形式に従ってください:

```
[領域] 変更内容の要約

詳細説明（必要な場合）
```

例:
- `[修正] eval のパイプコンテキスト判定漏れを修正`
- `[機能] warn アクションを追加`
- `[ドキュメント] DSL リファレンスにテンプレート例を追加`

## プルリクエスト

1. フォークしてブランチを作成
2. `go test ./...` と `go vet ./...` が通ることを確認
3. 変更に応じてテストを追加・更新
4. PR を作成

## 外部依存について

ccchain は意図的に外部依存を `mvdan.cc/sh` のみに抑えています。新しい外部依存を追加する場合は、事前に Issue で議論してください。

## ライセンス

コントリビューションは [MIT License](LICENSE) のもとで提供されます。

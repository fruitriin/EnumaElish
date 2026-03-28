# Plan 0020: CI パイプライン（Go テスト + ドキュメントビルド）

## 背景

現在の GitHub Actions は docs デプロイのみ。PR ごとに Go テスト + vet + ビルドを実行する CI がない。

## 実装

`.github/workflows/ci.yml`:
- Go テスト: `go test ./...`
- Go vet: `go vet ./...`
- Go ビルド: `go build ./cmd/ccchain`
- ドキュメントビルド: `npm run docs:build`
- 統合テスト: `go test ./internal/eval/ -run TestIntegration`
- フィクスチャテスト: `go test ./internal/eval/ -run TestFixture`

トリガー: push to main, pull_request

## 実装量: 小

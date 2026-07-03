# Plan 0023: ADDF 統合 — /addf-init での ccchain hook 自動設営

## 実装状況: 未着手（スタブ — Plan 0022 完了後に詳細化）

## 背景

Plan 0022 で ccchain が「auto モード時代の deny-first 安全網」として再定義される。ADDF 利用プロジェクトが `/addf-init` 時に ccchain hook を選択的に設営できれば、フレームワーク導入と同時に安全網が張られる。

## スコープ（詳細化時に確定）

- `/addf-init` のオプションフェーズとして ccchain の導入を提案（バイナリ検出 → settings.json への PreToolUse hook 追記 → `ccchain init --sentinel` 実行）
- settings.json マージロジックとの整合（addf-migrate の settings マージと衝突しないこと）
- ADDF アップストリームへのコントリビューション経路の検討（EnumaElish はダウンストリームであるため、スキル変更は upstream に提案する）

## 依存

- Plan 0022（sentinel プリセット・hook I/O 現代化）の完了

# Plan 0024: Slack/リモート承認連携

## 実装状況: 未着手（スタブ — Plan 0022 完了後に詳細化）

## 背景

Plan 0022 の承認トークン（`ccchain approve`）はローカルターミナルでの人間承認を前提とする。unattended 自走や headless 運用では、オーナーが手元の端末にいないため、リモートから pending を確認・承認できる経路が欲しい。

## スコープ（詳細化時に確定）

- pending 一覧の外部通知（Slack/Discord webhook 等。シングルバイナリ原則との整合を検討 — 通知は外部コマンド委譲か組み込み最小 HTTP か）
- リモート承認の認証設計（承認権限の所在、トークンの伝送経路の安全性）
- ADDF の unattended モード（`/addf-mode unattended --notify`）の uncertainty_notify との連携可能性

## 依存

- Plan 0022 Phase 3（承認トークンのストア設計）の完了

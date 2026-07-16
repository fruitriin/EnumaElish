---
title: VitePress は生の山括弧プレースホルダでビルドが落ちる
created: 2026-07-06
last_verified: 2026-07-06
depends_on:
  - file: docs/.vitepress/config.mts
status: active
---

# VitePress は生の山括弧プレースホルダでビルドが落ちる

## 発見した知見

- VitePress（Vue コンパイラ）は markdown 中の `<番号>` `<concept>` `<phase>` のような**生の山括弧プレースホルダを未クローズ HTML タグとして解釈**し、`Element is missing end tag` でビルド全体が失敗する
- タグ扱いされるのは `<` の直後が ASCII 英字で始まる場合（`<model名>` のような英字開始+日本語混在も該当）。コードフェンス内・コードスパン内は安全
- **エラーの行番号は実際の原因行とずれることがある**（Vue パーサが混乱した位置を報告するため）。エラー行を目視するより、コードスパン外の `<[A-Za-z]` を機械的に走査する方が早い:

```python
# コードフェンス・コードスパン外の <tag> 候補を全 md から抽出する要領
# フェンス状態をトグルし、行内は `<` より前のバッククォート数の偶奇でスパン内外を判定
```

- ビルドは**最初のエラーで停止する**ため、1件直して再ビルドを繰り返すと遅い。一括走査 → 全件修正 → 1回ビルドが正解

## プロジェクトへの適用

- 修正は2択: コードスパンで囲む（`` `speculative/<concept>` ``）か、HTML エンティティ（`&lt;番号&gt;`）にエスケープする。文中の書式仕様の説明など「コードでない」文脈ではエンティティを使う
- CI（`.github/workflows/ci.yml` の docs job、Plan 0020）が PR ごとに `npm run docs:build` を回すため、今後は混入時に PR で検出される
- ADDF マイグレーションで .claude/addf/plans-add/ や .claude/addf/project-overview/ に上流由来の md が入るとき、この形のプレースホルダが混入しやすい（2026-07-06 に5ファイル7箇所を検出。上流にも同じ問題がある可能性 — コントリビューション候補）

## 注意点・制約

- ローカルの `npm run docs:build` が通ることと deploy-docs（GitHub Pages）が通ることは同値。main へのドキュメント push 前にローカルビルドで検証すれば十分
- `docs/.vitepress/config.mts` の `srcExclude` でディレクトリごと除外する回避策もあるが、公開したいドキュメントには使えない — 原因箇所のエスケープが正攻法

## 参照

- 実例修正: PR #12（feature/0020-ci-pipeline のコミット 4365323）
- [doc-drift-pattern.md](doc-drift-pattern.md) — ドキュメントと実装の乖離パターン（本件は「ビルドを壊す書式」で別系統だが、docs 品質ゲートという点で関連）

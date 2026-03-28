# savanna-smell-detector Go プロジェクト導入知見

## 知見

### 導入手順

1. `cargo install --git https://github.com/fruitriin/savanna-smell-detector` でインストール
2. `.savanna.toml` をプロジェクトルートに作成（`language = "go"`, `fail-on-smell = false`）
3. Makefile に `smell` / `smell-report` ターゲット追加
4. `.gitignore` に `smell-report.md` 追加

### Go イディオムとの相性（v0.3.0 + 5caf079）

初回実行で 112 件検出 → issue でフィードバック → 修正対応 → 8 件に。さらにテスト修正で 0 件達成。

対応された Go 固有の改善:
- `if err != nil { t.Fatal }` を Conditional Test Logic から除外
- `if <任意条件> { t.Fatal/t.Error 系 }` も除外（汎用化）
- `assertEqual` 等のカスタムヘルパーをアサーション認識
- テーブル定義（構造体スライスリテラル）を Giant Test の行数から除外
- `t.Log` / `t.Logf` を Redundant Print から除外
- `Benchmark*` 関数を Missing Assertion から除外

### テストスメル修正パターン

| スメル | 修正手法 |
|---|---|
| Conditional Test Logic（for+if） | ヘルパー関数に抽出（`findAuditLine`, `countTokenType`） |
| Giant Test | サブテスト分割 or ヘルパー抽出 |
| Silent Skip（`return`） | `t.Skip("reason")` に変更 |
| Missing Assertion | アサーション追加 |
| 意図的なスメル | `// smell-allow: <type> — <reason>` コメント |

### フィードバックループの有効性

savanna-smell-detector は自分のプロジェクトなので issue → 修正 → 再テスト のサイクルが速い。外部ツールでも同様に、実プロジェクトでの検出結果レポートを issue として起票するパターンは有効。

# フィクスチャベーステスト設計

## 知見

### commands.txt × rules-*.conf の組み合わせテスト

統合テストのコマンドリストをテストコード内にハードコードするのではなく、外部フィクスチャに分離することで:

- **コマンド追加**: `testdata/eval/commands.txt` に1行追加するだけ
- **ルールセット追加**: `testdata/eval/rules-*.conf` にファイルを追加するだけ
- **組み合わせ爆発**: テストコード側は N×M の全組み合わせを自動実行

```
testdata/eval/
  commands.txt            # 132+ コマンド
  rules-default.conf      # デフォルトルールセット
  rules-strict.conf       # 厳格（fallback: deny）
  rules-permissive.conf   # 寛容（fallback: allow）
```

### 3種類のフィクスチャテスト

| テスト | 目的 |
|---|---|
| `TestFixtureCombination` | 全コマンド × 全ルールでパニック/エラーなし保証 |
| `TestFixtureDangerousNeverAllow` | 危険コマンドが全ルールセットで allow にならない |
| `TestFixtureCompareRulesets` | ルールセット間の判定差分レポート |

### ccchain test サブコマンドとの連携

`ccchain test` はフィクスチャテストの CLI 版。ユーザーが自分の conf + コマンドリストで試行錯誤できる:

```bash
ccchain test commands.txt                        # デフォルト conf
ccchain test --config ~/my-rules.conf cmds.txt   # 任意の conf
cat commands.txt | ccchain test --config rules.conf  # stdin
```

### テスト駆動ルール調整ワークフロー

1. セッションログからコマンドを収集（方法は下記参照）
2. `ccchain test commands.txt` で一括評価
3. ask が多すぎるコマンドに allow ルールを追加
4. deny すべきコマンドが allow になっていないか確認
5. 繰り返し → デフォルト 167 コマンドで allow=147, ask=8 に最適化できた


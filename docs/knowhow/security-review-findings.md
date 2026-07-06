# セキュリティレビューで発見された脆弱性パターン

## 知見

### 相対パスのスコープ判定（scope.go）

`file.txt` のような純粋な相対パスは CWD に関わらず `ScopeInside` に分類されていた。AI が `cd /tmp` した後に `Read file.txt` すると、ワークスペース外のファイルが inside 扱いになる問題。

**対策**: `CLAUDE_PROJECT_DIR` 環境変数を参照し、相対パスを絶対パスに変換してからスコープチェック。取得できなければ保守的に outside 扱い。

### スコープ外が ask 止まり（tool.go）

ワークスペース外パスへのアクセスは `allow` → `ask` にエスカレーションされるが、`deny` にはならない。ユーザーが承認すれば `.ssh/` も読める。デフォルトルールの `args:` で `.ssh/` と `.env` を deny しているため実害は限定的だが、ユーザーが独自ルールで `args:` 保護を省略すると穴になる。

**設計判断**: スコープは「確認（ask）」であり「禁止（deny）」ではない。ドキュメントでこの挙動を明記すべき。`scope_violation: ask|deny` の設定化も検討候補。

### MCP ツールの引数がスコープ未検査（hook.go）

MCP ツールの hook 分岐では `toolArg` を空文字列で渡していた。`mcp__filesystem__read_file` のようなツールがワークスペーススコープをすり抜ける。

**対策**: JSON input から `file_path`, `path`, `url` キーを best-effort で抽出する `extractMCPArg` 関数を追加。一般化は困難なので best-effort + ドキュメント明記。

### semantics テーブルの複合サブコマンド regex（table.go）

`DangerousSubcommands` に `"system prune"` のようなスペース含みの文字列があると、regex の `\b` 境界がスペース前で成立してしまい、意図したパターンにマッチしない。

**対策**: `normalizeSubcommands` 関数でスペースを `\s+` に正規化してから regex パターンに結合。

### git config の保護パターンが不完全

`editor|pager|hook` だけでなく、`core.fsmonitor`（任意スクリプト実行）、`filter.*.clean/smudge`、`diff.external`、`credential.helper` もコード実行に使える。`--global` の一律 deny も検討。


### CLI 引数パーサの未知フラグ透過による fail-open（main.go、2026-07-06）

`ccchain diff` 追加時に「サブコマンド確定後の未知フラグは cmdArgs へ無条件透過」とした変更が、cmdArgs を参照しない既存サブコマンド全てで**タイポの無音無視**を生んだ。`hook pre --defualt-action deny`（タイポ）が exit 0 で通り、フェイルクローズ設定が黙って無効化される。`eval --typo "rm -rf /"` は positional がずれて `--typo` の方を評価し、未評価のまま偽の安全シグナル（JSON）を返す。

**対策**: 透過は「自前のフラグパーサを持つサブコマンド」のホワイトリスト（`flagPassthroughCommands`、現状 `diff` のみ）に限定し、それ以外は従来どおり `error: unknown flag` で exit 1。パース処理を `parseCLIArgs()` に分離して回帰テストを常備する。

**教訓**: セキュリティツールの CLI では「エラーで大声で失敗する」経路を緩める変更は、対象サブコマンドに明示的にスコープすること。1箇所のグローバルなパーサ変更が全サブコマンドの引数契約を暗黙に変える。恒久対策として `eval`/`test`/`suggest` にも共通のサブコマンド専用フラグ検証ヘルパーを導入する案が別 Plan 候補。

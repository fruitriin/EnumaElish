# 承認トークン（`ccchain approve`）

承認トークンフローは、ccchain が [ask → deny 降格経路](./dsl.md#ask_strategy) で
ブロックしたコマンドを、人間のオーナーが別チャネルで承認して通せる仕組みです。
「auto モードで agent がブロックされて詰んだ」を「非同期の対話」に変換します。

## 脅威モデル（この設計の要）

ccchain が `permissionDecisionReason` として返す hint テキストは **agent の
コンテキストに入る**。**hint にトークンそのものを埋めると agent が自己承認
できてしまう**。したがってフローは:

- **hint にはトークンを載せない** — 手順（`ccchain approve --last`）のみ書く。
  実行できるのは Claude Code 外のシェルにいるオーナーだけ
- **保留リクエストをサーバー側に記録**（`~/.claude/ccchain/pending.jsonl`）
  し、オーナーが agent の依頼内容を確認できる
- **agent の Bash ツールから `ccchain approve` を締め出す** — [sentinel
  プリセット](./sentinel.md) が `allow ccchain / args: ^approve\b: deny`
  で防ぐ。二重防御として `settings.json` の `permissions.deny` にも
  `Bash(ccchain approve*)` を追加すること

## フロー

```
┌─────────────────────────────────┐
│ agent が Bash コマンドを実行     │
└──────────────┬──────────────────┘
               │
               ▼
      ┌────────────────┐    対話モード                ┌────────────────┐
      │ evaluate → ask │ ──────────────────────────► │ ask ダイアログ  │
      └───────┬────────┘                             │ (人間がクリック)│
              │ 非対話モード                          └────────────────┘
              │ (auto / dontAsk / …)
              ▼
   ┌──────────────────────────────┐
   │ ask が deny + hint に降格     │
   │ + pending.jsonl にエントリ追記│
   └──────────────┬───────────────┘
                  │
                  ▼
   agent は「`ccchain approve --last` を実行して」を見る
                  │
                  ▼
   オーナー (Claude Code 外のシェル):
     $ ccchain approve --last
   → approved.jsonl に書込 (TTL, スコープ付き)
                  │
                  ▼
   agent が同じコマンドを再実行
                  │
                  ▼
   ┌──────────────────────────────┐
   │ hook が承認一致で allow を返す │
   │ (ワンショット消費)             │
   └──────────────────────────────┘
```

## CLI

```bash
ccchain approve --last              # 直近の pending を承認
ccchain approve --list              # pending 一覧を表示（# / hash / age / command）
ccchain approve <hash-prefix>       # ハッシュ prefix 指定で承認（4 文字以上）
ccchain approve --revoke-all        # 未消費の承認をすべて破棄
```

フラグ:

| フラグ | 説明 |
|---|---|
| `--ttl <duration>` | 承認の有効期限。デフォルト `15m`。Go の duration 形式（`30s`, `1h`, ...） |
| `--global` | セッション / cwd を問わずマッチさせる。デフォルトは session+cwd 限定 |
| `-h, --help` | 使い方を表示 |

セッション例:

```bash
# 1. agent がコマンドを試み、ccchain が pending を記録
$ ccchain approve --list
#    HASH              AGE       COMMAND
1    a1b2c3d4e5f6      3s        git push origin main
```

```bash
# 2. オーナーが自身のターミナルで承認
$ ccchain approve --last --ttl 1h
approved: a1b2c3d4e5f6
  command: git push origin main
  scope:   session
  cwd:     /home/user/project
  session: 01HXYZ...
  ttl:     1h0m0s
```

同じセッション + cwd から TTL 内に agent が同じコマンドを実行すると、
ccchain は `permissionDecision: "allow"` を返し、pending エントリが消費されます
（ワンショット）。

## マッチング規則

- **正規化**: パース済みシェル AST を `mvdan.cc/sh` の printer で再出力してから
  SHA-256 でハッシュ化。空白やクォートの揺れ（`ls -la` / `ls  -la` /
  `ls "-la"`）は同じハッシュに畳まれる
- **スコープ（デフォルト: session）**: deny が起きたときに記録された `session_id`
  と `cwd` の両方が一致する必要がある。`--global` で「セッション・ディレクトリ
  問わず」に広がる
- **TTL（デフォルト: 15m）**: `ccchain approve` 実行時点から TTL 経過で失効。
  失効した承認は消費されない
- **ワンショット**: hook 呼び出し 1 回でアトミックに消費 — 1 つの承認は 1 回の
  再試行だけをカバーする

## 動的コマンドは承認対象外

`$VAR` / `$(...)` / バッククォートを含むコマンドは**承認対象になりません**。
展開が実行時状態に依存するため、ハッシュで「同じコマンド」を保証できないから
です。降格経路で動的コマンドを検出すると、ccchain は deny メッセージにその
理由を追記し、リテラルな引数への書き直し（または対話セッションでの実行）を
促します。

## ストレージ

- ディレクトリ: `CLAUDE_CONFIG_DIR` が設定されていれば
  `$CLAUDE_CONFIG_DIR/ccchain/`、そうでなければ `~/.claude/ccchain/`
- ファイル: `pending.jsonl` と `approved.jsonl`（追記のみ）
- パーミッション: `0600`（所有者のみ read/write）
- ロック: `O_EXCL` によるロックファイル方式で並行アクセスを安全化 — 外部依存
  なし
- 監査: 承認・消費は ccchain の audit ログにも記録される
  （[`ccchain audit`](../guide/cli.md#ccchain-audit) 参照）

## セキュリティ上の注意

- **agent のシェルから `ccchain approve` を実行させない**。sentinel プリセット
  は deny するが、それは sentinel プリセットを使っている場合の話。二重防御
  として `settings.json` の `permissions.deny` にも `Bash(ccchain approve*)`
  を追加すること
- 承認はマシンごとにローカル。ネットワーク経路は存在しない
- `--revoke-all` は非常停止ボタン — 未消費の承認をすべて無効化して、フローを
  最初からやり直せる

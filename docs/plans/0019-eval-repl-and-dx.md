# Plan 0019: 開発体験の改善

## 機能

### ccchain eval REPL モード
引数なしで `ccchain eval` を起動するとインタラクティブモード:
```
$ ccchain eval
ccchain> find . | rm
[deny] "don't pipe into destructive commands"
ccchain> ls -la | head
[allow]
ccchain> exit
```

### ccchain diff
2つの .conf ファイルの判定差分を表示:
```
$ ccchain diff rules-v1.conf rules-v2.conf --commands commands.txt
find . -delete    v1=[allow]  v2=[deny]   CHANGED
git push          v1=[ask]    v2=[ask]    same
...
```

### ccchain stats（将来）
hook 呼び出しログから統計を集計:
```
$ ccchain stats
Last 24h: 342 calls — allow=280, ask=45, deny=17
Top denied: curl|bash (8), find|rm (5), eval (4)
```

## 実装量: 小〜中

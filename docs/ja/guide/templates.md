# テンプレート

テンプレートで再利用可能なルールセットを定義し、複数のコマンドで共有できます。

## 定義

```
template primitive
  |,>>
    allow cat, echo, head, tail, wc, sort, uniq
```

## next: による委譲

```
allow ls
  next: primitive

allow find
  next: primitive
```

## extends: による継承

```
template bulkExec
  extends: safeRead
  |,>>
    deny rm  "破壊的コマンドにパイプしないでください"
```

## 継承チェーン

```
primitive → safeRead → bulkExec
```

`bulkExec` を使うコマンドは、primitive と safeRead のルールも継承します。

詳細は[英語版](/guide/templates)を参照してください。

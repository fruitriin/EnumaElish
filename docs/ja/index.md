---
layout: home

hero:
  name: ccchain
  text: Claude Code Chain
  tagline: Claude Code のための構造的パーミッション制御
  actions:
    - theme: brand
      text: はじめる
      link: /ja/guide/
    - theme: alt
      text: GitHub
      link: https://github.com/fruitriin/EnumaElish

features:
  - title: 構造的コンテキスト
    details: パイプ・リダイレクト・サブシェル内のコマンドをネストとして追跡。先頭の単語だけでなく構造を見る。
  - title: リセットセマンティクス
    details: && や ; で区切られたコマンドは独立に評価。シェルの実行セマンティクスに一致。
  - title: テンプレート・継承
    details: find, xargs, grep に共通するルールをテンプレートで共有。extends と next で再利用。
  - title: 監査可能
    details: 全ルールのフラット展開で「何が通って何が止まるか」を可視化。
  - title: AI 誘導付き deny
    details: ブロック理由を Claude に伝え、自律的な書き直しを可能にする。
  - title: シングルバイナリ
    details: Go 製。外部依存は mvdan.cc/sh のみ。ランタイム不要。
---

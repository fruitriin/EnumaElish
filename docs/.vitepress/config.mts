import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'ccchain',
  description: 'Claude Code Chain: Structural Permission Control',
  base: '/EnumaElish/',
  ignoreDeadLinks: true,

  head: [
    ['meta', { name: 'theme-color', content: '#5f67ee' }],
  ],

  themeConfig: {
    nav: [
      { text: 'Guide', link: '/guide/' },
      { text: 'DSL Reference', link: '/reference/dsl' },
      { text: 'GitHub', link: 'https://github.com/fruitriin/EnumaElish' },
    ],

    sidebar: {
      '/guide/': [
        {
          text: 'Getting Started',
          items: [
            { text: 'What is ccchain?', link: '/guide/' },
            { text: 'Installation', link: '/guide/installation' },
            { text: 'Quick Start', link: '/guide/quickstart' },
          ],
        },
        {
          text: 'Concepts',
          items: [
            { text: 'How It Works', link: '/guide/how-it-works' },
            { text: 'Structural Context', link: '/guide/structural-context' },
            { text: 'Templates', link: '/guide/templates' },
          ],
        },
        {
          text: 'Usage',
          items: [
            { text: 'CLI Commands', link: '/guide/cli' },
            { text: 'Default Rules', link: '/guide/default-rules' },
            { text: 'Customization', link: '/guide/customization' },
          ],
        },
      ],
      '/reference/': [
        {
          text: 'Reference',
          items: [
            { text: 'DSL Syntax', link: '/reference/dsl' },
            { text: 'Actions', link: '/reference/actions' },
            { text: 'Config Files', link: '/reference/config' },
          ],
        },
      ],
    },

    socialLinks: [
      { icon: 'github', link: 'https://github.com/fruitriin/EnumaElish' },
    ],

    footer: {
      message: 'Released under the MIT License.',
    },

    search: {
      provider: 'local',
    },
  },

  locales: {
    root: {
      label: 'English',
      lang: 'en',
    },
    ja: {
      label: '日本語',
      lang: 'ja',
      themeConfig: {
        nav: [
          { text: 'ガイド', link: '/ja/guide/' },
          { text: 'DSL リファレンス', link: '/ja/reference/dsl' },
        ],
        sidebar: {
          '/ja/guide/': [
            {
              text: 'はじめに',
              items: [
                { text: 'ccchain とは', link: '/ja/guide/' },
                { text: 'インストール', link: '/ja/guide/installation' },
                { text: 'クイックスタート', link: '/ja/guide/quickstart' },
              ],
            },
            {
              text: 'コンセプト',
              items: [
                { text: '仕組み', link: '/ja/guide/how-it-works' },
                { text: '構造的コンテキスト', link: '/ja/guide/structural-context' },
                { text: 'テンプレート', link: '/ja/guide/templates' },
              ],
            },
            {
              text: '使い方',
              items: [
                { text: 'CLI コマンド', link: '/ja/guide/cli' },
                { text: 'デフォルトルール', link: '/ja/guide/default-rules' },
                { text: 'カスタマイズ', link: '/ja/guide/customization' },
              ],
            },
          ],
          '/ja/reference/': [
            {
              text: 'リファレンス',
              items: [
                { text: 'DSL 構文', link: '/ja/reference/dsl' },
                { text: 'アクション', link: '/ja/reference/actions' },
                { text: '設定ファイル', link: '/ja/reference/config' },
              ],
            },
          ],
        },
      },
    },
  },
})

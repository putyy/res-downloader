import { fileURLToPath } from 'node:url'
import { defineConfig } from 'vitepress'
import { zh, zhSearch } from './locales/zh.mts'
import { en, enSearch } from './locales/en.mts'
import { createPageHead } from './seo.mts'

const hostname = 'https://res.putyy.com'

export default defineConfig({
  srcExclude: ['README.md', 'readme.md'],
  lang: zh.lang,
  title: 'res-downloader',
  description: zh.description,
  locales: { root: zh, en },
  rewrites: {
    'zh/:path*': ':path*',
  },
  cleanUrls: false,
  vite: {
    publicDir: fileURLToPath(new URL('../public', import.meta.url)),
  },
  head: [
    ['link', { rel: 'icon', href: '/favicon.ico' }],
    ['meta', { name: 'theme-color', content: '#177858' }],
    ['meta', { property: 'og:site_name', content: 'res-downloader' }],
    ['meta', { property: 'og:type', content: 'website' }],
    ['meta', { property: 'og:image', content: `${hostname}/images/show.png` }],
    ['meta', { name: 'twitter:card', content: 'summary_large_image' }],
    ['meta', { name: 'twitter:image', content: `${hostname}/images/show.png` }],
  ],
  sitemap: {
    hostname,
  },
  transformHead: (context) => createPageHead(context, hostname),
  themeConfig: {
    siteTitle: false,
    i18nRouting: true,
    logo: { src: '/images/logo.png', alt: 'res-downloader' },
    socialLinks: [{ icon: 'github', link: 'https://github.com/putyy/res-downloader' }],
    search: {
      provider: 'local',
      options: {
        // Use the same tokenizer when building indexes and searching in the
        // browser. Intl.Segmenter also handles Latin words in mixed text.
        miniSearch: {
          options: {
            tokenize(text) {
              const segmenter = new Intl.Segmenter('zh-CN', { granularity: 'word' })
              return Array.from(segmenter.segment(text), ({ segment }) => segment.toLowerCase())
                .filter((word) => /[\p{L}\p{N}]/u.test(word))
            },
          },
          searchOptions: { prefix: true, fuzzy: 0.2 },
        },
        locales: { root: zhSearch, en: enSearch },
      },
    },
  },
})

import type { HeadConfig, TransformContext } from 'vitepress'

export function createPageHead(
  { pageData, siteData, siteConfig, title, description }: TransformContext,
  hostname: string,
): HeadConfig[] {
  if (pageData.isNotFound || pageData.relativePath === '404.md') {
    return [['meta', { name: 'robots', content: 'noindex' }]]
  }

  const pageUrl = (page: string) => new URL(
    page.replace(/(^|\/)index\.md$/, '$1').replace(/\.md$/, '.html'),
    `${hostname}${siteData.base}`,
  ).href
  const url = pageUrl(pageData.relativePath)
  const head: HeadConfig[] = [
    ['link', { rel: 'canonical', href: url }],
    ['meta', { property: 'og:url', content: url }],
    ['meta', { property: 'og:locale', content: siteData.lang.replace(/-/g, '_') }],
    ['meta', { property: 'og:title', content: title }],
    ['meta', { property: 'og:description', content: description }],
    ['meta', { name: 'twitter:title', content: title }],
    ['meta', { name: 'twitter:description', content: description }],
  ]

  const locales = Object.entries(siteData.locales)
  const currentLocale = locales
    .filter(([key]) => key !== 'root' && pageData.relativePath.startsWith(`${key}/`))
    .sort(([a], [b]) => b.length - a.length)[0]?.[0]
  const relativePage = currentLocale
    ? pageData.relativePath.slice(currentLocale.length + 1)
    : pageData.relativePath
  const pages = new Set(siteConfig.pages.map((page) => siteConfig.rewrites.map[page] || page))
  const translations = locales.flatMap(([key, locale]) => {
    const page = key === 'root' ? relativePage : `${key}/${relativePage}`
    return locale.lang && pages.has(page)
      ? [{ key, lang: locale.lang, url: pageUrl(page) }]
      : []
  })

  // Advertise only real translations, so partially translated sites have no
  // hreflang links to missing pages.
  if (translations.length > 1) {
    for (const translation of translations) {
      head.push(['link', { rel: 'alternate', hreflang: translation.lang, href: translation.url }])
    }
    const defaultLocale = translations.find(({ key }) => key === 'root')
    if (defaultLocale) head.push(['link', { rel: 'alternate', hreflang: 'x-default', href: defaultLocale.url }])
  }

  return head
}

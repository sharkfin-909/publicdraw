/**
 * The organiser surface is not for crawlers, and neither is anything that
 * takes a query string. Draw pages are gated individually by a robots meta
 * tag rather than here, because the decision is per-draw.
 */
export default defineEventHandler((event) => {
  const { siteUrl } = useRuntimeConfig(event).public
  setHeader(event, 'content-type', 'text/plain; charset=utf-8')

  return [
    'User-agent: *',
    'Allow: /',
    '',
    '# Organiser dashboard — never public',
    'Disallow: /admin',
    '',
    `Sitemap: ${String(siteUrl).replace(/\/$/, '')}/sitemap.xml`,
    '',
  ].join('\n')
})

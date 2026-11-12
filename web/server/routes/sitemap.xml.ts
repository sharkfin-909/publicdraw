import { drawPath } from '~/utils/seo'

interface PublicDraw {
  draw_id: string
  title: string
  state: string
  created_at: number
  committed_at?: number
}

/**
 * Built from the service's public listing, so opting a draw in is the single
 * gate: if it is not listed, it is neither in here nor indexable, and neither
 * switch can be flipped by accident.
 */
export default defineEventHandler(async (event) => {
  const cfg = useRuntimeConfig(event)
  const site = String(cfg.public.siteUrl).replace(/\/$/, '')

  const statics = [
    { loc: '/', priority: '1.0', changefreq: 'weekly' },
    { loc: '/draws', priority: '0.9', changefreq: 'daily' },
    { loc: '/verify', priority: '0.8', changefreq: 'monthly' },
    { loc: '/technical', priority: '0.7', changefreq: 'monthly' },
    { loc: '/receipt', priority: '0.7', changefreq: 'monthly' },
    { loc: '/log', priority: '0.6', changefreq: 'daily' },
  ]

  let draws: PublicDraw[] = []
  try {
    const res = await $fetch<{ draws: PublicDraw[] }>(`${cfg.apiInternal}/public/draws`)
    draws = res.draws ?? []
  } catch {
    // A sitemap missing its draws is better than a 500: the static pages
    // still get crawled and the next fetch picks the rest up.
  }

  const iso = (unix?: number) =>
    unix ? new Date(unix * 1000).toISOString() : new Date().toISOString()

  const urls = [
    ...statics.map(
      (s) =>
        `  <url><loc>${site}${s.loc}</loc><changefreq>${s.changefreq}</changefreq><priority>${s.priority}</priority></url>`,
    ),
    ...draws.map(
      (d) =>
        `  <url><loc>${site}${drawPath(d.draw_id, d.title)}</loc><lastmod>${iso(
          d.committed_at ?? d.created_at,
        )}</lastmod><changefreq>${d.state === 'drawn' ? 'yearly' : 'hourly'}</changefreq><priority>0.7</priority></url>`,
    ),
  ]

  setHeader(event, 'content-type', 'application/xml; charset=utf-8')
  return `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
${urls.join('\n')}
</urlset>
`
})

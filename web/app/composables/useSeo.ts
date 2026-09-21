import { absolute, clamp } from '~/utils/seo'

interface SeoInput {
  title: string
  description: string
  path: string
  /** Default is to allow indexing; pass false for anything with people in it. */
  index?: boolean
  type?: 'website' | 'article'
}

/**
 * One place that sets title, description, canonical, robots and Open Graph,
 * so no page can quietly ship without a canonical or with the wrong robots
 * directive.
 */
export function useSeo(input: MaybeRefOrGetter<SeoInput>) {
  const site = useRuntimeConfig().public

  useHead(() => {
    const v = toValue(input)
    const url = absolute(site.siteUrl, v.path)
    const allow = v.index !== false
    return {
      link: [{ rel: 'canonical', href: url }],
      meta: [
        {
          name: 'robots',
          content: allow ? 'index, follow, max-image-preview:large' : 'noindex, nofollow',
        },
      ],
    }
  })

  useSeoMeta({
    title: () => `${toValue(input).title} — ${site.siteName}`,
    description: () => clamp(toValue(input).description),
    ogTitle: () => toValue(input).title,
    ogDescription: () => clamp(toValue(input).description),
    ogType: () => toValue(input).type ?? 'website',
    ogUrl: () => absolute(site.siteUrl, toValue(input).path),
    ogSiteName: () => site.siteName,
    twitterCard: 'summary_large_image',
    twitterTitle: () => toValue(input).title,
    twitterDescription: () => clamp(toValue(input).description),
  })
}

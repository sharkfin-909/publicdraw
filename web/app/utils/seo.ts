/**
 * URL and metadata helpers.
 *
 * The rule the rest of the app follows: a draw page is `noindex` unless its
 * organiser explicitly listed it. Most draws worth running involve housing
 * applicants, grant seekers or ballot entrants, and the winners are named on
 * the page by design — so the default has to be exclusion, not opt-out.
 */

/** URL-safe slug from a draw title. Empty for titles with nothing usable. */
export function slugify(title: string): string {
  return title
    .toLowerCase()
    .normalize('NFKD')
    .replace(/[̀-ͯ]/g, '')
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 72)
    .replace(/-+$/, '')
}

/** The canonical path for a draw. The slug is cosmetic; the id is the key. */
export function drawPath(id: string, title?: string): string {
  const s = title ? slugify(title) : ''
  return s ? `/draws/${id}/${s}` : `/draws/${id}`
}

export function absolute(siteUrl: string, path: string): string {
  return `${siteUrl.replace(/\/$/, '')}${path}`
}

/** Trim to a clean meta description without cutting mid-word. */
export function clamp(text: string, max = 155): string {
  const t = text.replace(/\s+/g, ' ').trim()
  if (t.length <= max) return t
  const cut = t.lastIndexOf(' ', max - 1)
  return `${t.slice(0, cut > 0 ? cut : max - 1)}…`
}

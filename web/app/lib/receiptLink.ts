/**
 * The receipt link: how an entrant carries their place in a draw.
 *
 *   {origin}/receipt/{draw}/{index}#s={salt}
 *
 * The draw and the position sit in the path because they are not secret. The
 * salt rides in the fragment, which browsers never send to a server — so it
 * stays out of access logs and Referer headers even though the link gets
 * emailed around. Building and reading the link are one contract, kept here
 * together and away from the browser so a round trip can be tested.
 */

export interface ReceiptLink {
  drawId: string
  index: number
  salt: string
}

export function receiptLink(origin: string, l: ReceiptLink): string {
  return `${origin.replace(/\/$/, '')}/receipt/${l.drawId}/${l.index}#s=${l.salt}`
}

/** The salt from a link's fragment ('#s=…'), or '' if it carries none. */
export function saltFromFragment(hash: string): string {
  return new URLSearchParams(hash.replace(/^#/, '')).get('s') ?? ''
}

/**
 * Read a pasted receipt link. Accepts a full URL or a site-relative one; base
 * resolves the relative form. Returns null for anything that is not a receipt
 * link. A link without its salt still parses — with salt '' — because the
 * page can then say exactly what is missing.
 */
export function parseReceiptLink(raw: string, base: string): ReceiptLink | null {
  const text = raw.trim()
  if (!text) return null
  let u: URL
  try {
    u = /^https?:\/\//i.test(text) ? new URL(text) : new URL(text, base)
  } catch {
    return null
  }
  const m = u.pathname.match(/\/receipt\/([^/]+)\/(\d+)\/?$/)
  if (!m) return null
  return { drawId: decodeURIComponent(m[1]!), index: Number(m[2]), salt: saltFromFragment(u.hash) }
}

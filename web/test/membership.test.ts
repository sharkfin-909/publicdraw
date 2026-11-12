import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { checkMembership } from '../app/lib/membership'
import { receiptLink, parseReceiptLink, saltFromFragment } from '../app/lib/receiptLink'
import type { Admission, Bundle, Receipt } from '../app/lib/types'

const at = (n: string) => fileURLToPath(new URL(`./fixtures/${n}`, import.meta.url))
const bundle = JSON.parse(readFileSync(at('live-bundle.json'), 'utf8')) as Bundle
const receipt = JSON.parse(readFileSync(at('live-receipt.json'), 'utf8')) as Receipt
const admissions = JSON.parse(readFileSync(at('live-admissions.json'), 'utf8')) as Admission[]
const committed = bundle.roster.commitment.root

describe('entrant membership', () => {
  it('agrees between a path and the leaf list for every entrant', () => {
    for (const a of admissions) {
      const byLeaves = checkMembership(a, { leaves: bundle.roster.leaves }, committed)
      expect(byLeaves.ok).toBe(true)
      expect(byLeaves.index).toBe(a.index)
    }
    const byPath = checkMembership({ salt: receipt.salt, label: receipt.entry.label }, { path: receipt }, committed)
    const sameEntrant = admissions.find((a) => a.index === receipt.index)!
    expect(byPath.leaf).toBe(checkMembership(sameEntrant, { leaves: bundle.roster.leaves }).leaf)
  })

  it('catches a valid proof for a different roster', () => {
    // A self-consistent proof with a root the draw never committed to.
    const other = checkMembership({ salt: receipt.salt, label: receipt.entry.label }, { path: receipt }, '00'.repeat(32))
    expect(other.included).toBe(true)
    expect(other.matchesCommitted).toBe(false)
    expect(other.ok).toBe(false)
  })

  it('catches published leaves that do not rebuild to the committed root', () => {
    const a = admissions[0]!
    const pruned = bundle.roster.leaves.filter((_, i) => i !== admissions[1]!.index)
    const m = checkMembership(a, { leaves: pruned }, committed)
    expect(m.included).toBe(true)
    expect(m.matchesCommitted).toBe(false)
    expect(m.ok).toBe(false)
  })

  it('does not know the roster when no committed root is given', () => {
    const m = checkMembership(admissions[0]!, { leaves: bundle.roster.leaves })
    expect(m.matchesCommitted).toBeNull()
    expect(m.ok).toBe(true)
  })

  it('fails closed on an unreadable salt', () => {
    const m = checkMembership({ salt: 'not hex', label: 'x' }, { leaves: bundle.roster.leaves }, committed)
    expect(m).toMatchObject({ included: false, index: -1, leaf: '', ok: false })
  })

  it('needs the exact label', () => {
    const a = admissions[0]!
    expect(checkMembership({ ...a, label: a.label + ' ' }, { leaves: bundle.roster.leaves }).included).toBe(false)
  })
})

describe('the receipt link', () => {
  const l = { drawId: bundle.draw.params.draw_id, index: 4472, salt: admissions[0]!.salt }

  it('round-trips through build and parse', () => {
    const url = receiptLink('https://publicdraw.example/', l)
    expect(url).toBe(`https://publicdraw.example/receipt/${l.drawId}/4472#s=${l.salt}`)
    expect(parseReceiptLink(url, 'https://elsewhere.example')).toEqual(l)
  })

  it('reads a site-relative link against the base', () => {
    expect(parseReceiptLink(`/receipt/${l.drawId}/4472#s=${l.salt}`, 'https://publicdraw.example')).toEqual(l)
  })

  it('still parses a link with its salt missing, so the page can say so', () => {
    expect(parseReceiptLink(`https://x.example/receipt/${l.drawId}/3`, 'https://x.example')).toEqual({
      drawId: l.drawId, index: 3, salt: '',
    })
  })

  it('rejects anything that is not a receipt link', () => {
    for (const raw of ['', 'hello', 'https://x.example/draws/abc', 'https://x.example/receipt/abc/notanumber']) {
      expect(parseReceiptLink(raw, 'https://x.example')).toBeNull()
    }
  })

  it('takes the salt from the fragment only', () => {
    expect(saltFromFragment('#s=abc')).toBe('abc')
    expect(saltFromFragment('')).toBe('')
  })
})

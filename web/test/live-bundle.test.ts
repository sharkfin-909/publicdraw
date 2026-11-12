import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { verifyBundle } from '../app/lib/verify'
import { checkMembership } from '../app/lib/membership'
import type { Admission, Bundle, Receipt } from '../app/lib/types'

/**
 * A real draw, produced by the Go service against a live drand round and
 * captured here. It is the end-to-end form of the same argument testvectors
 * makes about primitives: the implementation that verifies a bundle is not the
 * one that made it, so a hashing bug cannot confirm itself.
 */
const at = (n: string) => fileURLToPath(new URL(`./fixtures/${n}`, import.meta.url))
const bundle = JSON.parse(readFileSync(at('live-bundle.json'), 'utf8')) as Bundle
const receipt = JSON.parse(readFileSync(at('live-receipt.json'), 'utf8')) as Receipt
const admissions = JSON.parse(readFileSync(at('live-admissions.json'), 'utf8')) as Admission[]

describe('a live bundle from the Go service', () => {
  it('verifies in the browser implementation', () => {
    const rep = verifyBundle(bundle)
    const broken = rep.checks.filter((c) => !c.ok && !c.skipped)
    expect(broken.map((c) => c.name)).toEqual([])
    expect(rep.ok).toBe(true)
    expect(rep.skipped).toBe(2)
  })

  it('accepts a genuine receipt', () => {
    const m = checkMembership(
      { salt: receipt.salt, label: receipt.entry.label },
      { path: receipt },
      bundle.roster.commitment.root,
    )
    expect(m.ok).toBe(true)
    expect(m.matchesCommitted).toBe(true)
  })

  it('rejects a substituted winner', () => {
    const t = structuredClone(bundle)
    t.result.winners[0]!.entry.label = "the organiser's friend"
    expect(verifyBundle(t).ok).toBe(false)
  })

  it('rejects a roster frozen after the round published', () => {
    const t = structuredClone(bundle)
    t.roster.commitment.committed_at = t.beacon.round * 3 + 1_700_000_000
    expect(verifyBundle(t).ok).toBe(false)
  })

  it('rejects a tampered log record', () => {
    const t = structuredClone(bundle)
    t.log.records[1]!.logged_at += 1
    expect(verifyBundle(t).ok).toBe(false)
  })

  it('rejects a forged beacon signature', () => {
    const t = structuredClone(bundle)
    t.beacon.signature = t.beacon.signature.slice(0, -1) + (t.beacon.signature.endsWith('d') ? 'c' : 'd')
    expect(verifyBundle(t).ok).toBe(false)
  })

  it('rejects an altered receipt label', () => {
    expect(checkMembership({ salt: receipt.salt, label: 'someone else' }, { path: receipt }).included).toBe(false)
  })

  // The receipt-free path: an entrant holding the salt they were handed when
  // they entered can find their own leaf in the published list, with nothing
  // having had to be delivered to them at the cutoff.
  it('lets every entrant find their own leaf from the salt they were given', () => {
    expect(admissions.length).toBe(bundle.roster.leaves.length)
    for (const a of admissions) {
      const m = checkMembership(a, { leaves: bundle.roster.leaves }, bundle.roster.commitment.root)
      expect(m.ok).toBe(true)
      expect(m.index).toBe(a.index)
    }
  })

  it('does not find a leaf for someone who never entered', () => {
    const a = admissions[0]!
    expect(checkMembership({ salt: a.salt, label: 'someone-who-never-entered' }, { leaves: bundle.roster.leaves }).included).toBe(false)
  })

  it('rejects a bundle whose published leaves were pruned', () => {
    const t = structuredClone(bundle)
    t.roster.leaves.splice(3, 1)
    expect(verifyBundle(t).ok).toBe(false)
  })

  it('rejects a bundle whose published leaves were substituted', () => {
    const t = structuredClone(bundle)
    t.roster.leaves[2] = '00'.repeat(32)
    expect(verifyBundle(t).ok).toBe(false)
  })
})

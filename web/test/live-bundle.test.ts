import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { verifyBundle, verifyReceipt } from '../app/lib/verify'
import type { Bundle, Receipt } from '../app/lib/types'

/**
 * A real draw, produced by the Go service against a live drand round and
 * captured here. It is the end-to-end form of the same argument testvectors
 * makes about primitives: the implementation that verifies a bundle is not the
 * one that made it, so a hashing bug cannot confirm itself.
 */
const at = (n: string) => fileURLToPath(new URL(`./fixtures/${n}`, import.meta.url))
const bundle = JSON.parse(readFileSync(at('live-bundle.json'), 'utf8')) as Bundle
const receipt = JSON.parse(readFileSync(at('live-receipt.json'), 'utf8')) as Receipt

describe('a live bundle from the Go service', () => {
  it('verifies in the browser implementation', () => {
    const rep = verifyBundle(bundle)
    const broken = rep.checks.filter((c) => !c.ok && !c.skipped)
    expect(broken.map((c) => c.name)).toEqual([])
    expect(rep.ok).toBe(true)
    expect(rep.skipped).toBe(2)
  })

  it('accepts a genuine receipt', () => {
    expect(verifyReceipt(receipt)).toBe(true)
    expect(receipt.root).toBe(bundle.roster.commitment.root)
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
    expect(verifyReceipt({ ...receipt, entry: { label: 'someone else' } })).toBe(false)
  })
})

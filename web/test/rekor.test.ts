/**
 * The TypeScript verifier has to check the Rekor anchor too. An anchor checked
 * by the organiser's server would defeat the point of anchoring.
 */
import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

import { verifyBundle } from '../app/lib/verify'
import { verifyInclusion, parseCheckpoint, cosignatures, earliestCosignature } from '../app/lib/rekor'
import type { Bundle } from '../app/lib/types'

const at = (n: string) => fileURLToPath(new URL(`./fixtures/${n}`, import.meta.url))
const bundle = JSON.parse(readFileSync(at('anchored-bundle.json'), 'utf8')) as Bundle
const anchor = bundle.roster.rekor_anchor!

describe('a real Rekor anchor', () => {
  it('has one', () => expect(anchor).toBeTruthy())

  it('verifies its inclusion proof in the browser implementation', () => {
    expect(verifyInclusion(anchor)).toEqual({ ok: true })
  })

  it('parses a C2SP checkpoint that agrees with the proof', () => {
    const cp = parseCheckpoint(anchor.checkpoint)!
    expect(cp).toBeTruthy()
    expect(cp.rootHash).toBe(anchor.root_hash)
    expect(cp.treeSize).toBe(anchor.tree_size)
    expect(cp.origin).toMatch(/rekor/)
  })

  it('reads witness times and never claims they are verified', () => {
    const cp = parseCheckpoint(anchor.checkpoint)!
    const cos = cosignatures(cp)
    expect(cos.length).toBeGreaterThan(0)
    for (const c of cos) {
      expect(c.verified).toBe(false)
      expect(c.witness).not.toBe(cp.origin)
      expect(c.at.getTime()).toBeGreaterThan(1_600_000_000_000)
    }
    expect(earliestCosignature(cp)).toBeTruthy()
  })

  it.each([
    ['a proof hash', (a: typeof anchor) => (a.hashes[0] = 'AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=')],
    ['the body', (a: typeof anchor) => (a.body = btoa('a different entry'))],
    ['the root hash', (a: typeof anchor) => (a.root_hash = 'AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=')],
    ['the log index', (a: typeof anchor) => (a.log_index += 1)],
    ['the tree size', (a: typeof anchor) => (a.tree_size += 1)],
  ])('rejects a proof with %s altered', (_name, mutate) => {
    const t = structuredClone(anchor)
    mutate(t)
    expect(verifyInclusion(t).ok).toBe(false)
  })
})

describe('the anchored bundle as a whole', () => {
  it('verifies', () => {
    const rep = verifyBundle(bundle)
    const broken = rep.checks.filter((c) => !c.ok && !c.skipped)
    expect(broken.map((c) => c.name)).toEqual([])
    expect(rep.ok).toBe(true)
  })

  it('rejects an anchor for unrelated bytes', () => {
    const t = structuredClone(bundle)
    t.roster.rekor_anchor!.payload = btoa('different payload entirely')
    expect(verifyBundle(t).ok).toBe(false)
  })

  it('stays valid without an anchor, but stops claiming what it cannot show', () => {
    const t = structuredClone(bundle)
    delete t.roster.rekor_anchor
    const rep = verifyBundle(t)
    expect(rep.ok).toBe(true)
    const skipped = rep.checks.filter((c) => c.skipped).map((c) => c.id)
    expect(skipped).toContain('anchor.present')
    expect(skipped).toContain('log.not-retried')
  })

  // Saying a single bundle proved this would be the worst lie the system
  // could tell, so it must stay skipped even when anchored.
  it('leaves the retry question open even when anchored', () => {
    const rep = verifyBundle(bundle)
    const c = rep.checks.find((x) => x.id === 'log.not-retried')!
    expect(c.skipped).toBe(true)
  })
})

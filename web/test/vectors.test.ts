/**
 * The cross-implementation contract.
 *
 * Every value in ../../testdata/testvectors.json was produced by the Go
 * implementation. If this file and Go ever disagree by a single byte, a draw
 * verified in one and rejected in the other — so the disagreement has to fail
 * loudly here rather than silently in production.
 */
import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

import { Tag, lp, tagged, toHex, fromHex, utf8 } from '../app/lib/hashing'
import { leafHash, nodeHash, rootFromLeaves, canonicalEntryBytes } from '../app/lib/merkle'
import { rosterCommitValue, seed, select } from '../app/lib/draw'
import { genesis, headAfter } from '../app/lib/translog'
import { randomnessOf, verifyRound, timeOfRound, roundAt } from '../app/lib/beacon'

const vectorPath = fileURLToPath(new URL('../../testdata/testvectors.json', import.meta.url))
const v = JSON.parse(readFileSync(vectorPath, 'utf8'))

const u32be = (n: number) => {
  const b = new Uint8Array(4)
  new DataView(b.buffer).setUint32(0, n, false)
  return b
}
// The shipped encoder, not a copy of it: this is what the vectors pin.
const canonicalEntry = canonicalEntryBytes

describe('tags', () => {
  it('match the Go constants exactly', () => {
    expect(Tag.leaf).toBe(v.tags.leaf)
    expect(Tag.node).toBe(v.tags.node)
    expect(Tag.roster).toBe(v.tags.roster)
    expect(Tag.draw).toBe(v.tags.draw)
    expect(Tag.rosterSig).toBe(v.tags.roster_sig)
    expect(Tag.seed).toBe(v.tags.seed)
    expect(Tag.ticket).toBe(v.tags.ticket)
    expect(Tag.logEntry).toBe(v.tags.log_entry)
    expect(Tag.logGenesis).toBe(v.tags.log_genesis)
  })
})

describe('length prefixing', () => {
  it.each(v.length_prefix)('lp(%o)', (c: any) => {
    expect(toHex(lp(fromHex(c.in_hex)))).toBe(c.out_hex)
  })
})

describe('tagged hashing', () => {
  it.each(v.tagged_hash)('tagged(%o)', (c: any) => {
    expect(toHex(tagged(c.tag, ...c.parts_hex.map(fromHex)))).toBe(c.out)
  })
})

describe('merkle', () => {
  it.each(v.roster_leaf)('leaf(%o)', (c: any) => {
    expect(toHex(leafHash(fromHex(c.salt_hex), fromHex(c.entry_hex)))).toBe(c.leaf)
  })

  it('node', () => {
    expect(toHex(nodeHash(fromHex(v.merkle_node.Left), fromHex(v.merkle_node.Right)))).toBe(
      v.merkle_node.Out,
    )
  })

  // Sizes 1..17 cover every shape of promoted odd node, which is exactly where
  // a mismatch between two implementations would hide.
  it.each(v.merkle_roots)('root at size %o', (c: any) => {
    const leaves = Array.from({ length: c.size }, (_, i) => tagged('vc/testvec', u32be(i)))
    expect(toHex(rootFromLeaves(leaves))).toBe(c.root)
  })
})

describe('entry encoding', () => {
  it.each(v.entry_canonical)('canonical(%o)', (c: any) => {
    expect(toHex(canonicalEntry(c.label))).toBe(c.canonical_hex)
    const zero = new Uint8Array(32)
    expect(toHex(leafHash(zero, canonicalEntry(c.label)))).toBe(c.leaf_with_zero_salt)
  })
})

describe('roster commitment', () => {
  it.each(v.roster_commit)('commit(%o)', (c: any) => {
    expect(toHex(rosterCommitValue(fromHex(c.root), c.size))).toBe(c.commit)
  })
})

describe('seed derivation', () => {
  it.each(v.draw_seed)('seed(%o)', (c: any) => {
    expect(
      toHex(seed(fromHex(c.roster_root), c.roster_size, fromHex(c.randomness), c.draw_id)),
    ).toBe(c.seed)
  })
})

describe('selection', () => {
  it.each(v.draw_ranking)('ranking(%o)', (c: any) => {
    const { won, waitlist } = select(fromHex(c.seed), c.size, c.winners)
    expect(won).toEqual(c.won)
    // The Go generator truncates long waitlists to the first eight.
    expect(waitlist.slice(0, c.waitlist.length)).toEqual(c.waitlist)
  })
})

describe('transparency log', () => {
  it('replays the vector chain to Go’s heads', () => {
    const rows = v.log_chain
    expect(toHex(genesis(rows[0].log_id))).toBe(rows[0].head)

    for (const r of rows.slice(1)) {
      expect(
        toHex(headAfter(fromHex(r.prev), r.seq, r.kind, r.draw_id, fromHex(r.payload), r.logged_at)),
      ).toBe(r.head)
    }
  })
})

describe('beacon', () => {
  it('derives drand’s own randomness from the signature', () => {
    expect(toHex(randomnessOf({ round: v.beacon.round, signature: v.beacon.signature }))).toBe(
      v.beacon.randomness,
    )
  })

  it('verifies a real quicknet round against the pinned group key', () => {
    expect(verifyRound({ round: v.beacon.round, signature: v.beacon.signature }).ok).toBe(true)
  })

  it('rejects a tampered signature', () => {
    const bad = v.beacon.signature.slice(0, -1) + (v.beacon.signature.endsWith('d') ? 'c' : 'd')
    expect(verifyRound({ round: v.beacon.round, signature: bad }).ok).toBe(false)
  })

  it('binds a signature to its round number', () => {
    expect(verifyRound({ round: v.beacon.round + 1, signature: v.beacon.signature }).ok).toBe(false)
  })

  it('maps rounds to the times Go computes', () => {
    expect(Math.floor(timeOfRound(v.beacon.round).getTime() / 1000)).toBe(v.beacon.published_unix)
    expect(roundAt(timeOfRound(v.beacon.round))).toBe(v.beacon.round)
  })
})

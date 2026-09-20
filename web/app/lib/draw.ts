/**
 * Seed derivation and selection, mirroring the Go package `draw`.
 *
 * Selection is a ranking, not a lottery: every entry gets a ticket, entries
 * sort by ticket, and the first k win. That avoids a rejection loop, needs no
 * agreed PRNG for this file to reproduce Go's answer, and leaves the entries
 * after the winners as an ordered waitlist.
 */
import { Tag, tagged, u32, u64, utf8, compareBytes, fromHex } from './hashing'
import type { DrawParams, RosterCommitment } from './types'

export function rosterCommitValue(root: Uint8Array, size: number): Uint8Array {
  return tagged(Tag.roster, root, u32(size))
}

/**
 * Every input is load-bearing. The root binds the outcome to one specific
 * committed roster; the size fixes the shape; the randomness is the part
 * nobody could predict; the draw id separates draws landing on the same round.
 */
export function seed(
  root: Uint8Array,
  size: number,
  randomness: Uint8Array,
  drawId: string,
): Uint8Array {
  return tagged(Tag.seed, root, u32(size), randomness, utf8(drawId))
}

export function ticketFor(seedBytes: Uint8Array, index: number): Uint8Array {
  return tagged(Tag.ticket, seedBytes, u32(index))
}

export interface Ranked {
  index: number
  ticket: Uint8Array
}

/** Order every entry by ticket ascending, ties broken by index. */
export function rank(seedBytes: Uint8Array, size: number): Ranked[] {
  const tickets: Ranked[] = []
  for (let i = 0; i < size; i++) tickets.push({ index: i, ticket: ticketFor(seedBytes, i) })
  tickets.sort((a, b) => {
    const c = compareBytes(a.ticket, b.ticket)
    return c !== 0 ? c : a.index - b.index
  })
  return tickets
}

export function select(
  seedBytes: Uint8Array,
  size: number,
  winners: number,
): { won: number[]; waitlist: number[] } {
  if (size <= 0) throw new Error('draw: empty roster')
  if (winners < 1) throw new Error('draw: winners must be at least 1')
  if (winners > size) throw new Error(`draw: ${winners} winners requested from ${size} entries`)

  const ranked = rank(seedBytes, size)
  return {
    won: ranked.slice(0, winners).map((t) => t.index),
    waitlist: ranked.slice(winners).map((t) => t.index),
  }
}

/** The exact preimage the operator signs at creation. */
export function paramsSigningBytes(p: DrawParams): Uint8Array {
  return tagged(
    Tag.draw,
    utf8(p.draw_id),
    utf8(p.title),
    u32(p.winners),
    utf8(p.disclosure),
    u64(p.cutoff_at),
    u64(p.target_round),
    utf8(p.chain_hash),
    u64(p.created_at),
  )
}

/** The exact preimage the operator signs at the cutoff. */
export function rosterSigningBytes(rc: RosterCommitment): Uint8Array {
  return tagged(
    Tag.rosterSig,
    utf8(rc.draw_id),
    fromHex(rc.root),
    u32(rc.size),
    fromHex(rc.commit),
    u64(rc.committed_at),
  )
}

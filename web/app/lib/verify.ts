/**
 * The full bundle check, mirroring the Go package `bundle`.
 *
 * This runs entirely in the reader's browser against a pinned drand group key
 * and a pinned protocol. Nothing here asks the organiser's server whether the
 * organiser's draw was honest — which is the only reason a verifier is worth
 * having.
 *
 * It never short-circuits, and a skipped check is never counted as a pass.
 */
import { ed25519 } from '@noble/curves/ed25519'
import { fromHex, toHex, equal, u32 } from './hashing'
import { leafHash, verifyPath, rootFromLeaves } from './merkle'
import { rosterCommitValue, seed, select, paramsSigningBytes, rosterSigningBytes } from './draw'
import { replay } from './translog'
import { QUICKNET, randomnessOf, timeOfRound, verifyRound } from './beacon'
import type { Bundle } from './types'

/** Minimum gap between cutoff and target round, matching draw.MinLead in Go. */
export const MIN_LEAD_SECONDS = 15

export interface Check {
  name: string
  ok: boolean
  skipped: boolean
  detail: string
}

export interface Report {
  ok: boolean
  skipped: number
  checks: Check[]
}

function canonicalEntryBytes(label: string): Uint8Array {
  // Entry.CanonicalBytes in Go: the label, length-prefixed.
  const enc = new TextEncoder().encode(label)
  const out = new Uint8Array(4 + enc.length)
  out.set(u32(enc.length), 0)
  out.set(enc, 4)
  return out
}

export function verifyBundle(b: Bundle): Report {
  const checks: Check[] = []
  const add = (name: string, ok: boolean, detail: string) =>
    checks.push({ name, ok, skipped: false, detail })
  const skip = (name: string, detail: string) =>
    checks.push({ name, ok: false, skipped: true, detail })

  const p = b.draw.params
  const rc = b.roster.commitment

  // --- what the organiser committed to, and when ---------------------------

  try {
    add(
      'roster commitment binds root and size',
      equal(rosterCommitValue(fromHex(rc.root), rc.size), fromHex(rc.commit)),
      `commit=${rc.commit.slice(0, 16)}… size=${rc.size}`,
    )
  } catch (e) {
    add('roster commitment binds root and size', false, String(e))
  }

  add('roster commitment belongs to this draw', rc.draw_id === p.draw_id, `draw=${p.draw_id}`)
  add('draw parameters name this chain', p.chain_hash === QUICKNET.hash, `chain=${QUICKNET.name}`)

  try {
    const pk = fromHex(b.operator_pubkey)
    add(
      'operator signature over draw parameters',
      ed25519.verify(fromHex(b.draw.signature), paramsSigningBytes(p), pk),
      'valid',
    )
    add(
      'operator signature over roster commitment',
      ed25519.verify(fromHex(b.roster.signature), rosterSigningBytes(rc), pk),
      'valid',
    )
  } catch (e) {
    add('operator signature over draw parameters', false, String(e))
    add('operator signature over roster commitment', false, String(e))
  }

  // --- ordering: the part that makes the draw unsteerable -------------------

  add(
    'parameters fixed before the cutoff',
    p.created_at < p.cutoff_at,
    `created=${p.created_at} cutoff=${p.cutoff_at}`,
  )
  add(
    'roster frozen at or after the cutoff',
    rc.committed_at >= p.cutoff_at,
    `committed=${rc.committed_at} cutoff=${p.cutoff_at}`,
  )

  const roundUnix = Math.floor(timeOfRound(p.target_round).getTime() / 1000)
  add(
    'roster frozen before the beacon round existed',
    rc.committed_at < roundUnix,
    `lead=${roundUnix - rc.committed_at}s`,
  )
  add(
    'target round leaves the minimum lead after cutoff',
    roundUnix - p.cutoff_at >= MIN_LEAD_SECONDS,
    `gap=${roundUnix - p.cutoff_at}s min=${MIN_LEAD_SECONDS}s`,
  )

  // --- the randomness ------------------------------------------------------

  add(
    'beacon round is the committed target',
    b.beacon.round === p.target_round,
    `bundle=${b.beacon.round} committed=${p.target_round}`,
  )
  const bv = verifyRound(b.beacon)
  add('drand signature verifies against pinned group key', bv.ok, bv.error ?? 'valid')

  // --- the derivation ------------------------------------------------------

  let derivedOk = false
  try {
    const rnd = randomnessOf(b.beacon)
    const want = seed(fromHex(rc.root), rc.size, rnd, p.draw_id)
    derivedOk = equal(want, fromHex(b.result.seed))
    add('seed derives from roster and beacon', derivedOk, `seed=${b.result.seed.slice(0, 16)}…`)

    const { won, waitlist } = select(fromHex(b.result.seed), rc.size, p.winners)
    const published = b.result.winners.map((w) => w.index)
    const sameWinners = won.length === published.length && won.every((v, i) => v === published[i])
    const sameWait =
      b.result.waitlist.length === 0 ||
      (waitlist.length === b.result.waitlist.length &&
        waitlist.every((v, i) => v === b.result.waitlist[i]))
    add(
      'winners derive from the seed',
      sameWinners && sameWait,
      sameWinners
        ? `${won.length} winner(s) of ${rc.size} entries`
        : `derived=[${won}] published=[${published}]`,
    )
  } catch (e) {
    add('seed derives from roster and beacon', false, String(e))
    add('winners derive from the seed', false, 'no seed')
  }

  // --- disclosure ----------------------------------------------------------

  if (p.disclosure === 'public') {
    const entries = b.roster.entries ?? []
    if (entries.length !== rc.size) {
      add(
        'published roster rebuilds to the committed root',
        false,
        `bundle carries ${entries.length} entries, commitment says ${rc.size}`,
      )
    } else {
      try {
        const leaves = entries.map((s) => leafHash(fromHex(s.salt), canonicalEntryBytes(s.entry.label)))
        add(
          'published roster rebuilds to the committed root',
          equal(rootFromLeaves(leaves), fromHex(rc.root)),
          'root matches',
        )
      } catch (e) {
        add('published roster rebuilds to the committed root', false, String(e))
      }
    }
  } else if (p.disclosure === 'private') {
    add(
      'private disclosure withholds the entry list',
      (b.roster.entries?.length ?? 0) === 0,
      `${b.roster.entries?.length ?? 0} entries published`,
    )
  } else {
    add('disclosure mode is recognised', false, `unknown mode "${p.disclosure}"`)
  }

  let proven = b.result.winners.length > 0
  try {
    for (const w of b.result.winners) {
      const leaf = leafHash(fromHex(w.salt), canonicalEntryBytes(w.entry.label))
      if (!verifyPath(leaf, w.index, rc.size, w.merkle_path.map(fromHex), fromHex(rc.root))) {
        proven = false
        break
      }
    }
  } catch {
    proven = false
  }
  add(
    'each winner is proven to be in the committed roster',
    proven,
    `${b.result.winners.length} proof(s)`,
  )

  // --- transparency log ----------------------------------------------------

  const r = replay(b.log.records, b.log.log_id)
  add('log records form an unbroken chain', r.ok, r.error ?? 'chain intact')

  let covered = r.ok && r.head === b.log.head
  if (covered) {
    let created = false
    let committed = false
    let published = false
    for (const rec of b.log.records) {
      if (rec.draw_id !== p.draw_id) continue
      if (rec.kind === 'draw_created') created = equal(fromHex(rec.payload), paramsSigningBytes(p))
      if (rec.kind === 'roster_committed')
        committed = equal(fromHex(rec.payload), rosterSigningBytes(rc))
      if (rec.kind === 'result_published') published = true
    }
    covered = created && committed && published
  }
  add(
    "log contains this draw's creation, cutoff and result",
    covered,
    `${b.log.records.length} record(s)`,
  )

  // --- what cannot be answered from one bundle -----------------------------

  skip(
    'log head timestamped before the beacon round',
    b.roster.ots_proof
      ? `OTS proof present (${b.roster.ots_proof.length} bytes) — verify separately`
      : "no OTS proof in bundle; the operator's clock is taken on trust",
  )
  skip(
    'this draw was not abandoned and retried',
    'answerable only against the full public log, not from one bundle',
  )

  const skipped = checks.filter((c) => c.skipped).length
  return { ok: checks.every((c) => c.ok || c.skipped), skipped, checks }
}

/** Check one entrant's receipt against the root it claims membership in. */
export function verifyReceipt(r: {
  root: string
  size: number
  index: number
  salt: string
  entry: { label: string }
  merkle_path: string[]
}): boolean {
  try {
    const leaf = leafHash(fromHex(r.salt), canonicalEntryBytes(r.entry.label))
    return verifyPath(leaf, r.index, r.size, r.merkle_path.map(fromHex), fromHex(r.root))
  } catch {
    return false
  }
}

export { toHex }

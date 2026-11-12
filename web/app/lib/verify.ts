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
import { fromHex, toHex, equal } from './hashing'
import { entryLeaf, verifyPath, rootFromLeaves } from './merkle'
import { rosterCommitValue, seed, select, paramsSigningBytes, rosterSigningBytes } from './draw'
import { replay } from './translog'
import { QUICKNET, randomnessOf, timeOfRound, verifyRound } from './beacon'
import {
  bindsPayload,
  cosignatures,
  earliestCosignature,
  parseCheckpoint,
  verifyInclusion,
  type Anchor,
} from './rekor'
import type { Bundle } from './types'

/** Minimum gap between cutoff and target round, matching draw.MinLead in Go. */
export const MIN_LEAD_SECONDS = 15

export interface Check {
  /** Stable id, shared with the Go verifier; see CHECK_IDS. */
  id: string
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

/**
 * Every check's stable id, separate from the sentence shown to a person. These
 * are the ids bundle/checks.go uses: the shared bundle vectors
 * (testdata/bundlevectors.json) key Go's verdict on them, and the vector test
 * holds this verifier to the same verdict check by check. A check can be
 * reworded freely; a new check needs an id here and in Go.
 */
export const CHECK_IDS: Record<string, string> = {
  'roster commitment binds root and size': 'roster.commitment',
  'roster commitment belongs to this draw': 'roster.draw',
  'draw parameters name this chain': 'params.chain',
  'operator signature over draw parameters': 'sig.params',
  'operator signature over roster commitment': 'sig.roster',
  'parameters fixed before the cutoff': 'order.params-before-cutoff',
  'roster frozen at or after the cutoff': 'order.roster-after-cutoff',
  'roster frozen before the beacon round existed': 'order.roster-before-round',
  'target round leaves the minimum lead after cutoff': 'order.min-lead',
  'beacon round is the committed target': 'beacon.target',
  'drand signature verifies against pinned group key': 'beacon.signature',
  'seed derives from roster and beacon': 'derive.seed',
  'winners derive from the seed': 'derive.winners',
  'published leaves match the committed entry count': 'leaves.count',
  'published leaves rebuild to the committed root': 'leaves.root',
  'published entries match the committed entry count': 'entries.count',
  'published entries hash to the published leaves': 'entries.leaves',
  'private disclosure withholds the entry list': 'entries.withheld',
  'disclosure mode is recognised': 'disclosure.mode',
  "each winner's leaf sits at its claimed index": 'winners.at-index',
  'each winner is proven to be in the committed roster': 'winners.proven',
  'log records form an unbroken chain': 'log.chain',
  "log contains this draw's creation, cutoff and result": 'log.covers',
  'this draw was not abandoned and retried': 'log.not-retried',
  'log head published to a log the organiser does not control': 'anchor.present',
  'rekor inclusion proof verifies': 'anchor.inclusion',
  'anchor binds a head from this draw\u2019s log': 'anchor.binds',
  'checkpoint parses': 'anchor.checkpoint',
  'log head witnessed before the beacon round': 'anchor.witnessed',
}

export function verifyBundle(b: Bundle): Report {
  const checks: Check[] = []
  const add = (name: string, ok: boolean, detail: string) =>
    checks.push({ id: CHECK_IDS[name] ?? '', name, ok, skipped: false, detail })
  const skip = (name: string, detail: string) =>
    checks.push({ id: CHECK_IDS[name] ?? '', name, ok: false, skipped: true, detail })

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

  // --- the published leaves ------------------------------------------------

  const leaves = (b.roster.leaves ?? []).map((l) => {
    try {
      return fromHex(l)
    } catch {
      return new Uint8Array(0)
    }
  })

  if (leaves.length !== rc.size) {
    add(
      'published leaves match the committed entry count',
      false,
      `bundle carries ${leaves.length} leaves, commitment says ${rc.size}`,
    )
    add('published leaves rebuild to the committed root', false, 'wrong leaf count')
  } else {
    add('published leaves match the committed entry count', true, `${rc.size} leaves`)
    try {
      add(
        'published leaves rebuild to the committed root',
        equal(rootFromLeaves(leaves), fromHex(rc.root)),
        'root matches',
      )
    } catch (e) {
      add('published leaves rebuild to the committed root', false, String(e))
    }
  }

  // --- disclosure ----------------------------------------------------------

  if (p.disclosure === 'public') {
    const entries = b.roster.entries ?? []
    if (entries.length !== rc.size) {
      add(
        'published entries match the committed entry count',
        false,
        `bundle carries ${entries.length} entries, commitment says ${rc.size}`,
      )
    } else {
      try {
        let ok = true
        for (let i = 0; i < entries.length; i++) {
          const leaf = entryLeaf(entries[i]!.salt, entries[i]!.entry.label)
          if (!leaves[i] || !equal(leaf, leaves[i]!)) {
            ok = false
            break
          }
        }
        add('published entries hash to the published leaves', ok, `${entries.length} entries disclosed`)
      } catch (e) {
        add('published entries hash to the published leaves', false, String(e))
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

  // Winners are checked twice against different things: their leaf must sit at
  // the claimed index in the published list, and their proof must climb to the
  // root on its own — the latter is what makes a receipt self-contained.
  let atIndex = b.result.winners.length > 0
  try {
    for (const w of b.result.winners) {
      const leaf = entryLeaf(w.salt, w.entry.label)
      if (w.index < 0 || w.index >= leaves.length || !equal(leaf, leaves[w.index]!)) {
        atIndex = false
        break
      }
    }
  } catch {
    atIndex = false
  }
  add(
    "each winner's leaf sits at its claimed index",
    atIndex,
    `${b.result.winners.length} winner(s)`,
  )

  let proven = b.result.winners.length > 0
  try {
    for (const w of b.result.winners) {
      const leaf = entryLeaf(w.salt, w.entry.label)
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

  verifyAnchor(b, add, skip)

  const skipped = checks.filter((c) => c.skipped).length
  return { ok: checks.every((c) => c.ok || c.skipped), skipped, checks }
}

/**
 * Find which log record's head was anchored, and the phase it covers.
 *
 * The anchor is published when the roster is frozen, so it covers the head as
 * it stood then — not the head at the end of the draw, which the result record
 * moves. The question is therefore not "does this equal the final head" but
 * "is this a head the chain passed through, and had the roster been committed
 * by that point".
 */
function anchoredRecord(a: Anchor, b: Bundle): { index: number; phase: string } {
  let committedBy = -1
  for (let i = 0; i < b.log.records.length; i++) {
    const r = b.log.records[i]!
    if (r.draw_id === b.draw.params.draw_id && r.kind === 'roster_committed') committedBy = i
  }
  for (let i = 0; i < b.log.records.length; i++) {
    if (!bindsPayload(a, b.log.records[i]!.head)) continue
    if (committedBy < 0 || i < committedBy) return { index: i, phase: '' }
    return { index: i, phase: b.log.records[i]!.kind }
  }
  return { index: -1, phase: '' }
}

function verifyAnchor(
  b: Bundle,
  add: (n: string, ok: boolean, d: string) => void,
  skip: (n: string, d: string) => void,
) {
  const a = b.roster.rekor_anchor
  if (!a) {
    skip(
      'log head published to a log the organiser does not control',
      "no Rekor anchor in this bundle; the local chain rests on the operator's own publication",
    )
    skip(
      'this draw was not abandoned and retried',
      "without an anchor, answerable only against the organiser's own log",
    )
    return
  }

  const inc = verifyInclusion(a)
  const cp = parseCheckpoint(a.checkpoint)
  add(
    'rekor inclusion proof verifies',
    inc.ok,
    inc.ok && cp ? `${cp.origin} index ${a.log_index} of ${a.tree_size}` : (inc.error ?? 'invalid'),
  )

  const { index, phase } = anchoredRecord(a, b)
  if (index < 0) {
    add('anchor binds a head from this draw\u2019s log', false, 'the anchored payload is not any head in this chain')
  } else if (!phase) {
    add(
      'anchor binds a head from this draw\u2019s log',
      false,
      `anchored at record ${index}, before the roster was committed`,
    )
  } else {
    add('anchor binds a head from this draw\u2019s log', true, `record ${index}, immediately after ${phase}`)
  }

  if (!cp) {
    add('checkpoint parses', false, 'malformed checkpoint')
    return
  }

  // Witness times are the only clock Rekor v2 offers — integratedTime is
  // always zero. Reported as observed, never as verified: checking them needs
  // each witness's key and a policy for how many must agree.
  const roundAt = timeOfRound(b.draw.params.target_round)
  if (index < 0 || !phase) {
    skip(
      'log head witnessed before the beacon round',
      'the anchored head predates the roster commitment, so its time bounds nothing',
    )
    skip('this draw was not abandoned and retried', "the anchor does not cover this draw's roster commitment")
    return
  }
  const at = earliestCosignature(cp)
  if (!at) {
    skip('log head witnessed before the beacon round', 'checkpoint carries no witness cosignatures')
  } else if (at < roundAt) {
    const gap = Math.round((roundAt.getTime() - at.getTime()) / 1000)
    skip(
      'log head witnessed before the beacon round',
      `${cosignatures(cp).length} witness(es), earliest ${at.toISOString()}, ${gap}s before round ${b.draw.params.target_round} — signatures not verified here`,
    )
  } else {
    add(
      'log head witnessed before the beacon round',
      false,
      `earliest witness ${at.toISOString()} is after round ${b.draw.params.target_round} published`,
    )
  }

  skip(
    'this draw was not abandoned and retried',
    `the anchor puts this draw on a public log; deciding it was the only one needs that log scanned for ${a.anchor_key.slice(0, 16)}\u2026`,
  )
}

export { toHex }

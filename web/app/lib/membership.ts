/**
 * Was this entrant in the roster the draw committed to?
 *
 * One question, asked from three places — the receipt link, a receipt file,
 * and the self-check on a draw page — and answered here once. The entrant
 * brings the two things only they hold, the salt they were handed at entry and
 * the exact label they gave; the evidence is either a Merkle path (served by
 * the organiser, or carried in a receipt) or the full published leaf list.
 *
 * Neither kind of evidence is trusted for the root it claims. Given the root
 * the draw committed to, the check reports whether the evidence agrees with it,
 * so nobody has to compare long hex strings by eye.
 */
import { toHex, fromHex, equal } from './hashing'
import { entryLeaf, rootFromLeaves, verifyPath } from './merkle'

/** What only the entrant holds. */
export interface EntrantKey {
  salt: string
  label: string
}

/** A Merkle path for one position, as a proof or a receipt carries it. */
export interface PathEvidence {
  root: string
  size: number
  index: number
  merkle_path: string[]
}

export type Evidence = { path: PathEvidence } | { leaves: string[] }

export interface Membership {
  /** The entry's leaf is in the evidence: at the path's index, or anywhere in the list. */
  included: boolean
  /** Its position, or -1. */
  index: number
  /** The leaf computed from the entrant's salt and label, or '' if they were unreadable. */
  leaf: string
  /** The root the evidence describes: the path's claimed root, or the one the leaves rebuild to. */
  root: string
  /**
   * Whether that root is the one the draw committed to. null when no committed
   * root was given to compare against — then only `included` is known.
   */
  matchesCommitted: boolean | null
  /** Included in the committed roster: true only when nothing is unknown or wrong. */
  ok: boolean
}

export function checkMembership(key: EntrantKey, evidence: Evidence, committedRoot?: string): Membership {
  let leaf: Uint8Array
  try {
    leaf = entryLeaf(key.salt.trim(), key.label)
  } catch {
    return { included: false, index: -1, leaf: '', root: '', matchesCommitted: null, ok: false }
  }
  const leafHex = toHex(leaf)

  let included = false
  let index = -1
  let root = ''
  try {
    if ('path' in evidence) {
      const p = evidence.path
      root = p.root
      included = verifyPath(leaf, p.index, p.size, p.merkle_path.map(fromHex), fromHex(p.root))
      index = included ? p.index : -1
    } else {
      index = evidence.leaves.indexOf(leafHex)
      included = index >= 0
      root = evidence.leaves.length ? toHex(rootFromLeaves(evidence.leaves.map(fromHex))) : ''
    }
  } catch {
    included = false
    index = -1
  }

  const matchesCommitted =
    committedRoot === undefined ? null : root !== '' && equalHex(root, committedRoot)
  return { included, index, leaf: leafHex, root, matchesCommitted, ok: included && matchesCommitted !== false }
}

function equalHex(a: string, b: string): boolean {
  try {
    return equal(fromHex(a), fromHex(b))
  } catch {
    return false
  }
}

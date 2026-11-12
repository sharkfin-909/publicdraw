/**
 * Merkle verification, mirroring the Go package `merkle`.
 *
 * Two details carry security weight and must not be "simplified":
 *
 *   Leaves and internal nodes hash under different tags. Without that, an
 *   internal node can be presented as a leaf, forging membership for data
 *   that was never committed (RFC 6962 separates them the same way).
 *
 *   Odd nodes are promoted unchanged, never duplicated. Duplicating the last
 *   node makes distinct leaf sets collide to one root — CVE-2012-2459.
 */
import { Tag, tagged, equal, u32, fromHex } from './hashing'

export function leafHash(salt: Uint8Array, entry: Uint8Array): Uint8Array {
  return tagged(Tag.leaf, salt, entry)
}

/** Entry.CanonicalBytes in Go: the label, length-prefixed. */
export function canonicalEntryBytes(label: string): Uint8Array {
  const enc = new TextEncoder().encode(label)
  const out = new Uint8Array(4 + enc.length)
  out.set(u32(enc.length), 0)
  out.set(enc, 4)
  return out
}

/**
 * The committed leaf of an entry under its salt — roster.LeafOf in Go. Every
 * inclusion check derives the leaf here, so they cannot drift apart.
 */
export function entryLeaf(saltHex: string, label: string): Uint8Array {
  return leafHash(fromHex(saltHex), canonicalEntryBytes(label))
}

export function nodeHash(left: Uint8Array, right: Uint8Array): Uint8Array {
  return tagged(Tag.node, left, right)
}

/** Build a root from every leaf. This is how a public roster is verified. */
export function rootFromLeaves(leaves: Uint8Array[]): Uint8Array {
  if (leaves.length === 0) throw new Error('merkle: empty leaf set')
  let cur = leaves
  while (cur.length > 1) {
    const next: Uint8Array[] = []
    for (let i = 0; i < cur.length; i += 2) {
      next.push(i + 1 < cur.length ? nodeHash(cur[i]!, cur[i + 1]!) : cur[i]!)
    }
    cur = next
  }
  return cur[0]!
}

/**
 * Recompute a root from one leaf and its sibling path.
 *
 * `size` is required, not decorative: it fixes each level's width, which is
 * what decides where nodes were promoted and therefore which levels the path
 * has entries for. A verifier told the wrong size walks a different shape.
 */
export function verifyPath(
  leaf: Uint8Array,
  index: number,
  size: number,
  path: Uint8Array[],
  root: Uint8Array,
): boolean {
  if (!Number.isInteger(index) || !Number.isInteger(size)) return false
  if (index < 0 || size <= 0 || index >= size) return false

  let cur = leaf
  let i = index
  let width = size
  let used = 0

  while (width > 1) {
    const sib = i ^ 1
    if (sib < width) {
      if (used >= path.length) return false // path too short
      const s = path[used]!
      used++
      cur = i % 2 === 0 ? nodeHash(cur, s) : nodeHash(s, cur)
    }
    i >>= 1
    width = (width + 1) >> 1
  }
  if (used !== path.length) return false // trailing junk in the path
  return equal(cur, root)
}

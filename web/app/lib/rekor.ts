/**
 * Rekor anchor verification, mirroring the Go package `rekor`.
 *
 * The anchor is what makes the organiser's own hash chain mean something to a
 * stranger: a chain nobody witnessed can simply be rebuilt, but an entry in
 * Sigstore's public append-only log cannot be withdrawn.
 *
 * Everything here is SHA-256 and string parsing, so it runs in the browser
 * with no new dependency — which matters, because an anchor checked by the
 * organiser's server would defeat the purpose of anchoring.
 */
import { sha256 } from '@noble/hashes/sha2'
import { concat, equal, fromHex } from './hashing'

export interface Anchor {
  payload: string // base64
  log_index: number
  tree_size: number
  root_hash: string // base64
  hashes: string[] // base64
  checkpoint: string
  body: string // base64
  anchor_key: string // base64 DER
  submitted_at: number
}

export interface Checkpoint {
  origin: string
  treeSize: number
  rootHash: string
  body: string
  signatures: { name: string; raw: string }[]
}

/** A witness's countersignature, carrying the moment it saw the tree. */
export interface Cosignature {
  witness: string
  at: Date
  /**
   * Always false. Checking these needs each witness's public key and a policy
   * for how many independent witnesses must agree; until that exists the time
   * is evidence worth showing, not a proven fact.
   */
  verified: boolean
}

function b64(s: string): Uint8Array {
  const bin = atob(s)
  const out = new Uint8Array(bin.length)
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i)
  return out
}

// RFC 6962 hashing: leaves and internal nodes are prefixed differently, for
// the same second-preimage reason as the roster tree.
const leafHash = (b: Uint8Array) => sha256(concat(new Uint8Array([0x00]), b))
const nodeHash = (l: Uint8Array, r: Uint8Array) => sha256(concat(new Uint8Array([0x01]), l, r))

/** Parse a C2SP signed note. */
export function parseCheckpoint(envelope: string): Checkpoint | null {
  const i = envelope.indexOf('\n\n')
  if (i < 0) return null
  const body = envelope.slice(0, i + 1)
  const sigBlock = envelope.slice(i + 2)

  const lines = body.replace(/\n+$/, '').split('\n')
  if (lines.length < 3) return null
  const treeSize = Number(lines[1])
  if (!Number.isFinite(treeSize) || treeSize <= 0) return null

  const signatures: { name: string; raw: string }[] = []
  for (const l of sigBlock.split('\n')) {
    if (!l.startsWith('— ')) continue
    const f = l.split(/\s+/)
    if (f.length < 3) continue
    signatures.push({ name: f[1]!, raw: f[2]! })
  }
  return { origin: lines[0]!, treeSize, rootHash: lines[2]!, body, signatures }
}

/**
 * Witness cosignatures and the times they carry.
 *
 * A C2SP cosignature blob is a 4-byte key hash then an 8-byte big-endian unix
 * time. The log's own signature carries no time, so it is skipped.
 */
export function cosignatures(cp: Checkpoint): Cosignature[] {
  const out: Cosignature[] = []
  for (const s of cp.signatures) {
    if (s.name === cp.origin) continue
    let blob: Uint8Array
    try {
      blob = b64(s.raw)
    } catch {
      continue
    }
    if (blob.length < 12) continue
    const ts = Number(new DataView(blob.buffer, blob.byteOffset + 4, 8).getBigUint64(0, false))
    // Reject anything implausible rather than surfacing a misparse as a date.
    if (ts < 1_600_000_000 || ts > 4_000_000_000) continue
    out.push({ witness: s.name, at: new Date(ts * 1000), verified: false })
  }
  return out
}

export function earliestCosignature(cp: Checkpoint): Date | null {
  const cos = cosignatures(cp)
  if (!cos.length) return null
  return cos.reduce((a, b) => (b.at < a.at ? b : a)).at
}

/**
 * Recompute the tree root from the anchored entry and its proof, and check it
 * against both the proof's stated root and the root the checkpoint signs.
 */
export function verifyInclusion(a: Anchor): { ok: boolean; error?: string } {
  try {
    if (!a.tree_size || a.log_index >= a.tree_size || a.log_index < 0) {
      return { ok: false, error: `index ${a.log_index} outside tree of ${a.tree_size}` }
    }
    let h = leafHash(b64(a.body))
    let fn = BigInt(a.log_index)
    let sn = BigInt(a.tree_size - 1)

    for (const p of a.hashes) {
      const sib = b64(p)
      if (fn % 2n === 1n || fn === sn) {
        h = nodeHash(sib, h)
        while (fn % 2n === 0n && fn !== 0n) {
          fn >>= 1n
          sn >>= 1n
        }
      } else {
        h = nodeHash(h, sib)
      }
      fn >>= 1n
      sn >>= 1n
    }

    if (!equal(h, b64(a.root_hash))) {
      return { ok: false, error: 'inclusion proof does not reach the stated root' }
    }
    const cp = parseCheckpoint(a.checkpoint)
    if (!cp) return { ok: false, error: 'checkpoint did not parse' }
    if (cp.rootHash !== a.root_hash) {
      return { ok: false, error: 'checkpoint root differs from the proof root' }
    }
    if (cp.treeSize !== a.tree_size) {
      return { ok: false, error: `checkpoint covers ${cp.treeSize}, proof claims ${a.tree_size}` }
    }
    return { ok: true }
  } catch (e) {
    return { ok: false, error: e instanceof Error ? e.message : String(e) }
  }
}

/** Whether this anchor is for exactly these bytes. */
export function bindsPayload(a: Anchor, payloadHex: string): boolean {
  try {
    return equal(b64(a.payload), fromHex(payloadHex))
  } catch {
    return false
  }
}

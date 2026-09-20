/**
 * drand quicknet: fetching rounds, and verifying them locally.
 *
 * The group public key below is pinned in this file on purpose. Verification
 * that fetched the key from the same place as the signature would prove
 * nothing — it is the pinning that makes a check by a stranger meaningful.
 */
import { bls12_381 as bls } from '@noble/curves/bls12-381'
import { sha256 } from '@noble/hashes/sha2'
import { u64, fromHex, toHex } from './hashing'
import type { BeaconRound } from './types'

export const QUICKNET = {
  name: 'quicknet',
  hash: '52db9ba70e0cc0f6eaf7803dd07447a1f5477735fd3f661792ba94600c84e971',
  publicKey:
    '83cf0f2896adee7eb8b5f01fcad3912212c437e0073e911fb90022d3e760183c8c4b450b6a0a6c3ac6a5776a2d1064510d1fec758c921cc22b0e17e63aaf4bcb5ed66304de9cf809bd274ca73bab4af5a6e9c76a4bc09e76eae8991ef5ece45a',
  genesisTime: 1692803367,
  periodSeconds: 3,
  /** Signatures live on G1 for this chain; the domain tag is part of the scheme. */
  dst: 'BLS_SIG_BLS12381G1_XMD:SHA-256_SSWU_RO_NUL_',
} as const

/** When a round publishes. The mapping is fixed, so a round number is a clock. */
export function timeOfRound(round: number): Date {
  return new Date((QUICKNET.genesisTime + (round - 1) * QUICKNET.periodSeconds) * 1000)
}

/** The latest round published at or before `at`. */
export function roundAt(at: Date): number {
  const d = Math.floor(at.getTime() / 1000) - QUICKNET.genesisTime
  if (d < 0) return 1
  return Math.floor(d / QUICKNET.periodSeconds) + 1
}

/** The randomness is SHA-256 of the signature, matching drand's own derivation. */
export function randomnessOf(r: BeaconRound): Uint8Array {
  return sha256(fromHex(r.signature))
}

/**
 * Verify a round's BLS signature against the pinned group key.
 *
 * The message is SHA-256 of the big-endian round number and nothing else,
 * because the chain is unchained — which is also what lets a future round be
 * named before it exists.
 */
export function verifyRound(r: BeaconRound): { ok: boolean; error?: string } {
  try {
    const msg = sha256(u64(r.round))
    const point = bls.shortSignatures.hash(msg, QUICKNET.dst)
    const ok = bls.shortSignatures.verify(fromHex(r.signature), point, fromHex(QUICKNET.publicKey))
    return ok ? { ok: true } : { ok: false, error: 'signature does not match this round' }
  } catch (e) {
    // A malformed or off-curve signature throws rather than returning false.
    return { ok: false, error: e instanceof Error ? e.message : 'invalid signature' }
  }
}

export async function fetchRound(base: string, round?: number): Promise<BeaconRound> {
  const which = round && round > 0 ? String(round) : 'latest'
  const res = await fetch(`${base}/${QUICKNET.hash}/public/${which}`)
  if (!res.ok) throw new Error(`drand: ${res.status} for round ${which}`)
  const body = (await res.json()) as { round: number; signature: string }
  const r: BeaconRound = { round: body.round, signature: body.signature }
  const v = verifyRound(r)
  if (!v.ok) throw new Error(`drand: ${v.error}`)
  return r
}

export function randomnessHex(r: BeaconRound): string {
  return toHex(randomnessOf(r))
}

/**
 * Domain-separated, length-prefixed hashing.
 *
 * This file is a reimplementation of the Go package `hashing`, and the two
 * must agree byte-for-byte. That redundancy is deliberate: a single
 * implementation that both produces and verifies proofs cannot catch its own
 * hashing bug — it computes a wrong digest and then confirms it. The contract
 * between the two is `testdata/testvectors.json`, which test/vectors.test.ts
 * replays against this code.
 *
 * Two rules, both load-bearing:
 *
 *   1. Every hash carries a context tag, so a digest made in one part of the
 *      protocol can never be replayed as a digest from another.
 *   2. Every variable-length input is length-prefixed, because H(a, b) is
 *      otherwise ambiguous: "ab"+"cdef" and "abc"+"def" are the same bytes.
 */
import { sha256 } from '@noble/hashes/sha2'

/** Context tags. Changing any of these is a breaking protocol change. */
export const Tag = {
  leaf: 'vc/pool/leaf/v1',
  node: 'vc/pool/node/v1',
  roster: 'vc/roster/commit/v1',
  draw: 'vc/draw/params/v1',
  rosterSig: 'vc/draw/roster/v1',
  seed: 'vc/draw/seed/v1',
  ticket: 'vc/draw/ticket/v1',
  logEntry: 'vc/log/entry/v1',
  logGenesis: 'vc/log/genesis/v1',
} as const

export const DIGEST_SIZE = 32

const enc = new TextEncoder()

export function utf8(s: string): Uint8Array {
  return enc.encode(s)
}

/** 4-byte big-endian. */
export function u32(n: number): Uint8Array {
  const b = new Uint8Array(4)
  new DataView(b.buffer).setUint32(0, n, false)
  return b
}

/**
 * 8-byte big-endian. Takes a bigint so round numbers and unix timestamps stay
 * exact past 2^53 — Go writes a uint64 and a JS number would quietly round.
 */
export function u64(n: number | bigint): Uint8Array {
  const b = new Uint8Array(8)
  new DataView(b.buffer).setBigUint64(0, BigInt(n), false)
  return b
}

/** Prefix bytes with their length as 4 big-endian bytes. */
export function lp(b: Uint8Array): Uint8Array {
  const out = new Uint8Array(4 + b.length)
  out.set(u32(b.length), 0)
  out.set(b, 4)
  return out
}

export function concat(...parts: Uint8Array[]): Uint8Array {
  let n = 0
  for (const p of parts) n += p.length
  const out = new Uint8Array(n)
  let at = 0
  for (const p of parts) {
    out.set(p, at)
    at += p.length
  }
  return out
}

/** SHA-256 over the tag followed by each length-prefixed part. */
export function tagged(tag: string, ...parts: Uint8Array[]): Uint8Array {
  return sha256(concat(utf8(tag), ...parts.map(lp)))
}

export function toHex(b: Uint8Array): string {
  let s = ''
  for (const x of b) s += x.toString(16).padStart(2, '0')
  return s
}

export function fromHex(s: string): Uint8Array {
  const clean = s.startsWith('0x') ? s.slice(2) : s
  if (clean.length % 2 !== 0) throw new Error('hex string has an odd length')
  const out = new Uint8Array(clean.length / 2)
  for (let i = 0; i < out.length; i++) {
    const byte = Number.parseInt(clean.slice(i * 2, i * 2 + 2), 16)
    if (Number.isNaN(byte)) throw new Error(`invalid hex at byte ${i}`)
    out[i] = byte
  }
  return out
}

export function equal(a: Uint8Array, b: Uint8Array): boolean {
  if (a.length !== b.length) return false
  let diff = 0
  for (let i = 0; i < a.length; i++) diff |= a[i]! ^ b[i]!
  return diff === 0
}

/** Compare two digests as big-endian numbers. Used to order draw tickets. */
export function compareBytes(a: Uint8Array, b: Uint8Array): number {
  const n = Math.min(a.length, b.length)
  for (let i = 0; i < n; i++) {
    if (a[i] !== b[i]) return a[i]! < b[i]! ? -1 : 1
  }
  return a.length - b.length
}

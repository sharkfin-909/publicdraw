/** Transparency-log replay, mirroring the Go package `translog`. */
import { Tag, tagged, u64, utf8, fromHex, toHex, equal } from './hashing'
import type { LogRecord } from './types'

/** The head of an empty log, bound to the log's identity. */
export function genesis(logId: string): Uint8Array {
  return tagged(Tag.logGenesis, utf8(logId))
}

export function headAfter(
  prev: Uint8Array,
  seq: number,
  kind: string,
  drawId: string,
  payload: Uint8Array,
  loggedAt: number,
): Uint8Array {
  return tagged(Tag.logEntry, prev, u64(seq), utf8(kind), utf8(drawId), payload, u64(loggedAt))
}

export interface ReplayResult {
  ok: boolean
  head: string
  error?: string
}

/**
 * The log's identity, as the service derives it from the operator key
 * (cmd/publicdraw: "vc/log/" + the first 16 hex characters of the key).
 */
export function logIdFor(operatorKey: string): string {
  return `vc/log/${operatorKey.slice(0, 16)}`
}

/**
 * Check a published log: replay it from the genesis of the operator's log,
 * and require it to arrive at the head the server claims. A chain that
 * replays but ends somewhere else is not a clean log.
 */
export function checkLog(records: LogRecord[], claimedHead: string, operatorKey: string): ReplayResult {
  const r = replay(records, logIdFor(operatorKey))
  if (r.ok && r.head !== claimedHead.toLowerCase()) {
    return { ok: false, head: r.head, error: 'the replayed head does not match the head the server published' }
  }
  return r
}

/** Replay a chain from genesis and report the head it arrives at. */
export function replay(records: LogRecord[], logId: string): ReplayResult {
  let head = genesis(logId)
  for (let i = 0; i < records.length; i++) {
    const r = records[i]!
    if (r.seq !== i) return { ok: false, head: toHex(head), error: `record ${i} is out of sequence` }
    if (!equal(fromHex(r.prev), head)) {
      return { ok: false, head: toHex(head), error: `record ${i} breaks the chain` }
    }
    const want = headAfter(fromHex(r.prev), r.seq, r.kind, r.draw_id, fromHex(r.payload), r.logged_at)
    if (!equal(want, fromHex(r.head))) {
      return { ok: false, head: toHex(head), error: `record ${i} has a forged head` }
    }
    head = want
  }
  return { ok: true, head: toHex(head) }
}

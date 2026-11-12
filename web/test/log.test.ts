import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { checkLog, logIdFor } from '../app/lib/translog'
import type { Bundle } from '../app/lib/types'

const at = (n: string) => fileURLToPath(new URL(`./fixtures/${n}`, import.meta.url))
const bundle = JSON.parse(readFileSync(at('live-bundle.json'), 'utf8')) as Bundle
const key = bundle.operator_pubkey

describe('checking the published log', () => {
  it('derives the log id the service uses', () => {
    expect(logIdFor(key)).toBe(bundle.log.log_id)
  })

  it('accepts a log that replays to the head the server claims', () => {
    expect(checkLog(bundle.log.records, bundle.log.head, key)).toMatchObject({ ok: true, head: bundle.log.head })
  })

  it('rejects a log that replays cleanly but ends at a different head', () => {
    const r = checkLog(bundle.log.records.slice(0, -1), bundle.log.head, key)
    expect(r.ok).toBe(false)
    expect(r.error).toMatch(/does not match/)
  })

  it('rejects a log replayed under another operator’s identity', () => {
    expect(checkLog(bundle.log.records, bundle.log.head, 'f'.repeat(64)).ok).toBe(false)
  })
})

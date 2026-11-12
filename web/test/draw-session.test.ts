import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { createDrawSession, phaseOf, type Clock, type DrawPort, type LeafList } from '../app/lib/drawSession'
import { timeOfRound } from '../app/lib/beacon'
import type { Bundle, DrawSummary } from '../app/lib/types'

const at = (n: string) => fileURLToPath(new URL(`./fixtures/${n}`, import.meta.url))
const bundle = JSON.parse(readFileSync(at('live-bundle.json'), 'utf8')) as Bundle
const roundMs = timeOfRound(bundle.draw.params.target_round).getTime()
const cutoffMs = bundle.draw.params.cutoff_at * 1000

function summary(state: DrawSummary['state'], over: Partial<DrawSummary> = {}): DrawSummary {
  return {
    params: bundle.draw.params,
    params_sig: bundle.draw.signature,
    state,
    entry_count: bundle.roster.commitment.size,
    visibility: 'unlisted',
    ...(state === 'open' ? {} : { roster_commit: bundle.roster.commitment, roster_sig: bundle.roster.signature }),
    ...over,
  } as DrawSummary
}

/** A clock the test winds by hand. */
function fakeClock(start: number) {
  let now = start
  const ticks: (() => void)[] = []
  const clock: Clock = {
    now: () => now,
    every: (_ms, tick) => {
      ticks.push(tick)
      return () => ticks.splice(ticks.indexOf(tick), 1)
    },
  }
  return {
    clock,
    running: () => ticks.length,
    async advance(ms: number) {
      now += ms
      ticks.forEach((t) => t())
      await new Promise((r) => setTimeout(r, 0))
    },
  }
}

/** A draw service whose draw the test changes, counting every call. */
function fakeService(initial: DrawSummary) {
  let current = initial
  const calls = { getDraw: 0, bundle: 0, leaves: 0 }
  let bundleFails = false
  const leaves: LeafList = { root: bundle.roster.commitment.root, size: bundle.roster.leaves.length, leaves: bundle.roster.leaves }
  const port: DrawPort = {
    getDraw: async () => (calls.getDraw++, current),
    bundle: async () => {
      calls.bundle++
      if (bundleFails) throw new Error('bundle unavailable')
      return bundle
    },
    leaves: async () => (calls.leaves++, leaves),
  }
  return {
    port,
    calls,
    set: (d: DrawSummary) => (current = d),
    failBundle: () => (bundleFails = true),
  }
}

describe('phaseOf', () => {
  it('follows a draw through its life', () => {
    expect(phaseOf(summary('open'), cutoffMs - 1)).toBe('open')
    expect(phaseOf(summary('open'), cutoffMs)).toBe('closing')
    expect(phaseOf(summary('open', { entry_count: 0 }), cutoffMs)).toBe('stalled')
    expect(phaseOf(summary('open'), roundMs)).toBe('stalled')
    expect(phaseOf(summary('committed'), roundMs - 1)).toBe('committed')
    expect(phaseOf(summary('committed'), roundMs)).toBe('deciding')
    expect(phaseOf(summary('drawn'), roundMs)).toBe('drawn')
  })
})

describe('a draw session', () => {
  it('gives a fresh reader of a frozen draw the leaves, so the self-check can appear', async () => {
    const svc = fakeService(summary('committed'))
    const t = fakeClock(roundMs - 60_000)
    const s = createDrawSession(() => 'd', svc.port, { summary: summary('committed') }, { clock: t.clock })
    expect(s.state.checkLeaves).toBeUndefined()
    await s.start()
    expect(svc.calls.leaves).toBe(1)
    expect(s.state.checkLeaves).toEqual(bundle.roster.leaves)
  })

  it('uses a drawn bundle’s own leaves and verifies it, fetching no separate list', async () => {
    const svc = fakeService(summary('drawn'))
    const t = fakeClock(roundMs + 60_000)
    let verified = 0
    const s = createDrawSession(() => 'd', svc.port, { summary: summary('drawn') }, {
      clock: t.clock,
      verify: () => (verified++, { ok: true, skipped: 0, checks: [] }),
    })
    await s.start()
    expect(svc.calls.bundle).toBe(1)
    expect(svc.calls.leaves).toBe(0)
    expect(s.state.checkLeaves).toEqual(bundle.roster.leaves)
    expect(verified).toBe(1)
    expect(s.state.report?.ok).toBe(true)
  })

  it('notices the server freezing the draw at the cutoff', async () => {
    const svc = fakeService(summary('open'))
    const t = fakeClock(cutoffMs - 500)
    const s = createDrawSession(() => 'd', svc.port, { summary: summary('open') }, { clock: t.clock })
    await s.start()
    expect(s.state.phase).toBe('open')
    await t.advance(1000)
    expect(s.state.phase).toBe('closing')
    svc.set(summary('committed'))
    await t.advance(2000) // polls are POLL_MS apart
    expect(s.state.phase).toBe('committed')
    expect(s.state.checkLeaves).toBeDefined()
  })

  it('keeps looking while a result is due, and stops once it arrives', async () => {
    const svc = fakeService(summary('committed'))
    const t = fakeClock(roundMs + 1000)
    const s = createDrawSession(() => 'd', svc.port, { summary: summary('committed') }, { clock: t.clock })
    await s.start()
    expect(s.state.phase).toBe('deciding')
    await t.advance(2000)
    await t.advance(2000)
    expect(svc.calls.getDraw).toBe(2)
    svc.set(summary('drawn'))
    await t.advance(2000)
    expect(s.state.phase).toBe('drawn')
    expect(s.state.bundle).toEqual(bundle)
    const seen = svc.calls.getDraw
    await t.advance(10_000)
    expect(svc.calls.getDraw).toBe(seen)
  })

  it('does not poll while nothing is expected to change', async () => {
    const svc = fakeService(summary('open'))
    const t = fakeClock(cutoffMs - 60_000)
    const s = createDrawSession(() => 'd', svc.port, { summary: summary('open') }, { clock: t.clock })
    await s.start()
    await t.advance(20_000)
    expect(svc.calls.getDraw).toBe(0)
  })

  it('tells a drawn draw whose bundle failed to load apart from one not drawn yet', async () => {
    const svc = fakeService(summary('drawn'))
    svc.failBundle()
    const t = fakeClock(roundMs + 60_000)
    const s = createDrawSession(() => 'd', svc.port, { summary: summary('drawn') }, { clock: t.clock })
    await s.start()
    expect(s.state.bundle).toBeUndefined()
    expect(s.state.bundleError).toBe('bundle unavailable')
  })

  it('stops its clock when stopped', async () => {
    const t = fakeClock(0)
    const s = createDrawSession(() => 'd', fakeService(summary('open')).port, { summary: summary('open') }, { clock: t.clock })
    await s.start()
    expect(t.running()).toBe(1)
    s.stop()
    expect(t.running()).toBe(0)
  })
})

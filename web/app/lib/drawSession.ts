/**
 * A draw as one reader sees it, over time.
 *
 * A draw page has to know where the draw is in its life, what that stage
 * needs from the server, and when to look again — the server freezes the
 * roster at the cutoff and a result appears whenever the organiser runs it,
 * so the page has to notice both without being told. That knowledge used to
 * live in the page itself, untested, and it had gone wrong: a reader opening
 * a frozen or drawn draw never got the published leaves, so the self-check
 * never appeared, and nothing looked again while a result was due.
 *
 * It lives here instead, behind two seams so it can be driven by a test: the
 * draw service (DrawPort) and the passage of time (Clock). It has no Nuxt in
 * it; only Vue's reactivity, so a page can render its state directly.
 */
import { reactive } from 'vue'
import { timeOfRound } from './beacon'
import type { Bundle, DrawSummary } from './types'
import type { Report } from './verify'

export interface LeafList {
  root: string
  size: number
  leaves: string[]
}

/** What the session needs from the draw service. */
export interface DrawPort {
  getDraw(id: string): Promise<DrawSummary>
  bundle(id: string): Promise<Bundle>
  leaves(id: string): Promise<LeafList>
}

/** The passage of time: a reading, and a way to be called back. */
export interface Clock {
  now(): number
  every(ms: number, tick: () => void): () => void
}

export const systemClock: Clock = {
  now: () => Date.now(),
  every: (ms, tick) => {
    const t = setInterval(tick, ms)
    return () => clearInterval(t)
  },
}

/**
 * Where a draw is, as a reader experiences it — finer than the server's three
 * states, because what a page should do differs within them.
 *
 *   open       entries are being taken
 *   closing    the cutoff has passed and the server is about to freeze it
 *   stalled    the cutoff passed with too few entries, or the round published
 *              before it froze: it cannot be completed
 *   committed  the roster is frozen and the deciding round has not published
 *   deciding   the round has published; the result is due
 *   drawn      the result is published
 */
export type Phase = 'open' | 'closing' | 'stalled' | 'committed' | 'deciding' | 'drawn'

export function phaseOf(d: DrawSummary, nowMs: number): Phase {
  if (d.state === 'drawn') return 'drawn'
  const roundAt = timeOfRound(d.params.target_round).getTime()
  if (d.state === 'committed') return nowMs >= roundAt ? 'deciding' : 'committed'
  if (nowMs < d.params.cutoff_at * 1000) return 'open'
  if (d.entry_count < d.params.winners || nowMs >= roundAt) return 'stalled'
  return 'closing'
}

/** Phases in which the server is expected to change the draw without the reader doing anything. */
const WATCHED: Phase[] = ['closing', 'deciding']

export const TICK_MS = 1000
export const POLL_MS = 2000

export interface SessionOptions {
  clock?: Clock
  /** Verifies a bundle. Leave it out on the server: verification belongs to the reader's browser. */
  verify?: (b: Bundle) => Report
  /** Turns a failure into something worth showing a person. */
  describe?: (e: unknown) => string
  /**
   * The first clock reading. A server-rendered page passes the server's, so
   * the browser's first render agrees with it and hydration does not fail.
   */
  startAt?: number
}

export interface SessionState {
  summary?: DrawSummary
  bundle?: Bundle
  leaves?: LeafList
  report?: Report
  /** Loading the draw failed. */
  error: string
  /** The draw is drawn but its bundle could not be fetched — not the same as "not drawn yet". */
  bundleError: string
  now: number
  readonly phase?: Phase
  /** The leaves an entrant's self-check should use: the verified bundle's own when there is one. */
  readonly checkLeaves?: string[]
}

export interface DrawSession {
  state: SessionState
  /** Fetch the draw again, then whatever its phase now needs. */
  refresh(): Promise<void>
  /** Replace the draw outright — after server rendering, or on moving to another draw. */
  reset(initial: { summary: DrawSummary; bundle?: Bundle } | undefined): Promise<void>
  /** Start the clock and fetch what the current phase needs. */
  start(): Promise<void>
  stop(): void
}

export function createDrawSession(
  id: () => string,
  port: DrawPort,
  initial?: { summary: DrawSummary; bundle?: Bundle },
  opts: SessionOptions = {},
): DrawSession {
  const clock = opts.clock ?? systemClock
  const describe = opts.describe ?? ((e) => (e instanceof Error ? e.message : String(e)))

  const state = reactive<SessionState>({
    summary: initial?.summary,
    bundle: initial?.bundle,
    leaves: undefined,
    report: undefined,
    error: '',
    bundleError: '',
    now: opts.startAt ?? clock.now(),
    get phase() {
      return this.summary ? phaseOf(this.summary, this.now) : undefined
    },
    get checkLeaves() {
      return this.bundle?.roster.leaves ?? this.leaves?.leaves
    },
  })

  let lastPoll = -Infinity
  let inflight: Promise<void> | undefined
  let stopTicking: (() => void) | undefined

  /** Fetch only what the phase needs and the session does not yet have. */
  async function fill() {
    const d = state.summary
    if (!d) return
    if (d.state === 'drawn' && !state.bundle) {
      try {
        state.bundle = await port.bundle(id())
        state.bundleError = ''
      } catch (e) {
        state.bundleError = describe(e)
      }
    }
    // A drawn bundle carries its leaves; only a frozen, undrawn draw needs the list.
    if (d.state !== 'open' && !state.bundle && !state.leaves) {
      try {
        state.leaves = await port.leaves(id())
      } catch {
        // Without the list the self-check stays hidden; nothing else depends on it.
      }
    }
    if (state.bundle && opts.verify && !state.report) state.report = opts.verify(state.bundle)
  }

  async function load() {
    try {
      state.summary = await port.getDraw(id())
      state.error = ''
    } catch (e) {
      state.error = describe(e)
      return
    }
    await fill()
  }

  function refresh() {
    inflight ??= load().finally(() => (inflight = undefined))
    return inflight
  }

  function tick() {
    state.now = clock.now()
    const p = state.phase
    if (p && WATCHED.includes(p) && state.now - lastPoll >= POLL_MS) {
      lastPoll = state.now
      void refresh()
    }
  }

  return {
    state,
    refresh,
    async reset(next) {
      state.summary = next?.summary
      state.bundle = next?.bundle
      state.leaves = undefined
      state.report = undefined
      state.bundleError = ''
      await fill()
    },
    async start() {
      stopTicking?.()
      stopTicking = clock.every(TICK_MS, tick)
      await fill()
    },
    stop() {
      stopTicking?.()
      stopTicking = undefined
    },
  }
}

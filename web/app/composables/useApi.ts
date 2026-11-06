import type {
  Admission,
  Bundle,
  DrawSummary,
  LogRecord,
  Proof,
  PublicDraw,
  Receipt,
  RosterCommitment,
  DrawState,
} from '~/lib/types'

/**
 * Thin wrapper over the draw service.
 *
 * Nothing here is trusted. Everything the API returns that matters is
 * re-checked in the browser by `~/lib/verify` against a pinned drand key, so
 * these calls are a transport, not an authority.
 */
export function useApi() {
  const cfg = useRuntimeConfig()
  // On the server there is no browser proxy to ride, so talk to the service
  // directly. Pages must render their content server-side: a social scraper
  // does not run JavaScript, and a draw result that only appears after
  // hydration has no Open Graph preview and no reliable crawl.
  const base = import.meta.server ? cfg.apiInternal : cfg.public.apiBase

  const { token } = useOrganiser()

  const call = <T>(path: string, opts: Parameters<typeof $fetch>[1] = {}) =>
    $fetch<T>(`${base}${path}`, opts as never)

  /** The routes only an organiser may use. */
  const organiser = <T>(path: string, opts: Parameters<typeof $fetch>[1] = {}) =>
    call<T>(path, { ...opts, headers: { Authorization: `Bearer ${token.value}` } })

  return {
    health: () => call<{ ok: boolean; log_length: number }>('/health'),

    operator: () =>
      call<{
        public_key: string
        chain: { name: string; hash: string; period_seconds: number }
        log_head: string
      }>('/operator'),

    listDraws: () => organiser<{ draws: string[] }>('/draws'),

    /** Only draws whose organiser opted in. Backs the directory and sitemap. */
    listPublic: () => call<{ draws: PublicDraw[] }>('/public/draws'),

    setVisibility: (id: string, visibility: 'public' | 'unlisted') =>
      organiser<{ draw_id: string; visibility: string }>(`/draws/${id}/visibility`, {
        method: 'PUT',
        body: { visibility },
      }),

    getDraw: (id: string) => call<DrawSummary>(`/draws/${id}`),

    createDraw: (body: {
      title: string
      winners: number
      disclosure: string
      open_for_seconds?: number
      cutoff_at?: number
    }) => organiser<DrawSummary>('/draws', { method: 'POST', body }),

    /**
     * Returns one salt per entrant, and this is the only time they are
     * disclosed. An entrant who keeps theirs can prove their own inclusion
     * later from the published leaves alone.
     */
    addEntries: (id: string, labels: string[]) =>
      organiser<{ admitted: Admission[] }>(`/draws/${id}/entries`, {
        method: 'POST',
        body: { labels },
      }),

    leaves: (id: string) =>
      call<{ root: string; size: number; leaves: string[] }>(`/draws/${id}/leaves`),

    commit: (id: string) =>
      organiser<{ roster_commit: RosterCommitment; roster_sig: string; state: DrawState }>(
        `/draws/${id}/commit`,
        { method: 'POST' },
      ),

    /**
     * The path only. Salt and label stay with the entrant, who was given both
     * once at entry — serving them back disclosed the whole roster to anyone
     * who counted from zero.
     */
    proof: (id: string, index: number) => call<Proof>(`/draws/${id}/proof/${index}`),

    run: (id: string) => organiser<Bundle>(`/draws/${id}/run`, { method: 'POST' }),

    bundle: (id: string) => call<Bundle>(`/draws/${id}/bundle`),

    log: () => call<{ records: LogRecord[]; head: string; length: number }>('/log'),

    abandoned: () => call<{ abandoned: string[]; note: string }>('/log/abandoned'),
  }
}

/** Turn an unknown thrown value into something worth showing a person. */
export function errorText(e: unknown): string {
  if (typeof e === 'object' && e && 'data' in e) {
    const d = (e as { data?: { error?: string } }).data
    if (d?.error) return d.error
  }
  return e instanceof Error ? e.message : String(e)
}

import type {
  Admission,
  Bundle,
  DrawSummary,
  LogRecord,
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
  const base = useRuntimeConfig().public.apiBase

  const call = <T>(path: string, opts: Parameters<typeof $fetch>[1] = {}) =>
    $fetch<T>(`${base}${path}`, opts as never)

  return {
    health: () => call<{ ok: boolean; log_length: number }>('/health'),

    operator: () =>
      call<{
        public_key: string
        chain: { name: string; hash: string; period_seconds: number }
        log_head: string
      }>('/operator'),

    listDraws: () => call<{ draws: string[] }>('/draws'),

    getDraw: (id: string) => call<DrawSummary>(`/draws/${id}`),

    createDraw: (body: {
      title: string
      winners: number
      disclosure: string
      open_for_seconds?: number
      cutoff_at?: number
    }) => call<DrawSummary>('/draws', { method: 'POST', body }),

    /**
     * Returns one salt per entrant, and this is the only time they are
     * disclosed. An entrant who keeps theirs can prove their own inclusion
     * later from the published leaves alone.
     */
    addEntries: (id: string, labels: string[]) =>
      call<{ admitted: Admission[] }>(`/draws/${id}/entries`, {
        method: 'POST',
        body: { labels },
      }),

    leaves: (id: string) =>
      call<{ root: string; size: number; leaves: string[] }>(`/draws/${id}/leaves`),

    commit: (id: string) =>
      call<{ roster_commit: RosterCommitment; roster_sig: string; state: DrawState }>(
        `/draws/${id}/commit`,
        { method: 'POST' },
      ),

    receipt: (id: string, index: number) => call<Receipt>(`/draws/${id}/receipt/${index}`),

    run: (id: string) => call<Bundle>(`/draws/${id}/run`, { method: 'POST' }),

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

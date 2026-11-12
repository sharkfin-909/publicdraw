import type { Ref } from 'vue'
import { createDrawSession } from '~/lib/drawSession'
import { verifyBundle } from '~/lib/verify'
import type { Bundle, DrawSummary } from '~/lib/types'

/**
 * The draw session, wired to the draw service and the page's lifecycle.
 *
 * Verification runs only in the browser: it must run on the reader's machine
 * against the pinned drand key, never on a server they would then have to
 * trust. The server renders the result; the browser checks it.
 */
export function useDrawSession(
  id: () => string,
  initial: Ref<{ summary: DrawSummary; bundle?: Bundle } | undefined | null>,
) {
  const api = useApi()
  // Shared with the browser through the payload, so both render the same phase.
  const startAt = useState('draw-now', () => Date.now())
  const session = createDrawSession(
    id,
    { getDraw: api.getDraw, bundle: api.bundle, leaves: api.leaves },
    initial.value ?? undefined,
    { verify: import.meta.client ? verifyBundle : undefined, describe: errorText, startAt: startAt.value },
  )
  watch(initial, (v) => session.reset(v ?? undefined))
  onMounted(() => session.start())
  onUnmounted(() => session.stop())
  return session
}

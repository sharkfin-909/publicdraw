import { timeOfRound } from '~/lib/beacon'

/**
 * A live countdown to a drand round.
 *
 * This is not decoration. The gap between the roster being frozen and the
 * round publishing is the guarantee the whole protocol rests on, and it is the
 * only part of it a person can watch happen. Showing the round number ticking
 * toward a value nobody can yet compute is the most legible proof this system
 * has.
 */
export function useCountdown(round: MaybeRefOrGetter<number | undefined>) {
  const now = ref(Date.now())
  let timer: ReturnType<typeof setInterval> | undefined

  onMounted(() => {
    timer = setInterval(() => (now.value = Date.now()), 250)
  })
  onUnmounted(() => timer && clearInterval(timer))

  const target = computed(() => {
    const r = toValue(round)
    return r ? timeOfRound(r) : undefined
  })

  const msRemaining = computed(() => {
    const t = target.value
    return t ? t.getTime() - now.value : 0
  })

  const published = computed(() => msRemaining.value <= 0)

  const label = computed(() => {
    const ms = msRemaining.value
    if (ms <= 0) return 'published'
    const s = Math.ceil(ms / 1000)
    if (s < 60) return `${s}s`
    const m = Math.floor(s / 60)
    if (m < 60) return `${m}m ${s % 60}s`
    const h = Math.floor(m / 60)
    if (h < 24) return `${h}h ${m % 60}m`
    return `${Math.floor(h / 24)}d ${h % 24}h`
  })

  return { target, msRemaining, published, label, now }
}

export function fmtTime(unix: number | undefined): string {
  if (!unix) return '—'
  return new Date(unix * 1000).toLocaleString(undefined, {
    dateStyle: 'medium',
    timeStyle: 'medium',
  })
}

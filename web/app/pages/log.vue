<script setup lang="ts">
import { replay } from '~/lib/translog'
import type { LogRecord } from '~/lib/types'

useSeo({
  title: 'Transparency log',
  description:
    'Every draw\u2019s creation, cutoff and result, chained so that editing or removing any record breaks everything after it \u2014 and so a draw run and quietly abandoned is visible.',
  path: '/log',
})

const api = useApi()
const records = ref<LogRecord[]>([])
const head = ref('')
const abandoned = ref<string[]>([])
const err = ref('')
const chain = ref<{ ok: boolean; head: string; error?: string }>()
const logId = ref('')

async function load() {
  err.value = ''
  try {
    const [l, a, op] = await Promise.all([api.log(), api.abandoned(), api.operator()])
    records.value = l.records ?? []
    head.value = l.head
    abandoned.value = a.abandoned ?? []
    // The service derives its log id from the operator key; replaying the
    // chain here is what makes this page a check rather than a display.
    logId.value = `vc/log/${op.public_key.slice(0, 16)}`
    chain.value = replay(records.value, logId.value)
  } catch (e) {
    err.value = errorText(e)
  }
}

const kindLabel: Record<string, string> = {
  draw_created: 'parameters committed',
  roster_committed: 'roster frozen',
  result_published: 'result published',
}

onMounted(load)
</script>

<template>
  <main class="wrap stack">
    <header>
      <p class="eyebrow">append-only</p>
      <h1>Transparency log</h1>
      <p class="muted" style="max-width: 62ch">
        Every draw's creation, cutoff and result, chained so that editing or removing any record
        breaks everything after it. This is the only place a draw that was run and quietly
        abandoned can show up.
      </p>
    </header>

    <div v-if="err" class="error">{{ err }}</div>

    <section v-if="chain" class="card">
      <div class="spread">
        <h2>Chain</h2>
        <span class="pill" :class="chain.ok ? 'drawn' : 'open'">
          {{ chain.ok ? 'replays cleanly' : 'broken' }}
        </span>
      </div>
      <p class="muted" style="font-size: 0.85rem; margin-top: 8px">
        <template v-if="chain.ok">
          Replayed from genesis in your browser: {{ records.length }} records, arriving at the head
          the server reports.
        </template>
        <template v-else>{{ chain.error }}</template>
      </p>
      <p class="hash">head {{ head }}</p>
      <p
        v-if="chain.ok && chain.head !== head"
        class="notice"
      >
        The replayed head does not match the head the server published.
      </p>
    </section>

    <section v-if="abandoned.length" class="card" style="border-color: var(--amber)">
      <h2 style="color: var(--amber)">
        {{ abandoned.length }} draw{{ abandoned.length === 1 ? '' : 's' }} with no published result
      </h2>
      <p class="muted" style="font-size: 0.85rem">
        Created, and their round has since published, but no result was ever recorded. This is not
        proof of misconduct — draws get cancelled for honest reasons — but a draw run repeatedly
        until the organiser liked the winner could only look like this.
      </p>
      <ul class="mono" style="font-size: 0.8rem">
        <li v-for="d in abandoned" :key="d">
          <NuxtLink :to="`/draws/${d}`">{{ d }}</NuxtLink>
        </li>
      </ul>
    </section>

    <section v-if="records.length" class="card">
      <h2 style="margin-bottom: 12px">{{ records.length }} records</h2>
      <div class="scroller">
        <table>
          <thead>
            <tr><th>#</th><th>Event</th><th>Draw</th><th>Logged</th><th>Head</th></tr>
          </thead>
          <tbody>
            <tr v-for="r in records" :key="r.seq">
              <td class="mono">{{ r.seq }}</td>
              <td>{{ kindLabel[r.kind] ?? r.kind }}</td>
              <td class="mono"><NuxtLink :to="`/draws/${r.draw_id}`">{{ r.draw_id.slice(0, 12) }}…</NuxtLink></td>
              <td class="muted">{{ fmtTime(r.logged_at) }}</td>
              <td class="hash">{{ r.head.slice(0, 12) }}…</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </main>
</template>

<script setup lang="ts">
import type { DrawSummary } from '~/lib/types'
import { drawPath } from '~/utils/seo'

useSeo({
  title: 'Organiser dashboard',
  description: 'Create and run draws.',
  path: '/admin',
  index: false,
})

const api = useApi()
const draws = ref<DrawSummary[]>([])
const operator = ref<Awaited<ReturnType<typeof api.operator>>>()
const loadError = ref('')
const busy = ref(false)

const form = reactive({
  title: '',
  winners: 1,
  disclosure: 'private' as 'public' | 'private',
  openMinutes: 10,
})
const createError = ref('')

async function load() {
  loadError.value = ''
  try {
    const [{ draws: ids }, op] = await Promise.all([api.listDraws(), api.operator()])
    operator.value = op
    draws.value = await Promise.all((ids ?? []).map((id) => api.getDraw(id)))
    draws.value.sort((a, b) => b.params.created_at - a.params.created_at)
  } catch (e) {
    loadError.value = errorText(e)
  }
}

async function create() {
  createError.value = ''
  busy.value = true
  try {
    const made = await api.createDraw({
      title: form.title,
      winners: form.winners,
      disclosure: form.disclosure,
      open_for_seconds: Math.round(form.openMinutes * 60),
    })
    form.title = ''
    await navigateTo(drawPath(made.params.draw_id, made.params.title))
  } catch (e) {
    createError.value = errorText(e)
  } finally {
    busy.value = false
  }
}

onMounted(load)
</script>

<template>
  <main class="wrap stack">
    <header>
      <p class="eyebrow">organiser</p>
      <h1>Dashboard</h1>
      <p class="muted" style="max-width: 62ch">
        Every draw here commits to its rules before anyone enters, and to its entry list before the
        randomness that decides it exists. Results verify in your own browser.
      </p>
    </header>

    <div v-if="loadError" class="error">
      {{ loadError }} — is the draw service running? <code class="mono">publicdraw serve</code>
    </div>

    <section class="card">
      <h2>New draw</h2>
      <p class="muted" style="font-size: 0.86rem">
        These parameters are signed and logged now, before a single entry arrives. Changing any of
        them later produces a different draw.
      </p>
      <form class="stack" style="gap: 12px" @submit.prevent="create">
        <div>
          <label for="title">What is this draw for?</label>
          <input id="title" v-model="form.title" required placeholder="Two community grants" />
        </div>
        <div class="grid">
          <div>
            <label for="winners">Winners</label>
            <input id="winners" v-model.number="form.winners" type="number" min="1" required />
          </div>
          <div>
            <label for="open">Entries open for (minutes)</label>
            <input id="open" v-model.number="form.openMinutes" type="number" min="1" step="1" required />
          </div>
          <div>
            <label for="disclosure">Entry list</label>
            <select id="disclosure" v-model="form.disclosure">
              <option value="private">Private — winners only</option>
              <option value="public">Public — publish every entry</option>
            </select>
          </div>
        </div>
        <p class="muted" style="font-size: 0.8rem; margin: 0">
          <template v-if="form.disclosure === 'private'">
            Entrants confirm their own inclusion with a receipt; nobody else's entry is published.
          </template>
          <template v-else>
            Every entry is published and a verifier rebuilds the whole tree. Only use this where
            entrants would not be harmed by being named.
          </template>
        </p>
        <div v-if="createError" class="error">{{ createError }}</div>
        <div class="row">
          <button type="submit" :disabled="busy || !form.title">
            {{ busy ? 'Creating…' : 'Create and commit parameters' }}
          </button>
        </div>
      </form>
    </section>

    <section v-if="draws.length">
      <h2 style="margin-bottom: 12px">All draws</h2>
      <div class="stack" style="gap: 10px">
        <NuxtLink v-for="d in draws" :key="d.params.draw_id" :to="drawPath(d.params.draw_id, d.params.title)" class="card drawrow">
          <div class="spread">
            <div>
              <div class="dtitle">{{ d.params.title || 'Untitled draw' }}</div>
              <div class="muted mono" style="font-size: 0.74rem">{{ d.params.draw_id }}</div>
            </div>
            <div class="row" style="gap: 10px">
              <span class="pill" :class="d.state">{{ d.state }}</span>
              <span class="muted" style="font-size: 0.8rem">
                {{ d.entry_count }} {{ d.entry_count === 1 ? 'entry' : 'entries' }} ·
                {{ d.params.winners }} to win · {{ d.params.disclosure }}
              </span>
            </div>
          </div>
        </NuxtLink>
      </div>
    </section>

    <p v-if="operator" class="muted" style="font-size: 0.78rem">
      Operator key <code class="mono">{{ operator.public_key.slice(0, 24) }}…</code> ·
      beacon {{ operator.chain.name }} ({{ operator.chain.period_seconds }}s rounds) ·
      <NuxtLink to="/log">log head</NuxtLink>
      <code class="mono">{{ operator.log_head.slice(0, 16) }}…</code>
    </p>
  </main>
</template>

<style scoped>
.drawrow { display: block; color: inherit; }
.drawrow:hover { text-decoration: none; border-color: var(--seal); }
.dtitle { font-family: var(--serif); font-size: 1.05rem; font-weight: 600; }
</style>

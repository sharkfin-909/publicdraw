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
const { token, signIn, signOut } = useOrganiser()
const tokenInput = ref('')
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
  if (!token.value) return
  try {
    const [{ draws: ids }, op] = await Promise.all([api.listDraws(), api.operator()])
    operator.value = op
    draws.value = await Promise.all((ids ?? []).map((id) => api.getDraw(id)))
    draws.value.sort((a, b) => b.params.created_at - a.params.created_at)
  } catch (e) {
    if ((e as { statusCode?: number }).statusCode === 401) signOut()
    loadError.value = errorText(e)
  }
}

function enter() {
  signIn(tokenInput.value.trim())
  tokenInput.value = ''
}

watch(token, load)

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

</script>

<template>
  <UiPage>
    <header>
      <UiEyebrow>organiser</UiEyebrow>
      <h1>Dashboard</h1>
      <p class="max-w-[62ch] text-muted">
        Every draw here commits to its rules before anyone enters, and to its entry list before the
        randomness that decides it exists. Results verify in your own browser.
      </p>
    </header>

    <UiCallout v-if="loadError" tone="error">
      {{ loadError }} — is the draw service running? <code class="font-mono text-[0.85em]">publicdraw serve</code>
    </UiCallout>

    <UiCard v-if="!token">
      <h2>Organiser sign-in</h2>
      <p class="text-[0.95rem] text-muted">
        Anyone can read and verify a draw. Running one needs the organiser token.
      </p>
      <form class="gap-3 flex min-w-0 flex-col *:min-w-0" @submit.prevent="enter">
        <div>
          <label for="token">Organiser token</label>
          <input id="token" v-model="tokenInput" type="password" autocomplete="off" required />
        </div>
        <div class="flex flex-wrap items-center gap-3 *:min-w-0">
          <UiButton type="submit" :disabled="!tokenInput">Sign in</UiButton>
        </div>
      </form>
    </UiCard>

    <UiCard v-if="token">
      <h2>New draw</h2>
      <p class="text-[0.95rem] text-muted">
        These parameters are signed and logged now, before a single entry arrives. Changing any of
        them later produces a different draw.
      </p>
      <form class="gap-3 flex min-w-0 flex-col *:min-w-0" @submit.prevent="create">
        <div>
          <label for="title">What is this draw for?</label>
          <input id="title" v-model="form.title" required placeholder="Two community grants" />
        </div>
        <div class="grid grid-cols-[repeat(auto-fill,minmax(260px,1fr))] gap-3.5">
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
        <p class="m-0 text-[0.85rem] text-muted">
          <template v-if="form.disclosure === 'private'">
            Entrants confirm their own inclusion with a receipt; nobody else's entry is published.
          </template>
          <template v-else>
            Every entry is published and a verifier rebuilds the whole tree. Only use this where
            entrants would not be harmed by being named.
          </template>
        </p>
        <UiCallout v-if="createError" tone="error">{{ createError }}</UiCallout>
        <div class="flex flex-wrap items-center gap-3 *:min-w-0">
          <UiButton type="submit" :disabled="busy || !form.title">
            {{ busy ? 'Creating…' : 'Create and commit parameters' }}
          </UiButton>
        </div>
      </form>
    </UiCard>

    <div v-if="token" class="flex flex-wrap items-center gap-3 *:min-w-0">
      <UiButton variant="ghost" @click="signOut">Sign out</UiButton>
    </div>

    <section v-if="token && draws.length">
      <h2 class="mb-3">All draws</h2>
      <div class="gap-2.5 flex min-w-0 flex-col *:min-w-0">
        <UiCard
          v-for="d in draws" :key="d.params.draw_id" :to="drawPath(d.params.draw_id, d.params.title)"
          class="block text-inherit hover:border-seal hover:no-underline"
        >
          <div class="flex flex-wrap items-baseline justify-between gap-4 *:min-w-0">
            <div>
              <div class="font-serif text-[1.05rem] font-semibold">{{ d.params.title || 'Untitled draw' }}</div>
              <div class="font-mono text-[0.74rem] text-muted">{{ d.params.draw_id }}</div>
            </div>
            <div class="flex flex-wrap items-center gap-2.5 *:min-w-0">
              <UiPill :tone="d.state">{{ d.state }}</UiPill>
              <span class="text-[0.85rem] text-muted">
                {{ d.entry_count }} {{ d.entry_count === 1 ? 'entry' : 'entries' }} ·
                {{ d.params.winners }} to win · {{ d.params.disclosure }}
              </span>
            </div>
          </div>
        </UiCard>
      </div>
    </section>

    <p v-if="operator" class="text-[0.78rem] text-muted">
      Operator key <code class="font-mono text-[0.85em]">{{ operator.public_key.slice(0, 24) }}…</code> ·
      beacon {{ operator.chain.name }} ({{ operator.chain.period_seconds }}s rounds) ·
      <NuxtLink to="/log">log head</NuxtLink>
      <code class="font-mono text-[0.85em]">{{ operator.log_head.slice(0, 16) }}…</code>
    </p>
  </UiPage>
</template>

<script setup lang="ts">
import type { Admission } from '~/lib/types'
import { checkMembership } from '~/lib/membership'
import { timeOfRound } from '~/lib/beacon'
import { drawPath } from '~/utils/seo'

const route = useRoute()
const id = computed(() => String(route.params.id))
const api = useApi()
const { token } = useOrganiser()

/**
 * Fetched on the server, not in onMounted.
 *
 * A draw result is the thing people share, and no social scraper runs
 * JavaScript — a page whose content only appears after hydration has no link
 * preview and no dependable crawl. It also means the title and description
 * below are real by the time the HTML leaves the server.
 */
// The bundle arrives from the organiser's server and is checked in this
// browser, against a pinned drand key. The server's own /verify is deliberately
// not used: that answer would come from the party who ran the draw.
const { data: initial, error: loadErr } = await useAsyncData(
  () => `draw:${id.value}`,
  async () => {
    const summary = await api.getDraw(id.value)
    const withBundle =
      summary.state === 'drawn' ? await api.bundle(id.value).catch(() => undefined) : undefined
    return { summary, bundle: withBundle }
  },
  { watch: [id] },
)

// Where the draw is, what that needs from the server, and when to look again
// all live in the session; this page renders it and handles the organiser's
// actions.
const session = useDrawSession(() => id.value, initial)
const s = session.state
const draw = computed(() => s.summary)
const bundle = computed(() => s.bundle)
const report = computed(() => s.report)
const phase = computed(() => s.phase)

const { busy, error: err, run: act } = useAction()

// A missing draw must return 404, not a 200 with an empty shell — otherwise
// every mistyped id becomes an indexable soft-404.
if (loadErr.value || !draw.value) {
  throw createError({ statusCode: 404, statusMessage: 'No such draw', fatal: true })
}

/**
 * Indexing is opt-in, and the default is what matters.
 *
 * A drawn page names its winners — that is the whole point of publishing a
 * result — so for a housing ballot or a grant round it carries real people's
 * details. Nothing here reaches a search index unless the organiser
 * deliberately listed it.
 */
const listed = computed(() => draw.value?.visibility === 'public')

const canonicalPath = computed(() => drawPath(id.value, draw.value?.params.title))

const toggleVisibility = () =>
  act('visibility', () => api.setVisibility(id.value, listed.value ? 'unlisted' : 'public'), session.refresh)

useSeo(() => {
  const d = draw.value!
  const n = d.roster_commit?.size ?? d.entry_count
  const state =
    d.state === 'drawn'
      ? `Result published: ${d.params.winners} of ${n} selected.`
      : d.state === 'committed'
        ? `Entry list frozen at ${n} entries, waiting on drand round ${d.params.target_round}.`
        : 'Entries open.'
  return {
    title: d.params.title || 'Draw',
    description: `${state} Committed before the deciding randomness existed, and checkable in your own browser.`,
    path: canonicalPath.value,
    index: listed.value,
    type: 'article',
  }
})

const entriesText = ref('')
const admitted = ref<Admission[]>([])

// The receipt-free inclusion check: an entrant pastes the salt they were given
// when they entered, and the page looks for their leaf in the published list.
const selfCheck = reactive({ salt: '', label: '' })
const selfResult = ref<{ found: boolean; index: number; leaf: string } | null>(null)

const addEntries = () =>
  act('enter', async () => {
    const labels = entriesText.value.split('\n').map((l) => l.trim()).filter(Boolean)
    if (!labels.length) throw new Error('Add at least one entry, one per line')
    const { admitted: rows } = await api.addEntries(id.value, labels)
    admitted.value = [...admitted.value, ...rows]
    entriesText.value = ''
  }, session.refresh)

function runSelfCheck() {
  const leaves = s.checkLeaves
  if (!leaves) return (selfResult.value = null)
  const m = checkMembership(
    { salt: selfCheck.salt, label: selfCheck.label.trim() },
    { leaves },
    draw.value?.roster_commit?.root,
  )
  selfResult.value = { found: m.ok, index: m.index, leaf: m.leaf }
}

const run = () => act('run', () => api.run(id.value), session.refresh)
</script>

<template>
  <UiPage>
    <UiCallout v-if="err || s.error" tone="error">{{ err || s.error }}</UiCallout>
    <UiCallout v-if="s.bundleError" tone="error">
      This draw has a published result, but it could not be loaded to verify: {{ s.bundleError }}
    </UiCallout>

    <template v-if="draw">
      <header>
        <UiEyebrow>{{ draw.params.disclosure }} draw</UiEyebrow>
        <div class="flex flex-wrap items-baseline justify-between gap-4 *:min-w-0">
          <h1>{{ draw.params.title || 'Untitled draw' }}</h1>
          <UiPill :tone="draw.state">{{ draw.state }}</UiPill>
        </div>
        <p class="font-mono text-[0.78rem] text-muted wrap-anywhere">{{ draw.params.draw_id }}</p>
      </header>

      <RoundCountdown
        v-if="phase !== 'closing' && phase !== 'stalled'"
        :round="draw.params.target_round"
        :frozen-at="draw.roster_commit?.committed_at"
      />

      <UiCard>
        <h2>What was committed, and when</h2>
        <UiKeyValues class="mt-3">
          <dt>Parameters signed</dt>
          <dd>{{ fmtTime(draw.params.created_at) }}</dd>
          <dt>Entries close</dt>
          <dd>{{ fmtTime(draw.params.cutoff_at) }}</dd>
          <dt>Roster frozen</dt>
          <dd>{{ draw.roster_commit ? fmtTime(draw.roster_commit.committed_at) : 'not yet' }}</dd>
          <dt>Target round</dt>
          <dd>{{ draw.params.target_round }} · {{ fmtTime(timeOfRound(draw.params.target_round).getTime() / 1000) }}</dd>
          <dt>Winners</dt>
          <dd>{{ draw.params.winners }} of {{ draw.roster_commit?.size ?? draw.entry_count }}</dd>
          <dt v-if="draw.roster_commit">Roster root</dt>
          <dd v-if="draw.roster_commit">{{ draw.roster_commit.root }}</dd>
        </UiKeyValues>
      </UiCard>

      <!-- Entries, while they are open -->
      <UiCard v-if="token && draw.state === 'open'">
        <h2>Entries</h2>
        <p class="text-[0.95rem] text-muted">
          {{ draw.entry_count }} so far. Entering is not committing — the list is only frozen at the
          cutoff, and nothing before that point is tamper-evident.
        </p>
        <template v-if="phase === 'open'">
          <label for="entries">One entry per line</label>
          <textarea id="entries" v-model="entriesText" placeholder="alice@example.com&#10;bob@example.com" />
          <div class="mt-2.5 flex flex-wrap items-center gap-3 *:min-w-0">
            <UiButton :disabled="busy === 'enter'" @click="addEntries">
              {{ busy === 'enter' ? 'Adding…' : 'Add entries' }}
            </UiButton>
          </div>
          <AdmissionList v-if="admitted.length" :admitted="admitted" :draw-id="id" />
        </template>
        <UiCallout v-else class="mt-2.5">
          <template v-if="phase === 'stalled'">
            The cutoff passed with too few entries for {{ draw.params.winners }}
            {{ draw.params.winners === 1 ? 'winner' : 'winners' }}, so this draw cannot be completed.
          </template>
          <template v-else>Entries are closed. Freezing the roster…</template>
        </UiCallout>
      </UiCard>

      <!-- Committed, waiting for the round -->
      <UiCard v-if="token && draw.state === 'committed'">
        <h2>Roster frozen</h2>
        <p class="text-[0.95rem] text-muted">
          {{ draw.roster_commit?.size }} entries, signed and written to the public log. The deciding
          number does not exist yet.
        </p>
        <div class="flex flex-wrap items-center gap-3 *:min-w-0">
          <UiButton :disabled="phase !== 'deciding' || busy === 'run'" @click="run">
            {{ busy === 'run' ? 'Drawing…' : phase === 'deciding' ? 'Draw the winners' : 'Waiting for the round' }}
          </UiButton>
        </div>
      </UiCard>

      <!-- Result -->
      <UiCard v-if="bundle">
        <h2>Result</h2>
        <ol class="my-3">
          <li
            v-for="w in bundle.result.winners" :key="w.index"
            class="flex items-baseline gap-3.5 border-b border-rule py-[9px]"
          >
            <span class="flex-none rounded-[10px] bg-seal px-[9px] py-0.5 font-mono text-[0.72rem] text-surface">{{ w.rank }}</span>
            <span class="flex-1 font-medium break-all">{{ w.entry.label }}</span>
            <span class="font-mono text-[0.85em] text-muted">#{{ w.index }}</span>
          </li>
        </ol>
        <p v-if="bundle.result.waitlist.length" class="text-[0.87rem] text-muted">
          Waitlist, already in order:
          <span class="font-mono text-[0.85em]">{{ bundle.result.waitlist.slice(0, 12).map((i) => `#${i}`).join(' ') }}</span>
          <template v-if="bundle.result.waitlist.length > 12"> …</template>
        </p>
        <p class="mt-2.5 font-mono text-[0.78rem] text-muted wrap-anywhere">seed {{ bundle.result.seed }}</p>
      </UiCard>

      <UiCard v-if="report">
        <div class="mb-3 flex flex-wrap items-baseline justify-between gap-4 *:min-w-0">
          <h2>Verification</h2>
          <a :href="`${$config.public.apiBase}/draws/${id}/bundle`" download class="font-mono text-[0.78rem]">
            download bundle
          </a>
        </div>
        <CheckList :report="report" />
      </UiCard>

      <!-- Self-check against the published leaves -->
      <UiCard v-if="s.checkLeaves">
        <h2>Check your own entry</h2>
        <p class="text-[0.95rem] text-muted">
          {{ s.checkLeaves.length }} leaves published. If you entered, you were given a salt at that
          moment — paste it with your entry and this page will look for you in the committed list.
          Nothing is sent anywhere; the check runs here.
        </p>
        <div class="mt-3 grid grid-cols-[repeat(auto-fill,minmax(260px,1fr))] gap-3.5">
          <div>
            <label for="sc-label">Your entry</label>
            <input id="sc-label" v-model="selfCheck.label" placeholder="alice@example.com" />
          </div>
          <div>
            <label for="sc-salt">Your salt</label>
            <input id="sc-salt" v-model="selfCheck.salt" placeholder="bb9d7bf6…" />
          </div>
        </div>
        <div class="mt-2.5 flex flex-wrap items-center gap-3 *:min-w-0">
          <UiButton variant="ghost" :disabled="!selfCheck.salt || !selfCheck.label" @click="runSelfCheck">
            Look for my entry
          </UiButton>
        </div>
        <div v-if="selfResult" class="mt-3">
          <p v-if="selfResult.found" class="font-semibold text-seal">
            Found — you are entry #{{ selfResult.index }} of {{ s.checkLeaves.length }} in the committed
            roster.
          </p>
          <p v-else class="font-semibold text-fault">
            Not found. Your entry is not in the committed list — check the salt and entry are exactly
            as issued before concluding anything.
          </p>
          <p class="font-mono text-[0.78rem] text-muted wrap-anywhere">leaf {{ selfResult.leaf || '—' }}</p>
        </div>
      </UiCard>

      <UiCard v-if="token" :class="listed ? 'border-seal!' : ''">
        <div class="flex flex-wrap items-baseline justify-between gap-4 *:min-w-0">
          <div>
            <h2>{{ listed ? 'Published' : 'Unlisted' }}</h2>
            <p class="mt-1.5 mb-0 max-w-[54ch] text-[0.95rem] text-muted">
              <template v-if="listed">
                This draw appears in the public directory and may be indexed by search engines.
                Winners are named on this page.
              </template>
              <template v-else>
                Anyone with the link can open and verify this draw, but it is kept out of the
                directory, the sitemap and search results. This is the default, because a result
                page names its winners.
              </template>
            </p>
          </div>
          <UiButton variant="ghost" :disabled="busy === 'visibility'" @click="toggleVisibility">
            {{ busy === 'visibility' ? 'Saving\u2026' : listed ? 'Unlist' : 'Publish to directory' }}
          </UiButton>
        </div>
      </UiCard>

      <!-- Entrant checks -->
      <UiCard v-if="draw.roster_commit">
        <h2>Entrant checks</h2>
        <p class="text-[0.95rem] text-muted">
          Each entrant confirms their own inclusion from the link they were given when they
          entered. Nothing here can look one up for them: the salt that makes a check possible was
          handed over once, at entry, and is not held on the server — which is also why the entry
          list of a private draw cannot be read back out of it.
        </p>
        <p class="mt-2.5 font-mono text-[0.78rem] text-muted wrap-anywhere">
          committed roster {{ draw.roster_commit.root }}
        </p>
        <p class="mt-2 mb-0 text-[0.87rem] text-muted">
          An entrant should check that root against the one on their own result page.
          <NuxtLink to="/receipt">The check page</NuxtLink> takes their link.
        </p>
      </UiCard>
    </template>
  </UiPage>
</template>

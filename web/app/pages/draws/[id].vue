<script setup lang="ts">
import type { Bundle, DrawSummary } from '~/lib/types'
import { verifyBundle, type Report } from '~/lib/verify'
import { timeOfRound } from '~/lib/beacon'

const route = useRoute()
const id = computed(() => String(route.params.id))
const api = useApi()

const draw = ref<DrawSummary>()
const bundle = ref<Bundle>()
const report = ref<Report>()
const err = ref('')
const busy = ref('')

useHead(() => ({ title: draw.value ? `${draw.value.params.title} — truedraw` : 'Draw — truedraw' }))

const entriesText = ref('')
const receiptIndex = ref(0)
const receiptResult = ref<{ ok: boolean; label: string; root: string } | null>(null)

const { published } = useCountdown(() => draw.value?.params.target_round)

async function load() {
  err.value = ''
  try {
    draw.value = await api.getDraw(id.value)
    if (draw.value.state === 'drawn') await pullBundle()
  } catch (e) {
    err.value = errorText(e)
  }
}

/**
 * The bundle arrives from the organiser's server and is then checked here,
 * locally, against a pinned drand key. The server's own /verify endpoint is
 * deliberately not used: an answer about whether a draw was honest is worth
 * nothing when it comes from the party who ran it.
 */
async function pullBundle() {
  bundle.value = await api.bundle(id.value)
  report.value = verifyBundle(bundle.value)
}

async function act(name: string, fn: () => Promise<unknown>) {
  err.value = ''
  busy.value = name
  try {
    await fn()
    await load()
  } catch (e) {
    err.value = errorText(e)
  } finally {
    busy.value = ''
  }
}

const addEntries = () =>
  act('enter', async () => {
    const labels = entriesText.value.split('\n').map((l) => l.trim()).filter(Boolean)
    if (!labels.length) throw new Error('Add at least one entry, one per line')
    await api.addEntries(id.value, labels)
    entriesText.value = ''
  })

const commit = () => act('commit', () => api.commit(id.value))
const run = () => act('run', () => api.run(id.value))

async function getReceipt() {
  receiptResult.value = null
  err.value = ''
  try {
    const r = await api.receipt(id.value, receiptIndex.value)
    const { verifyReceipt } = await import('~/lib/verify')
    receiptResult.value = { ok: verifyReceipt(r), label: r.entry.label, root: r.root }
  } catch (e) {
    err.value = errorText(e)
  }
}

const cutoffPassed = computed(() =>
  draw.value ? Date.now() / 1000 >= draw.value.params.cutoff_at : false,
)

onMounted(load)
</script>

<template>
  <main class="wrap stack">
    <div v-if="err" class="error">{{ err }}</div>

    <template v-if="draw">
      <header>
        <p class="eyebrow">{{ draw.params.disclosure }} draw</p>
        <div class="spread">
          <h1>{{ draw.params.title || 'Untitled draw' }}</h1>
          <span class="pill" :class="draw.state">{{ draw.state }}</span>
        </div>
        <p class="hash">{{ draw.params.draw_id }}</p>
      </header>

      <RoundCountdown
        v-if="draw.state !== 'open' || !cutoffPassed"
        :round="draw.params.target_round"
        :frozen-at="draw.roster_commit?.committed_at"
      />

      <section class="card">
        <h2>What was committed, and when</h2>
        <dl class="kv" style="margin-top: 12px">
          <dt>Parameters signed</dt>
          <dd>{{ fmtTime(draw.params.created_at) }}</dd>
          <dt>Entries close</dt>
          <dd>{{ fmtTime(draw.params.cutoff_at) }}</dd>
          <dt>Roster frozen</dt>
          <dd>{{ draw.roster_commit ? fmtTime(draw.roster_commit.committed_at) : 'not yet' }}</dd>
          <dt>Target round</dt>
          <dd>{{ draw.params.target_round }} · {{ timeOfRound(draw.params.target_round).toLocaleString() }}</dd>
          <dt>Winners</dt>
          <dd>{{ draw.params.winners }} of {{ draw.roster_commit?.size ?? draw.entry_count }}</dd>
          <dt v-if="draw.roster_commit">Roster root</dt>
          <dd v-if="draw.roster_commit">{{ draw.roster_commit.root }}</dd>
        </dl>
      </section>

      <!-- Entries, while they are open -->
      <section v-if="draw.state === 'open'" class="card">
        <h2>Entries</h2>
        <p class="muted" style="font-size: 0.86rem">
          {{ draw.entry_count }} so far. Entering is not committing — the list is only frozen at the
          cutoff, and nothing before that point is tamper-evident.
        </p>
        <template v-if="!cutoffPassed">
          <label for="entries">One entry per line</label>
          <textarea id="entries" v-model="entriesText" placeholder="alice@example.com&#10;bob@example.com" />
          <div class="row" style="margin-top: 10px">
            <button :disabled="busy === 'enter'" @click="addEntries">
              {{ busy === 'enter' ? 'Adding…' : 'Add entries' }}
            </button>
          </div>
        </template>
        <div v-else class="notice" style="margin-top: 10px">
          The cutoff has passed. Entries are closed — freeze the roster to continue.
          <div class="row" style="margin-top: 10px">
            <button :disabled="busy === 'commit'" @click="commit">
              {{ busy === 'commit' ? 'Freezing…' : 'Freeze the roster' }}
            </button>
          </div>
        </div>
      </section>

      <!-- Committed, waiting for the round -->
      <section v-if="draw.state === 'committed'" class="card">
        <h2>Roster frozen</h2>
        <p class="muted" style="font-size: 0.86rem">
          {{ draw.roster_commit?.size }} entries, signed and written to the public log. The deciding
          number does not exist yet.
        </p>
        <div class="row">
          <button :disabled="!published || busy === 'run'" @click="run">
            {{ busy === 'run' ? 'Drawing…' : published ? 'Draw the winners' : 'Waiting for the round' }}
          </button>
        </div>
      </section>

      <!-- Result -->
      <section v-if="bundle" class="card">
        <h2>Result</h2>
        <ol class="winners">
          <li v-for="w in bundle.result.winners" :key="w.index">
            <span class="wrank">{{ w.rank }}</span>
            <span class="wlabel">{{ w.entry.label }}</span>
            <span class="muted mono">#{{ w.index }}</span>
          </li>
        </ol>
        <p v-if="bundle.result.waitlist.length" class="muted" style="font-size: 0.82rem">
          Waitlist, already in order:
          <span class="mono">{{ bundle.result.waitlist.slice(0, 12).map((i) => `#${i}`).join(' ') }}</span>
          <template v-if="bundle.result.waitlist.length > 12"> …</template>
        </p>
        <p class="hash" style="margin-top: 10px">seed {{ bundle.result.seed }}</p>
      </section>

      <section v-if="report" class="card">
        <div class="spread" style="margin-bottom: 12px">
          <h2>Verification</h2>
          <a :href="`${$config.public.apiBase}/draws/${id}/bundle`" download class="mono" style="font-size: 0.78rem">
            download bundle
          </a>
        </div>
        <CheckList :report="report" />
      </section>

      <!-- Receipts -->
      <section v-if="draw.roster_commit" class="card">
        <h2>Entrant receipt</h2>
        <p class="muted" style="font-size: 0.86rem">
          A receipt proves one entrant was in the committed roster without revealing anyone else.
          Check its root against the roster root above — a receipt for a different roster proves
          nothing about this draw.
        </p>
        <div class="row">
          <input
            v-model.number="receiptIndex"
            type="number"
            min="0"
            :max="draw.roster_commit.size - 1"
            style="max-width: 120px"
          />
          <button class="ghost" @click="getReceipt">Fetch and check</button>
        </div>
        <div v-if="receiptResult" style="margin-top: 12px">
          <p :style="{ color: receiptResult.ok ? 'var(--teal)' : 'var(--fault)', fontWeight: 600 }">
            {{ receiptResult.ok ? 'Receipt verifies' : 'Receipt does NOT verify' }} — {{ receiptResult.label }}
          </p>
          <p class="hash">root {{ receiptResult.root }}</p>
          <p
            v-if="receiptResult.ok && draw.roster_commit && receiptResult.root !== draw.roster_commit.root"
            class="notice"
          >
            This receipt is for a different roster than the one this draw committed to.
          </p>
        </div>
      </section>
    </template>
  </main>
</template>

<style scoped>
.winners { list-style: none; padding: 0; margin: 12px 0; }
.winners li {
  display: flex; gap: 14px; align-items: baseline;
  padding: 9px 0; border-bottom: 1px solid var(--rule);
}
.wrank {
  font-family: var(--mono); font-size: 0.72rem; color: var(--surface);
  background: var(--teal); border-radius: 10px; padding: 2px 9px; flex: none;
}
.wlabel { flex: 1; font-weight: 500; word-break: break-all; }
</style>

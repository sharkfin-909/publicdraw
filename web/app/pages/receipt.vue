<script setup lang="ts">
import { verifyReceipt } from '~/lib/verify'
import type { Receipt } from '~/lib/types'

useSeo({
  title: 'Check an entry receipt',
  description:
    'Confirm you were in a draw\u2019s committed entry list, at a specific position, without revealing anyone else. Checked in your browser.',
  path: '/receipt',
})

const raw = ref('')
const result = ref<{ ok: boolean; receipt: Receipt } | null>(null)
const err = ref('')

function check() {
  err.value = ''
  result.value = null
  try {
    const r = JSON.parse(raw.value) as Receipt
    result.value = { ok: verifyReceipt(r), receipt: r }
  } catch (e) {
    err.value = e instanceof Error ? e.message : String(e)
  }
}
</script>

<template>
  <main class="wrap stack">
    <header>
      <p class="eyebrow">entrants</p>
      <h1>Check a receipt</h1>
      <p class="muted" style="max-width: 62ch">
        A receipt proves you were in a draw's committed entry list, at a specific position, without
        revealing anyone else. Checked here in your browser.
      </p>
    </header>

    <section class="card">
      <label for="raw">Paste your receipt</label>
      <textarea id="raw" v-model="raw" placeholder="{ &quot;draw_id&quot;: …, &quot;merkle_path&quot;: [ … ] }" />
      <div class="row" style="margin-top: 10px">
        <button :disabled="!raw" @click="check">Check receipt</button>
      </div>
    </section>

    <div v-if="err" class="error">Could not read that receipt: {{ err }}</div>

    <section v-if="result" class="ticket" :class="{ bad: !result.ok }">
      <div class="ticket-head">
        <BrandMark :size="30" />
        <span class="ticket-title">{{ result.ok ? 'Keep this ticket' : 'Not in this roster' }}</span>
      </div>

      <h2 class="ticket-verdict">
        {{ result.ok ? 'Receipt verifies' : 'Receipt does not verify' }}
      </h2>

      <dl class="kv" style="margin-top: 12px">
        <dt>Entry</dt>
        <dd>{{ result.receipt.entry.label }}</dd>
        <dt>Position</dt>
        <dd>{{ result.receipt.index }} of {{ result.receipt.size }}</dd>
        <dt>Draw</dt>
        <dd>{{ result.receipt.draw_id }}</dd>
        <dt>Roster root</dt>
        <dd>{{ result.receipt.root }}</dd>
      </dl>

      <p v-if="result.ok" class="ticket-note">
        This proves membership in the roster with that root. Now check that root matches the one on
        the draw's published result — a receipt for a different roster says nothing about this draw.
        <NuxtLink :to="`/draws/${result.receipt.draw_id}`">Open the draw →</NuxtLink>
      </p>
    </section>
  </main>
</template>

<style scoped>
/*
  Paper, not UI. This is the one surface an entrant holds, so it is printed
  rather than rendered — and it keeps its colours in dark mode, because a
  ticket does not invert.
*/
.ticket {
  background: var(--raised);
  color: var(--ink);
  border: 1px solid var(--line);
  border-radius: 3px;
  padding: 24px 26px;
  position: relative;
  box-shadow: 0 1px 0 rgba(44, 38, 24, 0.06), 0 8px 24px -14px rgba(44, 38, 24, 0.5);
}
.ticket::before {
  content: '';
  position: absolute;
  left: 0; right: 0; top: 64px;
  border-top: 1px dashed var(--line);
}
.ticket-head { display: flex; align-items: center; gap: 11px; padding-bottom: 22px; }
.ticket-title { font-family: var(--serif); font-size: 1.12rem; font-weight: 600; }
.ticket-verdict {
  font-family: var(--serif); font-size: 1.3rem; margin: 22px 0 0;
  color: var(--teal);
}
.ticket.bad .ticket-verdict { color: #8f1d1d; }
.ticket :deep(.kv dt) { color: var(--muted); }
.ticket :deep(.kv dd) { color: var(--ink); }
.ticket-note {
  margin: 18px 0 0; padding: 12px 14px; border-radius: 3px;
  background: rgba(11, 95, 95, 0.09);
  font-size: 0.86rem; line-height: 1.55; color: var(--ink);
}
.ticket-note a { color: var(--teal); font-weight: 600; }
</style>

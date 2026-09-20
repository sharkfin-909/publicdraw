<script setup lang="ts">
import { verifyReceipt } from '~/lib/verify'
import type { Receipt } from '~/lib/types'

useHead({ title: 'Check a receipt — truedraw' })

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

    <section v-if="result" class="card">
      <h2 :style="{ color: result.ok ? 'var(--teal)' : 'var(--fault)' }">
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

      <p v-if="result.ok" class="notice" style="margin-top: 14px">
        This proves membership in the roster with that root. Now check that root matches the one on
        the draw's published result — a receipt for a different roster says nothing about this draw.
        <NuxtLink :to="`/draws/${result.receipt.draw_id}`">Open the draw →</NuxtLink>
      </p>
    </section>
  </main>
</template>

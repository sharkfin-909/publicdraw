<script setup lang="ts">
import { verifyReceipt } from '~/lib/verify'
import type { Receipt } from '~/lib/types'

useSeo({
  title: 'Check an entry receipt',
  description:
    'Confirm you were in a draw’s committed entry list, at a specific position, without revealing anyone else. Checked in your browser.',
  path: '/receipt',
})

const link = ref('')
const linkError = ref('')

/**
 * Entrants get a link, not a file. Everything needed is in it: the draw and
 * the position in the path, and the salt in the fragment, which browsers
 * never send to a server.
 */
function openLink() {
  linkError.value = ''
  const raw = link.value.trim()
  if (!raw) return
  try {
    const u = raw.startsWith('http') ? new URL(raw) : new URL(raw, location.origin)
    const m = u.pathname.match(/\/receipt\/([^/]+)\/(\d+)\/?$/)
    if (!m) throw new Error('that is not a receipt link')
    navigateTo({ path: `/receipt/${m[1]}/${m[2]}`, hash: u.hash })
  } catch {
    linkError.value =
      'That does not look like a receipt link. It should end in /receipt/<draw>/<position> and carry a #s= part.'
  }
}

// Kept for anyone holding a receipt as a file rather than a link.
const raw = ref('')
const result = ref<{ ok: boolean; receipt: Receipt } | null>(null)
const err = ref('')

function checkJson() {
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
      <h1>Check an entry receipt</h1>
      <p class="muted" style="max-width: 62ch">
        A receipt proves you were in a draw’s committed entry list, at a specific position, without
        revealing anyone else. Checked here in your browser.
      </p>
    </header>

    <section class="card">
      <form @submit.prevent="openLink">
        <label for="link">Paste the link you were given when you entered</label>
        <input
          id="link"
          v-model="link"
          autocomplete="off"
          placeholder="https://publicdraw.truelathe.com/receipt/…/4472#s=…"
        />
        <p class="muted" style="font-size: 0.8rem; margin: 8px 0 0">
          The part after <span class="mono">#</span> is the secret half and never leaves your
          browser — so paste the whole link, not a shortened one.
        </p>
        <div v-if="linkError" class="error" style="margin-top: 10px">{{ linkError }}</div>
        <div class="row" style="margin-top: 12px">
          <button type="submit" :disabled="!link">Check my entry</button>
        </div>
      </form>
    </section>

    <details class="card">
      <summary>I have a receipt file instead</summary>
      <p class="muted" style="font-size: 0.86rem; margin-top: 12px">
        Older receipts were issued as JSON. They still check the same way.
      </p>
      <label for="raw" style="margin-top: 10px">Paste the receipt</label>
      <textarea id="raw" v-model="raw" placeholder="{ &quot;draw_id&quot;: …, &quot;merkle_path&quot;: [ … ] }" />
      <div class="row" style="margin-top: 10px">
        <button class="ghost" :disabled="!raw" @click="checkJson">Check receipt</button>
      </div>

      <div v-if="err" class="error" style="margin-top: 12px">Could not read that receipt: {{ err }}</div>

      <div v-if="result" style="margin-top: 16px">
        <h2 :style="{ color: result.ok ? 'var(--seal)' : 'var(--fault)', fontSize: '1.1rem' }">
          {{ result.ok ? 'Receipt verifies' : 'Receipt does not verify' }}
        </h2>
        <dl class="kv" style="margin-top: 10px">
          <dt>Entry</dt><dd>{{ result.receipt.entry.label }}</dd>
          <dt>Position</dt><dd>{{ result.receipt.index }} of {{ result.receipt.size }}</dd>
          <dt>Roster root</dt><dd>{{ result.receipt.root }}</dd>
        </dl>
        <p v-if="result.ok" class="notice" style="margin-top: 12px">
          Now check that root matches the one on the draw’s published result.
          <NuxtLink :to="`/draws/${result.receipt.draw_id}`">Open the draw →</NuxtLink>
        </p>
      </div>
    </details>
  </main>
</template>

<style scoped>
details summary { cursor: pointer; font-weight: 600; font-size: 0.92rem; }
details[open] summary { margin-bottom: 4px; }
</style>

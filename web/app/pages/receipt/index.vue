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
  <UiPage>
    <header>
      <UiEyebrow>entrants</UiEyebrow>
      <h1>Check an entry receipt</h1>
      <p class="max-w-[62ch] text-muted">
        A receipt proves you were in a draw’s committed entry list, at a specific position, without
        revealing anyone else. Checked here in your browser.
      </p>
    </header>

    <UiCard>
      <form @submit.prevent="openLink">
        <label for="link">Paste the link you were given when you entered</label>
        <input
          id="link"
          v-model="link"
          autocomplete="off"
          placeholder="https://publicdraw.truelathe.com/receipt/…/4472#s=…"
        />
        <p class="mt-2 mb-0 text-[0.8rem] text-muted">
          The part after <span class="font-mono text-[0.85em]">#</span> is the secret half and never leaves your
          browser — so paste the whole link, not a shortened one.
        </p>
        <UiCallout v-if="linkError" tone="error" class="mt-2.5">{{ linkError }}</UiCallout>
        <div class="mt-3 flex flex-wrap items-center gap-3 *:min-w-0">
          <UiButton type="submit" :disabled="!link">Check my entry</UiButton>
        </div>
      </form>
    </UiCard>

    <UiCard as="details">
      <summary class="cursor-pointer text-[0.92rem] font-semibold [details[open]>&]:mb-1">I have a receipt file instead</summary>
      <p class="mt-3 text-[0.86rem] text-muted">
        Older receipts were issued as JSON. They still check the same way.
      </p>
      <label for="raw" class="mt-2.5">Paste the receipt</label>
      <textarea id="raw" v-model="raw" placeholder="{ &quot;draw_id&quot;: …, &quot;merkle_path&quot;: [ … ] }" />
      <div class="mt-2.5 flex flex-wrap items-center gap-3 *:min-w-0">
        <UiButton variant="ghost" :disabled="!raw" @click="checkJson">Check receipt</UiButton>
      </div>

      <UiCallout v-if="err" tone="error" class="mt-3">Could not read that receipt: {{ err }}</UiCallout>

      <div v-if="result" class="mt-4">
        <h2 class="text-[1.1rem]" :class="result.ok ? 'text-seal' : 'text-fault'">
          {{ result.ok ? 'Receipt verifies' : 'Receipt does not verify' }}
        </h2>
        <UiKeyValues class="mt-2.5">
          <dt>Entry</dt><dd>{{ result.receipt.entry.label }}</dd>
          <dt>Position</dt><dd>{{ result.receipt.index }} of {{ result.receipt.size }}</dd>
          <dt>Roster root</dt><dd>{{ result.receipt.root }}</dd>
        </UiKeyValues>
        <UiCallout v-if="result.ok" as="p" class="mt-3">
          Now check that root matches the one on the draw’s published result.
          <NuxtLink :to="`/draws/${result.receipt.draw_id}`">Open the draw →</NuxtLink>
        </UiCallout>
      </div>
    </UiCard>
  </UiPage>
</template>

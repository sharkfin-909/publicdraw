<script setup lang="ts">
import { verifyProof } from '~/lib/verify'
import type { Proof } from '~/lib/types'

/**
 * Checking one entrant's inclusion.
 *
 * The three things needed are split deliberately. The draw and the position
 * are in the path, because they are not secret. The salt arrives in the URL
 * *fragment*, which browsers never send to a server — so it stays out of
 * access logs and Referer headers even though the link gets emailed around.
 * The label is typed, because the one thing only the entrant knows is their
 * own name for themselves, and typing it is what makes this their check
 * rather than anyone's.
 *
 * The server supplies only the path. It cannot manufacture a pass.
 */
const route = useRoute()
const api = useApi()

const drawId = computed(() => String(route.params.draw))
const index = computed(() => Number(route.params.position))

const label = ref('')
const salt = ref('')
const proof = ref<Proof>()
const draw = ref<Awaited<ReturnType<typeof api.getDraw>>>()
const state = ref<'idle' | 'checking' | 'pass' | 'fail'>('idle')
const err = ref('')

useSeo({
  title: 'Check your entry',
  description:
    'Confirm you were in a draw’s committed entry list, at a specific position, without revealing anyone else.',
  path: `/receipt/${drawId.value}/${index.value}`,
  index: false,
})

onMounted(async () => {
  // The salt rides in the fragment, which browsers never send to a server —
  // so it stays out of access logs and Referer headers even though the link
  // gets emailed around. It is deliberately left in the address bar: stripping
  // it would break reload, and it is the entrant's own secret in the
  // entrant's own browser, arriving from a link they already hold.
  const frag = new URLSearchParams(location.hash.replace(/^#/, ''))
  salt.value = frag.get('s') ?? ''

  try {
    const [p, d] = await Promise.all([
      api.proof(drawId.value, index.value),
      api.getDraw(drawId.value).catch(() => undefined),
    ])
    proof.value = p
    draw.value = d
  } catch (e) {
    err.value = errorText(e)
  }
})

function check() {
  err.value = ''
  if (!proof.value) return
  if (!salt.value.trim()) {
    err.value = 'No salt in the link. Use the full link you were given when you entered.'
    return
  }
  state.value = 'checking'
  const ok = verifyProof(proof.value, salt.value.trim(), label.value.trim())
  state.value = ok ? 'pass' : 'fail'
}
</script>

<template>
  <main class="wrap stack">
    <header>
      <p class="eyebrow">entrants</p>
      <h1>Check your entry</h1>
      <p class="muted" style="max-width: 62ch">
        This confirms you were in the committed entry list for this draw, at position
        <span class="mono">{{ index }}</span
        >. It runs in your browser, and reveals nothing about anyone else.
      </p>
    </header>

    <div v-if="err" class="error">{{ err }}</div>

    <section v-if="proof" class="card">
      <dl class="kv">
        <dt>Draw</dt>
        <dd>{{ draw?.params.title || drawId }}</dd>
        <dt>Your position</dt>
        <dd>{{ index }} of {{ proof.size }}</dd>
        <dt>Committed roster</dt>
        <dd>{{ proof.root }}</dd>
      </dl>

      <form style="margin-top: 20px" @submit.prevent="check">
        <label for="label">Confirm the entry you gave — the exact email or name</label>
        <input
          id="label"
          v-model="label"
          autocomplete="off"
          placeholder="you@example.com"
          :disabled="!salt"
        />
        <p v-if="!salt" class="notice" style="margin-top: 10px">
          This link is missing its secret part. Open the full link you were given when you entered —
          it ends in <span class="mono">#s=…</span> — rather than one copied from the address bar.
        </p>
        <div class="row" style="margin-top: 12px">
          <button type="submit" :disabled="!label || !salt">Check my entry</button>
        </div>
      </form>
    </section>

    <section v-if="state === 'pass'" class="ticket">
      <div class="ticket-head">
        <BrandMark :size="30" />
        <span class="ticket-title">Keep this ticket</span>
      </div>
      <h2 class="ticket-verdict">You were entry {{ index }} of {{ proof!.size }}</h2>
      <p style="font-size: 0.9rem; margin: 10px 0 0">
        Your entry was in the list the organiser committed to, and that commitment was signed before
        the deciding randomness existed.
      </p>
      <p class="ticket-note">
        Now check that this roster is the one the winners came from — the root above should match
        the one on the published result.
        <NuxtLink :to="`/draws/${drawId}`">Open the draw →</NuxtLink>
      </p>
    </section>

    <section v-if="state === 'fail'" class="card" style="border-color: var(--fault)">
      <h2 style="color: var(--fault)">That does not match</h2>
      <p style="font-size: 0.92rem; margin: 10px 0 0">
        Nothing at position {{ index }} in this roster hashes to that entry and salt. Most often
        this is a typo in the entry — it has to be character for character what was submitted. If it
        is exactly right, then the entry at this position is not yours, and that is worth raising
        with whoever ran the draw.
      </p>
    </section>
  </main>
</template>

<style scoped>
.ticket {
  background: var(--surface);
  color: var(--ink);
  border: 1px solid var(--line);
  border-radius: 3px;
  padding: 24px 26px;
  position: relative;
  box-shadow: 0 1px 0 rgba(15, 22, 32, 0.04), 0 10px 26px -16px rgba(15, 22, 32, 0.45);
}
.ticket::before {
  content: '';
  position: absolute;
  left: 0;
  right: 0;
  top: 64px;
  border-top: 1px dashed var(--line);
}
.ticket-head { display: flex; align-items: center; gap: 11px; padding-bottom: 22px; }
.ticket-title { font-family: var(--serif); font-size: 1.12rem; font-weight: 600; }
.ticket-verdict { font-family: var(--serif); font-size: 1.3rem; margin: 22px 0 0; color: var(--seal); }
.ticket-note {
  margin: 18px 0 0; padding: 12px 14px; border-radius: 3px;
  background: var(--seal-bg); font-size: 0.86rem; line-height: 1.55;
}
.ticket-note a { color: var(--seal); font-weight: 600; }
</style>

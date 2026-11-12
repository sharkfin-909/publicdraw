<script setup lang="ts">
import { checkMembership, type Membership } from '~/lib/membership'
import { saltFromFragment } from '~/lib/receiptLink'
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
const state = ref<'idle' | 'checking' | 'pass' | 'fail' | 'other-roster'>('idle')
const result = ref<Membership>()
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
  salt.value = saltFromFragment(location.hash)

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
  // The proof's root is the server's word; the draw's committed root is what
  // the organiser signed. The check compares them, so the entrant need not.
  const m = checkMembership(
    { salt: salt.value, label: label.value.trim() },
    { path: proof.value },
    draw.value?.roster_commit?.root,
  )
  result.value = m
  state.value = !m.included ? 'fail' : m.matchesCommitted === false ? 'other-roster' : 'pass'
}
</script>

<template>
  <UiPage>
    <header>
      <UiEyebrow>entrants</UiEyebrow>
      <h1>Check your entry</h1>
      <p class="max-w-[62ch] text-muted">
        This confirms you were in the committed entry list for this draw, at position
        <span class="font-mono text-[0.85em]">{{ index }}</span
        >. It runs in your browser, and reveals nothing about anyone else.
      </p>
    </header>

    <UiCallout v-if="err" tone="error">{{ err }}</UiCallout>

    <UiCard v-if="proof">
      <UiKeyValues>
        <dt>Draw</dt>
        <dd>{{ draw?.params.title || drawId }}</dd>
        <dt>Your position</dt>
        <dd>{{ index }} of {{ proof.size }}</dd>
        <dt>Committed roster</dt>
        <dd>{{ proof.root }}</dd>
      </UiKeyValues>

      <form class="mt-5" @submit.prevent="check">
        <label for="label">Confirm the entry you gave — the exact email or name</label>
        <input
          id="label"
          v-model="label"
          autocomplete="off"
          placeholder="you@example.com"
          :disabled="!salt"
        />
        <UiCallout v-if="!salt" as="p" class="mt-2.5">
          This link is missing its secret part. Open the full link you were given when you entered —
          it ends in <span class="font-mono text-[0.85em]">#s=…</span> — rather than one copied from the address bar.
        </UiCallout>
        <div class="mt-3 flex flex-wrap items-center gap-3 *:min-w-0">
          <UiButton type="submit" :disabled="!label || !salt">Check my entry</UiButton>
        </div>
      </form>
    </UiCard>

    <!-- The ticket: a dashed tear line under its head, as on a paper stub. -->
    <section
      v-if="state === 'pass'"
      class="relative rounded-box border border-line bg-surface px-[26px] py-6 text-ink
             shadow-[0_1px_0_rgba(15,22,32,0.04),0_10px_26px_-16px_rgba(15,22,32,0.45)]
             before:absolute before:inset-x-0 before:top-16 before:border-t before:border-dashed before:border-line"
    >
      <div class="flex items-center gap-[11px] pb-[22px]">
        <BrandMark :size="30" />
        <span class="font-serif text-[1.12rem] font-semibold">Keep this ticket</span>
      </div>
      <h2 class="mt-[22px] font-serif text-[1.3rem] text-seal">You were entry {{ index }} of {{ proof!.size }}</h2>
      <p class="mt-2.5 mb-0 text-[1rem]">
        Your entry was in the list the organiser committed to, and that commitment was signed before
        the deciding randomness existed.
      </p>
      <p class="mt-[18px] mb-0 rounded-box bg-seal-bg px-3.5 py-3 text-[0.95rem] leading-[1.55] [&_a]:font-semibold">
        <template v-if="result?.matchesCommitted">
          Checked against the roster this draw signed and committed to. The draw page checks that
          the winners came from that same roster.
        </template>
        <template v-else>
          The draw's committed roster could not be loaded to compare against, so check that the
          root above matches the one on the draw page.
        </template>
        <NuxtLink :to="`/draws/${drawId}`">Open the draw →</NuxtLink>
      </p>
    </section>

    <UiCard v-if="state === 'other-roster'" class="border-fault!">
      <h2 class="text-fault">That proof is for a different roster</h2>
      <p class="mt-2.5 mb-0 text-[1rem]">
        Your entry is in the roster this proof describes, but that is not the roster draw
        {{ drawId }} signed and committed to. The proof you were served does not show you were in
        this draw. That is worth raising with whoever ran it.
      </p>
    </UiCard>

    <UiCard v-if="state === 'fail'" class="border-fault!">
      <h2 class="text-fault">That does not match</h2>
      <p class="mt-2.5 mb-0 text-[1rem]">
        Nothing at position {{ index }} in this roster hashes to that entry and salt. Most often
        this is a typo in the entry — it has to be character for character what was submitted. If it
        is exactly right, then the entry at this position is not yours, and that is worth raising
        with whoever ran the draw.
      </p>
    </UiCard>
  </UiPage>
</template>

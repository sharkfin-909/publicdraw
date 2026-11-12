<script setup lang="ts">
import { verifyBundle, type Report } from '~/lib/verify'
import type { Bundle } from '~/lib/types'

useSeo({
  title: 'Verify a draw result',
  description:
    'Check a published draw yourself. Verification runs entirely in your browser against a pinned randomness key — nothing is uploaded and no server is asked whether the draw was honest.',
  path: '/verify',
})

const raw = ref('')
const report = ref<Report>()
const bundle = ref<Bundle>()
const err = ref('')

function check(text: string) {
  err.value = ''
  report.value = undefined
  bundle.value = undefined
  try {
    const b = JSON.parse(text) as Bundle
    bundle.value = b
    report.value = verifyBundle(b)
  } catch (e) {
    err.value = e instanceof Error ? e.message : String(e)
  }
}

function onFile(ev: Event) {
  const f = (ev.target as HTMLInputElement).files?.[0]
  if (!f) return
  f.text().then((t) => {
    raw.value = t
    check(t)
  })
}

function onDrop(ev: DragEvent) {
  const f = ev.dataTransfer?.files?.[0]
  if (!f) return
  f.text().then((t) => {
    raw.value = t
    check(t)
  })
}
</script>

<template>
  <UiPage>
    <header>
      <UiEyebrow>independent check</UiEyebrow>
      <h1>Verify a bundle</h1>
      <p class="max-w-[62ch] text-muted">
        This runs entirely in your browser, against a drand group key pinned in the page source.
        Nothing is sent anywhere, and no server is asked whether the draw was honest — which is the
        only reason the answer is worth having.
      </p>
    </header>

    <UiCard class="border-dashed" @dragover.prevent @drop.prevent="onDrop">
      <div class="flex flex-wrap items-center gap-3 *:min-w-0">
        <input type="file" accept="application/json,.json" @change="onFile" />
        <UiButton variant="ghost" :disabled="!raw" @click="check(raw)">Check pasted JSON</UiButton>
      </div>
      <label for="raw" class="mt-3.5">…or paste the bundle</label>
      <textarea id="raw" v-model="raw" placeholder="{ &quot;version&quot;: 1, … }" />
      <p class="mt-2 mb-0 text-[0.8rem] text-muted">
        Drop a <code class="font-mono text-[0.85em]">bundle.json</code> anywhere on this panel.
      </p>
    </UiCard>

    <UiCallout v-if="err" tone="error">Could not read that bundle: {{ err }}</UiCallout>

    <UiCard v-if="report && bundle">
      <div class="mb-1 flex flex-wrap items-baseline justify-between gap-4 *:min-w-0">
        <h2>{{ bundle.draw.params.title || 'Untitled draw' }}</h2>
        <UiPill :tone="report.ok ? 'drawn' : 'open'">
          {{ report.ok ? 'verified' : 'failed' }}
        </UiPill>
      </div>
      <p class="font-mono text-[0.78rem] text-muted wrap-anywhere">{{ bundle.draw.params.draw_id }}</p>

      <ol v-if="bundle.result.winners.length" class="mt-3 mb-[18px]">
        <li
          v-for="w in bundle.result.winners" :key="w.index"
          class="flex items-baseline gap-3 border-b border-rule py-[7px]"
        >
          <span class="rounded-[10px] bg-seal px-2 py-0.5 font-mono text-[0.7rem] text-surface">{{ w.rank }}</span>
          <span>{{ w.entry.label }}</span>
          <span class="font-mono text-[0.85em] text-muted">#{{ w.index }}</span>
        </li>
      </ol>

      <CheckList :report="report" />
    </UiCard>
  </UiPage>
</template>

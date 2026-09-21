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
  <main class="wrap stack">
    <header>
      <p class="eyebrow">independent check</p>
      <h1>Verify a bundle</h1>
      <p class="muted" style="max-width: 62ch">
        This runs entirely in your browser, against a drand group key pinned in the page source.
        Nothing is sent anywhere, and no server is asked whether the draw was honest — which is the
        only reason the answer is worth having.
      </p>
    </header>

    <section class="card drop" @dragover.prevent @drop.prevent="onDrop">
      <div class="row">
        <input type="file" accept="application/json,.json" @change="onFile" />
        <button class="ghost" :disabled="!raw" @click="check(raw)">Check pasted JSON</button>
      </div>
      <label for="raw" style="margin-top: 14px">…or paste the bundle</label>
      <textarea id="raw" v-model="raw" placeholder="{ &quot;version&quot;: 1, … }" />
      <p class="muted" style="font-size: 0.8rem; margin: 8px 0 0">
        Drop a <code class="mono">bundle.json</code> anywhere on this panel.
      </p>
    </section>

    <div v-if="err" class="error">Could not read that bundle: {{ err }}</div>

    <section v-if="report && bundle" class="card">
      <div class="spread" style="margin-bottom: 4px">
        <h2>{{ bundle.draw.params.title || 'Untitled draw' }}</h2>
        <span class="pill" :class="report.ok ? 'drawn' : 'open'">
          {{ report.ok ? 'verified' : 'failed' }}
        </span>
      </div>
      <p class="hash">{{ bundle.draw.params.draw_id }}</p>

      <ol v-if="bundle.result.winners.length" class="winners">
        <li v-for="w in bundle.result.winners" :key="w.index">
          <span class="wrank">{{ w.rank }}</span>
          <span>{{ w.entry.label }}</span>
          <span class="muted mono">#{{ w.index }}</span>
        </li>
      </ol>

      <CheckList :report="report" />
    </section>
  </main>
</template>

<style scoped>
.drop { border-style: dashed; }
.winners { list-style: none; padding: 0; margin: 12px 0 18px; }
.winners li { display: flex; gap: 12px; align-items: baseline; padding: 7px 0; border-bottom: 1px solid var(--rule); }
.wrank { font-family: var(--mono); font-size: 0.7rem; color: var(--surface); background: var(--teal); border-radius: 10px; padding: 2px 8px; }
</style>

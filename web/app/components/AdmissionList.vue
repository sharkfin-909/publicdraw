<script setup lang="ts">
import type { Admission } from '~/lib/types'

const props = defineProps<{ admitted: Admission[]; drawId: string }>()

const origin = computed(() =>
  import.meta.client ? location.origin : useRuntimeConfig().public.siteUrl,
)
/** The salt rides in the fragment, which browsers never send to a server. */
const linkFor = (a: Admission) =>
  `${origin.value}/receipt/${props.drawId}/${a.index}#s=${a.salt}`

const asText = computed(() =>
  props.admitted.map((a) => `${a.label}\t${linkFor(a)}`).join('\n'),
)
const copied = ref(false)

async function copy() {
  try {
    await navigator.clipboard.writeText(asText.value)
    copied.value = true
    setTimeout(() => (copied.value = false), 1800)
  } catch {
    copied.value = false
  }
}
</script>

<template>
  <div class="admissions">
    <div class="spread">
      <h3>{{ admitted.length }} admitted — salts shown once</h3>
      <button class="ghost" @click="copy">{{ copied ? 'Copied' : 'Copy all' }}</button>
    </div>
    <p class="muted why">
      Send each entrant their link now. It is what lets them confirm their own inclusion
      after the cutoff, whether or not anything else ever reaches them — an entrant who was quietly
      dropped and never sent a receipt has nothing to notice.
    </p>
    <div class="scroller">
      <table>
        <thead><tr><th>#</th><th>Entry</th><th>Link to send</th></tr></thead>
        <tbody>
          <tr v-for="a in admitted" :key="a.index">
            <td class="mono">{{ a.index }}</td>
              <td>{{ a.label }}</td>
              <td class="hash">{{ linkFor(a) }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<style scoped>
.admissions {
  background: var(--beacon-bg); border: 1px solid var(--beacon);
  border-radius: 3px; padding: 16px 18px; margin-top: 14px;
}
.admissions h3 { color: var(--beacon); }
.why { font-size: 0.82rem; color: var(--beacon); max-width: 62ch; opacity: 0.92; }
.admissions td, .admissions th { border-bottom-color: color-mix(in srgb, var(--beacon) 25%, transparent); }
</style>

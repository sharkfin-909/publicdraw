<script setup lang="ts">
import type { Admission } from '~/lib/types'
import { receiptLink } from '~/lib/receiptLink'

const props = defineProps<{ admitted: Admission[]; drawId: string }>()

const origin = computed(() =>
  import.meta.client ? location.origin : useRuntimeConfig().public.siteUrl,
)
/** The salt rides in the fragment, which browsers never send to a server. */
const linkFor = (a: Admission) =>
  receiptLink(origin.value, { drawId: props.drawId, index: a.index, salt: a.salt })

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
  <div
    class="mt-3.5 rounded-box border border-beacon bg-beacon-bg px-[18px] py-4
           [&_td]:border-b-beacon/25 [&_th]:border-b-beacon/25"
  >
    <div class="flex flex-wrap items-baseline justify-between gap-4 *:min-w-0">
      <h3 class="text-beacon">{{ admitted.length }} admitted — salts shown once</h3>
      <UiButton variant="ghost" @click="copy">{{ copied ? 'Copied' : 'Copy all' }}</UiButton>
    </div>
    <p class="max-w-[62ch] text-[0.82rem] text-beacon opacity-[0.92]">
      Send each entrant their link now. It is what lets them confirm their own inclusion
      after the cutoff, whether or not anything else ever reaches them — an entrant who was quietly
      dropped and never sent a receipt has nothing to notice.
    </p>
    <div class="overflow-x-auto">
      <table>
        <thead><tr><th>#</th><th>Entry</th><th>Link to send</th></tr></thead>
        <tbody>
          <tr v-for="a in admitted" :key="a.index">
            <td class="font-mono text-[0.85em]">{{ a.index }}</td>
            <td>{{ a.label }}</td>
            <td class="font-mono text-[0.78rem] text-muted wrap-anywhere">{{ linkFor(a) }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

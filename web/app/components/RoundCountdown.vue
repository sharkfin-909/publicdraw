<script setup lang="ts">
import { timeOfRound } from '~/lib/beacon'

const props = defineProps<{ round: number; frozenAt?: number }>()
const { label, published } = useCountdown(() => props.round)
</script>

<template>
  <div
    class="rounded-box border px-5 py-[18px]"
    :class="published ? 'border-seal bg-seal-bg text-seal' : 'border-beacon bg-beacon-bg text-beacon'"
  >
    <div
      class="font-mono leading-[1.1] font-medium tabular-nums"
      :class="published ? 'text-[1.1rem]' : 'text-[2rem]'"
    >
      {{ label }}
    </div>
    <div class="mt-1 text-[0.85rem]">
      <template v-if="published">
        round <span class="font-mono text-[0.85em]">{{ round }}</span> published
        {{ fmtTime(timeOfRound(round).getTime() / 1000) }}
      </template>
      <template v-else>
        until drand round <span class="font-mono text-[0.85em]">{{ round }}</span> exists
      </template>
    </div>
    <p v-if="!published && frozenAt" class="mt-3 mb-0 max-w-[56ch] text-[0.8rem] opacity-90">
      The entry list was frozen {{ Math.max(0, Math.floor(timeOfRound(round).getTime() / 1000) - frozenAt) }}s
      before this number can be computed — by anyone, including the organiser. That gap is the proof.
    </p>
  </div>
</template>

<script setup lang="ts">
/** A draw state, or a verdict shown in a state's colour. */
const props = defineProps<{ tone: string }>()

const tones: Record<string, string> = {
  open: 'bg-beacon-bg text-beacon',
  committed: 'bg-seal-bg text-seal',
  drawn: 'bg-seal-bg text-seal',
}
</script>

<template>
  <span
    class="whitespace-nowrap rounded-[10px] px-[9px] py-0.5 font-mono text-[0.68rem] uppercase tracking-[0.06em]"
    :class="tones[props.tone]"
  >
    <slot />
  </span>
</template>

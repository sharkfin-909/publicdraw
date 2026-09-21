<script setup lang="ts">
/**
 * The mark: a sealed stub, a perforated cutoff, and the beacon.
 *
 * Read left to right it is the protocol — freeze the list, close entries,
 * then the moment arrives. Below 64px the perforation closes up and is
 * dropped, and the stroke thickens, so it still reads at nav and favicon size.
 */
const props = withDefaults(
  defineProps<{ size?: number; tone?: 'auto' | 'mono' }>(),
  { size: 30, tone: 'auto' },
)

const small = computed(() => props.size < 44)
const height = computed(() => Math.round((props.size / 96) * 64))
</script>

<template>
  <svg
    :width="size"
    :height="height"
    viewBox="0 0 96 64"
    role="img"
    aria-label="publicdraw"
    class="mark"
    :class="tone"
  >
    <rect
      x="3" y="11" width="90" height="42" rx="7"
      fill="none" class="edge" :stroke-width="small ? 5 : 4"
    />
    <path d="M5 18 a5 5 0 0 1 5 -5 h24 v38 h-24 a5 5 0 0 1 -5 -5 z" class="stub" />
    <line
      v-if="!small"
      x1="34" y1="15" x2="34" y2="49"
      class="perf" stroke-width="3" stroke-linecap="round" stroke-dasharray="1 6"
    />
    <circle cx="64" cy="32" :r="small ? 8 : 7.5" class="beacon" />
  </svg>
</template>

<style scoped>
.mark .edge   { stroke: var(--ink); }
.mark .stub   { fill: var(--teal); }
.mark .perf   { stroke: var(--surface); }
.mark .beacon { fill: var(--amber); }

/* Single colour, for stamps and print. */
.mark.mono .edge   { stroke: currentColor; }
.mark.mono .stub   { fill: currentColor; }
.mark.mono .perf   { stroke: transparent; }
.mark.mono .beacon { fill: none; stroke: currentColor; stroke-width: 3; }
</style>

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

// The stub fills the frame's inner edge exactly, so it moves with the stroke:
// a 5-unit edge leaves the inside at 5, a 3-unit edge at 4.5.
const stub = computed(() => small.value
  ? 'M5 18 a5 5 0 0 1 5 -5 h24 v38 h-24 a5 5 0 0 1 -5 -5 z'
  : 'M4.5 18 a5.5 5.5 0 0 1 5.5 -5.5 h24 v39 h-24 a5.5 5.5 0 0 1 -5.5 -5.5 z')

// The broadcast: rings spreading from the beacon, fading as they go and cut
// off by the ticket, so they read as a signal rather than a target. They are
// clipped to the open face, from the right edge of the perforation (32.5 + 3.5)
// to the frame's inner edge. Too fine to read below 44px, so dropped there.
const rings = [
  { r: 14, opacity: 0.7 },
  { r: 20, opacity: 0.5 },
  { r: 26, opacity: 0.34 },
  { r: 32, opacity: 0.2 },
]
// Ids are document-wide, and the mark can appear more than once on a page.
const clipId = `mark-face-${useId()}`
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
      fill="none" class="edge" :stroke-width="small ? 5 : 3"
    />
    <path :d="stub" class="stub" />
    <line
      v-if="!small"
      x1="32.5" y1="17" x2="32.5" y2="47.5"
      class="perf" stroke-width="7" stroke-linecap="round" stroke-dasharray="0 10"
    />
    <template v-if="!small">
      <clipPath :id="clipId">
        <path d="M36 12.5 h50 a5.5 5.5 0 0 1 5.5 5.5 v28 a5.5 5.5 0 0 1 -5.5 5.5 h-50 z" />
      </clipPath>
      <g :clip-path="`url(#${clipId})`" fill="none" stroke-width="2.5">
        <circle
          v-for="ring in rings" :key="ring.r"
          cx="64" cy="32" :r="ring.r" class="ring" :style="{ opacity: ring.opacity }"
        />
      </g>
    </template>
    <circle cx="64" cy="32" :r="small ? 10 : 9" class="beacon" />
  </svg>
</template>

<style scoped>
.mark .edge   { stroke: var(--ink); }
.mark .stub   { fill: var(--seal); }
.mark .perf   { stroke: var(--surface); }
/* Lighter than --beacon, which is tuned for text: at logo size the deep
   purple read as a black dot, and the beacon is the idea the mark is about. */
.mark .beacon { fill: #785964; }
.mark .ring   { stroke: #785964; }

/* Single colour, for stamps and print. */
.mark.mono .edge   { stroke: currentColor; }
.mark.mono .stub   { fill: currentColor; }
.mark.mono .perf   { stroke: transparent; }
.mark.mono .ring   { stroke: currentColor; }
.mark.mono .beacon { fill: none; stroke: currentColor; stroke-width: 3; }
</style>

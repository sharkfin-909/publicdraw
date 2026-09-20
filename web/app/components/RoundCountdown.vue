<script setup lang="ts">
import { timeOfRound } from '~/lib/beacon'

const props = defineProps<{ round: number; frozenAt?: number }>()
const { label, published } = useCountdown(() => props.round)
</script>

<template>
  <div class="countdown" :class="{ published }">
    <div class="num">{{ label }}</div>
    <div class="sub">
      <template v-if="published">
        round <span class="mono">{{ round }}</span> published
        {{ timeOfRound(round).toLocaleTimeString() }}
      </template>
      <template v-else>
        until drand round <span class="mono">{{ round }}</span> exists
      </template>
    </div>
    <p v-if="!published && frozenAt" class="why">
      The entry list was frozen {{ Math.max(0, Math.floor(timeOfRound(round).getTime() / 1000) - frozenAt) }}s
      before this number can be computed — by anyone, including the organiser. That gap is the proof.
    </p>
  </div>
</template>

<style scoped>
.countdown {
  background: var(--amber-bg); border: 1px solid var(--amber);
  border-radius: 3px; padding: 18px 20px;
}
.countdown.published { background: var(--teal-bg); border-color: var(--teal); }
.num {
  font-family: var(--mono); font-size: 2rem; font-weight: 500;
  line-height: 1.1; color: var(--amber); font-variant-numeric: tabular-nums;
}
.published .num { color: var(--teal); font-size: 1.1rem; }
.sub { font-size: 0.85rem; color: var(--amber); margin-top: 4px; }
.published .sub { color: var(--teal); }
.why { font-size: 0.8rem; color: var(--amber); margin: 12px 0 0; max-width: 56ch; opacity: 0.9; }
</style>

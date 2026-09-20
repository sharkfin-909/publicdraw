<script setup lang="ts">
const props = defineProps<{ value: string; label?: string; full?: boolean }>()
const shown = computed(() =>
  props.full || props.value.length <= 24 ? props.value : `${props.value.slice(0, 16)}…${props.value.slice(-8)}`,
)
</script>

<template>
  <span class="hv" :title="value">
    <span v-if="label" class="lbl">{{ label }}</span>
    <code>{{ shown }}</code>
  </span>
</template>

<style scoped>
.hv { display: inline-flex; gap: 8px; align-items: baseline; }
.lbl { font-size: 0.72rem; color: var(--muted); }
code { font-family: var(--mono); font-size: 0.76rem; color: var(--muted); overflow-wrap: anywhere; min-width: 0; }
</style>

<script setup lang="ts">
const props = defineProps<{ value: string; label?: string; full?: boolean }>()
const shown = computed(() =>
  props.full || props.value.length <= 24 ? props.value : `${props.value.slice(0, 16)}…${props.value.slice(-8)}`,
)
</script>

<template>
  <span class="inline-flex items-baseline gap-2" :title="value">
    <span v-if="label" class="text-[0.72rem] text-muted">{{ label }}</span>
    <code class="min-w-0 font-mono text-[0.76rem] text-muted wrap-anywhere">{{ shown }}</code>
  </span>
</template>

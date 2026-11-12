<script setup lang="ts">
/** An inline message: `error` in fault red, `notice` in beacon plum. */
withDefaults(defineProps<{ tone?: 'error' | 'notice'; as?: string }>(), { tone: 'notice', as: 'div' })
</script>

<template>
  <component
    :is="as"
    class="rounded-box px-[13px] py-2.5 text-[0.88rem]"
    :class="tone === 'error' ? 'bg-fault-bg text-fault' : 'bg-beacon-bg text-beacon'"
  >
    <slot />
  </component>
</template>

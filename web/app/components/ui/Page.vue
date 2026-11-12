<script setup lang="ts">
/** The content column every page sits in. `roomy` is the homepage's wider rhythm. */
defineProps<{ roomy?: boolean }>()
</script>

<template>
  <main
    class="mx-auto flex min-w-0 max-w-page flex-col px-5 pt-7 pb-20 *:min-w-0"
    :class="roomy ? 'gap-10 max-[760px]:gap-7' : 'gap-4'"
  >
    <slot />
  </main>
</template>

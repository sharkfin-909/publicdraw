<script setup lang="ts">
/**
 * Solid (seal) or ghost button. With `to` it is a router link styled as a
 * button — slightly roomier, as the homepage calls to action were.
 */
const props = withDefaults(
  defineProps<{ variant?: 'solid' | 'ghost'; to?: string; type?: 'button' | 'submit' }>(),
  { variant: 'solid', type: 'button' },
)
const NuxtLink = resolveComponent('NuxtLink')
</script>

<template>
  <NuxtLink
    v-if="props.to"
    :to="props.to"
    class="inline-block rounded-box border border-seal px-[18px] py-2.5 text-[0.88rem] font-semibold
           hover:no-underline hover:brightness-112"
    :class="props.variant === 'ghost' ? 'bg-transparent text-seal' : 'bg-seal text-white'"
  >
    <slot />
  </NuxtLink>
  <button
    v-else
    :type="props.type"
    class="cursor-pointer rounded-box border border-seal px-4 py-[9px] text-[0.88rem] font-semibold
           enabled:hover:brightness-112 disabled:cursor-not-allowed disabled:opacity-45"
    :class="props.variant === 'ghost' ? 'bg-transparent text-seal' : 'bg-seal text-surface'"
  >
    <slot />
  </button>
</template>

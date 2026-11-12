<script setup lang="ts">
/**
 * The bordered panel most of the site is built from. Renders a <section> by
 * default, any other tag through `as`, or a router link when given `to`.
 *
 * Stacked cards are spaced by the one before them (`card` is only a marker for
 * that), except as direct children of a grid, where the grid's gap does it.
 */
const props = withDefaults(defineProps<{ as?: string; to?: string }>(), { as: 'section' })
const NuxtLink = resolveComponent('NuxtLink')
</script>

<template>
  <component
    :is="props.to ? NuxtLink : props.as"
    :to="props.to"
    class="card min-w-0 font-serif text-[1rem] rounded-card border border-line bg-surface px-7 py-6 shadow-card max-[560px]:px-5 max-[560px]:py-5
           [&_.font-mono]:wrap-anywhere [&_code]:wrap-anywhere
           [.card+&]:mt-4 [.grid>&]:mt-0!"
  >
    <slot />
  </component>
</template>

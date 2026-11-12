<script setup lang="ts">
/**
 * Icons for the homepage cards. Paths are from Lucide (lucide-static 1.72.0,
 * ISC licence, https://lucide.dev), inlined rather than installed so the page
 * pulls nothing extra.
 */
const props = withDefaults(
  defineProps<{ name: 'stamp' | 'radio-tower' | 'laptop-check'; tone?: 'seal' | 'beacon' }>(),
  { tone: 'seal' },
)

const paths: Record<typeof props.name, string> = {
  stamp:
    '<path d="M14 13V8.5C14 7 15 7 15 5a3 3 0 0 0-6 0c0 2 1 2 1 3.5V13"/>' +
    '<path d="M20 15.5a2.5 2.5 0 0 0-2.5-2.5h-11A2.5 2.5 0 0 0 4 15.5V17a1 1 0 0 0 1 1h14a1 1 0 0 0 1-1z"/>' +
    '<path d="M5 22h14"/>',
  'radio-tower':
    '<path d="M4.9 16.1C1 12.2 1 5.8 4.9 1.9"/>' +
    '<path d="M7.8 4.7a6.14 6.14 0 0 0-.8 7.5"/>' +
    '<circle cx="12" cy="9" r="2"/>' +
    '<path d="M16.2 4.8c2 2 2.26 5.11.8 7.47"/>' +
    '<path d="M19.1 1.9a9.96 9.96 0 0 1 0 14.1"/>' +
    '<path d="M9.5 18h5"/>' +
    '<path d="m8 22 4-11 4 11"/>',
  'laptop-check':
    '<path d="M2 20h20"/>' +
    '<path d="m9 10 2 2 4-4"/>' +
    '<rect x="3" y="4" width="18" height="12" rx="2"/>',
}
</script>

<template>
  <span class="feature-icon" :class="tone" aria-hidden="true">
    <svg
      viewBox="0 0 24 24" width="28" height="28"
      fill="none" stroke="currentColor" stroke-width="1.5"
      stroke-linecap="round" stroke-linejoin="round"
      v-html="paths[name]"
    />
  </span>
</template>

<style scoped>
.feature-icon {
  display: grid; place-items: center;
  width: 52px; height: 52px; border-radius: 3px;
  margin: 0 auto 14px;
}
.feature-icon.seal   { color: var(--seal);   background: var(--seal-bg); }
.feature-icon.beacon { color: var(--beacon); background: var(--beacon-bg); }
</style>

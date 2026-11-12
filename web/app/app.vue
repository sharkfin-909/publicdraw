<script setup lang="ts">
import type { ComponentPublicInstance } from 'vue'

const links = [
  { to: '/draws', label: 'Draws' },
  { to: '/verify', label: 'Verify' },
  { to: '/receipt', label: 'Receipt' },
  { to: '/log', label: 'Log' },
  { to: '/technical', label: 'Technical details' },
  { to: '/admin', label: 'Dashboard', end: true },
]

/**
 * The current item, from the path rather than router-link-active: a draw page
 * (/draws/…) or an entry check (/receipt/…) is not a nested route of its menu
 * item, so the router would mark nothing. The brand is not an item, so on "/"
 * nothing is current and the marker fades out.
 */
const route = useRoute()
const current = computed(
  () => links.find((l) => route.path === l.to || route.path.startsWith(`${l.to}/`))?.to,
)

// The marker: a plum dot that sits under the current item and glides to the
// next one. Positions are measured, so it follows wrapping and resizing.
const row = ref<HTMLElement>()
const anchors = new Map<string, HTMLElement>()
const keep = (to: string) => (el: Element | ComponentPublicInstance | null) => {
  const node = el && '$el' in el ? (el.$el as HTMLElement) : (el as HTMLElement | null)
  if (node) anchors.set(to, node)
  else anchors.delete(to)
}

const dot = reactive({ x: 0, y: 0, shown: false, glide: false })
const DOT = 5

function place() {
  const el = current.value ? anchors.get(current.value) : undefined
  if (!el) {
    dot.shown = false // fade out where it is, rather than flying off
    return
  }
  dot.x = el.offsetLeft + el.offsetWidth / 2 - DOT / 2
  dot.y = el.offsetTop + el.offsetHeight + 2
  dot.shown = true
}

let resize: ResizeObserver | undefined
onMounted(() => {
  place()
  // Webfonts change the labels' widths; re-measure once they land, then let it
  // glide — enabling the transition only now stops it sliding in from 0,0.
  document.fonts?.ready.then(() => {
    place()
    requestAnimationFrame(() => (dot.glide = true))
  })
  resize = new ResizeObserver(place)
  if (row.value) resize.observe(row.value)
})
onBeforeUnmount(() => resize?.disconnect())
watch(current, () => nextTick(place))
</script>

<template>
  <div>
    <!-- Translucent white, so the sky-blue ground and the die's rings pass softly
         behind it on scroll, while the logo keeps a light ground to read on. -->
    <nav class="sticky top-[env(safe-area-inset-top,0px)] z-10 border-b border-rule bg-white/85 backdrop-blur-md [view-transition-name:site-nav]">
      <!-- Baseline, so the row lines its links up on the wordmark; the mark
           itself is centred against the word rather than sitting on its baseline. -->
      <div ref="row" class="relative mx-auto flex max-w-page flex-wrap items-baseline gap-5 px-5 py-3">
        <NuxtLink
          to="/"
          class="inline-flex items-baseline gap-2.5 font-serif text-[1.4rem] font-semibold text-ink hover:no-underline"
        >
          <BrandMark :size="52" class="self-center" />
          <span>publicdraw</span>
        </NuxtLink>
        <NuxtLink
          v-for="l in links" :key="l.to" :ref="keep(l.to)" :to="l.to"
          class="text-[0.86rem] text-beacon"
          :class="[{ 'ml-auto': l.end }, current === l.to && 'font-semibold']"
          :aria-current="current === l.to ? 'page' : undefined"
        >
          {{ l.label }}
        </NuxtLink>

        <!-- The current-page marker: a dot with a ring pulsing out of it. -->
        <span
          aria-hidden="true"
          class="pointer-events-none absolute top-0 left-0 size-[5px]
                 motion-reduce:transition-none"
          :class="dot.glide && 'transition-[transform,opacity] duration-500 ease-[cubic-bezier(0.2,0.8,0.2,1)]'"
          :style="{ transform: `translate(${dot.x}px, ${dot.y}px)`, opacity: dot.shown ? 1 : 0 }"
        >
          <span class="absolute inset-0 rounded-full bg-beacon" />
          <span
            class="absolute inset-0 rounded-full bg-beacon
                   animate-[ping_2s_cubic-bezier(0,0,0.2,1)_infinite] motion-reduce:animate-none"
          />
        </span>
      </div>
    </nav>
    <BeaconBackdrop />
    <NuxtPage />
  </div>
</template>

<script setup lang="ts">
const links = [
  { to: '/draws', label: 'Draws' },
  { to: '/verify', label: 'Verify' },
  { to: '/receipt', label: 'Receipt' },
  { to: '/log', label: 'Log' },
  { to: '/technical', label: 'Technical details' },
]
// The brand is not a nav item: on "/" it would match router-link-active and the
// wordmark would turn teal, so the logo changed colour depending on the page.
const active = '[&.router-link-active]:font-semibold [&.router-link-active]:text-seal'
</script>

<template>
  <div>
    <!-- Translucent white, so the sky-blue ground and the die's rings pass softly
         behind it on scroll, while the logo keeps a light ground to read on. -->
    <nav class="sticky top-[env(safe-area-inset-top,0px)] z-10 border-b border-rule bg-white/85 backdrop-blur-md">
      <!-- Baseline, so the row lines its links up on the wordmark; the mark
           itself is centred against the word rather than sitting on its baseline. -->
      <div class="mx-auto flex max-w-page flex-wrap items-baseline gap-5 px-5 py-3">
        <NuxtLink
          to="/"
          class="inline-flex items-baseline gap-2.5 font-serif text-[1.4rem] font-semibold text-ink hover:no-underline"
        >
          <BrandMark :size="52" class="self-center" />
          <span>publicdraw</span>
        </NuxtLink>
        <NuxtLink
          v-for="l in links" :key="l.to" :to="l.to"
          class="text-[0.86rem] text-muted" :class="active"
        >
          {{ l.label }}
        </NuxtLink>
        <NuxtLink to="/admin" class="ml-auto text-[0.86rem] text-faint" :class="active">Dashboard</NuxtLink>
      </div>
    </nav>
    <BeaconBackdrop />
    <NuxtPage />
  </div>
</template>

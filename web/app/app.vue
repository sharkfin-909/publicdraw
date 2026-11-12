<template>
  <div>
    <nav class="top">
      <div class="inner">
        <NuxtLink to="/" class="brand">
          <BrandMark :size="52" />
          <span>publicdraw</span>
        </NuxtLink>
        <NuxtLink to="/draws">Draws</NuxtLink>
        <NuxtLink to="/verify">Verify</NuxtLink>
        <NuxtLink to="/receipt">Receipt</NuxtLink>
        <NuxtLink to="/log">Log</NuxtLink>
        <NuxtLink to="/admin" class="admin">Dashboard</NuxtLink>
      </div>
    </nav>
    <BeaconBackdrop />
    <NuxtPage />
  </div>
</template>

<style>
nav.top { background: var(--surface); }
/* Baseline, so the row lines its links up on the wordmark; the mark itself
   is centred against the word rather than sitting on its baseline. */
nav.top a.brand { display: inline-flex; align-items: baseline; gap: 10px; }
nav.top a.brand .mark { align-self: center; }
nav.top a.brand:hover { text-decoration: none; }
nav.top a.admin { margin-left: auto; color: var(--faint); }
.vis { border-color: var(--line); }
.vis.listed { border-color: var(--seal); }
</style>

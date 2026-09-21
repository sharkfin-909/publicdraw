<template>
  <div>
    <nav class="top">
      <div class="inner">
        <NuxtLink to="/" class="brand">
          <BrandMark :size="30" />
          <span>publicdraw</span>
        </NuxtLink>
        <NuxtLink to="/draws">Draws</NuxtLink>
        <NuxtLink to="/verify">Verify</NuxtLink>
        <NuxtLink to="/receipt">Receipt</NuxtLink>
        <NuxtLink to="/log">Log</NuxtLink>
        <NuxtLink to="/admin" class="admin">Dashboard</NuxtLink>
      </div>
    </nav>
    <NuxtPage />
  </div>
</template>

<style>
nav.top a.brand { display: inline-flex; align-items: center; gap: 9px; }
nav.top a.brand:hover { text-decoration: none; }
nav.top a.admin { margin-left: auto; color: var(--faint); }
.vis { border-color: var(--line); }
.vis.listed { border-color: var(--teal); }
</style>

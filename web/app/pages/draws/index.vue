<script setup lang="ts">
import { drawPath } from '~/utils/seo'

const api = useApi()

// Server-rendered: this is the directory crawlers walk to find draw pages,
// so its contents must be in the HTML rather than appear after hydration.
const { data, error } = await useAsyncData('public-draws', () => api.listPublic())
const draws = computed(() => data.value?.draws ?? [])

useSeo({
  title: 'Published draws',
  description:
    'Draws whose organisers chose to publish them. Every result can be checked independently, in your own browser, against a public randomness beacon.',
  path: '/draws',
})
</script>

<template>
  <main class="wrap stack">
    <header>
      <p class="eyebrow">directory</p>
      <h1>Published draws</h1>
      <p class="muted" style="max-width: 62ch">
        Only draws whose organiser chose to publish them appear here. Most draws are unlisted by
        default — a ballot for housing or school places names real people, and has no business in
        a search index.
      </p>
    </header>

    <div v-if="error" class="error">Could not load the directory.</div>

    <p v-else-if="!draws.length" class="muted">No published draws yet.</p>

    <div v-else class="stack" style="gap: 10px">
      <NuxtLink
        v-for="d in draws"
        :key="d.draw_id"
        :to="drawPath(d.draw_id, d.title)"
        class="card drawrow"
      >
        <div class="spread">
          <div>
            <div class="dtitle">{{ d.title || 'Untitled draw' }}</div>
            <div class="muted" style="font-size: 0.8rem">
              {{ d.winners }} to win from {{ d.entry_count }} ·
              {{ d.disclosure }} entry list
            </div>
          </div>
          <span class="pill" :class="d.state">{{ d.state }}</span>
        </div>
      </NuxtLink>
    </div>
  </main>
</template>

<style scoped>
.drawrow { display: block; color: inherit; }
.drawrow:hover { text-decoration: none; border-color: var(--teal); }
.dtitle { font-family: var(--serif); font-size: 1.08rem; font-weight: 600; }
</style>

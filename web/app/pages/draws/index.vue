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
  <UiPage>
    <header>
      <UiEyebrow>directory</UiEyebrow>
      <h1>Published draws</h1>
      <p class="max-w-[62ch] text-muted">
        Only draws whose organiser chose to publish them appear here. Most draws are unlisted by
        default — a ballot for housing or school places names real people, and has no business in
        a search index.
      </p>
    </header>

    <UiCallout v-if="error" tone="error">Could not load the directory.</UiCallout>

    <p v-else-if="!draws.length" class="text-muted">No published draws yet.</p>

    <div v-else class="flex min-w-0 flex-col gap-2.5 *:min-w-0">
      <UiCard
        v-for="d in draws"
        :key="d.draw_id"
        :to="drawPath(d.draw_id, d.title)"
        class="block text-inherit hover:border-seal hover:no-underline"
      >
        <div class="flex flex-wrap items-baseline justify-between gap-4 *:min-w-0">
          <div>
            <div class="font-serif text-[1.08rem] font-semibold">{{ d.title || 'Untitled draw' }}</div>
            <div class="text-[0.85rem] text-muted">
              {{ d.winners }} to win from {{ d.entry_count }} ·
              {{ d.disclosure }} entry list
            </div>
          </div>
          <UiPill :tone="d.state">{{ d.state }}</UiPill>
        </div>
      </UiCard>
    </div>
  </UiPage>
</template>

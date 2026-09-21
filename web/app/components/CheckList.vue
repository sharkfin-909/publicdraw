<script setup lang="ts">
import type { Report } from '~/lib/verify'

defineProps<{ report: Report }>()
</script>

<template>
  <div>
    <ul class="checks">
      <li v-for="c in report.checks" :key="c.name">
        <span class="tag" :class="c.skipped ? 'skip' : c.ok ? 'ok' : 'fail'">
          {{ c.skipped ? 'skip' : c.ok ? 'ok' : 'FAIL' }}
        </span>
        <span class="name">{{ c.name }}</span>
        <span class="detail">{{ c.detail }}</span>
      </li>
    </ul>

    <p v-if="report.ok" class="verdict pass">
      Verified in your browser. Nothing here was taken on the organiser's word.
    </p>
    <p v-else class="verdict fail">
      Failed — this bundle does not prove what it claims.
    </p>

    <p v-if="report.skipped" class="muted footnote">
      {{ report.skipped }} check{{ report.skipped === 1 ? '' : 's' }} skipped. A skipped check is
      not a passed one; the reason is on the line.
    </p>
  </div>
</template>

<style scoped>
.checks { list-style: none; padding: 0; margin: 0; }
.checks li {
  display: grid;
  grid-template-columns: 52px minmax(0, 1fr) minmax(0, auto);
  gap: 12px; align-items: baseline;
  padding: 7px 0; border-bottom: 1px solid var(--rule);
  font-size: 0.87rem;
}
.tag {
  font-family: var(--mono); font-size: 0.66rem; letter-spacing: 0.06em;
  padding: 2px 0; border-radius: 2px; text-align: center;
}
.tag.ok   { background: var(--seal-bg);  color: var(--seal); }
.tag.skip { background: var(--beacon-bg); color: var(--beacon); }
.tag.fail { background: var(--fault-bg); color: var(--fault); }
.detail { font-family: var(--mono); font-size: 0.74rem; color: var(--faint); text-align: right; }
.verdict { margin: 16px 0 0; font-weight: 600; font-size: 0.92rem; }
.verdict.pass { color: var(--seal); }
.verdict.fail { color: var(--fault); }
.footnote { font-size: 0.8rem; margin: 6px 0 0; }
@media (max-width: 620px) {
  .checks li { grid-template-columns: 52px 1fr; }
  .detail { grid-column: 2; text-align: left; }
}
</style>

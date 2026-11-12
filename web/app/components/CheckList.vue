<script setup lang="ts">
import type { Report } from '~/lib/verify'

defineProps<{ report: Report }>()
</script>

<template>
  <div>
    <ul>
      <li
        v-for="c in report.checks" :key="c.name"
        class="grid grid-cols-[52px_minmax(0,1fr)_minmax(0,auto)] items-baseline gap-3 border-b border-rule py-[7px]
               text-[0.95rem] max-[620px]:grid-cols-[52px_1fr]"
      >
        <span
          class="rounded-[2px] py-0.5 text-center font-mono text-[0.66rem] tracking-[0.06em]"
          :class="c.skipped ? 'bg-beacon-bg text-beacon' : c.ok ? 'bg-seal-bg text-seal' : 'bg-fault-bg text-fault'"
        >
          {{ c.skipped ? 'skip' : c.ok ? 'ok' : 'FAIL' }}
        </span>
        <span>{{ c.name }}</span>
        <span class="text-right font-mono text-[0.74rem] text-faint max-[620px]:col-start-2 max-[620px]:text-left">
          {{ c.detail }}
        </span>
      </li>
    </ul>

    <p v-if="report.ok" class="mt-4 mb-0 text-[1rem] font-semibold text-seal">
      Verified in your browser. Nothing here was taken on the organiser's word.
    </p>
    <p v-else class="mt-4 mb-0 text-[1rem] font-semibold text-fault">
      Failed — this bundle does not prove what it claims.
    </p>

    <p v-if="report.skipped" class="mt-1.5 mb-0 text-[0.85rem] text-muted">
      {{ report.skipped }} check{{ report.skipped === 1 ? '' : 's' }} skipped. A skipped check is
      not a passed one; the reason is on the line.
    </p>
  </div>
</template>

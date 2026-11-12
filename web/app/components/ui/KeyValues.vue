<template>
  <!-- A label column and a monospace value column; one column on a phone. -->
  <dl
    class="grid grid-cols-[minmax(110px,auto)_minmax(0,1fr)] gap-x-4 gap-y-1.5 text-[0.95rem]
           [&_dd]:m-0 [&_dd]:min-w-0 [&_dd]:font-mono [&_dd]:text-[0.88rem] [&_dd]:wrap-anywhere
           [&_dt]:min-w-0 [&_dt]:text-muted
           max-[560px]:grid-cols-1 max-[560px]:gap-x-0 max-[560px]:gap-y-0.5 max-[560px]:[&_dt]:mt-2"
  >
    <slot />
  </dl>
</template>

<script setup lang="ts">
const site = useRuntimeConfig().public

useSeo({
  title: 'Draws anyone can check',
  description:
    'Run a prize draw, ballot or allocation so the organiser demonstrably could not have steered the result — and so anyone can confirm that afterwards, in their own browser.',
  path: '/',
})

// Modest and accurate. Marking up claims the page does not make is worse
// than marking up nothing.
useHead({
  script: [
    {
      type: 'application/ld+json',
      innerHTML: JSON.stringify({
        '@context': 'https://schema.org',
        '@type': 'SoftwareApplication',
        name: site.siteName,
        applicationCategory: 'BusinessApplication',
        operatingSystem: 'Web',
        description:
          'Verifiable random selection for prize draws, ballots and allocation. Open protocol, independently checkable results.',
      }),
    },
  ],
})
</script>

<template>
  <main class="wrap stack">
    <header class="hero">
      <p class="eyebrow">verifiable random selection</p>
      <h1>Draws anyone can check</h1>
      <p class="lede">
        A draw is fair when the entry list was frozen before the randomness that picks from it
        existed. Both halves of that are claims about <em>when a value was fixed</em> — which is
        something you can prove, rather than ask people to take on trust.
      </p>
      <div class="row" style="margin-top: 22px">
        <NuxtLink to="/draws" class="btn">See published draws</NuxtLink>
        <NuxtLink to="/verify" class="btn ghost">Verify a result</NuxtLink>
      </div>
    </header>

    <section class="grid three">
      <div class="card">
        <h2>Committed before, not after</h2>
        <p>
          The rules are signed before anyone enters. The entry list is signed at the cutoff, while
          the deciding number still does not exist. An organiser who could freeze the list after
          seeing the randomness could pick the winner — so the ordering is what carries the
          guarantee, and every result is checked against it.
        </p>
      </div>
      <div class="card">
        <h2>Randomness nobody chose</h2>
        <p>
          The deciding number comes from
          <a href="https://drand.love" rel="noopener">drand</a>, a public beacon run by
          independent organisations including Cloudflare and EPFL. No single party can predict it,
          and — because the signature scheme is deterministic — none can influence it either.
        </p>
      </div>
      <div class="card">
        <h2>Checked in your browser</h2>
        <p>
          Verification runs on your machine against a key pinned in the page, not on ours. An
          answer about whether a draw was honest is worth nothing when it comes from the people
          who ran it.
        </p>
      </div>
    </section>

    <section class="card">
      <h2>What it does not prove</h2>
      <p>
        Worth stating plainly, because someone will find it otherwise. Receipts catch an entrant
        being dropped; they do not catch a list padded with entries that were never real.
        Timestamps are the operator's word until the log is anchored externally. And whether
        everyone who should have been able to enter was told the draw existed is outside what any
        of this can see.
      </p>
      <p style="margin-bottom: 0">
        <NuxtLink to="/log">The transparency log</NuxtLink> covers the one remaining gap a single
        result cannot: whether a draw was run repeatedly until the organiser liked the answer.
      </p>
    </section>
  </main>
</template>

<style scoped>
.hero { padding-block: 22px 8px; max-width: 62ch; }
.lede { font-family: var(--serif); font-size: 1.22rem; line-height: 1.5; color: var(--muted); }
.grid.three { grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); }
.grid .card h2 { font-size: 1.05rem; margin-bottom: 8px; }
.grid .card p { font-size: 0.9rem; margin: 0; }
.btn {
  display: inline-block; font-weight: 600; font-size: 0.88rem;
  padding: 10px 18px; border-radius: 3px;
  background: var(--teal); color: var(--surface); border: 1px solid var(--teal);
}
.btn:hover { text-decoration: none; filter: brightness(1.12); }
.btn.ghost { background: transparent; color: var(--teal); }
</style>

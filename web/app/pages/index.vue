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
  link: [
    {
      rel: 'preload',
      as: 'image',
      type: 'image/avif',
      href: '/hero/tickets-960.avif',
      imagesrcset:
        '/hero/tickets-640.avif 640w, /hero/tickets-960.avif 960w, /hero/tickets-1280.avif 1280w',
      imagesizes: '(max-width: 760px) calc(100vw - 40px), 352px',
      fetchpriority: 'high',
    },
  ],
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
    <section class="hero">
      <div class="hero-copy">
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
      </div>

      <!--
        Photograph by Planet Volumes via Unsplash (8QeyvEmQO6w), used under the
        Unsplash License. Cropped to 4:5 and re-encoded; see web/README.md.
      -->
      <figure class="hero-art">
        <!--
          Eager and high priority: this is the largest element above the fold,
          so it is the LCP candidate, and a hero that arrives late is the usual
          reason that metric fails. Width and height are set so the box is
          reserved before the bytes land and nothing shifts.
        -->
        <picture class="frame">
          <source
            type="image/avif"
            srcset="/hero/tickets-640.avif 640w, /hero/tickets-960.avif 960w, /hero/tickets-1280.avif 1280w"
            sizes="(max-width: 760px) calc(100vw - 40px), 352px"
          />
          <source
            type="image/webp"
            srcset="/hero/tickets-640.webp 640w, /hero/tickets-960.webp 960w, /hero/tickets-1280.webp 1280w"
            sizes="(max-width: 760px) calc(100vw - 40px), 352px"
          />
          <img
            src="/hero/tickets-960.jpg"
            srcset="/hero/tickets-640.jpg 640w, /hero/tickets-960.jpg 960w, /hero/tickets-1280.jpg 1280w"
            sizes="(max-width: 760px) calc(100vw - 40px), 352px"
            width="1280"
            height="1600"
            alt="Raffle tickets falling through the air, each one printed “keep this ticket”"
            fetchpriority="high"
            decoding="async"
          />
        </picture>
        <figcaption>Keep this ticket — every entrant gets a receipt they can check themselves.</figcaption>
      </figure>
    </section>

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
/*
  No band. The page ground and the photograph's paper are the same cream, so
  the hero is simply the top of the page and the image dissolves into it.
  That is what the feather was always for; on the old dark band there was
  nothing to dissolve into and it had to be switched off.
*/
.hero {
  display: grid;
  grid-template-columns: 1fr minmax(0, 0.72fr);
  gap: 40px;
  align-items: stretch;
  padding-block: 28px 12px;
  min-width: 0;
}
.hero-copy { min-width: 0; display: flex; flex-direction: column; justify-content: center; }
.hero .eyebrow { color: var(--seal); }
.lede {
  font-family: var(--serif); font-size: 1.22rem; line-height: 1.5;
  color: var(--muted); margin-bottom: 0;
}

.hero-art { margin: 0; min-width: 0; display: flex; flex-direction: column; }
/*
  No feather. The page ground and the photograph's paper are the same cream,
  so the join is already invisible where it matters, and the gradient that was
  meant to soften it cost three separate bugs — a mask that erased the image,
  stops that never engaged, then stops that washed it out. Not worth it.
*/
.hero-art .frame { display: block; flex: 1; min-height: 0; }
.hero-art img {
  display: block; width: 100%; height: 100%;
  max-height: 344px;
  object-fit: cover;
  border-radius: 3px;
}
.hero-art figcaption {
  margin-top: 6px; font-size: 0.78rem; color: var(--muted);
  font-style: italic; text-align: center;
}

.grid.three { grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); }
.grid .card h2 { font-size: 1.05rem; margin-bottom: 8px; }
.grid .card p { font-size: 0.9rem; margin: 0; }

.btn {
  display: inline-block; font-weight: 600; font-size: 0.88rem;
  padding: 10px 18px; border-radius: 3px;
  background: var(--seal); color: #ffffff; border: 1px solid var(--seal);
}
.btn:hover { text-decoration: none; filter: brightness(1.12); }
.btn.ghost { background: transparent; color: var(--seal); }

@media (max-width: 760px) {
  .hero { grid-template-columns: 1fr; gap: 22px; padding-block: 18px 8px; }
  .hero-art { order: -1; }
  .hero-art .frame { flex: none; }
  .hero-art img { aspect-ratio: 16 / 10; height: auto; max-height: none; }
}
</style>

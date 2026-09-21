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
  The photograph is warm cream and the rest of the site is cool slate. Rather
  than tint the image toward the palette and lose what makes it good, the hero
  gets its own field in the paper's exact colour, so the join is invisible and
  the warmth reads as a deliberate zone rather than a clash.
*/
.hero {
  --band: #eae2cc;
  --band-ink: #2c2618;
  --band-muted: #6a5f47;

  display: grid;
  grid-template-columns: 1fr minmax(0, 0.78fr);
  gap: 36px;
  align-items: center;
  background: var(--band);
  border-radius: 4px;
  padding: 40px;
  margin-block: 4px 8px;
  color: var(--band-ink);
  overflow: hidden;
}
.hero .eyebrow { color: #6f5320; }
.hero h1 { color: var(--band-ink); }
.hero-copy { min-width: 0; }
.lede {
  font-family: var(--serif); font-size: 1.22rem; line-height: 1.5;
  color: var(--band-muted); margin-bottom: 0;
}
.hero-art { margin: 0; min-width: 0; }
/*
  The photo's own paper is the same cream as the band, so a hard edge reads as
  a rendering artefact. It is feathered with an overlay painted in the band
  colour rather than a CSS mask: a mask depends on mask-mode resolving to
  alpha, and when it resolves to luminance instead the black gradient erases
  the image completely — which is exactly what happened here. An overlay has
  no such ambiguity and, because it uses var(--band), it stays correct when
  the band turns dark.
*/
.hero-art .frame { position: relative; display: block; }
.hero-art .frame::after {
  content: '';
  position: absolute; inset: 0;
  border-radius: 3px;
  background: radial-gradient(115% 105% at 50% 45%, transparent 55%, var(--band) 100%);
  pointer-events: none;
}
.hero-art img {
  display: block; width: 100%; height: auto;
  aspect-ratio: 4 / 5; object-fit: cover;
  border-radius: 3px;
}
.hero-art figcaption {
  margin-top: 12px; font-size: 0.78rem; color: var(--band-muted);
  font-style: italic; text-align: center;
}
.hero .btn { background: #2c2618; border-color: #2c2618; color: var(--band); }
.hero .btn.ghost { background: transparent; color: #2c2618; }

@media (prefers-color-scheme: dark) {
  .hero {
    --band: #241d12;
    --band-ink: #ece3cf;
    --band-muted: #a4957a;
  }
  .hero .eyebrow { color: #c79a4e; }
  .hero-art .frame::after {
    background:
      radial-gradient(115% 105% at 50% 45%, rgba(0, 0, 0, 0) 55%, var(--band) 100%),
      rgba(36, 29, 18, 0.18);
  }
  .hero .btn { background: #ece3cf; border-color: #ece3cf; color: #241d12; }
  .hero .btn.ghost { background: transparent; color: #ece3cf; }
}

@media (max-width: 760px) {
  .hero { grid-template-columns: 1fr; padding: 26px 20px; gap: 26px; }
  .hero-art { order: -1; }
  .hero-art img { aspect-ratio: 16 / 10; }
}
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

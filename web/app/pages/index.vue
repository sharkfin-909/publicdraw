<template>
  <UiPage roomy>
    <section class="grid grid-cols-2 items-stretch gap-10 max-[760px]:grid-cols-1 max-[760px]:gap-[22px]">
      <div class="flex min-w-0 flex-col justify-end">
        <UiEyebrow>verifiable random selection</UiEyebrow>
        <h1>Public Draws. Fair and square.</h1>
        <p class="mb-0 font-serif text-[1.22rem] leading-normal text-muted">
	When opportunities or resources need to be distributed fairly amongst a group of people, Publicdraw solves that problem.</p>
        <p class="mt-3.5 mb-0 font-serif text-[1.22rem] leading-normal text-muted">
          No one can steer the result, and anyone can check it.
        </p>
      </div>

      <!--
        Photograph by Planet Volumes via Unsplash (wcDU8ijwm6Y), used under the
        Unsplash License. Resized and re-encoded at its own 1.4:1 proportions; the
        frame crops it with object-fit. See web/README.md.
      -->
      <figure class="m-0 flex min-w-0 flex-col max-[760px]:-order-1">
        <!--
          Eager and high priority: this is the largest element above the fold,
          so it is the LCP candidate, and a hero that arrives late is the usual
          reason that metric fails. Width and height are set so the box is
          reserved before the bytes land and nothing shifts.
        -->
        <picture class="block min-h-0 flex-1 max-[760px]:flex-none">
          <source
            type="image/avif"
            srcset="/hero/ticket-strip-640.avif 640w, /hero/ticket-strip-960.avif 960w, /hero/ticket-strip-1280.avif 1280w"
            sizes="(max-width: 760px) calc(100vw - 40px), 420px"
          />
          <source
            type="image/webp"
            srcset="/hero/ticket-strip-640.webp 640w, /hero/ticket-strip-960.webp 960w, /hero/ticket-strip-1280.webp 1280w"
            sizes="(max-width: 760px) calc(100vw - 40px), 420px"
          />
          <!-- Toned down so the photo sits quietly on the sky-blue ground, with
               the same faint lift as the cards. -->
          <img
            src="/hero/ticket-strip-960.jpg"
            srcset="/hero/ticket-strip-640.jpg 640w, /hero/ticket-strip-960.jpg 960w, /hero/ticket-strip-1280.jpg 1280w"
            sizes="(max-width: 760px) calc(100vw - 40px), 420px"
            width="1280"
            height="914"
            alt="Strips of raffle tickets curling through the air, printed “ticket” and “keep this ticket”"
            fetchpriority="high"
            decoding="async"
            class="block size-full max-h-[344px] rounded-box border border-line object-cover shadow-card saturate-55
                   max-[760px]:aspect-[16/10] max-[760px]:h-auto max-[760px]:max-h-none"
          />
        </picture>
      </figure>
    </section>

<section class="grid grid-cols-[repeat(auto-fit,minmax(240px,1fr))] gap-7">
	<div class="">
  <!-- Icon and title sit above their card, not inside it. -->
  <div class="mb-3 text-center">
    <FeatureIcon name="tickets" raised />
    <h2 class="text-[1.05rem]">Examples</h2>
  </div>
  <UiCard as="div">
    <p class="mb-2 text-[0.9rem]">
      Whenever there are more people than places, and everyone needs to trust the choice was fair:
    </p>
    <ul class="m-0 list-disc pl-5 text-[0.9rem]">
      <li>Tickets for a sold-out gig or match</li>
      <li>Homes in a new affordable housing development</li>
      <li>Places at a popular school, club or course</li>
      <li>Community grants, prizes and giveaways</li>
    </ul>
  </UiCard>
  <!--
    Photograph by Curated Lifestyle via Unsplash (nSf9hpy35Yw), used under the
    Unsplash License. Below the fold, so it loads lazily; cropped to a band
    by object-fit, held a little above centre to keep the faces in frame.
  -->
  <figure class="m-0 mt-4">
    <picture>
      <source
        type="image/avif"
        srcset="/home/huddle-640.avif 640w, /home/huddle-960.avif 960w, /home/huddle-1280.avif 1280w, /home/huddle-1760.avif 1760w"
        sizes="(max-width: 760px) calc(100vw - 40px), 880px"
      />
      <source
        type="image/webp"
        srcset="/home/huddle-640.webp 640w, /home/huddle-960.webp 960w, /home/huddle-1280.webp 1280w, /home/huddle-1760.webp 1760w"
        sizes="(max-width: 760px) calc(100vw - 40px), 880px"
      />
      <img
        src="/home/huddle-960.jpg"
        srcset="/home/huddle-640.jpg 640w, /home/huddle-960.jpg 960w, /home/huddle-1280.jpg 1280w, /home/huddle-1760.jpg 1760w"
        sizes="(max-width: 760px) calc(100vw - 40px), 880px"
        width="1760"
        height="1174"
        alt="A group of people standing close together, arms around one another’s shoulders"
        loading="lazy"
        decoding="async"
        class="block aspect-[5/2] w-full rounded-box border border-line object-cover object-[50%_40%] shadow-card max-[760px]:aspect-[4/3]"
      />
    </picture>
  </figure>
  <div class="mt-8 mb-3 text-center">
    <FeatureIcon name="cog" tone="beacon" raised />
    <h2 class="text-[1.05rem]">How it works</h2>
  </div>
  <UiCard as="div">
    <ol class="m-0 list-decimal space-y-1.5 pl-5 text-[0.9rem]">
      <li>
        <strong>The rules are set.</strong> The organiser chooses how many winners there will be
        and when entries close. These are locked in straight away, before anyone enters.
      </li>
      <li>
        <strong>People enter.</strong> Each person gets their own link, which they can keep and
        use later to check they were included.
      </li>
      <li>
        <strong>Entries close.</strong> The list of entrants is sealed. From this moment nobody,
        not even the organiser, can add or remove anyone.
      </li>
      <li>
        <strong>A random number picks the winners.</strong> A few seconds later, a group of
        independent organisations publishes a random number. Nobody could know it in advance, so
        nobody could have planned around it.
      </li>
      <li>
        <strong>Anyone can check.</strong> The result comes with everything needed to confirm it
        was done fairly, and you can check it yourself in your browser.
      </li>
    </ol>
    <p class="mt-3 mb-0 text-[0.9rem]">
      <NuxtLink to="/technical">The technical details →</NuxtLink>
    </p>
  </UiCard>
        <!-- The two calls to action split the column evenly. -->
        <div class="mt-[22px] grid grid-cols-2 gap-3">
          <UiButton to="/draws" class="text-center">See published draws</UiButton>
          <UiButton to="/verify" variant="ghost" class="text-center">Verify a result</UiButton>
        </div>
  </div>
</section>

  </UiPage>
</template>

<script setup lang="ts">
const site = useRuntimeConfig().public

useSeo({
  title: 'Public Draws. Fair and square.',
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
      href: '/hero/ticket-strip-960.avif',
      imagesrcset:
        '/hero/ticket-strip-640.avif 640w, /hero/ticket-strip-960.avif 960w, /hero/ticket-strip-1280.avif 1280w',
      imagesizes: '(max-width: 760px) calc(100vw - 40px), 420px',
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

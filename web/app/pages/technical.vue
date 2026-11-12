<script setup lang="ts">
useSeo({
  title: 'Technical details',
  description:
    'Why a publicdraw result cannot be steered: parameters and entry list committed before the randomness exists, a public drand beacon nobody controls, and verification in your own browser.',
  path: '/technical',
})
</script>

<template>
  <UiPage>
    <header>
      <UiEyebrow>technical details</UiEyebrow>
      <h1>Why the result can be trusted</h1>
      <p class="max-w-[62ch] text-muted">
        What makes a draw fair here, what the checks prove, and where their limits are.
      </p>
    </header>

    <!-- Up to four across; with three cards, three equal columns. -->
    <section class="grid grid-cols-[repeat(auto-fit,minmax(200px,1fr))] gap-3.5">
      <UiCard as="div">
        <FeatureIcon name="stamp" />
        <h2 class="mb-2 text-[1.05rem]">Committed before, not after</h2>
        <p class="m-0 text-[0.9rem]">
          The rules are signed before anyone enters. The entry list is signed at the cutoff, while
          the deciding number still does not exist. An organiser who could freeze the list after
          seeing the randomness could pick the winner — so the ordering is what carries the
          guarantee, and every result is checked against it.
        </p>
      </UiCard>
      <UiCard as="div">
        <FeatureIcon name="radio-tower" tone="beacon" />
        <h2 class="mb-2 text-[1.05rem]">Randomness nobody chose</h2>
        <p class="m-0 text-[0.9rem]">
          The deciding number comes from
          <a href="https://drand.love" rel="noopener">drand</a>, a public beacon run by
          independent organisations including Cloudflare and EPFL. No single party can predict it,
          and — because the signature scheme is deterministic — none can influence it either.
        </p>
      </UiCard>
      <UiCard as="div">
        <FeatureIcon name="laptop-check" />
        <h2 class="mb-2 text-[1.05rem]">Checked in your browser</h2>
        <p class="m-0 text-[0.9rem]">
          Verification runs on your machine against a key pinned in the page, not on ours. An
          answer about whether a draw was honest is worth nothing when it comes from the people
          who ran it.
        </p>
      </UiCard>
    </section>

    <UiCard>
      <h2>What it does not prove</h2>
      <p>
        Worth stating plainly, because someone will find it otherwise. Receipts catch an entrant
        being dropped; they do not catch a list padded with entries that were never real.
        Timestamps are the operator's word until the log is anchored externally. And whether
        everyone who should have been able to enter was told the draw existed is outside what any
        of this can see.
      </p>
      <p class="mb-0">
        <NuxtLink to="/log">The transparency log</NuxtLink> covers the one remaining gap a single
        result cannot: whether a draw was run repeatedly until the organiser liked the answer.
      </p>
    </UiCard>

    <UiCard>
      <h2>The process in full</h2>
      <p class="max-w-[62ch] text-[0.9rem] text-muted">
        Every stage of a draw, from the moment it is set up to the point anyone can check it. Each
        step can only happen after the one before, and the bundle records when each one did.
      </p>
      <ol class="mt-5 mb-0 space-y-4">
      <li class="grid grid-cols-[2rem_minmax(0,1fr)] gap-x-3 border-t border-rule pt-4 first:border-t-0 first:pt-0">
        <span class="flex size-7 items-center justify-center rounded-full bg-seal-bg font-mono text-[0.78rem] text-seal">1</span>
        <div>
          <h3 class="mb-1.5 font-serif text-[1.05rem]">The rules are fixed and signed</h3>
          <p class="m-0 text-[0.9rem]">
          When a draw is created its parameters are fixed in one record: a random draw id, the
          title, the number of winners, the cutoff time, whether the entry list will be public or
          private, and the <em>target round</em> — the drand round whose randomness will decide it,
          chosen to fall at least 15 seconds after the cutoff. The organiser's key (Ed25519) signs
          that record and it is appended to the public transparency log, all before anyone has
          entered. Changing any of it afterwards would produce a different draw, with a different
          signature and a different log entry.
          </p>
        </div>
      </li>
      <li class="grid grid-cols-[2rem_minmax(0,1fr)] gap-x-3 border-t border-rule pt-4 first:border-t-0 first:pt-0">
        <span class="flex size-7 items-center justify-center rounded-full bg-seal-bg font-mono text-[0.78rem] text-seal">2</span>
        <div>
          <h3 class="mb-1.5 font-serif text-[1.05rem]">Entries are salted as they arrive</h3>
          <p class="m-0 text-[0.9rem]">
          Each entry is given a position and 32 bytes of fresh randomness, its <em>salt</em>. The
          salt goes straight back to the entrant, inside the link they are handed, and is not kept
          on the server. What goes into the list is a <em>leaf</em>: a hash of the entry and its
          salt together. Without the salt a leaf reveals nothing, so even an email address cannot be
          guessed back out of it.
          </p>
        </div>
      </li>
      <li class="grid grid-cols-[2rem_minmax(0,1fr)] gap-x-3 border-t border-rule pt-4 first:border-t-0 first:pt-0">
        <span class="flex size-7 items-center justify-center rounded-full bg-seal-bg font-mono text-[0.78rem] text-seal">3</span>
        <div>
          <h3 class="mb-1.5 font-serif text-[1.05rem]">The list is frozen at the cutoff</h3>
          <p class="m-0 text-[0.9rem]">
          When entries close, the service builds a Merkle tree over the leaves, in order. Its root is
          a single fingerprint of the entire list: change, add or remove one entry and the root
          changes. The root and the number of entries are signed, written to the log, and every
          leaf is published. The log head is also posted to Sigstore's Rekor, a public log the
          organiser does not run. At this point the target round has not yet happened, so the
          number that will pick the winners does not exist.
          </p>
        </div>
      </li>
      <li class="grid grid-cols-[2rem_minmax(0,1fr)] gap-x-3 border-t border-rule pt-4 first:border-t-0 first:pt-0">
        <span class="flex size-7 items-center justify-center rounded-full bg-seal-bg font-mono text-[0.78rem] text-seal">4</span>
        <div>
          <h3 class="mb-1.5 font-serif text-[1.05rem]">A public beacon publishes the randomness</h3>
          <p class="m-0 text-[0.9rem]">
          <a href="https://drand.love" rel="noopener">drand</a> publishes a new random value every
          three seconds. Each one is a threshold signature made jointly by independent
          organisations in the League of Entropy: no single member, and no organiser, can predict
          it before it appears or choose what it will be. When the target round arrives, its value
          is checked against drand's public key, which is pinned in this site's code.
          </p>
        </div>
      </li>
      <li class="grid grid-cols-[2rem_minmax(0,1fr)] gap-x-3 border-t border-rule pt-4 first:border-t-0 first:pt-0">
        <span class="flex size-7 items-center justify-center rounded-full bg-seal-bg font-mono text-[0.78rem] text-seal">5</span>
        <div>
          <h3 class="mb-1.5 font-serif text-[1.05rem]">The winners are computed, not chosen</h3>
          <p class="m-0 text-[0.9rem]">
          A <em>seed</em> is made by hashing together the list's root, its size, the round's
          randomness and the draw id. Each entry then gets a <em>ticket</em>, a hash of the seed and
          its position, and every entry is sorted by ticket. The first ones are the winners; the
          rest, in order, are the waitlist. Nothing in this step involves a decision: anyone with
          the same inputs gets the same winners, and binding the root and draw id means a different
          list or a different draw cannot reuse a lucky value.
          </p>
        </div>
      </li>
      <li class="grid grid-cols-[2rem_minmax(0,1fr)] gap-x-3 border-t border-rule pt-4 first:border-t-0 first:pt-0">
        <span class="flex size-7 items-center justify-center rounded-full bg-seal-bg font-mono text-[0.78rem] text-seal">6</span>
        <div>
          <h3 class="mb-1.5 font-serif text-[1.05rem]">Everything is published as a bundle</h3>
          <p class="m-0 text-[0.9rem]">
          The result is a single file holding every piece above: the signed parameters, the signed
          roster commitment, the leaves, the drand round and its signature, the winners, the log
          records, and the Rekor proof. A public draw includes every entry, so the whole list can be
          rebuilt; a private one names only the winners, each with a proof that they were in the
          committed list.
          </p>
        </div>
      </li>
      <li class="grid grid-cols-[2rem_minmax(0,1fr)] gap-x-3 border-t border-rule pt-4 first:border-t-0 first:pt-0">
        <span class="flex size-7 items-center justify-center rounded-full bg-seal-bg font-mono text-[0.78rem] text-seal">7</span>
        <div>
          <h3 class="mb-1.5 font-serif text-[1.05rem]">Anyone checks it, in their own browser</h3>
          <p class="m-0 text-[0.9rem]">
          The verifier on this site checks the bundle step by step: the signatures, that the
          rules were fixed before the cutoff, that the list was frozen before the randomness
          existed, the drand signature, that the winners follow from the seed, the proofs, and the
          log chain. It is a separate implementation from the server, written in a different
          language, so a bug in one cannot quietly confirm itself in the other. Nothing is uploaded,
          and no server is asked for its opinion.
          </p>
        </div>
      </li>
      <li class="grid grid-cols-[2rem_minmax(0,1fr)] gap-x-3 border-t border-rule pt-4 first:border-t-0 first:pt-0">
        <span class="flex size-7 items-center justify-center rounded-full bg-seal-bg font-mono text-[0.78rem] text-seal">8</span>
        <div>
          <h3 class="mb-1.5 font-serif text-[1.05rem]">Entrants confirm they were included</h3>
          <p class="m-0 text-[0.9rem]">
          An entrant needs only their link. With the salt in it, this site hashes their entry and looks
          for that leaf in the published list. The part of the link holding the salt is never sent
          to a server. Entrants can also keep a <em>receipt</em>, a short proof of their own place that
          checks against the root alone.
          </p>
        </div>
      </li>
      <li class="grid grid-cols-[2rem_minmax(0,1fr)] gap-x-3 border-t border-rule pt-4 first:border-t-0 first:pt-0">
        <span class="flex size-7 items-center justify-center rounded-full bg-seal-bg font-mono text-[0.78rem] text-seal">9</span>
        <div>
          <h3 class="mb-1.5 font-serif text-[1.05rem]">The log shows draws that were abandoned</h3>
          <p class="m-0 text-[0.9rem]">
          One trick survives every check above: run a draw, dislike the result, quietly drop it and run
          another. So every creation, cutoff and result is appended to the transparency log, a chain
          in which changing any record breaks every one after it. A draw whose round has passed with
          no result recorded is listed openly. That is not proof of foul play, since draws are
          cancelled for honest reasons, but it is the only place that trick could show up, and it
          shows up there.
          </p>
        </div>
      </li>
      </ol>
    </UiCard>
  </UiPage>
</template>

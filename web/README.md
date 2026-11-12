# publicdraw web

Nuxt 4 front end for the draw service, and — more to the point — a **second,
independent implementation of the verifier**.

## Why the verification is in the browser

The Go service exposes `POST /v1/verify`. This app does not call it.

An answer about whether a draw was honest is worth nothing when it comes from
the party who ran the draw. So every bundle is checked here instead, in the
reader's own browser, against a drand group public key pinned in
`app/lib/beacon.ts`. Nothing is uploaded and no server is consulted.

That is also why this code exists at all rather than reusing the Go verifier
via WASM. A single implementation that both produces and verifies proofs cannot
catch its own hashing bug — it computes a wrong digest and then confirms it.
Two implementations in two languages fail loudly instead.

The contract between them is `../testdata/testvectors.json`, replayed by
`test/vectors.test.ts`. If Go and TypeScript ever disagree by one byte, that
suite goes red.

## Layout

```
app/lib/           the verifier — no Nuxt, no DOM, portable
  hashing.ts       domain-separated, length-prefixed digests
  merkle.ts        leaf/node separation, odd-node promotion
  draw.ts          seed derivation, ticket ranking, signing preimages
  translog.ts      hash-chain replay
  beacon.ts        drand quicknet, BLS verification, pinned group key
  verify.ts        the nineteen checks
app/pages/         draws · verify · receipt · log
app/components/    CheckList · RoundCountdown · HashValue
test/              cross-implementation vectors, and a captured live bundle
```

## Running

```sh
# terminal 1 — the draw service
cd .. && go run ./cmd/vc serve

# terminal 2 — this app (proxies /api to :8080, so the Go server needs no CORS)
pnpm dev
```

`pnpm test` runs both suites without either server.

## Salts are issued at entry, not at the cutoff

When someone enters, they are handed their index and salt immediately — the
`POST /draws/{id}/entries` response carries them, and the UI shows them once.

That timing is the whole point. At the cutoff the full list of committed
**leaves** is published (`GET /draws/{id}/leaves`, and in the bundle), and an
entrant holding their salt can hash their own label and look for themselves in
it. Nothing has to be delivered to them.

Without this, receipts were the only defence against being dropped — and a
receipt that never arrived is indistinguishable from one that was never issued,
so an entrant removed before the commit would have had nothing to notice.

Publishing every leaf is safe in either disclosure mode: a leaf is
`H(tag, salt, label)` with 32 bytes of fresh randomness, so it discloses
nothing to anyone who does not already hold that salt. It also leaks no more
than a Merkle proof already does, since a proof reveals sibling leaves anyway.

It buys one thing beyond the self-check: the root becomes independently
rebuildable even for a private draw, so verification no longer rests on
per-winner proofs alone.

## Notes

The countdown on a draw page is not decoration. The gap between the roster
being frozen and the target round publishing is the guarantee the protocol
rests on, and it is the only part of it a person can watch happen — so it is
shown at full size, against the round number, rather than hidden.

`POST /v1/verify` stays in the Go API for callers that genuinely cannot run a
verifier. It is the weakest way to check a bundle and this app does not model
that habit.

## Rekor anchoring

The local transparency log is a hash chain, and a hash chain is tamper-evident
only against someone who already saw an earlier head. If nobody wrote one down,
an organiser can rebuild it from scratch and there is nothing to compare
against.

So each log head is published to [Rekor](https://docs.sigstore.dev/logging/overview/),
Sigstore's public append-only log, at the moment the roster is frozen. The
bundle carries the inclusion proof, and `app/lib/rekor.ts` checks it here in
the browser — an anchor verified by the organiser's server would defeat the
point of anchoring.

Two details worth knowing:

**Rekor v2 reports `integratedTime` as 0.** It is not a timestamp authority.
Time comes instead from the *witness cosignatures* on the checkpoint, where
several independent operators countersign the tree head along with the moment
they saw it. Those are parsed and shown, and reported as **observed rather than
verified** — checking them needs each witness's key and a policy for how many
must agree.

**The anchor covers the head as it stood when it was published**, not the head
at the end of the draw, which the result record moves. Verification therefore
checks that the anchored payload is a head the chain passed through *and* that
the roster had been committed by that point. An anchor from before the entry
list was frozen bounds nothing about when it was frozen, and is reported that
way.

## Not done

CORS on the Go service, so a page not served from this origin can fetch a
bundle directly. Right now cross-origin verification means downloading the
bundle and dropping it on `/verify`, which works but is a worse path.

## Image credit

The hero photograph is by Planet Volumes via Unsplash (`wcDU8ijwm6Y`), used
under the Unsplash License, which permits commercial use without attribution —
credited here anyway. Its derivatives in `web/public/hero/` (`ticket-strip-*`)
keep the photo's own 1.4:1 proportions, since the frame crops it with
object-fit, and are re-encoded to AVIF, WebP and JPEG at three widths; the
original is not in the repo.

The social card, `og-tickets.jpg`, is still cut from the earlier photograph
(`8QeyvEmQO6w`, same photographer and licence), as are the unused `tickets-*`
files beside it.

The homepage photograph above "How it works" is by Curated Lifestyle via
Unsplash (`nSf9hpy35Yw`), under the same licence. Its derivatives are in
`web/public/home/` (`huddle-*`, four widths in each format); the frame crops it
to a band with object-fit.

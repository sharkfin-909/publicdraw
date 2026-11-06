# publicdraw

Runs public draws — grants, ballots, allocations, giveaways — so that **the
organiser cannot steer the result** and anyone can check it offline, without
trusting the organiser or this software.

```
$ publicdraw verify -timeline bundle.json

  created   2026-09-20 18:13:06Z  parameters fixed, nobody has entered yet
  cutoff    2026-09-20 18:13:31Z  entries close
  frozen    2026-09-20 18:13:32Z  roster committed and signed
  round 32374888  2026-09-20 18:13:48Z  drand publishes — 16s after the roster was frozen

  ok    roster commitment binds root and size                 commit=a360c7aa… size=8
  ok    operator signature over draw parameters               valid
  ok    parameters fixed before the cutoff                    created=1789927986 cutoff=1789928011
  ok    roster frozen before the beacon round existed         lead=16s
  ok    drand signature verifies against chain public key     valid
  ok    winners derive from the seed                          2 winner(s) of 8 entries
  ok    each winner is proven to be in the committed roster   2 proof(s)
  ok    log records form an unbroken chain                    chain intact
  SKIP  log head timestamped before the beacon round          no OTS proof in bundle
  SKIP  this draw was not abandoned and retried               answerable only against the full log

VERIFIED — "Two community grants", 2 winner(s) of 8 entries
  1. #5  frank@example.com
  2. #7  heidi@example.com
```

## The idea

A draw is fair when two things are true, and both are questions about *when a
value was fixed*:

- the **entry list** was frozen before the randomness existed, and
- the **randomness** was not chosen by anybody.

Two primitives answer those, bounding time from opposite directions:

| | gives | direction |
|---|---|---|
| [drand](https://drand.love) beacon | "this could not have existed before round *N*" | lower bound |
| [OpenTimestamps](https://opentimestamps.org) | "this existed by time *T*" | upper bound |

The organiser commits to the parameters before anyone enters, commits to the
entry list at the cutoff, and the winners fall out of a beacon round that had
not yet published when the list was frozen.

```
create ───── entries accumulate ───── CUTOFF ───── target round ───── publish
  │                                     │                              │
  params committed                      roster committed               winners derived
  (cutoff, round, winners,              (Merkle root + size,           (seed = H(root, randomness, id);
   disclosure — all signed)              signed; randomness             entries ranked by ticket)
                                         does not exist yet)
```

## Why the ordering carries the whole guarantee

Deriving winners from a public beacon is not enough on its own.

If the organiser can fix the entry list *after* seeing the beacon value, they
can add or drop entries until the winner suits them. If they can pick the
cutoff moment after watching who entered, same thing. If they can choose a new
draw id after the fact, they can grind ids until the index lands where they
like — and every hash and signature in the resulting bundle still verifies
perfectly.

So the times are pinned in advance and checked afterwards:

- parameters signed **before** the cutoff
- roster signed **at or after** the cutoff and **before** the target round
- at least 15s between cutoff and round, so the round is unambiguously unborn

Waiting is not overhead. It is the proof.

## Privacy is the organiser's choice

Every roster is a salted Merkle tree either way. What the organiser picks is
how much the published bundle discloses.

| | bundle contains | verified by |
|---|---|---|
| `public` | every leaf, plus every entry | rebuilding the tree from the leaves, and checking the entries hash to them |
| `private` | every leaf, winners only disclosed | rebuilding the tree from the leaves, plus per-winner proofs |

Both modes publish the **committed leaves**, in order. That is safe: a leaf is
`H(tag ‖ salt ‖ label)` with 32 bytes of fresh randomness, so it discloses
nothing to anyone who does not already hold that salt — and it leaks no more
than a Merkle proof already does, since a proof reveals sibling leaves anyway.
It makes the root independently rebuildable even for a private draw.

Salts are not optional in either mode. Entry labels are low-entropy — an email
address, a name, a ticket number — so an unsalted leaf can be brute-forced,
and anyone holding the root could test "was alice@example.com in this draw?"

## How an entrant knows they were included

**Salts are issued at entry, not at the cutoff.** Whoever enters is handed
their index and salt immediately. Once the leaves are published they hash their
own label and look for themselves in the list — no receipt, and nothing has to
be delivered to them.

That timing carries more weight than it looks. Receipts alone are a weak
defence against being dropped, because an entrant removed before the commit
does not get a receipt that fails to verify — they get no receipt at all, and
with no expectation of one they have nothing to notice. A salt held since entry
is different.

A **receipt** is still useful and still issued: it is one entrant's Merkle
proof of their own place, self-contained, so they can verify against the root
alone without downloading every leaf. It just is not the only path any more.

Neither closes the opposite gap. A roster padded with entrants who were never
real passes every check here.

## The transparency log

One attack survives all of the above: create a draw, see the result, quietly
abandon it, try again. Every individual bundle verifies. The fraud lives in
what was *not* published.

So every draw's creation, cutoff and result are appended to a public hash
chain. A draw that was created and whose round has since published, with no
result logged, shows up in `publicdraw log -abandoned`. That is not proof of
misconduct — draws get cancelled for honest reasons — but it is the only place
the attack can appear at all.

Same shape as Certificate Transparency: the answer to invisible misissuance is
to make the *set* of commitments public and append-only.

## Anchoring the log

A hash chain only bites once someone has seen a head. Left to itself the log
rests on the organiser having published it honestly — which is the thing under
question.

So `publicdraw commit` publishes the head to **Sigstore's Rekor**, a public
append-only log the organiser does not run, and keeps the inclusion proof in
the bundle. Verification recomputes the proof (RFC 6962) and checks it lands on
the root the log's signed checkpoint carries.

```sh
publicdraw anchor -draw ID      # publish the current head by hand
```

Rekor v2's `integratedTime` is always 0 — it is not a timestamp authority. Time
comes from the **witness cosignatures** on the checkpoint: several independent
operators countersign the tree head with the moment they saw it. `publicdraw verify`
surfaces those and marks them `SKIP`, because verifying them needs each
witness's key and a policy for how many must agree. An observed time from three
independent witnesses is worth showing; it is not worth calling proven.

The anchor is a second key: ECDSA P-256, because Rekor's `hashedrekord` type
signs a digest and rejects Ed25519 outright. The two keys have different jobs —
the Ed25519 operator key signs what the organiser commits to, and this one only
says "these bytes were published".

## What is not proven

Stated plainly, because someone will find these otherwise:

- **Roster stuffing.** Receipts catch an entrant being dropped. They do not
  catch a roster padded with entries that were never real.
- **The operator's clock.** Timestamps are the operator's word. Anchoring into
  Rekor narrows this — three independent witnesses countersign the tree head
  with a time — but those cosignatures are not verified here, so the check
  stays `SKIP` rather than passing.
- **Retries**, from a single bundle. That question is answerable only against
  the full public log, and is also reported as `SKIP`.

## Install

Needs Go 1.25 or later.

```sh
go install github.com/sharkfin-909/publicdraw/cmd/publicdraw@latest
```

## Usage

```sh
publicdraw keygen

publicdraw create -title "Two community grants" -winners 2 -disclosure private -open 10m
publicdraw enter  -draw ID -file entrants.txt -salts salts.tsv   # salts shown once
publicdraw leaves -draw ID                      # the published leaf list
publicdraw commit -draw ID                      # at the cutoff; freezes and logs the roster
publicdraw receipt -draw ID -index 5 -out receipt.json
publicdraw run    -draw ID                      # at the target round; publishes the bundle

publicdraw verify bundle.json [-timeline]
publicdraw check-receipt receipt.json
publicdraw log [-abandoned]
PUBLICDRAW_ORGANISER_TOKEN=... publicdraw serve -addr :8080
```

Anchor the log head after each cutoff, so the ordering does not rest on your
own clock:

```sh
echo -n <log head> | xxd -r -p > log.head && ots stamp log.head
```

## HTTP API

`publicdraw serve` exposes the same rules — every ordering constraint lives in the
service layer, so the API cannot permit something the CLI forbids.

While it runs, `serve` freezes each draw as its cutoff passes, because the
window before the target round is only seconds long. A draw with too few entries
stays open and cannot be completed.

Routes marked `*` need the organiser token as `Authorization: Bearer …`, because
they sign with the operator key, write to the log or list unlisted draws.
Everything a verifier needs stays open. `serve` refuses to start without a token
of at least 32 characters, unless `-no-auth` is passed for local development.

```
GET  /v1/operator                      public key, chain, log head
POST /v1/draws                       * create
POST /v1/draws/{id}/entries          * add entries; returns a salt per entrant
POST /v1/draws/{id}/commit           * freeze the roster
GET  /v1/draws/{id}/leaves             every committed leaf, in order
GET  /v1/draws/{id}/proof/{index}      one entrant's inclusion proof
GET  /v1/draws/{id}/bundle             the published result, once drawn
POST /v1/draws/{id}/run              * draw and publish
GET  /v1/log  ·  /v1/log/abandoned     the public chain
POST /v1/verify                        check a posted bundle
```

`POST /v1/verify` is the weakest way to verify, because the answer comes from
the server. The CLI exists so nobody has to take it.

## Hosting

`docker-compose.yml` runs the service and the web app, with only the web app
reachable, on loopback, for a reverse proxy in front. `deploy/` holds the nginx
site. In a `.env` beside the compose file set `PUBLICDRAW_ORGANISER_TOKEN`
(`openssl rand -hex 32`) and `NUXT_PUBLIC_SITE_URL`, then:

```sh
docker compose run --rm api keygen
docker compose up -d --build
```

## Packages

| | |
|---|---|
| `hashing` | domain-separated, length-prefixed digests |
| `merkle` | distinct leaf/node tags (second preimage); odd nodes promoted, not duplicated (CVE-2012-2459) |
| `roster` | salted entries, disclosure modes, receipts |
| `draw` | two-phase commitment, seed derivation, ranked selection |
| `beacon` | drand quicknet, BLS verified locally against the group key |
| `translog` | append-only hash chain |
| `service` | lifecycle and ordering enforcement |
| `httpapi` | thin HTTP surface |
| `rekor` | anchors the log head in Sigstore's Rekor; proofs recomputed locally |
| `bundle` | runs every check, with no short-circuit |
| `web` | Nuxt front end, with a second verifier in TypeScript ([details](web/README.md)) |

## Design notes

**Selection is a ranking, not a lottery.** Each entry gets a ticket
`H(seed ‖ index)`; entries sort by ticket; the first *k* win. No rejection
loop, no PRNG for another implementation to disagree about, and the places
after the winners are a waitlist — which allocation draws always need. Winners
are a prefix of the ranking, so a *k+1*-winner draw has the same first *k*.

**Size is committed.** The signed value is `H(root ‖ size)`, so a tree cannot
be re-padded to change which index a seed selects.

**Length-prefixed fields**, because `"ab"+"cdef"` and `"abc"+"def"` are
otherwise the same bytes.

**Verification never short-circuits**, and never counts a skipped check as a
pass. A verifier that quietly omits a check is worse than one that admits it.

## Cross-implementation contract

`testdata/testvectors.json` pins every hash, encoding and derivation. Any other
implementation must reproduce it byte-for-byte.

The redundancy is deliberate: a single implementation that both produces and
verifies proofs cannot catch its own hashing bug — it computes a wrong digest
and then confirms it.

```sh
go test -run TestVectors -update .
```

## Status

The Go core, service, CLI and HTTP API are complete and tested, and `web/` holds
a browser client with its own verifier. Both run the same test vectors.

Still to come: OpenTimestamps verification inside `publicdraw verify`, and durable
storage behind the file-backed service.

## Licence

[MIT](LICENSE)

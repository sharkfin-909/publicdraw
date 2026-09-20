# verifiable-challenge

Runs public draws — grants, ballots, allocations, giveaways — so that **the
organiser cannot steer the result** and anyone can check it offline, without
trusting the organiser or this software.

```
$ vc verify -timeline bundle.json

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

| | bundle contains | verified by | entrants check inclusion |
|---|---|---|---|
| `public` | every entry | rebuilding the whole tree | by reading the list |
| `private` | winners only, with proofs | per-winner Merkle proofs | with a **receipt** |

Salts are not optional in either mode. Entry labels are low-entropy — an email
address, a name, a ticket number — so an unsalted leaf can be brute-forced,
and anyone holding the root could test "was alice@example.com in this draw?"

A **receipt** is one entrant's Merkle proof of their own place in the committed
roster. It lets them confirm they were included without seeing anybody else's
entry, and it is issued at the cutoff because the root does not exist before
then.

## The transparency log

One attack survives all of the above: create a draw, see the result, quietly
abandon it, try again. Every individual bundle verifies. The fraud lives in
what was *not* published.

So every draw's creation, cutoff and result are appended to a public hash
chain. A draw that was created and whose round has since published, with no
result logged, shows up in `vc log -abandoned`. That is not proof of
misconduct — draws get cancelled for honest reasons — but it is the only place
the attack can appear at all.

Same shape as Certificate Transparency: the answer to invisible misissuance is
to make the *set* of commitments public and append-only.

## What is not proven

Stated plainly, because someone will find these otherwise:

- **Roster stuffing.** Receipts catch an entrant being dropped. They do not
  catch a roster padded with entries that were never real.
- **The operator's clock.** Timestamps are the operator's word until the log
  head is anchored with OpenTimestamps. `vc verify` reports that check as
  `SKIP`, never as a pass.
- **Retries**, from a single bundle. That question is answerable only against
  the full public log, and is also reported as `SKIP`.

## Usage

```sh
vc keygen

vc create -title "Two community grants" -winners 2 -disclosure private -open 10m
vc enter  -draw ID -file entrants.txt
vc commit -draw ID                      # at the cutoff; freezes and logs the roster
vc receipt -draw ID -index 5 -out receipt.json
vc run    -draw ID                      # at the target round; publishes the bundle

vc verify bundle.json [-timeline]
vc check-receipt receipt.json
vc log [-abandoned]
vc serve -addr :8080
```

Anchor the log head after each cutoff, so the ordering does not rest on your
own clock:

```sh
echo -n <log head> | xxd -r -p > log.head && ots stamp log.head
```

## HTTP API

`vc serve` exposes the same rules — every ordering constraint lives in the
service layer, so the API cannot permit something the CLI forbids.

```
GET  /v1/operator                      public key, chain, log head
POST /v1/draws                         create
POST /v1/draws/{id}/entries            add entries (refused after cutoff)
POST /v1/draws/{id}/commit             freeze the roster
GET  /v1/draws/{id}/receipt/{index}    one entrant's inclusion proof
POST /v1/draws/{id}/run                draw and publish
GET  /v1/log  ·  /v1/log/abandoned     the public chain
POST /v1/verify                        check a posted bundle
```

`POST /v1/verify` is the weakest way to verify, because the answer comes from
the server. The CLI exists so nobody has to take it.

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
| `bundle` | nineteen checks, no short-circuit |

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

Go core, service, CLI and HTTP API are complete and tested (10 packages).

Still to come: OpenTimestamps verification inside `vc verify`, the WASM
verifier, a browser client, and durable storage behind the file-backed service.

## Licence

MIT

# verifiable-challenge

Selects a coding challenge from a committed pool so that **the candidate cannot
know it in advance** and **the operator cannot steer it** — and produces a
bundle anyone can verify offline, without trusting either party.

```
$ vc verify bundle.json
  ok    pool commitment binds root and size               commit=7a45746163e6…
  ok    signed commitment matches this pool and chain     chain=52db9ba70e0c… size=5
  ok    operator signature over pre-beacon commitment     valid
  ok    commitment predates its target round              issued=1789844060 target_round_time=1789844073 lead=13s
  ok    beacon round is the committed target              round=32346903
  ok    drand signature verifies against chain public key  valid
  ok    challenge index derives from beacon               derived=2 claimed=2
  ok    merkle proof places challenge at that index       leaf=723400324389… path_len=3
  SKIP  pool commitment timestamped before target round   no OTS proof in bundle

VERIFIED — challenge "rate-limit" was fixed in advance and selected unpredictably
```

## Why

Anyone can claim their assessment was fair. Almost nobody can prove it. The two
failures that matter are opposite in direction:

- the **candidate** saw the problem early, or
- the **operator** picked a problem to suit the candidate — easy for a friend,
  brutal for someone else.

Surveillance addresses neither. Both are questions about *when a value was
fixed*, and that is answerable with cryptography rather than webcams.

## How

Two primitives bound time from opposite directions.

| | gives | direction |
|---|---|---|
| [drand](https://drand.love) beacon | "this could not have existed before round *N*" | lower bound |
| [OpenTimestamps](https://opentimestamps.org) | "this existed by time *T*" | upper bound |

The pool is committed and timestamped in advance. The challenge is then derived
from a drand round *named before that round existed*:

```
index = H(SELECT_TAG ‖ pool_root ‖ pool_size ‖ drand_randomness ‖ session_id) mod pool_size
```

### The ordering is the whole guarantee

If the operator assigns `session_id` **after** seeing the beacon value, they can
generate IDs in a loop until the index lands wherever they like. The resulting
bundle verifies perfectly — every hash correct, every signature valid.

`session/session_test.go` demonstrates the attack: steering a 1000-challenge
pool to a chosen index takes about 1200 tries and no measurable time.

So the operator must commit first. `vc begin` signs
`{session_id, target_round, pool_root, pool_size, issued_at}` naming a round
~15 seconds out, and hands that to the candidate before the round's value
exists. Verification then checks `issued_at < time(target_round)`.

That wait is not overhead. It is the proof. Show it to the candidate as a
countdown against the round number.

## Verifying

Nine checks, all offline apart from fetching the beacon (which is itself
verified against the chain's group public key — trusting the HTTP response
would reintroduce the trusted third party the protocol exists to remove).

`vc verify` never short-circuits, and never counts a skipped check as a pass.
OpenTimestamps verification needs a Bitcoin header source and is not done
in-process; it reports `SKIP` so nobody mistakes an unproven bundle for a
proven one.

## Usage

```sh
vc keygen -out operator.key.json

vc commit -in challenges.json -pool pool.secret.json -out commitment.json
# then timestamp the commit value, days ahead of any session:
echo -n <commit> | xxd -r -p > pool.commit && ots stamp pool.commit

vc begin  -commit commitment.json -key operator.key.json -out session.json
vc reveal -session session.json -pool pool.secret.json -out bundle.json
vc verify bundle.json
```

`pool.secret.json` holds the salts and must stay private; publishing it burns
the pool. `commitment.json` is public and is what you timestamp.

## Design notes

**Salted leaves.** Each entry is `H(LEAF_TAG ‖ salt ‖ challenge)` with a fresh
32-byte salt. Without it, anyone holding the root could confirm a guessed
challenge by brute force, and the pool would burn after one disclosure.

**Merkle proofs, not full reveals.** Opening one challenge discloses that
challenge and ~log₂(n) hashes. The rest of the pool stays committed and
reusable.

**Domain-separated hashing.** Leaves and internal nodes use different tags, so
an internal node cannot be presented as a leaf — the second-preimage attack RFC
6962 solves the same way.

**Promotion, not duplication.** Odd trailing nodes are promoted unchanged.
Duplicating them makes distinct leaf sets collide to one root (CVE-2012-2459).

**Length-prefixed fields.** `H(a, b)` prefixes every part with its length,
because `"ab"+"cdef"` and `"abc"+"def"` are otherwise the same bytes.

**Size is committed.** The timestamped value is `H(root ‖ size)`, so a tree
cannot later be re-padded to a different size.

**Modulo bias** is about `pool_size / 2²⁵⁶` — on the order of 1e-74. Rejection
sampling would be theatre.

## Cross-implementation contract

`testdata/testvectors.json` pins every hash, encoding and derivation. The
browser client is written in TypeScript and must reproduce it byte-for-byte.

That redundancy is deliberate. A single implementation that both produces and
verifies proofs cannot catch its own hashing bug — it computes a wrong digest
and then confirms it. Two independent implementations disagreeing by one byte
fails loudly instead.

Regenerate after an intentional protocol change:

```sh
go test -run TestVectors -update .
```

## Status

The Go core and CLI are complete and tested. Still to come: the TypeScript
browser client, OpenTimestamps verification in `vc verify`, and the session
recording chain this selection is meant to anchor.

## Licence

MIT

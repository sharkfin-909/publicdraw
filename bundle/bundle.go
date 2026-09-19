// Package bundle defines the verification artifact and checks it.
//
// A bundle is everything a third party needs to confirm that a challenge was
// fixed in advance, selected unpredictably, and not steered — using only
// public data and no contact with the operator.
package bundle

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"

	"github.com/sharkfin009/verifiable-challenge/beacon"
	"github.com/sharkfin009/verifiable-challenge/hashing"
	"github.com/sharkfin009/verifiable-challenge/merkle"
	"github.com/sharkfin009/verifiable-challenge/pool"
	"github.com/sharkfin009/verifiable-challenge/session"
)

// Bundle is the published proof for one challenge selection.
type Bundle struct {
	Version int          `json:"version"`
	Pool    PoolRef      `json:"pool"`
	Session SessionRef   `json:"session"`
	Beacon  beacon.Round `json:"beacon"`
	Reveal  Reveal       `json:"reveal"`
}

// PoolRef is the public description of the pool the challenge came from.
type PoolRef struct {
	Root   hashing.Digest `json:"root"`
	Size   int            `json:"size"`
	Commit hashing.Digest `json:"commit"`
	// OTSProof is a base64 OpenTimestamps proof over Commit. It is what
	// establishes that the pool predates the beacon round; without it the
	// bundle proves the selection was fair only if you already trust that the
	// pool was not built to order.
	OTSProof string `json:"ots_proof,omitempty"`
}

// SessionRef carries the operator's pre-beacon commitment and its signature.
type SessionRef struct {
	Commitment session.Commitment `json:"commitment"`
	OperatorPK string             `json:"operator_pubkey"`
	Signature  string             `json:"signature"`
}

// Reveal opens one leaf of the committed pool.
type Reveal struct {
	Index      int              `json:"index"`
	Salt       string           `json:"salt"`
	Challenge  pool.Challenge   `json:"challenge"`
	MerklePath []hashing.Digest `json:"merkle_path"`
}

// Check is the outcome of one verification step.
type Check struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Skipped bool   `json:"skipped"`
	Detail  string `json:"detail"`
}

// Report is the full verification outcome. OK is true only if every check
// that ran passed; skipped checks are reported separately and never silently
// counted as passes.
type Report struct {
	OK      bool    `json:"ok"`
	Checks  []Check `json:"checks"`
	Skipped int     `json:"skipped"`
}

func (r *Report) add(name string, ok bool, format string, args ...any) {
	r.Checks = append(r.Checks, Check{Name: name, OK: ok, Detail: fmt.Sprintf(format, args...)})
}

func (r *Report) skip(name string, format string, args ...any) {
	r.Checks = append(r.Checks, Check{Name: name, OK: false, Skipped: true, Detail: fmt.Sprintf(format, args...)})
	r.Skipped++
}

// Verify runs every check and returns a report. It never short-circuits: a
// failing bundle should show which specific property broke.
func Verify(b Bundle, chain beacon.Chain) Report {
	var rep Report
	com := b.Session.Commitment

	// 1. The pool commitment is internally consistent, so Root and Size are
	//    both genuinely bound by the value that gets timestamped.
	want := pool.CommitValue(b.Pool.Root, b.Pool.Size)
	rep.add("pool commitment binds root and size", want == b.Pool.Commit,
		"commit=%s", b.Pool.Commit)

	// 2. The commitment the operator signed describes this pool and chain.
	matches := com.PoolRoot == b.Pool.Root && com.PoolSize == b.Pool.Size &&
		com.PoolCommit == b.Pool.Commit && com.ChainHash == chain.Hash
	rep.add("signed commitment matches this pool and chain", matches,
		"chain=%s size=%d", com.ChainHash, com.PoolSize)

	// 3. The operator's signature over that commitment verifies.
	pkb, err := hex.DecodeString(b.Session.OperatorPK)
	sigb, err2 := hex.DecodeString(b.Session.Signature)
	switch {
	case err != nil || err2 != nil:
		rep.add("operator signature", false, "malformed hex in key or signature")
	default:
		e := session.VerifySignature(ed25519.PublicKey(pkb), com, sigb)
		rep.add("operator signature over pre-beacon commitment", e == nil, "%s", detail(e, "valid"))
	}

	// 4. THE ANTI-GRINDING CHECK. The commitment was issued before the target
	//    round existed, so the session ID cannot have been ground against a
	//    known beacon value to steer the index.
	target := chain.TimeOf(com.TargetRound)
	issued := com.IssuedAt
	rep.add("commitment predates its target round", issued < target.Unix(),
		"issued=%d target_round_time=%d lead=%ds", issued, target.Unix(), target.Unix()-issued)

	// 5. The beacon round is the one named, and its BLS signature is valid
	//    against the chain's group key.
	if b.Beacon.Round != com.TargetRound {
		rep.add("beacon round is the committed target", false,
			"bundle has round %d, commitment named %d", b.Beacon.Round, com.TargetRound)
	} else {
		rep.add("beacon round is the committed target", true, "round=%d", b.Beacon.Round)
	}
	e := chain.Verify(b.Beacon)
	rep.add("drand signature verifies against chain public key", e == nil, "%s", detail(e, "valid"))

	// 6. The index is exactly what the published inputs derive.
	rnd, rerr := b.Beacon.Randomness()
	if rerr != nil {
		rep.add("challenge index derives from beacon", false, "%v", rerr)
	} else {
		idx, ierr := session.Index(b.Pool.Root, b.Pool.Size, rnd, com.SessionID)
		rep.add("challenge index derives from beacon", ierr == nil && idx == b.Reveal.Index,
			"derived=%d claimed=%d", idx, b.Reveal.Index)
	}

	// 7. The revealed challenge really sits at that index in the committed tree.
	salt, serr := hex.DecodeString(b.Reveal.Salt)
	if serr != nil {
		rep.add("merkle proof places challenge at that index", false, "malformed salt hex")
	} else {
		leaf := merkle.LeafHash(salt, b.Reveal.Challenge.CanonicalBytes())
		ok := merkle.Verify(leaf, b.Reveal.Index, b.Pool.Size, b.Reveal.MerklePath, b.Pool.Root)
		rep.add("merkle proof places challenge at that index", ok,
			"leaf=%s path_len=%d", leaf, len(b.Reveal.MerklePath))
	}

	// 8. Timestamping. Not verified here — an OpenTimestamps proof needs a
	//    Bitcoin header source. Reported as skipped rather than passed,
	//    because a verifier that silently omits a check is worse than one that
	//    admits it: without this step the bundle does not rule out a pool
	//    constructed after the fact.
	if b.Pool.OTSProof == "" {
		rep.skip("pool commitment timestamped before target round",
			"no OTS proof in bundle; pool could have been built to order")
	} else {
		rep.skip("pool commitment timestamped before target round",
			"OTS proof present (%d bytes) — verify with: ots verify", len(b.Pool.OTSProof))
	}

	rep.OK = true
	for _, c := range rep.Checks {
		if !c.OK && !c.Skipped {
			rep.OK = false
		}
	}
	return rep
}

func detail(err error, okMsg string) string {
	if err != nil {
		return err.Error()
	}
	return okMsg
}

// Read decodes a bundle from JSON.
func Read(r io.Reader) (Bundle, error) {
	var b Bundle
	if err := json.NewDecoder(r).Decode(&b); err != nil {
		return b, fmt.Errorf("bundle: decode: %w", err)
	}
	return b, nil
}

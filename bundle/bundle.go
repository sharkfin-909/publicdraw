// Package bundle defines the published proof of a draw and checks it.
//
// A bundle is everything a third party needs to confirm that the entry list
// was frozen before the randomness existed, that the randomness was not
// chosen, and that the winners follow from the two — using only public data
// and no contact with the organiser.
package bundle

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/sharkfin009/verifiable-challenge/beacon"
	"github.com/sharkfin009/verifiable-challenge/draw"
	"github.com/sharkfin009/verifiable-challenge/hashing"
	"github.com/sharkfin009/verifiable-challenge/merkle"
	"github.com/sharkfin009/verifiable-challenge/roster"
	"github.com/sharkfin009/verifiable-challenge/translog"
)

// Bundle is the published proof for one draw.
type Bundle struct {
	Version  int          `json:"version"`
	Operator string       `json:"operator_pubkey"`
	Draw     DrawRef      `json:"draw"`
	Roster   RosterRef    `json:"roster"`
	Beacon   beacon.Round `json:"beacon"`
	Result   Result       `json:"result"`
	Log      LogRef       `json:"log"`
}

// DrawRef carries the parameters committed at creation.
type DrawRef struct {
	Params    draw.Params `json:"params"`
	Signature string      `json:"signature"`
}

// RosterRef carries the entry list commitment made at cutoff.
//
// Entries is populated only when the draw was created with public disclosure,
// in which case a verifier rebuilds the entire tree instead of checking
// per-winner proofs. Under private disclosure it is empty and every entrant
// checks their own inclusion with a receipt.
type RosterRef struct {
	Commitment draw.RosterCommitment `json:"commitment"`
	Signature  string                `json:"signature"`
	Entries    []roster.Sealed       `json:"entries,omitempty"`
	// OTSProof is a base64 OpenTimestamps proof over the log head. It is what
	// establishes that the commitments predate the beacon round to someone who
	// does not trust the operator's clock.
	OTSProof string `json:"ots_proof,omitempty"`
}

// Winner is one selected entry, in rank order.
type Winner struct {
	Rank       int              `json:"rank"`
	Index      int              `json:"index"`
	Salt       string           `json:"salt"`
	Entry      roster.Entry     `json:"entry"`
	MerklePath []hashing.Digest `json:"merkle_path"`
}

// Result is the derived outcome.
type Result struct {
	Seed     hashing.Digest `json:"seed"`
	Winners  []Winner       `json:"winners"`
	Waitlist []int          `json:"waitlist"`
}

// LogRef carries this draw's transparency-log records.
type LogRef struct {
	LogID   string            `json:"log_id"`
	Records []translog.Record `json:"records"`
	Head    hashing.Digest    `json:"head"`
}

// Check is the outcome of one verification step.
type Check struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Skipped bool   `json:"skipped"`
	Detail  string `json:"detail"`
}

// Report is the full verification outcome. Skipped checks are reported
// separately and are never silently counted as passes.
type Report struct {
	OK      bool    `json:"ok"`
	Checks  []Check `json:"checks"`
	Skipped int     `json:"skipped"`
}

func (r *Report) add(name string, ok bool, format string, args ...any) {
	r.Checks = append(r.Checks, Check{Name: name, OK: ok, Detail: fmt.Sprintf(format, args...)})
}

func (r *Report) skip(name, format string, args ...any) {
	r.Checks = append(r.Checks, Check{Name: name, Skipped: true, Detail: fmt.Sprintf(format, args...)})
	r.Skipped++
}

// Verify runs every check and returns a report. It never short-circuits: a
// failing bundle should show which specific property broke, not just the first.
func Verify(b Bundle, chain beacon.Chain) Report {
	var rep Report
	p := b.Draw.Params
	rc := b.Roster.Commitment

	pkb, pkErr := hex.DecodeString(b.Operator)
	pub := ed25519.PublicKey(pkb)

	// --- what the organiser committed to, and when ------------------------

	rep.add("roster commitment binds root and size",
		roster.CommitValue(rc.Root, rc.Size) == rc.Commit, "commit=%s size=%d", rc.Commit, rc.Size)

	rep.add("roster commitment belongs to this draw",
		rc.DrawID == p.DrawID, "draw=%s", p.DrawID)

	rep.add("draw parameters name this chain",
		p.ChainHash == chain.Hash, "chain=%s", p.ChainHash)

	if pkErr != nil {
		rep.add("operator signature over draw parameters", false, "malformed operator key")
		rep.add("operator signature over roster commitment", false, "malformed operator key")
	} else {
		sig, err := hex.DecodeString(b.Draw.Signature)
		e := err
		if e == nil {
			e = draw.VerifyParamsSignature(pub, p, sig)
		}
		rep.add("operator signature over draw parameters", e == nil, "%s", detail(e, "valid"))

		rsig, err := hex.DecodeString(b.Roster.Signature)
		e = err
		if e == nil {
			e = draw.VerifyRosterSignature(pub, rc, rsig)
		}
		rep.add("operator signature over roster commitment", e == nil, "%s", detail(e, "valid"))
	}

	// --- ordering: the part that makes the draw unsteerable ---------------

	rep.add("parameters fixed before the cutoff",
		p.CreatedAt < p.CutoffAt, "created=%d cutoff=%d", p.CreatedAt, p.CutoffAt)

	rep.add("roster frozen at or after the cutoff",
		rc.CommittedAt >= p.CutoffAt, "committed=%d cutoff=%d", rc.CommittedAt, p.CutoffAt)

	target := chain.TimeOf(p.TargetRound)
	rep.add("roster frozen before the beacon round existed",
		rc.CommittedAt < target.Unix(),
		"committed=%d round_at=%d lead=%ds", rc.CommittedAt, target.Unix(), target.Unix()-rc.CommittedAt)

	rep.add("target round leaves the minimum lead after cutoff",
		target.Unix()-p.CutoffAt >= int64(draw.MinLead.Seconds()),
		"gap=%ds min=%ds", target.Unix()-p.CutoffAt, int64(draw.MinLead.Seconds()))

	// --- the randomness ---------------------------------------------------

	rep.add("beacon round is the committed target",
		b.Beacon.Round == p.TargetRound, "bundle=%d committed=%d", b.Beacon.Round, p.TargetRound)

	e := chain.Verify(b.Beacon)
	rep.add("drand signature verifies against chain public key", e == nil, "%s", detail(e, "valid"))

	// --- the derivation ---------------------------------------------------

	rnd, rerr := b.Beacon.Randomness()
	if rerr != nil {
		rep.add("seed derives from roster and beacon", false, "%v", rerr)
		rep.add("winners derive from the seed", false, "no seed")
	} else {
		wantSeed := draw.Seed(rc.Root, rc.Size, rnd, p.DrawID)
		seedOK := wantSeed == b.Result.Seed
		rep.add("seed derives from roster and beacon", seedOK, "seed=%s", b.Result.Seed)

		won, wait, serr := draw.Select(b.Result.Seed, rc.Size, p.Winners)
		switch {
		case serr != nil:
			rep.add("winners derive from the seed", false, "%v", serr)
		case !sameInts(won, winnerIndices(b.Result.Winners)):
			rep.add("winners derive from the seed", false,
				"derived=%v published=%v", won, winnerIndices(b.Result.Winners))
		case len(b.Result.Waitlist) > 0 && !sameInts(wait, b.Result.Waitlist):
			rep.add("winners derive from the seed", false, "waitlist order does not match the ranking")
		default:
			rep.add("winners derive from the seed", true, "%d winner(s) of %d entries", len(won), rc.Size)
		}
	}

	// --- disclosure -------------------------------------------------------

	switch p.Disclosure {
	case roster.Public:
		if len(b.Roster.Entries) != rc.Size {
			rep.add("published roster rebuilds to the committed root", false,
				"bundle carries %d entries, commitment says %d", len(b.Roster.Entries), rc.Size)
		} else {
			got, err := roster.Rebuild(b.Roster.Entries)
			rep.add("published roster rebuilds to the committed root",
				err == nil && got == rc.Root, "%s", detail(err, "root matches"))
		}
	case roster.Private:
		rep.add("private disclosure withholds the entry list",
			len(b.Roster.Entries) == 0, "%d entries published", len(b.Roster.Entries))
	default:
		rep.add("disclosure mode is recognised", false, "unknown mode %q", p.Disclosure)
	}

	rep.add("each winner is proven to be in the committed roster",
		winnersProven(b, rc), "%d proof(s)", len(b.Result.Winners))

	// --- transparency log -------------------------------------------------

	head, lerr := translog.Verify(b.Log.Records, b.Log.LogID)
	rep.add("log records form an unbroken chain", lerr == nil, "%s", detail(lerr, "chain intact"))

	rep.add("log contains this draw's creation, cutoff and result",
		logCovers(b.Log.Records, p, rc, head, b.Log.Head),
		"%d record(s)", len(b.Log.Records))

	// --- what is not checked here -----------------------------------------

	if b.Roster.OTSProof == "" {
		rep.skip("log head timestamped before the beacon round",
			"no OTS proof in bundle; the operator's clock is taken on trust")
	} else {
		rep.skip("log head timestamped before the beacon round",
			"OTS proof present (%d bytes) — verify separately with: ots verify", len(b.Roster.OTSProof))
	}

	rep.skip("this draw was not abandoned and retried",
		"answerable only against the full public log, not from one bundle")

	rep.OK = true
	for _, c := range rep.Checks {
		if !c.OK && !c.Skipped {
			rep.OK = false
		}
	}
	return rep
}

func winnersProven(b Bundle, rc draw.RosterCommitment) bool {
	if len(b.Result.Winners) == 0 {
		return false
	}
	for _, w := range b.Result.Winners {
		salt, err := hex.DecodeString(w.Salt)
		if err != nil {
			return false
		}
		leaf := merkle.LeafHash(salt, w.Entry.CanonicalBytes())
		if !merkle.Verify(leaf, w.Index, rc.Size, w.MerklePath, rc.Root) {
			return false
		}
	}
	return true
}

func logCovers(records []translog.Record, p draw.Params, rc draw.RosterCommitment, replayed, claimed hashing.Digest) bool {
	if replayed != claimed {
		return false
	}
	var created, committed, published bool
	for _, r := range records {
		if r.DrawID != p.DrawID {
			continue
		}
		switch r.Kind {
		case translog.KindDrawCreated:
			created = r.Payload == p.Commit()
		case translog.KindRosterCommitted:
			var want hashing.Digest
			copy(want[:], rc.SigningBytes())
			committed = r.Payload == want
		case translog.KindResultPublished:
			published = true
		}
	}
	return created && committed && published
}

func winnerIndices(ws []Winner) []int {
	out := make([]int, len(ws))
	for i, w := range ws {
		out[i] = w.Index
	}
	return out
}

func sameInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
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

// Timeline renders the ordering a bundle asserts, for humans.
func Timeline(b Bundle, chain beacon.Chain) string {
	p := b.Draw.Params
	at := func(u int64) string { return time.Unix(u, 0).UTC().Format("2006-01-02 15:04:05Z") }
	return fmt.Sprintf(
		"  created   %s  parameters fixed, nobody has entered yet\n"+
			"  cutoff    %s  entries close\n"+
			"  frozen    %s  roster committed and signed\n"+
			"  round %d  %s  drand publishes — %ds after the roster was frozen\n",
		at(p.CreatedAt), at(p.CutoffAt), at(b.Roster.Commitment.CommittedAt),
		p.TargetRound, at(chain.TimeOf(p.TargetRound).Unix()),
		chain.TimeOf(p.TargetRound).Unix()-b.Roster.Commitment.CommittedAt)
}

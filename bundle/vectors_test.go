package bundle_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sharkfin-909/publicdraw/beacon"
	"github.com/sharkfin-909/publicdraw/bundle"
	"github.com/sharkfin-909/publicdraw/draw"
	"github.com/sharkfin-909/publicdraw/hashing"
	"github.com/sharkfin-909/publicdraw/roster"
	"github.com/sharkfin-909/publicdraw/translog"
)

// The bundle vectors are the verifier contract between Go and TypeScript at
// the level that matters: whole bundles, honest and tampered, with Go's verdict
// on every check recorded by id. The TypeScript suite verifies the same bundles
// and must reach the same verdict on every check, so a rule tightened on one
// side and not the other turns a test red instead of shipping.
//
//	go test ./bundle -run TestBundleVectors -update   # regenerate
var update = flag.Bool("update", false, "rewrite testdata/bundlevectors.json")

const vectorFile = "../testdata/bundlevectors.json"

type vectorCase struct {
	Name     string            `json:"name"`
	Bundle   bundle.Bundle     `json:"bundle"`
	Expected map[string]string `json:"expected"` // check id -> "pass" | "fail" | "skip"
	OK       bool              `json:"ok"`
}

type vectorFileShape struct {
	Note  string       `json:"note"`
	Chain string       `json:"chain"`
	Cases []vectorCase `json:"cases"`
}

// fixed builds an honest bundle from fixed inputs — key, salts, times, round —
// so the file is reproducible byte for byte.
func fixed(t *testing.T, disc roster.Disclosure, winners, n int) (bundle.Bundle, ed25519.PrivateKey) {
	t.Helper()
	seed := sha256.Sum256([]byte("publicdraw bundle vectors operator"))
	priv := ed25519.NewKeyFromSeed(seed[:])
	pub := priv.Public().(ed25519.PublicKey)

	p := draw.Params{
		DrawID: fmt.Sprintf("%032x", sha256.Sum256([]byte("draw "+string(disc))))[:32],
		Title:  "vector draw (" + string(disc) + ")", Winners: winners, Disclosure: disc,
		CutoffAt: tCutoff, TargetRound: vecRound, ChainHash: beacon.Quicknet.Hash, CreatedAt: tCreate,
	}
	psig := ed25519.Sign(priv, p.SigningBytes())

	sealed := make([]roster.Sealed, n)
	for i := range sealed {
		salt := sha256.Sum256([]byte(fmt.Sprintf("salt %s %d", disc, i)))
		sealed[i] = roster.Sealed{Salt: salt[:], Entry: roster.Entry{Label: fmt.Sprintf("entrant-%02d@example.org", i)}}
	}
	ros, err := roster.Load(sealed)
	if err != nil {
		t.Fatal(err)
	}
	rc, rsig, err := draw.CommitRoster(p, ros.Commitment(), priv, time.Unix(tCutoff, 0))
	if err != nil {
		t.Fatal(err)
	}
	round := beacon.Round{Round: vecRound, Signature: vecSig}

	// The log, as the service would have written it.
	logID := "vc/log/" + hex.EncodeToString(pub)[:16]
	head := translog.Genesis(logID)
	var records []translog.Record
	appendRec := func(kind translog.Kind, payload hashing.Digest, at int64) {
		r := translog.Record{Seq: uint64(len(records)), Kind: kind, DrawID: p.DrawID, Payload: payload, LoggedAt: at, Prev: head}
		r.Head = translog.HeadAfter(r.Prev, r.Seq, r.Kind, r.DrawID, r.Payload, r.LoggedAt)
		head = r.Head
		records = append(records, r)
	}
	appendRec(translog.KindDrawCreated, p.Commit(), tCreate)
	var rcPayload hashing.Digest
	copy(rcPayload[:], rc.SigningBytes())
	appendRec(translog.KindRosterCommitted, rcPayload, tCutoff)

	src := bundle.Source{
		Operator: hex.EncodeToString(pub), Params: p, ParamsSig: hex.EncodeToString(psig),
		Sealed: sealed, Commit: rc, RosterSig: hex.EncodeToString(rsig),
		Round: round, LogID: logID,
	}
	pre, err := bundle.Assemble(src)
	if err != nil {
		t.Fatal(err)
	}
	appendRec(translog.KindResultPublished, pre.Result.Seed, tRound+6)
	src.Records, src.Head = records, head
	b, err := bundle.Assemble(src)
	if err != nil {
		t.Fatal(err)
	}
	return b, priv
}

func clone(t *testing.T, b bundle.Bundle) bundle.Bundle {
	t.Helper()
	raw, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	var out bundle.Bundle
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func outcome(rep bundle.Report) map[string]string {
	out := map[string]string{}
	for _, c := range rep.Checks {
		switch {
		case c.Skipped:
			out[c.ID] = "skip"
		case c.OK:
			out[c.ID] = "pass"
		default:
			out[c.ID] = "fail"
		}
	}
	return out
}

func buildVectors(t *testing.T) vectorFileShape {
	t.Helper()
	pub, ppriv := fixed(t, roster.Public, 2, 7)
	prv, _ := fixed(t, roster.Private, 2, 8)

	type tamper struct {
		name string
		base bundle.Bundle
		edit func(*bundle.Bundle)
	}
	resignParams := func(b *bundle.Bundle) {
		b.Draw.Signature = hex.EncodeToString(ed25519.Sign(ppriv, b.Draw.Params.SigningBytes()))
	}
	cases := []tamper{
		{"honest public draw", pub, func(*bundle.Bundle) {}},
		{"honest private draw", prv, func(*bundle.Bundle) {}},
		{"winner's label substituted", prv, func(b *bundle.Bundle) { b.Result.Winners[0].Entry.Label = "the organiser's friend" }},
		{"winners reordered", prv, func(b *bundle.Bundle) {
			b.Result.Winners[0], b.Result.Winners[1] = b.Result.Winners[1], b.Result.Winners[0]
		}},
		{"winner moved to another index", prv, func(b *bundle.Bundle) { b.Result.Winners[0].Index = (b.Result.Winners[0].Index + 1) % 8 }},
		{"waitlist reordered", prv, func(b *bundle.Bundle) {
			w := b.Result.Waitlist
			w[0], w[1] = w[1], w[0]
		}},
		{"seed replaced", prv, func(b *bundle.Bundle) { b.Result.Seed[0] ^= 1 }},
		{"roster frozen after the round", prv, func(b *bundle.Bundle) { b.Roster.Commitment.CommittedAt = tRound + 1 }},
		{"roster frozen before the cutoff", prv, func(b *bundle.Bundle) { b.Roster.Commitment.CommittedAt = tCutoff - 1 }},
		{"parameters dated after the cutoff, re-signed", pub, func(b *bundle.Bundle) {
			b.Draw.Params.CreatedAt = tCutoff + 1
			resignParams(b)
		}},
		{"target round too soon after the cutoff, re-signed", pub, func(b *bundle.Bundle) {
			b.Draw.Params.CutoffAt = tRound - 5
			resignParams(b)
		}},
		{"beacon signature forged", prv, func(b *bundle.Bundle) {
			s := []byte(b.Beacon.Signature)
			s[len(s)-1] ^= 1
			b.Beacon.Signature = string(s)
		}},
		{"beacon round substituted", prv, func(b *bundle.Bundle) { b.Beacon.Round++ }},
		{"operator key swapped", prv, func(b *bundle.Bundle) {
			other := ed25519.NewKeyFromSeed(make([]byte, 32)).Public().(ed25519.PublicKey)
			b.Operator = hex.EncodeToString(other)
		}},
		{"published leaf pruned", prv, func(b *bundle.Bundle) { b.Roster.Leaves = b.Roster.Leaves[:len(b.Roster.Leaves)-1] }},
		{"published leaf substituted", prv, func(b *bundle.Bundle) { b.Roster.Leaves[3][0] ^= 1 }},
		{"public entry label altered", pub, func(b *bundle.Bundle) { b.Roster.Entries[2].Entry.Label = "someone else" }},
		{"public entry withheld", pub, func(b *bundle.Bundle) { b.Roster.Entries = b.Roster.Entries[:len(b.Roster.Entries)-1] }},
		{"private draw leaks its entries", prv, func(b *bundle.Bundle) {
			b.Roster.Entries = []roster.Sealed{{Salt: make([]byte, 32), Entry: roster.Entry{Label: "leaked"}}}
		}},
		{"unknown disclosure mode, re-signed", pub, func(b *bundle.Bundle) {
			b.Draw.Params.Disclosure = "secret"
			resignParams(b)
		}},
		{"log record edited", prv, func(b *bundle.Bundle) { b.Log.Records[1].LoggedAt++ }},
		{"result record dropped from the log", prv, func(b *bundle.Bundle) {
			b.Log.Records = b.Log.Records[:2]
			b.Log.Head = b.Log.Records[1].Head
		}},
	}

	out := vectorFileShape{
		Note: "Generated by `go test ./bundle -run TestBundleVectors -update`. Each case is a bundle and " +
			"Go's verdict on every check, by stable id. The TypeScript verifier must agree on every check.",
		Chain: beacon.Quicknet.Name,
	}
	for _, c := range cases {
		b := clone(t, c.base)
		c.edit(&b)
		rep := bundle.Verify(b, beacon.Quicknet)
		out.Cases = append(out.Cases, vectorCase{Name: c.name, Bundle: b, Expected: outcome(rep), OK: rep.OK})
	}
	return out
}

func TestBundleVectors(t *testing.T) {
	v := buildVectors(t)
	for _, c := range v.Cases {
		honest := c.Name == "honest public draw" || c.Name == "honest private draw"
		if c.OK != honest {
			t.Errorf("%s: ok = %v, want %v", c.Name, c.OK, honest)
		}
	}

	got, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	if *update {
		if err := os.WriteFile(filepath.Clean(vectorFile), got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(vectorFile)
	if err != nil {
		t.Fatalf("%v (generate with: go test ./bundle -run TestBundleVectors -update)", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("testdata/bundlevectors.json is stale: regenerate with go test ./bundle -run TestBundleVectors -update")
	}
}

func TestEveryCheckHasAnID(t *testing.T) {
	for _, c := range buildVectors(t).Cases {
		for _, chk := range bundle.Verify(c.Bundle, beacon.Quicknet).Checks {
			if chk.ID == "" {
				t.Errorf("check %q has no id in checkIDs", chk.Name)
			}
		}
	}
}

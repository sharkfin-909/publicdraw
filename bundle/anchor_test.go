package bundle_test

// A draw anchored into Sigstore's public Rekor instance, captured whole.
// These exercise the parts of verification that only exist because the anchor
// does — the local hash chain cannot establish any of them on its own.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sharkfin009/verifiable-challenge/beacon"
	"github.com/sharkfin009/verifiable-challenge/bundle"
)

func anchored(t *testing.T) bundle.Bundle {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "anchored-bundle.json"))
	if err != nil {
		t.Fatal(err)
	}
	var out bundle.Bundle
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.Roster.RekorAnchor == nil {
		t.Fatal("fixture has no rekor anchor")
	}
	return out
}

func check(rep bundle.Report, substr string) (bundle.Check, bool) {
	for _, c := range rep.Checks {
		if strings.Contains(c.Name, substr) {
			return c, true
		}
	}
	return bundle.Check{}, false
}

func TestAnchoredBundleVerifies(t *testing.T) {
	rep := bundle.Verify(anchored(t), beacon.Quicknet)
	if !rep.OK {
		for _, c := range rep.Checks {
			t.Logf("ok=%v skip=%v %s — %s", c.OK, c.Skipped, c.Name, c.Detail)
		}
		t.Fatal("an honest anchored bundle failed verification")
	}
}

// The inclusion proof is the whole point of anchoring: without it the anchor
// is just an assertion that something was published.
func TestAnchorInclusionProofIsChecked(t *testing.T) {
	b := anchored(t)
	b.Roster.RekorAnchor.Hashes[0] = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="

	rep := bundle.Verify(b, beacon.Quicknet)
	if rep.OK {
		t.Fatal("a bundle with a broken inclusion proof verified")
	}
	c, _ := check(rep, "rekor inclusion proof")
	if c.OK || c.Skipped {
		t.Fatal("the inclusion check did not fire")
	}
}

// An anchor for somebody else's bytes proves nothing about this draw.
func TestAnchorForADifferentPayloadIsRejected(t *testing.T) {
	b := anchored(t)
	b.Roster.RekorAnchor.Payload = "ZGlmZmVyZW50IHBheWxvYWQgZW50aXJlbHk="

	rep := bundle.Verify(b, beacon.Quicknet)
	if rep.OK {
		t.Fatal("a bundle verified with an anchor for unrelated bytes")
	}
	c, _ := check(rep, "anchor binds a head")
	if c.OK || c.Skipped {
		t.Fatal("the binding check did not fire")
	}
}

// An anchor from before the roster was frozen bounds nothing about when it was
// frozen, so its witness time must not be presented as if it did.
func TestAnchorPredatingTheRosterCommitmentBoundsNothing(t *testing.T) {
	b := anchored(t)
	for _, r := range b.Log.Records {
		if r.Kind == "draw_created" && r.DrawID == b.Draw.Params.DrawID {
			b.Roster.RekorAnchor.Payload = toB64(r.Head[:])
		}
	}
	rep := bundle.Verify(b, beacon.Quicknet)
	c, ok := check(rep, "witnessed before the beacon round")
	if !ok || !c.Skipped {
		t.Fatal("an anchor predating the roster commitment was treated as a time bound")
	}
	if !strings.Contains(c.Detail, "predates") {
		t.Fatalf("detail does not explain why: %q", c.Detail)
	}
}

// Without an anchor the bundle is still valid, but two things become
// unanswerable and must be reported as skipped rather than passed.
func TestBundleWithoutAnAnchorSkipsRatherThanPasses(t *testing.T) {
	b := anchored(t)
	b.Roster.RekorAnchor = nil

	rep := bundle.Verify(b, beacon.Quicknet)
	if !rep.OK {
		t.Fatal("removing the anchor should not make an otherwise honest bundle fail")
	}
	for _, name := range []string{"organiser does not control", "abandoned and retried"} {
		c, ok := check(rep, name)
		if !ok || !c.Skipped {
			t.Errorf("%q is not reported as skipped without an anchor", name)
		}
	}
}

// The retry question stays open even with an anchor — one bundle cannot show
// it was the only one — and saying otherwise would be the worst kind of lie
// this system could tell.
func TestRetryQuestionStaysSkippedEvenWhenAnchored(t *testing.T) {
	rep := bundle.Verify(anchored(t), beacon.Quicknet)
	c, ok := check(rep, "abandoned and retried")
	if !ok || !c.Skipped {
		t.Fatal("the retry check claims to have been answered from a single bundle")
	}
}

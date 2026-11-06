package bundle_test

// External test package: these tests drive a real draw through the service and
// then attack the published bundle, which means importing service — the
// direction bundle itself must never depend on.

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/sharkfin-909/publicdraw/beacon"
	"github.com/sharkfin-909/publicdraw/bundle"
	"github.com/sharkfin-909/publicdraw/draw"
	"github.com/sharkfin-909/publicdraw/hashing"
	"github.com/sharkfin-909/publicdraw/merkle"
	"github.com/sharkfin-909/publicdraw/roster"
	"github.com/sharkfin-909/publicdraw/service"
)

const (
	vecRound = 20000000
	vecSig   = "96892582a33552a7b67ba44ef09c3ccd535bbebe760c93ecf45be8958d0c0f06390c6d19d7bf492eb806af7eef6b125c"
	tCreate  = 1752803300
	tCutoff  = 1752803347
	tRound   = 1752803364
)

// honest runs a complete draw and returns its bundle plus the operator key,
// so tests can re-sign after tampering and prove the failure is the property
// breaking rather than merely a broken signature.
func honest(t *testing.T, winners int, disc roster.Disclosure, n int) (bundle.Bundle, ed25519.PrivateKey) {
	t.Helper()
	_, priv, err := draw.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(tCreate, 0)
	svc, err := service.New(t.TempDir(), "test-log", priv,
		service.WithClock(func() time.Time { return now }),
		service.WithFetcher(func(context.Context, uint64) (beacon.Round, error) {
			return beacon.Round{Round: vecRound, Signature: vecSig}, nil
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	labels := make([]string, n)
	for i := range labels {
		labels[i] = string(rune('a'+i%26)) + "-entrant"
	}
	st, err := svc.Create("a public draw", winners, disc, time.Unix(tCutoff, 0))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Enter(st.Params.DrawID, labels...); err != nil {
		t.Fatal(err)
	}
	now = time.Unix(tCutoff, 0)
	if _, err := svc.CommitRoster(st.Params.DrawID); err != nil {
		t.Fatal(err)
	}
	now = time.Unix(tRound+6, 0)
	b, err := svc.Run(context.Background(), st.Params.DrawID)
	if err != nil {
		t.Fatal(err)
	}
	return b, priv
}

func failed(rep bundle.Report, substr string) bool {
	for _, c := range rep.Checks {
		if strings.Contains(c.Name, substr) {
			return !c.OK && !c.Skipped
		}
	}
	return false
}

func mustFail(t *testing.T, b bundle.Bundle, check string) {
	t.Helper()
	rep := bundle.Verify(b, beacon.Quicknet)
	if rep.OK {
		t.Fatal("a tampered bundle verified")
	}
	if !failed(rep, check) {
		for _, c := range rep.Checks {
			t.Logf("ok=%v skip=%v %s", c.OK, c.Skipped, c.Name)
		}
		t.Fatalf("expected the %q check to fire", check)
	}
}

func TestHonestBundlesVerify(t *testing.T) {
	for _, disc := range []roster.Disclosure{roster.Public, roster.Private} {
		b, _ := honest(t, 3, disc, 30)
		rep := bundle.Verify(b, beacon.Quicknet)
		if !rep.OK {
			for _, c := range rep.Checks {
				t.Logf("ok=%v skip=%v %s — %s", c.OK, c.Skipped, c.Name, c.Detail)
			}
			t.Fatalf("an honest %s draw failed verification", disc)
		}
	}
}

// Two checks are structurally unanswerable from a single bundle and must be
// reported as skipped rather than quietly passed.
func TestUnanswerableChecksAreSkipped(t *testing.T) {
	b, _ := honest(t, 1, roster.Private, 10)
	rep := bundle.Verify(b, beacon.Quicknet)
	if rep.Skipped != 2 {
		t.Fatalf("expected 2 skipped checks, got %d", rep.Skipped)
	}
	var sawRetry bool
	for _, c := range rep.Checks {
		if strings.Contains(c.Name, "abandoned") {
			sawRetry = c.Skipped
		}
	}
	if !sawRetry {
		t.Fatal("the retry-attack limitation is not reported")
	}
}

// The core ordering property. Freezing a roster after the beacon published
// would let the organiser choose entries to suit a known value.
func TestRosterFrozenAfterTheRoundIsRejected(t *testing.T) {
	b, priv := honest(t, 1, roster.Private, 20)
	b.Roster.Commitment.CommittedAt = tRound + 1
	b.Roster.Signature = hex.EncodeToString(ed25519.Sign(priv, b.Roster.Commitment.SigningBytes()))
	mustFail(t, b, "frozen before the beacon round")
}

func TestParametersFixedAfterCutoffAreRejected(t *testing.T) {
	b, priv := honest(t, 1, roster.Private, 20)
	b.Draw.Params.CreatedAt = b.Draw.Params.CutoffAt + 1
	b.Draw.Signature = hex.EncodeToString(ed25519.Sign(priv, b.Draw.Params.SigningBytes()))
	mustFail(t, b, "fixed before the cutoff")
}

func TestSubstitutedWinnerIsRejected(t *testing.T) {
	b, _ := honest(t, 1, roster.Private, 20)
	b.Result.Winners[0].Entry.Label = "the organiser's friend"
	mustFail(t, b, "proven to be in the committed roster")
}

func TestReorderedWinnersAreRejected(t *testing.T) {
	b, _ := honest(t, 3, roster.Private, 40)
	b.Result.Winners[0], b.Result.Winners[2] = b.Result.Winners[2], b.Result.Winners[0]
	mustFail(t, b, "winners derive from the seed")
}

func TestForgedSeedIsRejected(t *testing.T) {
	b, _ := honest(t, 1, roster.Private, 20)
	b.Result.Seed = hashing.Tagged("test", []byte("a seed I preferred"))
	mustFail(t, b, "seed derives from roster and beacon")
}

func TestSubstitutedBeaconRoundIsRejected(t *testing.T) {
	b, _ := honest(t, 1, roster.Private, 20)
	b.Beacon.Round = vecRound + 1
	mustFail(t, b, "beacon round is the committed target")
}

func TestForgedBeaconSignatureIsRejected(t *testing.T) {
	b, _ := honest(t, 1, roster.Private, 20)
	b.Beacon.Signature = vecSig[:len(vecSig)-1] + "d"
	mustFail(t, b, "drand signature")
}

func TestWrongOperatorKeyIsRejected(t *testing.T) {
	b, _ := honest(t, 1, roster.Private, 20)
	other, _, _ := draw.GenerateKey()
	b.Operator = hex.EncodeToString(other)
	mustFail(t, b, "signature over draw parameters")
}

func TestRepaddedRosterSizeIsRejected(t *testing.T) {
	b, _ := honest(t, 1, roster.Private, 20)
	b.Roster.Commitment.Size = 40
	mustFail(t, b, "binds root and size")
}

// Public disclosure is verified by rebuilding the whole tree, so removing an
// entrant from the published list must break even though no proof changed.
func TestDroppedEntryInAPublicDrawIsRejected(t *testing.T) {
	b, _ := honest(t, 1, roster.Public, 20)
	b.Roster.Entries = append(b.Roster.Entries[:5], b.Roster.Entries[6:]...)
	mustFail(t, b, "published entries match")
}

func TestAlteredEntryInAPublicDrawIsRejected(t *testing.T) {
	b, _ := honest(t, 1, roster.Public, 20)
	b.Roster.Entries[7].Entry.Label = "swapped-in"
	mustFail(t, b, "entries hash to the published leaves")
}

// The published leaves are what an entrant checks their own inclusion against,
// so a roster that quietly differs from them has to break.
func TestDroppedLeafIsRejected(t *testing.T) {
	b, _ := honest(t, 1, roster.Private, 20)
	b.Roster.Leaves = append(b.Roster.Leaves[:3], b.Roster.Leaves[4:]...)
	mustFail(t, b, "match the committed entry count")
}

func TestSubstitutedLeafIsRejected(t *testing.T) {
	b, _ := honest(t, 1, roster.Private, 20)
	b.Roster.Leaves[9] = hashing.Tagged("test", []byte("a leaf that was never committed"))
	mustFail(t, b, "rebuild to the committed root")
}

// An entrant whose leaf is missing can detect it from the published list
// alone, which is the whole reason the list is published.
func TestEntrantCanDetectBeingDroppedFromTheLeafList(t *testing.T) {
	b, _ := honest(t, 1, roster.Private, 20)

	// Stand in for an entrant holding the salt they were given at entry.
	mine := b.Result.Winners[0]
	salt, err := hex.DecodeString(mine.Salt)
	if err != nil {
		t.Fatal(err)
	}
	leaf := merkle.LeafHash(salt, mine.Entry.CanonicalBytes())

	present := false
	for _, l := range b.Roster.Leaves {
		if l == leaf {
			present = true
		}
	}
	if !present {
		t.Fatal("an entrant could not find their own leaf in the published list")
	}

	// Now drop them, as a dishonest organiser would have before committing.
	pruned := make([]hashing.Digest, 0, len(b.Roster.Leaves))
	for _, l := range b.Roster.Leaves {
		if l != leaf {
			pruned = append(pruned, l)
		}
	}
	for _, l := range pruned {
		if l == leaf {
			t.Fatal("leaf survived the prune")
		}
	}
}

// Winners are checked against the leaf list as well as by proof, so the two
// must not be able to disagree.
func TestWinnerNotAtItsClaimedLeafIsRejected(t *testing.T) {
	b, _ := honest(t, 2, roster.Private, 20)
	b.Roster.Leaves[b.Result.Winners[0].Index] = hashing.Tagged("test", []byte("someone else"))
	mustFail(t, b, "sits at its claimed index")
}

// A private draw that ships the entry list is not private, whatever it claims.
func TestPrivateDrawLeakingItsRosterIsRejected(t *testing.T) {
	pubB, _ := honest(t, 1, roster.Public, 20)
	b, _ := honest(t, 1, roster.Private, 20)
	b.Roster.Entries = pubB.Roster.Entries
	mustFail(t, b, "withholds the entry list")
}

func TestTamperedLogIsRejected(t *testing.T) {
	b, _ := honest(t, 1, roster.Private, 20)
	b.Log.Records[1].LoggedAt += 1
	mustFail(t, b, "unbroken chain")
}

func TestLogMissingTheResultRecordIsRejected(t *testing.T) {
	b, _ := honest(t, 1, roster.Private, 20)
	b.Log.Records = b.Log.Records[:len(b.Log.Records)-1]
	mustFail(t, b, "creation, cutoff and result")
}

func TestVerifyReportsEveryFailureNotJustTheFirst(t *testing.T) {
	b, _ := honest(t, 1, roster.Private, 20)
	b.Beacon.Signature = vecSig[:len(vecSig)-1] + "d"
	b.Result.Seed = hashing.Tagged("test", []byte("forged"))

	rep := bundle.Verify(b, beacon.Quicknet)
	if !failed(rep, "drand signature") || !failed(rep, "seed derives") {
		t.Fatal("verification short-circuited instead of reporting both failures")
	}
}

func TestBundleSurvivesJSONRoundTrip(t *testing.T) {
	for _, disc := range []roster.Disclosure{roster.Public, roster.Private} {
		b, _ := honest(t, 2, disc, 15)
		raw, err := json.Marshal(b)
		if err != nil {
			t.Fatal(err)
		}
		back, err := bundle.Read(strings.NewReader(string(raw)))
		if err != nil {
			t.Fatal(err)
		}
		if !bundle.Verify(back, beacon.Quicknet).OK {
			t.Fatalf("%s bundle stopped verifying after a JSON round trip", disc)
		}
	}
}

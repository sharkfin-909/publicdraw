package draw

import (
	"fmt"
	"testing"
	"time"

	"github.com/sharkfin-909/publicdraw/beacon"
	"github.com/sharkfin-909/publicdraw/hashing"
	"github.com/sharkfin-909/publicdraw/roster"
)

var (
	testRoot = hashing.Tagged("test", []byte("root"))
	testRand = hashing.Tagged("test", []byte("randomness"))
)

func seed(id string) hashing.Digest { return Seed(testRoot, 1000, testRand, id) }

func TestSelectionIsDeterministic(t *testing.T) {
	s := seed("d1")
	a, _, _ := Select(s, 1000, 5)
	b, _, _ := Select(s, 1000, 5)
	for i := range a {
		if a[i] != b[i] {
			t.Fatal("selection is not deterministic")
		}
	}
}

func TestWinnersAreDistinct(t *testing.T) {
	won, _, err := Select(seed("d1"), 500, 50)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[int]bool{}
	for _, i := range won {
		if seen[i] {
			t.Fatalf("index %d selected twice", i)
		}
		seen[i] = true
	}
	if len(won) != 50 {
		t.Fatalf("got %d winners, want 50", len(won))
	}
}

// The ranking is a full ordering, so the entries after the winners are a
// waitlist. Allocation draws always need one.
func TestWaitlistCoversEveryRemainingEntry(t *testing.T) {
	const size, winners = 40, 7
	won, wait, err := Select(seed("d1"), size, winners)
	if err != nil {
		t.Fatal(err)
	}
	if len(wait) != size-winners {
		t.Fatalf("waitlist has %d, want %d", len(wait), size-winners)
	}
	seen := map[int]bool{}
	for _, i := range append(append([]int{}, won...), wait...) {
		if seen[i] {
			t.Fatalf("index %d appears twice across winners and waitlist", i)
		}
		seen[i] = true
	}
	if len(seen) != size {
		t.Fatalf("ranking covers %d of %d entries", len(seen), size)
	}
}

// Winners must be a prefix of the full ranking, so drawing k+1 winners gives
// the same first k. Otherwise a waitlist promotion would contradict the draw.
func TestWinnersArePrefixOfTheRanking(t *testing.T) {
	s := seed("d1")
	five, _, _ := Select(s, 200, 5)
	ten, _, _ := Select(s, 200, 10)
	for i := range five {
		if five[i] != ten[i] {
			t.Fatalf("rank %d differs between a 5-winner and 10-winner draw", i)
		}
	}
}

func TestEveryEntryCanWin(t *testing.T) {
	const size = 12
	seen := map[int]bool{}
	for i := 0; i < 4000; i++ {
		won, _, _ := Select(Seed(testRoot, size, testRand, fmt.Sprintf("draw-%d", i)), size, 1)
		seen[won[0]] = true
	}
	if len(seen) != size {
		t.Fatalf("only %d of %d entries ever won", len(seen), size)
	}
}

func TestEveryInputChangesTheSeed(t *testing.T) {
	base := Seed(testRoot, 10, testRand, "d")
	if Seed(hashing.Tagged("test", []byte("other")), 10, testRand, "d") == base {
		t.Error("roster root does not affect the seed")
	}
	if Seed(testRoot, 11, testRand, "d") == base {
		t.Error("roster size does not affect the seed")
	}
	if Seed(testRoot, 10, hashing.Tagged("test", []byte("x")), "d") == base {
		t.Error("beacon randomness does not affect the seed")
	}
	if Seed(testRoot, 10, testRand, "d2") == base {
		t.Error("draw id does not affect the seed")
	}
}

func TestSelectRejectsImpossibleDraws(t *testing.T) {
	if _, _, err := Select(seed("d"), 3, 4); err == nil {
		t.Error("allowed more winners than entries")
	}
	if _, _, err := Select(seed("d"), 0, 1); err == nil {
		t.Error("allowed an empty roster")
	}
	if _, _, err := Select(seed("d"), 5, 0); err == nil {
		t.Error("allowed zero winners")
	}
}

// The ordering guarantee: parameters are fixed before anyone enters, and the
// target round is placed far enough past the cutoff that it cannot already
// exist when the roster is frozen.
func TestCreateLeavesLeadBetweenCutoffAndRound(t *testing.T) {
	_, priv, _ := GenerateKey()
	now := time.Now()
	cutoff := now.Add(10 * time.Minute)

	p, _, err := Create(beacon.Quicknet, "test", 1, roster.Private, cutoff, priv, now)
	if err != nil {
		t.Fatal(err)
	}
	gap := beacon.Quicknet.TimeOf(p.TargetRound).Unix() - p.CutoffAt
	if gap < int64(MinLead.Seconds()) {
		t.Fatalf("gap between cutoff and round is %ds, want at least %.0f", gap, MinLead.Seconds())
	}
	if p.CreatedAt >= p.CutoffAt {
		t.Fatal("parameters were not fixed before the cutoff")
	}
}

func TestCreateRejectsPastCutoffAndBadInput(t *testing.T) {
	_, priv, _ := GenerateKey()
	now := time.Now()
	if _, _, err := Create(beacon.Quicknet, "t", 1, roster.Private, now.Add(-time.Minute), priv, now); err == nil {
		t.Error("accepted a cutoff in the past")
	}
	if _, _, err := Create(beacon.Quicknet, "t", 0, roster.Private, now.Add(time.Hour), priv, now); err == nil {
		t.Error("accepted zero winners")
	}
	if _, _, err := Create(beacon.Quicknet, "t", 1, "semi", now.Add(time.Hour), priv, now); err == nil {
		t.Error("accepted an unknown disclosure mode")
	}
}

func TestCommitRosterRefusesBeforeCutoff(t *testing.T) {
	_, priv, _ := GenerateKey()
	now := time.Now()
	p, _, _ := Create(beacon.Quicknet, "t", 1, roster.Private, now.Add(time.Hour), priv, now)
	if _, _, err := CommitRoster(p, roster.Commitment{}, priv, now); err == nil {
		t.Fatal("committed a roster while entries were still open")
	}
}

func TestParamsSignatureCoversEveryField(t *testing.T) {
	pub, priv, _ := GenerateKey()
	now := time.Now()
	p, sig, _ := Create(beacon.Quicknet, "t", 3, roster.Public, now.Add(time.Hour), priv, now)
	if err := VerifyParamsSignature(pub, p, sig); err != nil {
		t.Fatalf("honest signature rejected: %v", err)
	}
	for name, mutate := range map[string]func(*Params){
		"draw id":      func(p *Params) { p.DrawID = "other" },
		"title":        func(p *Params) { p.Title = "other" },
		"winners":      func(p *Params) { p.Winners++ },
		"disclosure":   func(p *Params) { p.Disclosure = roster.Private },
		"cutoff":       func(p *Params) { p.CutoffAt += 60 },
		"target round": func(p *Params) { p.TargetRound++ },
		"chain":        func(p *Params) { p.ChainHash = "other" },
		"created at":   func(p *Params) { p.CreatedAt -= 60 },
	} {
		m := p
		mutate(&m)
		if err := VerifyParamsSignature(pub, m, sig); err == nil {
			t.Errorf("signature still verified after altering %s", name)
		}
	}
}

func TestRosterSignatureCoversEveryField(t *testing.T) {
	pub, priv, _ := GenerateKey()
	now := time.Now()
	p, _, _ := Create(beacon.Quicknet, "t", 1, roster.Private, now.Add(time.Second), priv, now)
	c := roster.Commitment{Root: testRoot, Size: 9, Commit: roster.CommitValue(testRoot, 9)}
	rc, sig, err := CommitRoster(p, c, priv, time.Unix(p.CutoffAt, 0))
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyRosterSignature(pub, rc, sig); err != nil {
		t.Fatalf("honest signature rejected: %v", err)
	}
	for name, mutate := range map[string]func(*RosterCommitment){
		"draw id":      func(r *RosterCommitment) { r.DrawID = "other" },
		"root":         func(r *RosterCommitment) { r.Root = testRand },
		"size":         func(r *RosterCommitment) { r.Size++ },
		"commit":       func(r *RosterCommitment) { r.Commit = testRand },
		"committed at": func(r *RosterCommitment) { r.CommittedAt -= 60 },
	} {
		m := rc
		mutate(&m)
		if err := VerifyRosterSignature(pub, m, sig); err == nil {
			t.Errorf("signature still verified after altering %s", name)
		}
	}
}

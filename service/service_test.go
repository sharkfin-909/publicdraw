package service

import (
	"context"
	"testing"
	"time"

	"github.com/sharkfin-909/publicdraw/beacon"
	"github.com/sharkfin-909/publicdraw/bundle"
	"github.com/sharkfin-909/publicdraw/draw"
	"github.com/sharkfin-909/publicdraw/roster"
)

// A captured quicknet round, so the lifecycle can be driven to completion
// without waiting fifteen seconds of real time per test. Its signature is
// still verified by the beacon package, so this cannot smuggle in randomness
// the chain never produced.
const (
	vecRound = 20000000
	vecSig   = "96892582a33552a7b67ba44ef09c3ccd535bbebe760c93ecf45be8958d0c0f06390c6d19d7bf492eb806af7eef6b125c"

	// Chosen so Create lands on vecRound: cutoff+MinLead falls inside round
	// 19999999, so the target becomes 20000000, which publishes 17s later.
	tCreate = 1752803300
	tCutoff = 1752803347
	tRound  = 1752803364
)

type harness struct {
	*Service
	now time.Time
}

func (h *harness) set(unix int64) { h.now = time.Unix(unix, 0) }

func newHarness(t *testing.T) *harness {
	t.Helper()
	_, priv, err := draw.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{now: time.Unix(tCreate, 0)}
	svc, err := New(t.TempDir(), "test-log", priv,
		WithClock(func() time.Time { return h.now }),
		WithFetcher(func(context.Context, uint64) (beacon.Round, error) {
			return beacon.Round{Round: vecRound, Signature: vecSig}, nil
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	h.Service = svc
	return h
}

func (h *harness) fullDraw(t *testing.T, winners int, disc roster.Disclosure, labels ...string) bundle.Bundle {
	t.Helper()
	st, err := h.Create("test draw", winners, disc, time.Unix(tCutoff, 0))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.Enter(st.Params.DrawID, labels...); err != nil {
		t.Fatal(err)
	}
	h.set(tCutoff)
	if _, err := h.CommitRoster(st.Params.DrawID); err != nil {
		t.Fatal(err)
	}
	h.set(tRound + 6)
	b, err := h.Run(context.Background(), st.Params.DrawID)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func names(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = string(rune('a'+i%26)) + "-entrant"
	}
	return out
}

func TestPrivateDrawVerifies(t *testing.T) {
	h := newHarness(t)
	b := h.fullDraw(t, 3, roster.Private, names(25)...)

	rep := bundle.Verify(b, beacon.Quicknet)
	if !rep.OK {
		for _, c := range rep.Checks {
			t.Logf("ok=%v skip=%v %s — %s", c.OK, c.Skipped, c.Name, c.Detail)
		}
		t.Fatal("an honest private draw failed verification")
	}
	if len(b.Roster.Entries) != 0 {
		t.Fatal("a private draw published its entry list")
	}
	if len(b.Result.Winners) != 3 {
		t.Fatalf("got %d winners, want 3", len(b.Result.Winners))
	}
}

func TestPublicDrawVerifiesAndDisclosesEveryEntry(t *testing.T) {
	h := newHarness(t)
	b := h.fullDraw(t, 2, roster.Public, names(25)...)

	rep := bundle.Verify(b, beacon.Quicknet)
	if !rep.OK {
		for _, c := range rep.Checks {
			t.Logf("ok=%v skip=%v %s — %s", c.OK, c.Skipped, c.Name, c.Detail)
		}
		t.Fatal("an honest public draw failed verification")
	}
	if len(b.Roster.Entries) != 25 {
		t.Fatalf("public draw published %d of 25 entries", len(b.Roster.Entries))
	}
}

// Disclosure is the organiser's choice and nothing else changes: the same
// entries under the same beacon must select the same winners either way.
func TestDisclosureDoesNotChangeTheOutcome(t *testing.T) {
	labels := names(30)

	hp := newHarness(t)
	pub := hp.fullDraw(t, 4, roster.Public, labels...)

	hs := newHarness(t)
	priv := hs.fullDraw(t, 4, roster.Private, labels...)

	// Different draw ids mean different seeds, so compare the mechanism
	// rather than the indices: re-derive each from its own inputs.
	if len(pub.Result.Winners) != len(priv.Result.Winners) {
		t.Fatal("winner counts differ between disclosure modes")
	}
	for _, b := range []bundle.Bundle{pub, priv} {
		want, _, err := draw.Select(b.Result.Seed, b.Roster.Commitment.Size, b.Draw.Params.Winners)
		if err != nil {
			t.Fatal(err)
		}
		for i, w := range b.Result.Winners {
			if w.Index != want[i] {
				t.Fatalf("%s draw: winner %d is #%d, derivation says #%d",
					b.Draw.Params.Disclosure, i, w.Index, want[i])
			}
		}
	}
}

func TestEntriesRefusedAfterCutoff(t *testing.T) {
	h := newHarness(t)
	st, _ := h.Create("t", 1, roster.Private, time.Unix(tCutoff, 0))
	if _, err := h.Enter(st.Params.DrawID, names(5)...); err != nil {
		t.Fatal(err)
	}
	h.set(tCutoff)
	if _, err := h.Enter(st.Params.DrawID, "late-arrival"); err == nil {
		t.Fatal("an entry was accepted after the cutoff")
	}
}

// Salts are issued when an entrant enters, not when the roster is frozen, so
// the entrant holds their own evidence from that moment.
func TestEnterReturnsASaltPerEntrantImmediately(t *testing.T) {
	h := newHarness(t)
	st, _ := h.Create("t", 1, roster.Private, time.Unix(tCutoff, 0))

	admitted, err := h.Enter(st.Params.DrawID, "alice", "bob", "carol")
	if err != nil {
		t.Fatal(err)
	}
	if len(admitted) != 3 {
		t.Fatalf("got %d admissions, want 3", len(admitted))
	}
	seen := map[string]bool{}
	for i, a := range admitted {
		if a.Index != i {
			t.Errorf("admission %d has index %d", i, a.Index)
		}
		if len(a.Salt) != roster.SaltSize {
			t.Errorf("admission %d has a %d-byte salt", i, len(a.Salt))
		}
		if seen[a.Salt.String()] {
			t.Errorf("salt reused at index %d", i)
		}
		seen[a.Salt.String()] = true
	}

	// The salt handed out at entry must be the one committed at the cutoff.
	more, err := h.Enter(st.Params.DrawID, "dave")
	if err != nil {
		t.Fatal(err)
	}
	h.set(tCutoff)
	if _, err := h.CommitRoster(st.Params.DrawID); err != nil {
		t.Fatal(err)
	}
	stored, err := h.Get(st.Params.DrawID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Sealed[more[0].Index].Salt.String() != more[0].Salt.String() {
		t.Fatal("the committed salt is not the one the entrant was given")
	}
	if stored.Sealed[more[0].Index].Entry.Label != "dave" {
		t.Fatal("the committed label is not the one at that index")
	}
}

func TestRosterCannotBeCommittedTwice(t *testing.T) {
	h := newHarness(t)
	st, _ := h.Create("t", 1, roster.Private, time.Unix(tCutoff, 0))
	h.Enter(st.Params.DrawID, names(5)...)
	h.set(tCutoff)
	if _, err := h.CommitRoster(st.Params.DrawID); err != nil {
		t.Fatal(err)
	}
	if _, err := h.CommitRoster(st.Params.DrawID); err == nil {
		t.Fatal("the roster was committed a second time")
	}
}

// The central guarantee: once the beacon has published, freezing a roster
// against that round would let the organiser pick entries to suit a known
// value. The service must refuse.
func TestRosterCannotBeCommittedOnceTheRoundHasPublished(t *testing.T) {
	h := newHarness(t)
	st, _ := h.Create("t", 1, roster.Private, time.Unix(tCutoff, 0))
	h.Enter(st.Params.DrawID, names(5)...)

	h.set(tRound + 1)
	_, err := h.CommitRoster(st.Params.DrawID)
	if err == nil {
		t.Fatal("a roster was frozen after its beacon round had already published")
	}
}

func TestDrawCannotRunBeforeItsRound(t *testing.T) {
	h := newHarness(t)
	st, _ := h.Create("t", 1, roster.Private, time.Unix(tCutoff, 0))
	h.Enter(st.Params.DrawID, names(5)...)
	h.set(tCutoff)
	h.CommitRoster(st.Params.DrawID)

	h.set(tRound - 1)
	if _, err := h.Run(context.Background(), st.Params.DrawID); err == nil {
		t.Fatal("a draw ran before its target round published")
	}
}

func TestRunIsIdempotent(t *testing.T) {
	h := newHarness(t)
	b1 := h.fullDraw(t, 2, roster.Private, names(20)...)
	b2, err := h.Run(context.Background(), b1.Draw.Params.DrawID)
	if err != nil {
		t.Fatal(err)
	}
	if b1.Result.Seed != b2.Result.Seed {
		t.Fatal("running a draw twice produced different seeds")
	}
	if len(h.Log().ForDraw(b1.Draw.Params.DrawID)) != 3 {
		t.Fatal("re-running a draw wrote an extra log record")
	}
}

func TestCannotDrawMoreWinnersThanEntries(t *testing.T) {
	h := newHarness(t)
	st, _ := h.Create("t", 10, roster.Private, time.Unix(tCutoff, 0))
	h.Enter(st.Params.DrawID, names(3)...)
	h.set(tCutoff)
	if _, err := h.CommitRoster(st.Params.DrawID); err == nil {
		t.Fatal("committed a roster too small for the promised winner count")
	}
}

func TestReceiptsAreUnavailableUntilTheRosterIsFrozen(t *testing.T) {
	h := newHarness(t)
	st, _ := h.Create("t", 1, roster.Private, time.Unix(tCutoff, 0))
	h.Enter(st.Params.DrawID, names(5)...)
	if _, err := h.Receipt(st.Params.DrawID, 0); err == nil {
		t.Fatal("issued a receipt before the root existed")
	}
	h.set(tCutoff)
	h.CommitRoster(st.Params.DrawID)

	rec, err := h.Receipt(st.Params.DrawID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Root != h.mustCommit(t, st.Params.DrawID).Root {
		t.Fatal("receipt names a root the draw did not commit to")
	}
}

func (h *harness) mustCommit(t *testing.T, id string) draw.RosterCommitment {
	t.Helper()
	st, err := h.Get(id)
	if err != nil || st.RosterCommit == nil {
		t.Fatal("no roster commitment")
	}
	return *st.RosterCommit
}

// Every phase must leave a trace, or the log cannot answer what happened.
func TestEveryPhaseIsLogged(t *testing.T) {
	h := newHarness(t)
	b := h.fullDraw(t, 1, roster.Private, names(6)...)

	kinds := map[string]bool{}
	for _, r := range h.Log().ForDraw(b.Draw.Params.DrawID) {
		kinds[string(r.Kind)] = true
	}
	for _, want := range []string{"draw_created", "roster_committed", "result_published"} {
		if !kinds[want] {
			t.Errorf("no %s record in the log", want)
		}
	}
}

func TestAbandonedDrawIsVisibleInTheLog(t *testing.T) {
	h := newHarness(t)

	// Created and entered, then quietly never run — the shape a retry attack
	// leaves behind. It has to be set up before the clock advances, because
	// the service refuses to create a draw whose cutoff has already passed.
	dropped, err := h.Create("one the organiser did not like", 1, roster.Private, time.Unix(tCutoff, 0))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.Enter(dropped.Params.DrawID, names(6)...); err != nil {
		t.Fatal(err)
	}

	h.fullDraw(t, 1, roster.Private, names(6)...)

	got := mustAbandoned(t, h.Service)
	if len(got) != 1 || got[0] != dropped.Params.DrawID {
		t.Fatalf("abandoned = %v, want [%s]", got, dropped.Params.DrawID)
	}
}

func mustAbandoned(t *testing.T, s *Service) []string {
	t.Helper()
	got, err := s.Abandoned()
	if err != nil {
		t.Fatal(err)
	}
	return got
}

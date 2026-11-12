package service

import (
	"context"
	"crypto/ed25519"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/sharkfin-909/publicdraw/draw"
	"github.com/sharkfin-909/publicdraw/hashing"
	"github.com/sharkfin-909/publicdraw/rekor"
	"github.com/sharkfin-909/publicdraw/roster"
	"github.com/sharkfin-909/publicdraw/translog"
)

// fakeRekor is the test adapter at the anchoring seam. It records every head
// it was asked to anchor, and fails on demand.
type fakeRekor struct {
	mu    sync.Mutex
	heads []hashing.Digest
	fail  error
	// seen, if set, runs inside each call, so a test can look at the
	// service's state at the moment anchoring happens.
	seen func(hashing.Digest)
}

func (f *fakeRekor) anchorer() Anchorer {
	return func(_ context.Context, head hashing.Digest) (*rekor.Anchor, error) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.heads = append(f.heads, head)
		if f.seen != nil {
			f.seen(head)
		}
		if f.fail != nil {
			return nil, f.fail
		}
		return &rekor.Anchor{LogIndex: uint64(len(f.heads))}, nil
	}
}

func openDraw(t *testing.T, h *harness, winners, entries int) string {
	t.Helper()
	st, err := h.Create("d", winners, roster.Private, time.Unix(tCutoff, 0))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.Enter(st.Params.DrawID, names(entries)...); err != nil {
		t.Fatal(err)
	}
	return st.Params.DrawID
}

func TestFreezeAnchorsTheHeadThatRecordsTheRoster(t *testing.T) {
	rk := &fakeRekor{}
	h := newHarness(t, WithAnchorer(rk.anchorer()))
	id := openDraw(t, h, 1, 3)
	h.set(tCutoff)

	f, err := h.Freeze(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	rec, ok := h.Log().Find(id, translog.KindRosterCommitted)
	if !ok {
		t.Fatal("no roster record in the log")
	}
	if len(rk.heads) != 1 || rk.heads[0] != rec.Head || f.Head != rec.Head {
		t.Fatalf("anchored %v, want exactly the roster record's head %s", rk.heads, rec.Head)
	}
	if f.Anchor == nil || f.AnchorErr != nil {
		t.Fatalf("anchor = %v, err = %v", f.Anchor, f.AnchorErr)
	}
	st, _ := h.Get(id)
	if st.AnchoredHead != rec.Head.String() {
		t.Fatalf("anchored head recorded as %q, want %s", st.AnchoredHead, rec.Head)
	}
	if h.AnchorFor(rec.Head) == nil {
		t.Fatal("anchor proof was not kept for the bundle")
	}
}

func TestAFailedAnchorDoesNotUndoTheFreeze(t *testing.T) {
	rk := &fakeRekor{fail: errors.New("rekor is down")}
	h := newHarness(t, WithAnchorer(rk.anchorer()))
	id := openDraw(t, h, 1, 2)
	h.set(tCutoff)

	f, err := h.Freeze(context.Background(), id)
	if err != nil {
		t.Fatalf("freeze failed with the anchor: %v", err)
	}
	if f.AnchorErr == nil || f.Anchor != nil {
		t.Fatalf("anchor = %v, err = %v; want the failure reported", f.Anchor, f.AnchorErr)
	}
	if f.Draw.State != StateCommitted {
		t.Fatalf("state = %q, want committed", f.Draw.State)
	}

	// The retry anchors the same commit head, not whatever the log says now.
	rk.fail = nil
	if _, err := h.Create("later", 1, roster.Private, time.Unix(tCutoff+60, 0)); err != nil {
		t.Fatal(err)
	}
	g, err := h.Reanchor(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if g.Head != f.Head || rk.heads[len(rk.heads)-1] != f.Head {
		t.Fatalf("reanchored %s, want the commit head %s", rk.heads[len(rk.heads)-1], f.Head)
	}
}

func TestFreezeDueCommitsEveryDrawBeforeAnchoringAny(t *testing.T) {
	rk := &fakeRekor{}
	h := newHarness(t, WithAnchorer(rk.anchorer()))
	a, b := openDraw(t, h, 1, 2), openDraw(t, h, 1, 2)
	rk.seen = func(hashing.Digest) {
		for _, id := range []string{a, b} {
			if _, ok := h.Log().Find(id, translog.KindRosterCommitted); !ok {
				t.Errorf("anchoring began before %s was committed", id)
			}
		}
	}
	h.set(tCutoff)

	frozen, err := h.FreezeDue(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(frozen) != 2 || len(rk.heads) != 2 {
		t.Fatalf("froze %d, anchored %d; want 2 and 2", len(frozen), len(rk.heads))
	}
}

func TestKeepFreezingFreezesUntilCancelled(t *testing.T) {
	h := newHarness(t)
	id := openDraw(t, h, 1, 2)
	h.set(tCutoff)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	reported := make(chan []Frozen, 1)
	go func() {
		h.KeepFreezing(ctx, 5*time.Millisecond, func(f []Frozen, _ error) { reported <- f })
		close(done)
	}()
	select {
	case f := <-reported:
		if len(f) != 1 || f[0].Draw.Params.DrawID != id {
			t.Fatalf("froze %v, want %s", f, id)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("nothing was frozen")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("KeepFreezing did not stop when cancelled")
	}
}

// A log that will not take a write is how an append fails in practice: a full
// or read-only disk.
func breakLog(t *testing.T, dir string) (restore func()) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root ignores file permissions")
	}
	p := filepath.Join(dir, "log.jsonl")
	if err := os.Chmod(p, 0o444); err != nil {
		t.Fatal(err)
	}
	return func() { os.Chmod(p, 0o644) }
}

func countRecords(l *translog.Log, id string, kind translog.Kind) int {
	n := 0
	for _, r := range l.Records() {
		if r.DrawID == id && r.Kind == kind {
			n++
		}
	}
	return n
}

func TestARunWhoseLogWriteFailedIsLoggedOnRetry(t *testing.T) {
	dir := t.TempDir()
	h := newHarnessIn(t, dir)
	id := openDraw(t, h, 1, 3)
	h.set(tCutoff)
	if _, err := h.Freeze(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	h.set(tRound + 6)

	restore := breakLog(t, dir)
	if _, err := h.Run(context.Background(), id); err == nil {
		t.Fatal("Run reported success with a log that cannot be written")
	}
	restore()

	// Before, the draw was now marked drawn with no result in the log, and
	// every retry skipped the append because the draw already looked drawn.
	if _, err := h.Run(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if n := countRecords(h.Log(), id, translog.KindResultPublished); n != 1 {
		t.Fatalf("%d result records, want exactly 1", n)
	}
	if ab, _ := h.Abandoned(); len(ab) != 0 {
		t.Fatalf("a drawn draw is listed as abandoned: %v", ab)
	}
}

func TestReopeningFinishesAStateChangeLeftHalfLogged(t *testing.T) {
	dir := t.TempDir()
	_, priv, err := draw.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{now: time.Unix(tCreate, 0)}
	h.open(t, dir, priv)
	id := openDraw(t, h, 1, 3)
	h.set(tCutoff)

	restore := breakLog(t, dir)
	if _, err := h.Freeze(context.Background(), id); err == nil {
		t.Fatal("Freeze reported success with a log that cannot be written")
	}
	restore()
	h.Close()

	h.open(t, dir, priv)
	if n := countRecords(h.Log(), id, translog.KindRosterCommitted); n != 1 {
		t.Fatalf("%d roster records after reopening, want 1", n)
	}
	st, _ := h.Get(id)
	if st.Pending != nil || st.State != StateCommitted {
		t.Fatalf("state %q, pending %v; want committed with nothing pending", st.State, st.Pending)
	}
	// The record that was logged is the one that was signed.
	rec, _ := h.Log().Find(id, translog.KindRosterCommitted)
	var want hashing.Digest
	copy(want[:], st.RosterCommit.SigningBytes())
	if rec.Payload != want {
		t.Fatal("the logged roster record does not match the signed commitment")
	}
}

func TestASecondProcessCannotOpenTheSameDirectory(t *testing.T) {
	dir := t.TempDir()
	_, priv, _ := draw.GenerateKey()
	first, err := New(dir, "test-log", priv)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(dir, "test-log", priv); !errors.Is(err, ErrLocked) {
		t.Fatalf("second open: %v, want ErrLocked", err)
	}
	first.Close()
	second, err := New(dir, "test-log", ed25519.PrivateKey(priv))
	if err != nil {
		t.Fatalf("open after Close: %v", err)
	}
	second.Close()
}

func TestErrorsSayWhetherTheCallerCouldHaveAvoidedThem(t *testing.T) {
	h := newHarness(t)
	if _, err := h.Get("nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing draw: %v, want ErrNotFound", err)
	}
	id := openDraw(t, h, 1, 2)
	h.set(tCutoff)
	if _, err := h.Enter(id, "late"); !errors.Is(err, ErrRefused) {
		t.Fatalf("entering after the cutoff: %v, want ErrRefused", err)
	}
	if _, err := h.Leaves(id); !errors.Is(err, ErrRefused) {
		t.Fatalf("leaves before the freeze: %v, want ErrRefused", err)
	}
}

func TestTheViewShowsLabelsOnlyForAFrozenPublicDraw(t *testing.T) {
	h := newHarness(t)
	pub, _ := h.Create("pub", 1, roster.Public, time.Unix(tCutoff, 0))
	priv, _ := h.Create("priv", 1, roster.Private, time.Unix(tCutoff, 0))
	for _, id := range []string{pub.Params.DrawID, priv.Params.DrawID} {
		if _, err := h.Enter(id, "alice", "bob"); err != nil {
			t.Fatal(err)
		}
	}
	if v, _ := h.View(pub.Params.DrawID); v.Labels != nil {
		t.Fatalf("an open public draw shows labels: %v", v.Labels)
	}
	h.set(tCutoff)
	for _, id := range []string{pub.Params.DrawID, priv.Params.DrawID} {
		if _, err := h.Freeze(context.Background(), id); err != nil {
			t.Fatal(err)
		}
	}
	if v, _ := h.View(pub.Params.DrawID); len(v.Labels) != 2 || v.Labels[0] != "alice" {
		t.Fatalf("frozen public draw labels = %v", v.Labels)
	}
	if v, _ := h.View(priv.Params.DrawID); v.Labels != nil {
		t.Fatalf("a private draw shows labels: %v", v.Labels)
	}
}

func TestCreateOpenForCountsFromTheServiceClock(t *testing.T) {
	h := newHarness(t)
	st, err := h.CreateOpenFor("ten minutes", 1, roster.Private, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if want := int64(tCreate + 600); st.Params.CutoffAt != want {
		t.Fatalf("cutoff = %d, want %d", st.Params.CutoffAt, want)
	}
	if _, err := h.CreateOpenFor("never", 1, roster.Private, 0); !errors.Is(err, ErrRefused) {
		t.Fatalf("zero duration: %v, want ErrRefused", err)
	}
}

func TestAReaderOpensBesideTheWriterAndChangesNothing(t *testing.T) {
	dir := t.TempDir()
	_, priv, _ := draw.GenerateKey()
	w, err := New(dir, "test-log", priv)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	st, err := w.Create("d", 1, roster.Private, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	id := st.Params.DrawID

	r, err := New(dir, "test-log", priv, ReadOnly())
	if err != nil {
		t.Fatalf("reader beside a writer: %v", err)
	}
	defer r.Close()
	if got, err := r.Get(id); err != nil || got.Params.DrawID != id {
		t.Fatalf("reader Get: %v, %v", got, err)
	}
	if r.Log().Len() != w.Log().Len() {
		t.Fatalf("reader sees %d records, writer %d", r.Log().Len(), w.Log().Len())
	}
	if _, err := r.Enter(id, "alice"); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("reader Enter: %v, want ErrReadOnly", err)
	}
	if _, err := r.Create("x", 1, roster.Private, time.Now().Add(time.Hour)); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("reader Create: %v, want ErrReadOnly", err)
	}
	if n, _ := w.List(); len(n) != 1 {
		t.Fatalf("%d draws on disk, want 1", len(n))
	}
	if w.Log().Len() != 1 {
		t.Fatalf("log has %d records after refused writes, want 1", w.Log().Len())
	}
}

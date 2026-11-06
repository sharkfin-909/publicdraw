package translog

import (
	"path/filepath"
	"testing"

	"github.com/sharkfin-909/publicdraw/hashing"
)

func d(s string) hashing.Digest { return hashing.Tagged("test", []byte(s)) }

func newLog(t *testing.T) *Log {
	t.Helper()
	l, err := Open(filepath.Join(t.TempDir(), "log.jsonl"), "test-log")
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestEmptyLogStartsAtGenesis(t *testing.T) {
	l := newLog(t)
	if l.Head() != Genesis("test-log") {
		t.Fatal("empty log head is not genesis")
	}
	if l.Len() != 0 {
		t.Fatal("empty log is not empty")
	}
}

// Two logs must never share a prefix, or records could be transplanted between
// them.
func TestGenesisIsBoundToTheLogIdentity(t *testing.T) {
	if Genesis("log-a") == Genesis("log-b") {
		t.Fatal("different logs share a genesis head")
	}
}

func TestAppendChainsAndPersists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log.jsonl")

	l, _ := Open(path, "test-log")
	for i, k := range []Kind{KindDrawCreated, KindRosterCommitted, KindResultPublished} {
		if _, err := l.Append(k, "draw-1", d(string(k)), int64(1000+i)); err != nil {
			t.Fatal(err)
		}
	}
	head := l.Head()

	reopened, err := Open(path, "test-log")
	if err != nil {
		t.Fatal(err)
	}
	if reopened.Head() != head {
		t.Fatal("reopening the log produced a different head")
	}
	if reopened.Len() != 3 {
		t.Fatalf("reopened log has %d records, want 3", reopened.Len())
	}
}

func TestVerifyReplaysToTheSameHead(t *testing.T) {
	l := newLog(t)
	for i := 0; i < 5; i++ {
		l.Append(KindDrawCreated, "d", d(string(rune('a'+i))), int64(i))
	}
	head, err := Verify(l.Records(), "test-log")
	if err != nil {
		t.Fatal(err)
	}
	if head != l.Head() {
		t.Fatal("replay arrived at a different head")
	}
}

// Tamper-evidence: editing any field of any record must break the chain.
func TestEditedRecordBreaksTheChain(t *testing.T) {
	l := newLog(t)
	for i := 0; i < 4; i++ {
		l.Append(KindDrawCreated, "d", d(string(rune('a'+i))), int64(i))
	}
	for name, mutate := range map[string]func(*Record){
		"payload":   func(r *Record) { r.Payload = d("swapped") },
		"kind":      func(r *Record) { r.Kind = KindResultPublished },
		"draw id":   func(r *Record) { r.DrawID = "other" },
		"timestamp": func(r *Record) { r.LoggedAt += 1 },
		"head":      func(r *Record) { r.Head = d("forged") },
	} {
		records := l.Records()
		mutate(&records[1])
		if _, err := Verify(records, "test-log"); err == nil {
			t.Errorf("chain still verified after altering %s of record 1", name)
		}
	}
}

func TestRemovingARecordBreaksTheChain(t *testing.T) {
	l := newLog(t)
	for i := 0; i < 4; i++ {
		l.Append(KindDrawCreated, "d", d(string(rune('a'+i))), int64(i))
	}
	records := l.Records()
	shortened := append(records[:1:1], records[2:]...)
	if _, err := Verify(shortened, "test-log"); err == nil {
		t.Fatal("chain verified with a record removed")
	}
}

func TestOpenRefusesACorruptedLog(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log.jsonl")
	l, _ := Open(path, "test-log")
	l.Append(KindDrawCreated, "d", d("x"), 1)
	l.Append(KindDrawCreated, "d", d("y"), 2)

	// Rewrite the file with a forged payload in the first record.
	records := l.Records()
	records[0].Payload = d("forged")
	rewrite(t, path, records)

	if _, err := Open(path, "test-log"); err == nil {
		t.Fatal("opened a log whose chain does not verify")
	}
}

// The retry attack shows up only as a draw that was created and never
// published, so the log has to surface those rather than filter them out.
func TestUnresolvedDrawsAreSurfaced(t *testing.T) {
	l := newLog(t)
	l.Append(KindDrawCreated, "kept", d("p1"), 100)
	l.Append(KindRosterCommitted, "kept", d("r1"), 110)
	l.Append(KindResultPublished, "kept", d("s1"), 120)

	l.Append(KindDrawCreated, "quietly-dropped", d("p2"), 130)
	l.Append(KindRosterCommitted, "quietly-dropped", d("r2"), 140)

	got := l.Unresolved(200)
	if len(got) != 1 || got[0] != "quietly-dropped" {
		t.Fatalf("abandoned = %v, want [quietly-dropped]", got)
	}
}

func TestForDrawFiltersByDraw(t *testing.T) {
	l := newLog(t)
	l.Append(KindDrawCreated, "a", d("1"), 1)
	l.Append(KindDrawCreated, "b", d("2"), 2)
	l.Append(KindResultPublished, "a", d("3"), 3)

	if got := l.ForDraw("a"); len(got) != 2 {
		t.Fatalf("got %d records for draw a, want 2", len(got))
	}
}

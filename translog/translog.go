// Package translog is the append-only transparency log.
//
// It exists to close an attack that no amount of per-draw cryptography can
// reach: an organiser who creates a draw, sees the result, quietly abandons it
// and tries again. Every individual bundle verifies perfectly. The fraud is
// invisible because it lives in what was *not* published.
//
// The fix is the same one Certificate Transparency applies to misissued
// certificates — make the set of commitments public and append-only, so an
// abandoned draw leaves a permanent, visible trace. A draw whose parameters
// were logged but whose result never appeared is then an open question the
// organiser has to answer.
//
// The log is a hash chain rather than a CT-style Merkle tree. A chain gives
// ordering and tamper-evidence intrinsically and is far simpler; the cost is
// that proving one record's membership means replaying from it to an anchored
// head, which is fine at the sizes this serves. Moving to a Merkle log with
// O(log n) inclusion and consistency proofs is the upgrade path if a log ever
// grows past the point where replay is cheap.
package translog

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"sync"

	"github.com/sharkfin009/verifiable-challenge/hashing"
)

// Kind labels what a record commits to.
type Kind string

const (
	// KindDrawCreated records the parameters, fixed before anyone enters.
	KindDrawCreated Kind = "draw_created"
	// KindRosterCommitted records the frozen entry list at cutoff.
	KindRosterCommitted Kind = "roster_committed"
	// KindResultPublished records the drawn outcome.
	KindResultPublished Kind = "result_published"
)

// Record is one entry in the chain.
type Record struct {
	Seq      uint64         `json:"seq"`
	Kind     Kind           `json:"kind"`
	DrawID   string         `json:"draw_id"`
	Payload  hashing.Digest `json:"payload"`
	LoggedAt int64          `json:"logged_at"`
	Prev     hashing.Digest `json:"prev"`
	Head     hashing.Digest `json:"head"`
}

// Genesis is the head of an empty log, bound to the log's identity so that two
// different logs can never share a prefix.
func Genesis(logID string) hashing.Digest {
	return hashing.Tagged(hashing.TagLogGenesis, []byte(logID))
}

// HeadAfter computes the head that results from appending this record to prev.
func HeadAfter(prev hashing.Digest, seq uint64, kind Kind, drawID string, payload hashing.Digest, loggedAt int64) hashing.Digest {
	return hashing.Tagged(hashing.TagLogEntry,
		prev[:],
		hashing.U64(seq),
		[]byte(kind),
		[]byte(drawID),
		payload[:],
		hashing.U64(uint64(loggedAt)),
	)
}

// Log is an append-only chain persisted as one JSON record per line.
type Log struct {
	mu      sync.Mutex
	id      string
	path    string
	head    hashing.Digest
	seq     uint64
	records []Record
}

// Open loads a log from path, replaying and verifying the whole chain. A log
// that does not verify is not opened: silently continuing from a broken chain
// would defeat the point of keeping one.
func Open(path, logID string) (*Log, error) {
	l := &Log{id: logID, path: path, head: Genesis(logID)}

	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return l, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var r Record
		if err := json.Unmarshal(line, &r); err != nil {
			return nil, fmt.Errorf("translog: record %d: %w", l.seq, err)
		}
		if err := l.applyLocked(r); err != nil {
			return nil, err
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return l, nil
}

func (l *Log) applyLocked(r Record) error {
	if r.Seq != l.seq {
		return fmt.Errorf("translog: record out of sequence: got %d, want %d", r.Seq, l.seq)
	}
	if r.Prev != l.head {
		return fmt.Errorf("translog: record %d does not chain to the previous head", r.Seq)
	}
	want := HeadAfter(r.Prev, r.Seq, r.Kind, r.DrawID, r.Payload, r.LoggedAt)
	if want != r.Head {
		return fmt.Errorf("translog: record %d has a forged head", r.Seq)
	}
	l.head = r.Head
	l.seq++
	l.records = append(l.records, r)
	return nil
}

// Append adds a record and flushes it to disk before returning, so a crash
// cannot leave a draw that happened but was never logged.
func (l *Log) Append(kind Kind, drawID string, payload hashing.Digest, loggedAt int64) (Record, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	r := Record{
		Seq: l.seq, Kind: kind, DrawID: drawID, Payload: payload,
		LoggedAt: loggedAt, Prev: l.head,
	}
	r.Head = HeadAfter(r.Prev, r.Seq, r.Kind, r.DrawID, r.Payload, r.LoggedAt)

	line, err := json.Marshal(r)
	if err != nil {
		return Record{}, err
	}
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return Record{}, err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		f.Close()
		return Record{}, err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return Record{}, err
	}
	if err := f.Close(); err != nil {
		return Record{}, err
	}

	if err := l.applyLocked(r); err != nil {
		return Record{}, err
	}
	return r, nil
}

// Head returns the current chain head — the value to anchor with OpenTimestamps.
func (l *Log) Head() hashing.Digest {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.head
}

// Len returns the number of records.
func (l *Log) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return int(l.seq)
}

// Records returns a copy of the chain.
func (l *Log) Records() []Record {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]Record, len(l.records))
	copy(out, l.records)
	return out
}

// ForDraw returns every record belonging to one draw.
func (l *Log) ForDraw(drawID string) []Record {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []Record
	for _, r := range l.records {
		if r.DrawID == drawID {
			out = append(out, r)
		}
	}
	return out
}

// Unresolved lists draws whose creation was logged before `before` but whose
// result never was.
//
// This is a raw structural fact about the log, not a judgement: a draw whose
// round has not yet published is unresolved and perfectly healthy. Deciding
// which unresolved draws are actually *overdue* needs each draw's target
// round, which this package deliberately does not know — see
// service.Abandoned.
func (l *Log) Unresolved(before int64) []string {
	l.mu.Lock()
	defer l.mu.Unlock()

	created := map[string]int64{}
	published := map[string]bool{}
	for _, r := range l.records {
		switch r.Kind {
		case KindDrawCreated:
			created[r.DrawID] = r.LoggedAt
		case KindResultPublished:
			published[r.DrawID] = true
		}
	}
	var out []string
	for id, at := range created {
		if !published[id] && at <= before {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

// Verify replays a chain from genesis and reports the head it arrives at.
func Verify(records []Record, logID string) (hashing.Digest, error) {
	head := Genesis(logID)
	for i, r := range records {
		if r.Seq != uint64(i) {
			return head, fmt.Errorf("translog: record %d is out of sequence", i)
		}
		if r.Prev != head {
			return head, fmt.Errorf("translog: record %d breaks the chain", i)
		}
		want := HeadAfter(r.Prev, r.Seq, r.Kind, r.DrawID, r.Payload, r.LoggedAt)
		if want != r.Head {
			return head, fmt.Errorf("translog: record %d has a forged head", i)
		}
		head = r.Head
	}
	return head, nil
}

// ReadRecords decodes a JSONL log stream.
func ReadRecords(rd io.Reader) ([]Record, error) {
	var out []Record
	sc := bufio.NewScanner(rd)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		if len(sc.Bytes()) == 0 {
			continue
		}
		var r Record
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, sc.Err()
}

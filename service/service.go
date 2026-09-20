// Package service orchestrates the draw lifecycle over file-backed storage.
//
// It is the single place that enforces ordering — parameters before entries,
// entries before cutoff, roster before the beacon round — so that the CLI and
// the HTTP API cannot drift apart on the rules that make a draw meaningful.
package service

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sharkfin009/verifiable-challenge/beacon"
	"github.com/sharkfin009/verifiable-challenge/bundle"
	"github.com/sharkfin009/verifiable-challenge/draw"
	"github.com/sharkfin009/verifiable-challenge/hashing"
	"github.com/sharkfin009/verifiable-challenge/roster"
	"github.com/sharkfin009/verifiable-challenge/translog"
)

// State is where a draw has reached in its lifecycle.
type State string

const (
	StateOpen      State = "open"      // accepting entries
	StateCommitted State = "committed" // roster frozen, waiting for the round
	StateDrawn     State = "drawn"     // result published
)

// Stored is the on-disk record of a draw.
type Stored struct {
	Params       draw.Params            `json:"params"`
	ParamsSig    string                 `json:"params_sig"`
	State        State                  `json:"state"`
	Sealed       []roster.Sealed        `json:"sealed,omitempty"`
	RosterCommit *draw.RosterCommitment `json:"roster_commit,omitempty"`
	RosterSig    string                 `json:"roster_sig,omitempty"`
}

// Service owns the operator key, the storage directory and the log.
type Service struct {
	mu    sync.Mutex
	dir   string
	chain beacon.Chain
	priv  ed25519.PrivateKey
	pub   ed25519.PublicKey
	log   *translog.Log
	logID string
	now   func() time.Time
	fetch func(context.Context, uint64) (beacon.Round, error)
}

// Option customises a Service.
type Option func(*Service)

// WithClock replaces the time source. Tests use it to drive the lifecycle
// without waiting for real drand rounds.
func WithClock(f func() time.Time) Option { return func(s *Service) { s.now = f } }

// WithChain selects a drand chain.
func WithChain(c beacon.Chain) Option { return func(s *Service) { s.chain = c } }

// WithFetcher replaces the beacon source. Tests use it to drive a draw to
// completion against a captured round instead of waiting for a live one; the
// signature is still verified, so a fake fetcher cannot smuggle in a round the
// chain never produced.
func WithFetcher(f func(context.Context, uint64) (beacon.Round, error)) Option {
	return func(s *Service) { s.fetch = f }
}

// New opens (or creates) a service rooted at dir.
func New(dir, logID string, priv ed25519.PrivateKey, opts ...Option) (*Service, error) {
	if len(priv) != ed25519.PrivateKeySize {
		return nil, errors.New("service: bad operator private key")
	}
	s := &Service{
		dir: dir, chain: beacon.Quicknet, priv: priv,
		pub: priv.Public().(ed25519.PublicKey), logID: logID, now: time.Now,
	}
	s.fetch = func(ctx context.Context, r uint64) (beacon.Round, error) { return s.chain.Fetch(ctx, r) }
	for _, o := range opts {
		o(s)
	}
	if err := os.MkdirAll(filepath.Join(dir, "draws"), 0o755); err != nil {
		return nil, err
	}
	l, err := translog.Open(filepath.Join(dir, "log.jsonl"), logID)
	if err != nil {
		return nil, err
	}
	s.log = l
	return s, nil
}

// OperatorKey returns the public key bundles are verified against.
func (s *Service) OperatorKey() string { return hex.EncodeToString(s.pub) }

// Chain returns the drand chain in use.
func (s *Service) Chain() beacon.Chain { return s.chain }

// Log exposes the transparency log.
func (s *Service) Log() *translog.Log { return s.log }

func (s *Service) path(id string) string { return filepath.Join(s.dir, "draws", id+".json") }

func (s *Service) load(id string) (*Stored, error) {
	b, err := os.ReadFile(s.path(id))
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("service: no draw %q", id)
	}
	if err != nil {
		return nil, err
	}
	var st Stored
	if err := json.Unmarshal(b, &st); err != nil {
		return nil, err
	}
	return &st, nil
}

func (s *Service) save(st *Stored) error {
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(st.Params.DrawID), append(b, '\n'), 0o600)
}

// Create fixes and logs a draw's parameters. Nothing about the entry list is
// known yet, which is the point: the cutoff and the target round are pinned
// before the organiser can see who enters.
func (s *Service) Create(title string, winners int, disc roster.Disclosure, cutoff time.Time) (*Stored, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	p, sig, err := draw.Create(s.chain, title, winners, disc, cutoff, s.priv, now)
	if err != nil {
		return nil, err
	}
	st := &Stored{Params: p, ParamsSig: hex.EncodeToString(sig), State: StateOpen}
	if err := s.save(st); err != nil {
		return nil, err
	}
	if _, err := s.log.Append(translog.KindDrawCreated, p.DrawID, p.Commit(), now.Unix()); err != nil {
		return nil, err
	}
	return st, nil
}

// Admission is what an entrant is handed the moment they enter: their
// position, and the salt that hides their label in the tree.
//
// Handing the salt over now rather than at the cutoff is what makes the
// entrant's later self-check possible without anything having to be delivered
// to them. A receipt that never arrived is indistinguishable from one that was
// never issued; a salt held since entry is not.
type Admission struct {
	Index int         `json:"index"`
	Salt  roster.Salt `json:"salt"`
	Label string      `json:"label"`
}

// Enter appends entries, salting each one as it arrives, and returns what each
// entrant needs to keep.
//
// It refuses after the cutoff: a list that can still grow once the roster is
// committed is not a committed list.
func (s *Service) Enter(id string, labels ...string) ([]Admission, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	st, err := s.load(id)
	if err != nil {
		return nil, err
	}
	if st.State != StateOpen {
		return nil, fmt.Errorf("service: draw %s is %s, not open", id, st.State)
	}
	if s.now().Unix() >= st.Params.CutoffAt {
		return nil, fmt.Errorf("service: entries for %s closed at %d", id, st.Params.CutoffAt)
	}
	if len(labels) == 0 {
		return nil, nil
	}

	out := make([]Admission, 0, len(labels))
	for _, l := range labels {
		sealed, err := roster.SealOne(l)
		if err != nil {
			return nil, err
		}
		out = append(out, Admission{Index: len(st.Sealed), Salt: sealed.Salt, Label: l})
		st.Sealed = append(st.Sealed, sealed)
	}
	if err := s.save(st); err != nil {
		return nil, err
	}
	return out, nil
}

// CommitRoster freezes the entry list, signs it and logs it. It must happen at
// or after the cutoff and before the target round publishes; both bounds are
// enforced here and re-checked by any verifier.
func (s *Service) CommitRoster(id string) (*Stored, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	st, err := s.load(id)
	if err != nil {
		return nil, err
	}
	if st.State != StateOpen {
		return nil, fmt.Errorf("service: draw %s is already %s", id, st.State)
	}
	if len(st.Sealed) < st.Params.Winners {
		return nil, fmt.Errorf("service: %d entries cannot yield %d winners", len(st.Sealed), st.Params.Winners)
	}
	now := s.now()
	if now.Unix() >= s.chain.TimeOf(st.Params.TargetRound).Unix() {
		return nil, fmt.Errorf("service: round %d has already published; this draw can no longer be committed honestly", st.Params.TargetRound)
	}

	// Entries were salted on arrival, so freezing is only a matter of
	// committing to the order they already sit in.
	r, err := roster.Load(st.Sealed)
	if err != nil {
		return nil, err
	}
	rc, sig, err := draw.CommitRoster(st.Params, r.Commitment(), s.priv, now)
	if err != nil {
		return nil, err
	}
	st.RosterCommit = &rc
	st.RosterSig = hex.EncodeToString(sig)
	st.State = StateCommitted
	if err := s.save(st); err != nil {
		return nil, err
	}

	var payload hashing.Digest
	copy(payload[:], rc.SigningBytes())
	if _, err := s.log.Append(translog.KindRosterCommitted, id, payload, now.Unix()); err != nil {
		return nil, err
	}
	return st, nil
}

// Receipt issues one entrant their proof of inclusion. Available only once the
// roster is committed, because the root does not exist before that.
func (s *Service) Receipt(id string, index int) (roster.Receipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	st, err := s.load(id)
	if err != nil {
		return roster.Receipt{}, err
	}
	if st.RosterCommit == nil {
		return roster.Receipt{}, fmt.Errorf("service: draw %s has no committed roster yet", id)
	}
	r, err := roster.Load(st.Sealed)
	if err != nil {
		return roster.Receipt{}, err
	}
	sealed, path, err := r.Open(index)
	if err != nil {
		return roster.Receipt{}, err
	}
	return roster.Receipt{
		DrawID: id, Root: st.RosterCommit.Root, Size: st.RosterCommit.Size,
		Index: index, Salt: hex.EncodeToString(sealed.Salt), Entry: sealed.Entry,
		MerklePath: path,
	}, nil
}

// Run fetches the target round, derives the winners and publishes the bundle.
func (s *Service) Run(ctx context.Context, id string) (bundle.Bundle, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	st, err := s.load(id)
	if err != nil {
		return bundle.Bundle{}, err
	}
	if st.State == StateOpen {
		return bundle.Bundle{}, fmt.Errorf("service: draw %s has no committed roster", id)
	}
	target := s.chain.TimeOf(st.Params.TargetRound)
	if s.now().Before(target) {
		return bundle.Bundle{}, fmt.Errorf("service: round %d does not publish for another %s",
			st.Params.TargetRound, time.Until(target).Truncate(time.Second))
	}

	r, err := s.fetch(ctx, st.Params.TargetRound)
	if err != nil {
		return bundle.Bundle{}, err
	}
	b, err := s.assemble(st, r)
	if err != nil {
		return bundle.Bundle{}, err
	}

	if st.State != StateDrawn {
		st.State = StateDrawn
		if err := s.save(st); err != nil {
			return bundle.Bundle{}, err
		}
		var payload hashing.Digest
		copy(payload[:], b.Result.Seed[:])
		if _, err := s.log.Append(translog.KindResultPublished, id, payload, s.now().Unix()); err != nil {
			return bundle.Bundle{}, err
		}
	}
	// Re-read the log so the bundle carries the result record too.
	return s.assemble(st, r)
}

func (s *Service) assemble(st *Stored, r beacon.Round) (bundle.Bundle, error) {
	rnd, err := r.Randomness()
	if err != nil {
		return bundle.Bundle{}, err
	}
	rc := *st.RosterCommit
	seed := draw.Seed(rc.Root, rc.Size, rnd, st.Params.DrawID)
	won, wait, err := draw.Select(seed, rc.Size, st.Params.Winners)
	if err != nil {
		return bundle.Bundle{}, err
	}
	ros, err := roster.Load(st.Sealed)
	if err != nil {
		return bundle.Bundle{}, err
	}

	winners := make([]bundle.Winner, len(won))
	for rank, idx := range won {
		sealed, path, err := ros.Open(idx)
		if err != nil {
			return bundle.Bundle{}, err
		}
		winners[rank] = bundle.Winner{
			Rank: rank + 1, Index: idx, Salt: hex.EncodeToString(sealed.Salt),
			Entry: sealed.Entry, MerklePath: path,
		}
	}

	rref := bundle.RosterRef{Commitment: rc, Signature: st.RosterSig, Leaves: ros.Leaves()}
	if st.Params.Disclosure == roster.Public {
		rref.Entries = st.Sealed
	}

	return bundle.Bundle{
		Version:  1,
		Operator: hex.EncodeToString(s.pub),
		Draw:     bundle.DrawRef{Params: st.Params, Signature: st.ParamsSig},
		Roster:   rref,
		Beacon:   r,
		Result:   bundle.Result{Seed: seed, Winners: winners, Waitlist: wait},
		Log: bundle.LogRef{
			LogID:   s.logID,
			Records: s.log.Records(),
			Head:    s.log.Head(),
		},
	}, nil
}

// Abandoned lists draws whose target round has already published but whose
// result never was.
//
// The distinction from translog.Unresolved matters: a draw that has not
// reached its round yet is simply pending, and calling it abandoned would cry
// wolf on every draw in flight. Once the round has published, though, the
// randomness is public and the organiser has everything needed to publish a
// result — so silence from that point on is the shape a retry attack leaves,
// and is worth surfacing.
//
// It is still not proof of misconduct. Draws are cancelled for honest reasons.
// It is the one place the attack can show up at all.
func (s *Service) Abandoned() ([]string, error) {
	now := s.now()
	var out []string
	for _, id := range s.log.Unresolved(now.Unix()) {
		st, err := s.load(id)
		if err != nil {
			continue // a log record with no stored draw is someone else's problem
		}
		if !now.Before(s.chain.TimeOf(st.Params.TargetRound)) {
			out = append(out, id)
		}
	}
	return out, nil
}

// Get returns a stored draw.
func (s *Service) Get(id string) (*Stored, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load(id)
}

// List returns every draw id known to this service.
func (s *Service) List() ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(s.dir, "draws"))
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".json" {
			out = append(out, e.Name()[:len(e.Name())-5])
		}
	}
	return out, nil
}

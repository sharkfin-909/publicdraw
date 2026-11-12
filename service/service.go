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
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sharkfin-909/publicdraw/beacon"
	"github.com/sharkfin-909/publicdraw/bundle"
	"github.com/sharkfin-909/publicdraw/draw"
	"github.com/sharkfin-909/publicdraw/hashing"
	"github.com/sharkfin-909/publicdraw/rekor"
	"github.com/sharkfin-909/publicdraw/roster"
	"github.com/sharkfin-909/publicdraw/translog"
)

// State is where a draw has reached in its lifecycle.
type State string

const (
	StateOpen      State = "open"      // accepting entries
	StateCommitted State = "committed" // roster frozen, waiting for the round
	StateDrawn     State = "drawn"     // result published
)

// Visibility controls whether a draw is listed publicly and offered to search
// engines. It is deliberately NOT part of draw.Params and never enters the
// signed commitment: it is presentation, not protocol. A draw's bundle must
// verify identically whether or not anyone chose to publicise it, and
// changing visibility afterwards must not invalidate a signature.
//
// Unlisted is the default, and the default matters. Most draws worth running
// involve housing applicants, grant seekers or ballot entrants, and a page
// naming them has no business in a search index because nobody remembered to
// opt out.
type Visibility string

const (
	Unlisted Visibility = "unlisted"
	Listed   Visibility = "public"
)

// Valid reports whether v is a known visibility.
func (v Visibility) Valid() bool { return v == Unlisted || v == Listed }

// Stored is the on-disk record of a draw.
type Stored struct {
	Params       draw.Params            `json:"params"`
	ParamsSig    string                 `json:"params_sig"`
	State        State                  `json:"state"`
	Sealed       []roster.Sealed        `json:"sealed,omitempty"`
	RosterCommit *draw.RosterCommitment `json:"roster_commit,omitempty"`
	RosterSig    string                 `json:"roster_sig,omitempty"`

	// Visibility is presentation, not protocol — see the Visibility type.
	Visibility Visibility `json:"visibility,omitempty"`

	// AnchoredHead is the log head that was published to Rekor for this draw,
	// recorded so the bundle can carry the matching inclusion proof.
	AnchoredHead string `json:"anchored_head,omitempty"`

	// Pending is a log record this state change has committed to but not yet
	// confirmed in the log — see PendingRecord.
	Pending *PendingRecord `json:"pending_record,omitempty"`
}

// Visible returns the effective visibility, treating an absent value as
// unlisted so that draws stored before this field existed stay out of the
// index rather than silently opting in.
func (s *Stored) Visible() Visibility {
	if s.Visibility == Listed {
		return Listed
	}
	return Unlisted
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

	anchor Anchorer // nil when anchoring is not configured
	lock   *os.File // held for the service's lifetime; see ErrLocked
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

// New opens (or creates) a service rooted at dir, and holds it exclusively
// until Close: a second process writing the same directory would keep its own
// log head and fork the chain. Any state change a crash left half-logged is
// finished before New returns.
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
	lock, err := lockDir(dir)
	if err != nil {
		return nil, err
	}
	s.lock = lock
	l, err := translog.Open(filepath.Join(dir, "log.jsonl"), logID)
	if err != nil {
		lock.Close()
		return nil, err
	}
	s.log = l
	if err := s.recover(); err != nil {
		lock.Close()
		return nil, err
	}
	return s, nil
}

// Close releases the service directory for another process.
func (s *Service) Close() error { return s.lock.Close() }

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
		return nil, notFound(id)
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
	if _, err := s.transition(st, translog.KindDrawCreated, p.Commit(), now.Unix()); err != nil {
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

	st, err := s.loadSettled(id)
	if err != nil {
		return nil, err
	}
	if st.State != StateOpen {
		return nil, refuse("service: draw %s is %s, not open", id, st.State)
	}
	if s.now().Unix() >= st.Params.CutoffAt {
		return nil, refuse("service: entries for %s closed at %d", id, st.Params.CutoffAt)
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

// Proof returns the public half of a receipt for one index: the root, the
// size and the Merkle path, with no salt and no label. It is what the HTTP
// API serves, because everything omitted here is already held by the entrant
// and serving it back would disclose the roster to anyone who asked.
func (s *Service) Proof(id string, index int) (roster.Proof, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	st, err := s.load(id)
	if err != nil {
		return roster.Proof{}, err
	}
	if st.RosterCommit == nil {
		return roster.Proof{}, refuse("service: draw %s has no committed roster yet", id)
	}
	r, err := roster.Load(st.Sealed)
	if err != nil {
		return roster.Proof{}, err
	}
	_, path, err := r.Open(index)
	if err != nil {
		return roster.Proof{}, err
	}
	return roster.Proof{
		DrawID: id, Root: st.RosterCommit.Root, Size: st.RosterCommit.Size,
		Index: index, MerklePath: path,
	}, nil
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
		return roster.Receipt{}, refuse("service: draw %s has no committed roster yet", id)
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

	st, err := s.loadSettled(id)
	if err != nil {
		return bundle.Bundle{}, err
	}
	if st.State == StateOpen {
		return bundle.Bundle{}, refuse("service: draw %s has no committed roster", id)
	}
	target := s.chain.TimeOf(st.Params.TargetRound)
	if now := s.now(); now.Before(target) {
		return bundle.Bundle{}, refuse("service: round %d does not publish for another %s",
			st.Params.TargetRound, target.Sub(now).Truncate(time.Second))
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
		var payload hashing.Digest
		copy(payload[:], b.Result.Seed[:])
		if _, err := s.transition(st, translog.KindResultPublished, payload, s.now().Unix()); err != nil {
			return bundle.Bundle{}, err
		}
	}
	// Re-read the log so the bundle carries the result record too.
	return s.assemble(st, r)
}

// Bundle returns the published bundle of a drawn draw. Unlike Run it changes
// nothing, so it is safe to serve to anyone.
func (s *Service) Bundle(ctx context.Context, id string) (bundle.Bundle, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	st, err := s.load(id)
	if err != nil {
		return bundle.Bundle{}, err
	}
	if st.State != StateDrawn {
		return bundle.Bundle{}, refuse("service: draw %s has not been drawn", id)
	}
	r, err := s.fetch(ctx, st.Params.TargetRound)
	if err != nil {
		return bundle.Bundle{}, err
	}
	return s.assemble(st, r)
}

func (s *Service) assemble(st *Stored, r beacon.Round) (bundle.Bundle, error) {
	src := bundle.Source{
		Operator: hex.EncodeToString(s.pub),
		Params:   st.Params, ParamsSig: st.ParamsSig,
		Sealed: st.Sealed, Commit: *st.RosterCommit, RosterSig: st.RosterSig,
		Round: r,
		LogID: s.logID, Records: s.log.Records(), Head: s.log.Head(),
	}
	if st.AnchoredHead != "" {
		if h, err := hashing.ParseDigest(st.AnchoredHead); err == nil {
			src.Anchor = s.AnchorFor(h)
		}
	}
	return bundle.Assemble(src)
}

func (s *Service) anchorPath() string { return filepath.Join(s.dir, "anchors.json") }

func (s *Service) loadAnchors() (map[string]*rekor.Anchor, error) {
	out := map[string]*rekor.Anchor{}
	b, err := os.ReadFile(s.anchorPath())
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// AnchorFor returns the stored anchor for a log head, if one exists.
func (s *Service) AnchorFor(head hashing.Digest) *rekor.Anchor {
	anchors, err := s.loadAnchors()
	if err != nil {
		return nil
	}
	return anchors[head.String()]
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

// SetVisibility marks a draw listed or unlisted. It touches no signed material.
func (s *Service) SetVisibility(id string, v Visibility) (*Stored, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !v.Valid() {
		return nil, refuse("service: unknown visibility %q", v)
	}
	st, err := s.loadSettled(id)
	if err != nil {
		return nil, err
	}
	st.Visibility = v
	if err := s.save(st); err != nil {
		return nil, err
	}
	return st, nil
}

// ListPublic returns only the draws their organiser chose to publicise. It is
// what the sitemap is built from, so that opting in is the single gate.
func (s *Service) ListPublic() ([]*Stored, error) {
	ids, err := s.List()
	if err != nil {
		return nil, err
	}
	var out []*Stored
	for _, id := range ids {
		st, err := s.Get(id)
		if err != nil {
			continue
		}
		if st.Visible() == Listed {
			out = append(out, st)
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

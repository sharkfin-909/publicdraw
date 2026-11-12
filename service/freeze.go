package service

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/sharkfin-909/publicdraw/draw"
	"github.com/sharkfin-909/publicdraw/hashing"
	"github.com/sharkfin-909/publicdraw/rekor"
	"github.com/sharkfin-909/publicdraw/roster"
	"github.com/sharkfin-909/publicdraw/translog"
)

// Anchorer publishes a transparency-log head somewhere the organiser does not
// control and returns proof that it landed there. It is the seam between the
// freeze and Sigstore's Rekor: production uses Rekor (WithRekor), tests a fake
// (WithAnchorer). An Anchorer must only return an anchor it has verified.
type Anchorer func(ctx context.Context, head hashing.Digest) (*rekor.Anchor, error)

// WithRekor anchors each frozen draw's log head into Sigstore's Rekor.
//
// Without it the local log is tamper-evident only against someone who already
// saw an earlier head. With it, each head lands in an append-only log the
// organiser does not control, so a draw that was committed and then quietly
// abandoned cannot be made to disappear.
func WithRekor(c *rekor.Client, key *ecdsa.PrivateKey) Option {
	return WithAnchorer(func(ctx context.Context, head hashing.Digest) (*rekor.Anchor, error) {
		a, err := c.Submit(ctx, head[:], key)
		if err != nil {
			return nil, err
		}
		if err := a.VerifyInclusion(); err != nil {
			return nil, fmt.Errorf("service: rekor returned a proof that does not verify: %w", err)
		}
		return a, nil
	})
}

// WithAnchorer replaces where log heads are anchored.
func WithAnchorer(a Anchorer) Option { return func(s *Service) { s.anchor = a } }

// AnchorsEnabled reports whether this service anchors frozen draws.
func (s *Service) AnchorsEnabled() bool { return s.anchor != nil }

// anchorTimeout bounds one anchoring attempt. The window between the cutoff and
// the target round is about fifteen seconds, and an anchor witnessed after the
// round is worth nothing to a verifier, so there is no point waiting longer.
const anchorTimeout = 30 * time.Second

// Frozen is the outcome of freezing one draw.
//
// Anchoring can fail without the freeze failing: a draw with no anchor is still
// a valid draw, it just rests more on the organiser's own word, and
// verification says so. AnchorErr reports that case; it is nil when anchoring
// succeeded or is not configured.
type Frozen struct {
	Draw      *Stored
	Head      hashing.Digest // the log head right after the roster record
	Anchor    *rekor.Anchor
	AnchorErr error
}

// Freeze commits a draw's entry list and anchors the log head that records it.
//
// This is the one way a roster is frozen — the CLI, the HTTP API and the
// background freezer all come here — so every frozen draw gets the same
// treatment. The commit must happen at or after the cutoff and before the
// target round publishes; both bounds are enforced here and re-checked by any
// verifier.
func (s *Service) Freeze(ctx context.Context, id string) (Frozen, error) {
	f, err := s.commit(id)
	if err != nil {
		return Frozen{}, err
	}
	s.anchorFrozen(ctx, &f)
	return f, nil
}

// Reanchor anchors a frozen draw's commit head again — for when Rekor was
// unreachable at the freeze. It anchors the head the log had right after this
// draw's roster record, not the current head, so the anchor still says what the
// roster was. A verifier credits it only if Rekor's witnesses saw it before the
// target round.
func (s *Service) Reanchor(ctx context.Context, id string) (Frozen, error) {
	s.mu.Lock()
	st, err := s.loadSettled(id)
	if err != nil {
		s.mu.Unlock()
		return Frozen{}, err
	}
	rec, ok := s.log.Find(id, translog.KindRosterCommitted)
	s.mu.Unlock()
	if !ok {
		return Frozen{}, refuse("service: draw %s has no committed roster to anchor", id)
	}
	if !s.AnchorsEnabled() {
		return Frozen{}, errors.New("service: anchoring is not configured")
	}
	if err := s.writable(); err != nil {
		return Frozen{}, err
	}
	f := Frozen{Draw: st, Head: rec.Head}
	s.anchorFrozen(ctx, &f)
	return f, f.AnchorErr
}

// commit freezes the roster under the lock and logs it. It holds no network
// call: anchoring happens afterwards, outside the lock.
func (s *Service) commit(id string) (Frozen, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	st, err := s.loadSettled(id)
	if err != nil {
		return Frozen{}, err
	}
	if st.State != StateOpen {
		return Frozen{}, refuse("service: draw %s is already %s", id, st.State)
	}
	now := s.now()
	if now.Unix() < st.Params.CutoffAt {
		return Frozen{}, refuse("service: entries for %s are open until %d", id, st.Params.CutoffAt)
	}
	if len(st.Sealed) < st.Params.Winners {
		return Frozen{}, refuse("service: %d entries cannot yield %d winners", len(st.Sealed), st.Params.Winners)
	}
	if now.Unix() >= s.chain.TimeOf(st.Params.TargetRound).Unix() {
		return Frozen{}, refuse("service: round %d has already published; this draw can no longer be committed honestly", st.Params.TargetRound)
	}

	// Entries were salted on arrival, so freezing is only a matter of
	// committing to the order they already sit in.
	r, err := roster.Load(st.Sealed)
	if err != nil {
		return Frozen{}, err
	}
	rc, sig, err := draw.CommitRoster(st.Params, r.Commitment(), s.priv, now)
	if err != nil {
		return Frozen{}, err
	}
	st.RosterCommit = &rc
	st.RosterSig = hex.EncodeToString(sig)
	st.State = StateCommitted

	var payload hashing.Digest
	copy(payload[:], rc.SigningBytes())
	rec, err := s.transition(st, translog.KindRosterCommitted, payload, now.Unix())
	if err != nil {
		return Frozen{}, err
	}
	return Frozen{Draw: st, Head: rec.Head}, nil
}

// anchorFrozen anchors f.Head and records it against the draw, filling in
// f.Anchor or f.AnchorErr. It is a no-op when anchoring is not configured.
func (s *Service) anchorFrozen(ctx context.Context, f *Frozen) {
	if s.anchor == nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, anchorTimeout)
	defer cancel()
	a, err := s.anchor(ctx, f.Head)
	if err != nil {
		f.AnchorErr = err
		return
	}
	st, err := s.recordAnchor(f.Draw.Params.DrawID, f.Head, a)
	if err != nil {
		f.AnchorErr = err
		return
	}
	f.Draw, f.Anchor = st, a
}

// recordAnchor keeps the anchor's inclusion proof and marks which head was
// anchored for the draw, so its bundle carries the matching proof.
func (s *Service) recordAnchor(id string, head hashing.Digest, a *rekor.Anchor) (*Stored, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	anchors, err := s.loadAnchors()
	if err != nil {
		return nil, err
	}
	anchors[head.String()] = a
	b, err := json.MarshalIndent(anchors, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(s.anchorPath(), append(b, '\n'), 0o644); err != nil {
		return nil, err
	}
	st, err := s.loadSettled(id)
	if err != nil {
		return nil, err
	}
	st.AnchoredHead = head.String()
	return st, s.save(st)
}

// FreezeDue freezes every open draw whose cutoff has passed. The window before
// the target round is only seconds long, so this cannot wait for a person.
//
// Every due draw is committed before any is anchored: anchoring is a network
// call that can take seconds, and one slow anchor must not eat another draw's
// window. A draw the service refuses to freeze — short of entries, past its
// round, or frozen by someone else meanwhile — is skipped, not reported: it
// stays open, and shows up as abandoned once its round has published.
func (s *Service) FreezeDue(ctx context.Context) ([]Frozen, error) {
	ids, err := s.List()
	if err != nil {
		return nil, err
	}
	var frozen []Frozen
	var errs []error
	for _, id := range ids {
		st, err := s.Get(id)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if st.State != StateOpen || s.now().Unix() < st.Params.CutoffAt {
			continue
		}
		f, err := s.commit(id)
		if errors.Is(err, ErrRefused) {
			continue
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("freeze %s: %w", id, err))
			continue
		}
		frozen = append(frozen, f)
	}
	for i := range frozen {
		s.anchorFrozen(ctx, &frozen[i])
	}
	return frozen, errors.Join(errs...)
}

// KeepFreezing runs FreezeDue every interval until ctx is cancelled, reporting
// each round that froze something or failed. It is what `serve` runs alongside
// the HTTP server; it lives here, rather than in main, so it can be started,
// stopped and tested like the rest of the lifecycle.
func (s *Service) KeepFreezing(ctx context.Context, every time.Duration, report func([]Frozen, error)) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			frozen, err := s.FreezeDue(ctx)
			if report != nil && (len(frozen) > 0 || err != nil) {
				report(frozen, err)
			}
		}
	}
}

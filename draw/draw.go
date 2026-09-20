// Package draw implements the two-phase commitment and the selection itself.
//
// There are two commitments, made at different times, because they answer
// different objections:
//
//	create ──── entries accumulate ──── CUTOFF ──── target round ──── reveal
//	  │                                   │                             │
//	  └ params committed                  └ roster committed            └ winners derived
//
// Committing the *parameters* at creation stops an organiser choosing the
// cutoff moment after watching who entered. Committing the *roster* at cutoff
// is unavoidable — the list does not exist until then — and committing it
// before the target round publishes is what stops the roster being tuned to a
// known beacon value.
package draw

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/sharkfin009/verifiable-challenge/beacon"
	"github.com/sharkfin009/verifiable-challenge/hashing"
	"github.com/sharkfin009/verifiable-challenge/roster"
)

// MinLead is the smallest gap allowed between the cutoff and the target round.
// It must be large enough that the target round is unambiguously unpublished
// when the roster is committed.
const MinLead = 15 * time.Second

// Params are fixed when the draw is created, before anybody enters.
type Params struct {
	DrawID      string            `json:"draw_id"`
	Title       string            `json:"title"`
	Winners     int               `json:"winners"`
	Disclosure  roster.Disclosure `json:"disclosure"`
	CutoffAt    int64             `json:"cutoff_at"`
	TargetRound uint64            `json:"target_round"`
	ChainHash   string            `json:"chain_hash"`
	CreatedAt   int64             `json:"created_at"`
}

// SigningBytes is the exact preimage the operator signs at creation.
func (p Params) SigningBytes() []byte {
	d := hashing.Tagged(hashing.TagDraw,
		[]byte(p.DrawID),
		[]byte(p.Title),
		hashing.U32(uint32(p.Winners)),
		[]byte(p.Disclosure),
		hashing.U64(uint64(p.CutoffAt)),
		hashing.U64(p.TargetRound),
		[]byte(p.ChainHash),
		hashing.U64(uint64(p.CreatedAt)),
	)
	return d[:]
}

// Commit is the digest of the parameters, used as this draw's identity in the
// transparency log.
func (p Params) Commit() hashing.Digest {
	var d hashing.Digest
	copy(d[:], p.SigningBytes())
	return d
}

// NewID returns a fresh random draw identifier.
func NewID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Create fixes the draw parameters and signs them.
func Create(chain beacon.Chain, title string, winners int, disc roster.Disclosure, cutoff time.Time, priv ed25519.PrivateKey, now time.Time) (Params, []byte, error) {
	if winners < 1 {
		return Params{}, nil, errors.New("draw: winners must be at least 1")
	}
	if !disc.Valid() {
		return Params{}, nil, fmt.Errorf("draw: unknown disclosure mode %q", disc)
	}
	if !cutoff.After(now) {
		return Params{}, nil, errors.New("draw: cutoff must be in the future")
	}
	id, err := NewID()
	if err != nil {
		return Params{}, nil, err
	}
	// The target round is derived from the cutoff, not from the moment of
	// creation, so the gap that matters is cutoff -> round.
	target := chain.RoundAt(cutoff.Add(MinLead)) + 1
	p := Params{
		DrawID: id, Title: title, Winners: winners, Disclosure: disc,
		CutoffAt: cutoff.Unix(), TargetRound: target,
		ChainHash: chain.Hash, CreatedAt: now.Unix(),
	}
	return p, ed25519.Sign(priv, p.SigningBytes()), nil
}

// RosterCommitment is signed at cutoff, once the entry list is frozen.
type RosterCommitment struct {
	DrawID      string         `json:"draw_id"`
	Root        hashing.Digest `json:"root"`
	Size        int            `json:"size"`
	Commit      hashing.Digest `json:"commit"`
	CommittedAt int64          `json:"committed_at"`
}

// SigningBytes is the exact preimage the operator signs at cutoff.
func (rc RosterCommitment) SigningBytes() []byte {
	d := hashing.Tagged(hashing.TagRosterSig,
		[]byte(rc.DrawID),
		rc.Root[:],
		hashing.U32(uint32(rc.Size)),
		rc.Commit[:],
		hashing.U64(uint64(rc.CommittedAt)),
	)
	return d[:]
}

// CommitRoster freezes a roster against a draw and signs it.
func CommitRoster(p Params, c roster.Commitment, priv ed25519.PrivateKey, now time.Time) (RosterCommitment, []byte, error) {
	if now.Unix() < p.CutoffAt {
		return RosterCommitment{}, nil, fmt.Errorf("draw: cutoff is at %d, it is %d", p.CutoffAt, now.Unix())
	}
	rc := RosterCommitment{
		DrawID: p.DrawID, Root: c.Root, Size: c.Size, Commit: c.Commit, CommittedAt: now.Unix(),
	}
	return rc, ed25519.Sign(priv, rc.SigningBytes()), nil
}

// Seed derives this draw's selection seed from the beacon.
//
// Binding the roster root means a different entry list cannot reuse a
// favourable beacon value; binding the draw id separates draws that land on
// the same round.
func Seed(root hashing.Digest, size int, randomness hashing.Digest, drawID string) hashing.Digest {
	return hashing.Tagged(hashing.TagSeed,
		root[:], hashing.U32(uint32(size)), randomness[:], []byte(drawID))
}

// Ticket is one entry's sort key.
type Ticket struct {
	Index int            `json:"index"`
	Value hashing.Digest `json:"value"`
}

// TicketFor computes the ticket for an entry index.
func TicketFor(seed hashing.Digest, index int) hashing.Digest {
	return hashing.Tagged(hashing.TagTicket, seed[:], hashing.U32(uint32(index)))
}

// Rank orders every entry by ticket, ascending.
//
// Sorting by a keyed hash rather than drawing indices one at a time avoids a
// rejection loop, needs no specified PRNG for another implementation to
// reproduce, and yields a full ordering rather than just k winners — so the
// places after the winners are a waitlist, which allocation draws always need.
//
// Ties break on index. SHA-256 will not collide here, but leaving it
// unspecified would leave two implementations free to disagree.
func Rank(seed hashing.Digest, size int) []Ticket {
	tickets := make([]Ticket, size)
	for i := 0; i < size; i++ {
		tickets[i] = Ticket{Index: i, Value: TicketFor(seed, i)}
	}
	sort.Slice(tickets, func(a, b int) bool {
		for k := 0; k < hashing.Size; k++ {
			if tickets[a].Value[k] != tickets[b].Value[k] {
				return tickets[a].Value[k] < tickets[b].Value[k]
			}
		}
		return tickets[a].Index < tickets[b].Index
	})
	return tickets
}

// Select returns the winning indices in rank order, and the waitlist that
// follows them.
func Select(seed hashing.Digest, size, winners int) (won []int, waitlist []int, err error) {
	if size <= 0 {
		return nil, nil, errors.New("draw: empty roster")
	}
	if winners < 1 {
		return nil, nil, errors.New("draw: winners must be at least 1")
	}
	if winners > size {
		return nil, nil, fmt.Errorf("draw: %d winners requested from %d entries", winners, size)
	}
	ranked := Rank(seed, size)
	for i, t := range ranked {
		if i < winners {
			won = append(won, t.Index)
		} else {
			waitlist = append(waitlist, t.Index)
		}
	}
	return won, waitlist, nil
}

// VerifyParamsSignature checks the operator's signature over draw parameters.
func VerifyParamsSignature(pub ed25519.PublicKey, p Params, sig []byte) error {
	if len(pub) != ed25519.PublicKeySize {
		return errors.New("draw: bad public key size")
	}
	if !ed25519.Verify(pub, p.SigningBytes(), sig) {
		return errors.New("draw: parameter signature does not verify")
	}
	return nil
}

// VerifyRosterSignature checks the operator's signature over a roster commitment.
func VerifyRosterSignature(pub ed25519.PublicKey, rc RosterCommitment, sig []byte) error {
	if len(pub) != ed25519.PublicKeySize {
		return errors.New("draw: bad public key size")
	}
	if !ed25519.Verify(pub, rc.SigningBytes(), sig) {
		return errors.New("draw: roster signature does not verify")
	}
	return nil
}

// GenerateKey returns a fresh operator signing keypair.
func GenerateKey() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("draw: keygen: %w", err)
	}
	return pub, priv, nil
}

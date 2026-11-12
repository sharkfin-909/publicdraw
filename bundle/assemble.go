package bundle

import (
	"encoding/hex"

	"github.com/sharkfin-909/publicdraw/beacon"
	"github.com/sharkfin-909/publicdraw/draw"
	"github.com/sharkfin-909/publicdraw/hashing"
	"github.com/sharkfin-909/publicdraw/rekor"
	"github.com/sharkfin-909/publicdraw/roster"
	"github.com/sharkfin-909/publicdraw/translog"
)

// Source is everything a bundle is built from: a frozen draw as stored, the
// round that decides it, and the log as it stands.
type Source struct {
	Operator  string // hex Ed25519 public key
	Params    draw.Params
	ParamsSig string
	Sealed    []roster.Sealed
	Commit    draw.RosterCommitment
	RosterSig string
	Round     beacon.Round
	LogID     string
	Records   []translog.Record
	Head      hashing.Digest
	Anchor    *rekor.Anchor // nil when the commit head was never anchored
}

// Assemble builds the published bundle. It is the counterpart of Verify and
// lives beside it: what a bundle must contain — the leaves always, the entries
// only for a public draw, a proof for each winner — is decided here, in a pure
// function with no storage behind it, so the rules can be tested next to the
// checks that hold a bundle to them.
func Assemble(s Source) (Bundle, error) {
	rnd, err := s.Round.Randomness()
	if err != nil {
		return Bundle{}, err
	}
	rc := s.Commit
	seed := draw.Seed(rc.Root, rc.Size, rnd, s.Params.DrawID)
	won, wait, err := draw.Select(seed, rc.Size, s.Params.Winners)
	if err != nil {
		return Bundle{}, err
	}
	ros, err := roster.Load(s.Sealed)
	if err != nil {
		return Bundle{}, err
	}

	winners := make([]Winner, len(won))
	for rank, idx := range won {
		sealed, path, err := ros.Open(idx)
		if err != nil {
			return Bundle{}, err
		}
		winners[rank] = Winner{
			Rank: rank + 1, Index: idx, Salt: hex.EncodeToString(sealed.Salt),
			Entry: sealed.Entry, MerklePath: path,
		}
	}

	rref := RosterRef{Commitment: rc, Signature: s.RosterSig, Leaves: ros.Leaves(), RekorAnchor: s.Anchor}
	if s.Params.Disclosure == roster.Public {
		rref.Entries = s.Sealed
	}

	return Bundle{
		Version:  1,
		Operator: s.Operator,
		Draw:     DrawRef{Params: s.Params, Signature: s.ParamsSig},
		Roster:   rref,
		Beacon:   s.Round,
		Result:   Result{Seed: seed, Winners: winners, Waitlist: wait},
		Log:      LogRef{LogID: s.LogID, Records: s.Records, Head: s.Head},
	}, nil
}

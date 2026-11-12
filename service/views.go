package service

import (
	"time"

	"github.com/sharkfin-909/publicdraw/draw"
	"github.com/sharkfin-909/publicdraw/hashing"
	"github.com/sharkfin-909/publicdraw/roster"
)

// DrawView is what anyone may see of a draw. It never carries the sealed
// entries or their salts: those are the roster's hiding, and handing them out
// would make private disclosure a lie.
//
// Whether entry labels may be shown is a protocol rule, so it is decided here
// once, for the HTTP API and the CLI alike: a public draw's labels become
// visible once its roster is frozen, and a private draw's never do.
type DrawView struct {
	Params       draw.Params            `json:"params"`
	ParamsSig    string                 `json:"params_sig"`
	State        State                  `json:"state"`
	EntryCount   int                    `json:"entry_count"`
	Visibility   Visibility             `json:"visibility"`
	RosterCommit *draw.RosterCommitment `json:"roster_commit,omitempty"`
	RosterSig    string                 `json:"roster_sig,omitempty"`
	Labels       []string               `json:"labels,omitempty"`
}

// View returns the public view of a draw.
func (s *Service) View(id string) (DrawView, error) {
	st, err := s.Get(id)
	if err != nil {
		return DrawView{}, err
	}
	v := DrawView{
		Params: st.Params, ParamsSig: st.ParamsSig, State: st.State,
		EntryCount: len(st.Sealed), Visibility: st.Visible(),
		RosterCommit: st.RosterCommit, RosterSig: st.RosterSig,
	}
	if st.Params.Disclosure == roster.Public && st.State != StateOpen {
		v.Labels = make([]string, len(st.Sealed))
		for i, e := range st.Sealed {
			v.Labels[i] = e.Entry.Label
		}
	}
	return v, nil
}

// LeafList is every committed leaf of a draw, in roster order. Safe to publish
// in either disclosure mode: a leaf hides its label behind a salt only the
// entrant holds, and an entrant with that salt needs nothing else to find
// themselves in it.
type LeafList struct {
	Root   hashing.Digest   `json:"root"`
	Size   int              `json:"size"`
	Leaves []hashing.Digest `json:"leaves"`
}

// Leaves returns a frozen draw's committed leaves.
func (s *Service) Leaves(id string) (LeafList, error) {
	st, err := s.Get(id)
	if err != nil {
		return LeafList{}, err
	}
	if st.RosterCommit == nil {
		return LeafList{}, refuse("service: draw %s has no committed roster yet", id)
	}
	ros, err := roster.Load(st.Sealed)
	if err != nil {
		return LeafList{}, err
	}
	return LeafList{Root: st.RosterCommit.Root, Size: st.RosterCommit.Size, Leaves: ros.Leaves()}, nil
}

// CreateOpenFor creates a draw whose entries stay open for d from now, by the
// service's own clock — so a draw opened "for ten minutes" means the same thing
// whether it was created over HTTP, from the CLI, or in a test.
func (s *Service) CreateOpenFor(title string, winners int, disc roster.Disclosure, d time.Duration) (*Stored, error) {
	if d <= 0 {
		return nil, refuse("service: a draw must stay open for some time, not %s", d)
	}
	return s.Create(title, winners, disc, s.now().Add(d))
}

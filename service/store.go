package service

import (
	"github.com/sharkfin-909/publicdraw/hashing"
	"github.com/sharkfin-909/publicdraw/translog"
)

// PendingRecord is a log record that a saved state change has committed to,
// but that has not yet been confirmed in the log.
//
// A state change and the record announcing it live in two files, and either
// write can fail. Saving the draw first and appending second used to leave a
// draw marked drawn with no result in the log if the append failed — and every
// retry then skipped the append, because the draw already looked drawn.
// Appending first is no better for the roster commit, whose payload carries
// the moment it was signed: a retry would sign a different payload from the
// one already logged.
//
// So the record rides along with the state change. It is saved with the draw,
// then appended, then cleared; and settle finishes the job, exactly once, from
// wherever a failure left off.
type PendingRecord struct {
	Kind     translog.Kind  `json:"kind"`
	Payload  hashing.Digest `json:"payload"`
	LoggedAt int64          `json:"logged_at"`
}

// transition saves st together with the log record that announces its new
// state, and returns that record. st must already carry the new state.
// Callers hold s.mu.
func (s *Service) transition(st *Stored, kind translog.Kind, payload hashing.Digest, at int64) (translog.Record, error) {
	st.Pending = &PendingRecord{Kind: kind, Payload: payload, LoggedAt: at}
	if err := s.save(st); err != nil {
		return translog.Record{}, err
	}
	return s.settle(st)
}

// settle makes sure a draw's pending record is in the log, appending it if a
// failure stopped it getting there, and then clears it. A draw with nothing
// pending is left alone. Callers hold s.mu.
func (s *Service) settle(st *Stored) (translog.Record, error) {
	p := st.Pending
	if p == nil {
		return translog.Record{}, nil
	}
	rec, ok := s.log.Find(st.Params.DrawID, p.Kind)
	switch {
	case ok && rec.Payload != p.Payload:
		// The log already says something else happened. Appending a second,
		// different record would be worse than stopping here.
		return translog.Record{}, refuse("service: draw %s: log holds a different %s record than the one pending",
			st.Params.DrawID, p.Kind)
	case !ok:
		r, err := s.log.Append(p.Kind, st.Params.DrawID, p.Payload, p.LoggedAt)
		if err != nil {
			return translog.Record{}, err
		}
		rec = r
	}
	st.Pending = nil
	return rec, s.save(st)
}

// loadSettled loads a draw for a change, first finishing any record a previous
// failure left pending, so no change is ever made on top of an unlogged one.
// Callers hold s.mu.
func (s *Service) loadSettled(id string) (*Stored, error) {
	st, err := s.load(id)
	if err != nil {
		return nil, err
	}
	if _, err := s.settle(st); err != nil {
		return nil, err
	}
	return st, nil
}

// recover settles every draw at startup, so a crash between saving a state
// change and logging it is repaired before the service serves anything.
func (s *Service) recover() error {
	ids, err := s.List()
	if err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := s.loadSettled(id); err != nil {
			return err
		}
	}
	return nil
}

package service

import (
	"errors"
	"fmt"
)

// Errors a caller can act on. Everything else the service returns is a fault —
// a disk, network or encoding failure — and should be reported as one, not
// mistaken for something the caller did.
var (
	// ErrNotFound: there is no draw with that id.
	ErrNotFound = errors.New("service: no such draw")

	// ErrRefused: the request breaks the draw's ordering — entering after the
	// cutoff, freezing twice, drawing before the round. The caller could have
	// avoided it, and retrying unchanged will not help.
	ErrRefused = errors.New("service: refused")

	// ErrLocked: another process holds this service directory. Two writers
	// would each keep their own idea of the log head and fork the chain.
	ErrLocked = errors.New("service: another publicdraw process is using this directory; " +
		"stop it, or make the change through its HTTP API")
)

// classified carries a human message while matching one of the sentinels above
// under errors.Is, so messages stay specific and callers stay simple.
type classified struct {
	msg  string
	kind error
}

func (c classified) Error() string        { return c.msg }
func (c classified) Is(target error) bool { return target == c.kind }

func refuse(format string, a ...any) error {
	return classified{msg: fmt.Sprintf(format, a...), kind: ErrRefused}
}

func notFound(id string) error {
	return classified{msg: fmt.Sprintf("service: no draw %q", id), kind: ErrNotFound}
}

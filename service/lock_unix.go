//go:build unix

package service

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// lockDir takes an exclusive, non-blocking lock on the service directory for
// as long as the returned file stays open. The kernel drops it if the process
// dies, so a crash never leaves the directory wedged.
func lockDir(dir string) (*os.File, error) {
	f, err := os.OpenFile(filepath.Join(dir, ".lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrLocked
		}
		return nil, err
	}
	return f, nil
}

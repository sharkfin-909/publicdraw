//go:build !unix

package service

import (
	"os"
	"path/filepath"
)

// lockDir on platforms without flock only marks the directory; it cannot stop
// a second writer. Run one publicdraw process per directory there.
func lockDir(dir string) (*os.File, error) {
	return os.OpenFile(filepath.Join(dir, ".lock"), os.O_CREATE|os.O_RDWR, 0o644)
}

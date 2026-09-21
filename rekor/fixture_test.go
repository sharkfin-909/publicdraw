package rekor

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// loadFixture reads the captured anchor from testdata.
func loadFixture() (*Anchor, error) {
	b, err := os.ReadFile(filepath.Join("testdata", "anchor.json"))
	if err != nil {
		return nil, err
	}
	var a Anchor
	if err := json.Unmarshal(b, &a); err != nil {
		return nil, err
	}
	return &a, nil
}

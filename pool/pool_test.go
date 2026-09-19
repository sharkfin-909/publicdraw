package pool

import (
	"bytes"
	"testing"
)

func sample() []Challenge {
	return []Challenge{
		{ID: "c1", Language: "go", Title: "Reverse a list", Prompt: "...", StarterCode: "func f() {}"},
		{ID: "c2", Language: "go", Title: "LRU cache", Prompt: "...", StarterCode: "type C struct{}"},
		{ID: "c3", Language: "ts", Title: "Debounce", Prompt: "...", StarterCode: "export const d = 1"},
	}
}

func TestCanonicalBytesAreDeterministic(t *testing.T) {
	c := sample()[0]
	if !bytes.Equal(c.CanonicalBytes(), c.CanonicalBytes()) {
		t.Fatal("canonical encoding is not stable")
	}
}

// Field contents must not be able to slide between fields — the property that
// length prefixing buys, restated at the challenge level.
func TestFieldBoundariesAreUnambiguous(t *testing.T) {
	a := Challenge{ID: "ab", Language: "cd"}
	b := Challenge{ID: "abc", Language: "d"}
	if bytes.Equal(a.CanonicalBytes(), b.CanonicalBytes()) {
		t.Fatal("challenges with differently split fields encode identically")
	}
}

// Salts make leaves hiding. Without them, anyone with the root could confirm a
// guessed challenge and the pool would burn after one disclosure.
func TestSaltsMakeIdenticalChallengesDistinctLeaves(t *testing.T) {
	c := sample()[0]
	p, err := New([]Challenge{c, c})
	if err != nil {
		t.Fatal(err)
	}
	if p.Entries[0].Leaf() == p.Entries[1].Leaf() {
		t.Fatal("identical challenges produced identical leaves")
	}
	if bytes.Equal(p.Entries[0].Salt, p.Entries[1].Salt) {
		t.Fatal("salts were reused")
	}
}

func TestCommitmentBindsSize(t *testing.T) {
	p, _ := New(sample())
	c := p.Commitment()
	if CommitValue(c.Root, c.Size) != c.Commit {
		t.Fatal("commitment is not H(root, size)")
	}
	if CommitValue(c.Root, c.Size+1) == c.Commit {
		t.Fatal("a re-padded pool size yields the same commitment")
	}
}

func TestLoadReproducesTheSameRoot(t *testing.T) {
	p, _ := New(sample())
	again, err := Load(p.Entries)
	if err != nil {
		t.Fatal(err)
	}
	if again.Commitment() != p.Commitment() {
		t.Fatal("reloading the same entries changed the commitment")
	}
}

func TestOpenProducesAVerifiableProof(t *testing.T) {
	p, _ := New(sample())
	c := p.Commitment()
	for i := range p.Entries {
		e, path, err := p.Open(i)
		if err != nil {
			t.Fatal(err)
		}
		if !verifyEntry(e, i, c.Size, path, c) {
			t.Fatalf("index %d: proof did not verify", i)
		}
	}
}

func TestLoadRejectsBadSalt(t *testing.T) {
	p, _ := New(sample())
	p.Entries[1].Salt = []byte("short")
	if _, err := Load(p.Entries); err == nil {
		t.Fatal("accepted an entry with a truncated salt")
	}
}

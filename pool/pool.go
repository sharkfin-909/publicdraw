// Package pool builds and opens the committed challenge pool.
//
// The pool is fixed once, in advance, and its commitment is published and
// timestamped. Individual challenges are revealed one at a time with a Merkle
// proof, so disclosing the challenge a candidate received says nothing about
// the rest of the pool and the pool stays reusable.
package pool

import (
	"crypto/rand"
	"errors"
	"fmt"

	"github.com/sharkfin009/verifiable-challenge/hashing"
	"github.com/sharkfin009/verifiable-challenge/merkle"
)

// SaltSize is the per-entry salt length. 32 bytes puts brute-force recovery of
// a leaf's contents out of reach even when the challenge text is guessable.
const SaltSize = 32

// Challenge is one problem in the pool. Field order is part of the wire
// format: reordering changes every leaf hash and invalidates the commitment.
type Challenge struct {
	ID          string `json:"id"`
	Language    string `json:"language"`
	Title       string `json:"title"`
	Prompt      string `json:"prompt"`
	StarterCode string `json:"starter_code"`
}

// CanonicalBytes is the deterministic encoding of a challenge. It is a
// length-prefixed concatenation rather than JSON, because two implementations
// in two languages must agree on it byte-for-byte and canonical JSON is a far
// larger surface to get wrong.
func (c Challenge) CanonicalBytes() []byte {
	var out []byte
	for _, f := range []string{c.ID, c.Language, c.Title, c.Prompt, c.StarterCode} {
		out = append(out, hashing.LP([]byte(f))...)
	}
	return out
}

// Entry is a challenge with the salt that hides it in the committed tree.
// Entries are secret until revealed.
type Entry struct {
	Salt      []byte    `json:"salt"`
	Challenge Challenge `json:"challenge"`
}

// Leaf computes this entry's Merkle leaf.
func (e Entry) Leaf() hashing.Digest {
	return merkle.LeafHash(e.Salt, e.Challenge.CanonicalBytes())
}

// Commitment is the public, publishable description of a pool. Commit is the
// value to timestamp; it binds Size as well as Root, so the tree cannot later
// be re-padded to a different size.
type Commitment struct {
	Root   hashing.Digest `json:"root"`
	Size   int            `json:"size"`
	Commit hashing.Digest `json:"commit"`
}

// CommitValue derives the timestampable commitment from a root and size.
func CommitValue(root hashing.Digest, size int) hashing.Digest {
	return hashing.Tagged(hashing.TagCommit, root[:], hashing.U32(uint32(size)))
}

// Pool is the private side: every entry, including salts.
type Pool struct {
	Entries []Entry `json:"entries"`

	tree *merkle.Tree
}

// New salts each challenge with fresh randomness and builds the tree.
func New(challenges []Challenge) (*Pool, error) {
	if len(challenges) == 0 {
		return nil, errors.New("pool: no challenges")
	}
	entries := make([]Entry, len(challenges))
	for i, c := range challenges {
		salt := make([]byte, SaltSize)
		if _, err := rand.Read(salt); err != nil {
			return nil, fmt.Errorf("pool: salt: %w", err)
		}
		entries[i] = Entry{Salt: salt, Challenge: c}
	}
	return Load(entries)
}

// Load rebuilds a pool from existing entries, preserving their salts and order.
func Load(entries []Entry) (*Pool, error) {
	if len(entries) == 0 {
		return nil, errors.New("pool: no entries")
	}
	leaves := make([]hashing.Digest, len(entries))
	for i, e := range entries {
		if len(e.Salt) != SaltSize {
			return nil, fmt.Errorf("pool: entry %d has %d-byte salt, want %d", i, len(e.Salt), SaltSize)
		}
		leaves[i] = e.Leaf()
	}
	t, err := merkle.New(leaves)
	if err != nil {
		return nil, err
	}
	return &Pool{Entries: entries, tree: t}, nil
}

// Commitment returns the pool's public commitment.
func (p *Pool) Commitment() Commitment {
	root := p.tree.Root()
	return Commitment{Root: root, Size: p.tree.Size(), Commit: CommitValue(root, p.tree.Size())}
}

// Size returns the number of challenges.
func (p *Pool) Size() int { return len(p.Entries) }

// Open returns the entry at index along with its Merkle proof.
func (p *Pool) Open(index int) (Entry, []hashing.Digest, error) {
	if index < 0 || index >= len(p.Entries) {
		return Entry{}, nil, errors.New("pool: index out of range")
	}
	proof, err := p.tree.Proof(index)
	if err != nil {
		return Entry{}, nil, err
	}
	return p.Entries[index], proof, nil
}

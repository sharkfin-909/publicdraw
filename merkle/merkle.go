// Package merkle implements a binary Merkle tree with two properties that
// matter for this protocol.
//
// Leaves and internal nodes are hashed under different context tags. Without
// that separation an internal node can be presented as a leaf, which forges
// membership for data that was never committed (the second-preimage attack on
// naive Merkle trees; RFC 6962 solves it the same way).
//
// Odd nodes are promoted unchanged rather than duplicated. Duplicating the
// final node makes distinct leaf sets produce identical roots, which was
// CVE-2012-2459 in Bitcoin.
package merkle

import (
	"errors"

	"github.com/sharkfin-909/publicdraw/hashing"
)

// LeafHash hashes a salted entry into a Merkle leaf. The salt makes leaves
// hiding: without it, anyone holding the root can confirm a guessed challenge
// by brute force, which would burn the pool after a single disclosure.
func LeafHash(salt, entry []byte) hashing.Digest {
	return hashing.Tagged(hashing.TagLeaf, salt, entry)
}

// NodeHash combines two child digests into their parent.
func NodeHash(left, right hashing.Digest) hashing.Digest {
	return hashing.Tagged(hashing.TagNode, left[:], right[:])
}

// Tree is a built Merkle tree retaining every level, so proofs can be
// extracted without recomputation.
type Tree struct {
	levels [][]hashing.Digest
}

// New builds a tree over the given leaves, which must be non-empty.
func New(leaves []hashing.Digest) (*Tree, error) {
	if len(leaves) == 0 {
		return nil, errors.New("merkle: empty leaf set")
	}
	cur := make([]hashing.Digest, len(leaves))
	copy(cur, leaves)
	levels := [][]hashing.Digest{cur}

	for len(cur) > 1 {
		next := make([]hashing.Digest, 0, (len(cur)+1)/2)
		for i := 0; i < len(cur); i += 2 {
			if i+1 < len(cur) {
				next = append(next, NodeHash(cur[i], cur[i+1]))
			} else {
				next = append(next, cur[i]) // promote, never duplicate
			}
		}
		levels = append(levels, next)
		cur = next
	}
	return &Tree{levels: levels}, nil
}

// Root returns the tree root.
func (t *Tree) Root() hashing.Digest { return t.levels[len(t.levels)-1][0] }

// Size returns the number of leaves.
func (t *Tree) Size() int { return len(t.levels[0]) }

// Proof returns the sibling digests needed to recompute the root from the
// leaf at index. Levels where the node was promoted contribute no sibling, so
// the proof length is not always ceil(log2(size)).
func (t *Tree) Proof(index int) ([]hashing.Digest, error) {
	if index < 0 || index >= t.Size() {
		return nil, errors.New("merkle: index out of range")
	}
	var path []hashing.Digest
	i := index
	for level := 0; level < len(t.levels)-1; level++ {
		sib := i ^ 1
		if sib < len(t.levels[level]) {
			path = append(path, t.levels[level][sib])
		}
		i >>= 1
	}
	return path, nil
}

// Verify recomputes a root from a leaf and its proof. size is required
// because it determines each level's width, which determines where nodes were
// promoted and therefore which levels the proof has entries for.
func Verify(leaf hashing.Digest, index, size int, path []hashing.Digest, root hashing.Digest) bool {
	if index < 0 || size <= 0 || index >= size {
		return false
	}
	cur := leaf
	i := index
	width := size
	used := 0

	for width > 1 {
		sib := i ^ 1
		if sib < width {
			if used >= len(path) {
				return false // proof too short
			}
			s := path[used]
			used++
			if i%2 == 0 {
				cur = NodeHash(cur, s)
			} else {
				cur = NodeHash(s, cur)
			}
		}
		i >>= 1
		width = (width + 1) / 2
	}
	if used != len(path) {
		return false // trailing junk in the proof
	}
	return cur == root
}

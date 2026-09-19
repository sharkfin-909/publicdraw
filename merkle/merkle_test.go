package merkle

import (
	"fmt"
	"testing"

	"github.com/sharkfin009/verifiable-challenge/hashing"
)

func leaves(n int) []hashing.Digest {
	out := make([]hashing.Digest, n)
	for i := range out {
		out[i] = hashing.Tagged("test/leaf", []byte(fmt.Sprintf("entry-%d", i)))
	}
	return out
}

// Sizes 1..33 cover every shape of promoted odd node, which is where a
// mismatch between build and verify would hide.
func TestProofsRoundTripAtEverySize(t *testing.T) {
	for n := 1; n <= 33; n++ {
		ls := leaves(n)
		tree, err := New(ls)
		if err != nil {
			t.Fatalf("n=%d: %v", n, err)
		}
		for i := 0; i < n; i++ {
			path, err := tree.Proof(i)
			if err != nil {
				t.Fatalf("n=%d i=%d: %v", n, i, err)
			}
			if !Verify(ls[i], i, n, path, tree.Root()) {
				t.Fatalf("n=%d i=%d: valid proof rejected", n, i)
			}
		}
	}
}

func TestTamperedLeafIsRejected(t *testing.T) {
	ls := leaves(8)
	tree, _ := New(ls)
	path, _ := tree.Proof(3)
	forged := hashing.Tagged("test/leaf", []byte("not the committed entry"))
	if Verify(forged, 3, 8, path, tree.Root()) {
		t.Fatal("a substituted leaf verified against the original root")
	}
}

func TestProofIsBoundToItsIndex(t *testing.T) {
	ls := leaves(8)
	tree, _ := New(ls)
	path, _ := tree.Proof(3)
	if Verify(ls[3], 4, 8, path, tree.Root()) {
		t.Fatal("a proof for index 3 verified at index 4")
	}
}

// The classic second-preimage attack: without distinct leaf and node tags, an
// internal node can be passed off as a leaf, forging membership for data that
// was never committed to the tree.
func TestInternalNodeCannotBePresentedAsALeaf(t *testing.T) {
	ls := leaves(4)
	tree, _ := New(ls)
	internal := NodeHash(ls[0], ls[1]) // the real left-subtree parent

	for idx := 0; idx < 4; idx++ {
		for _, path := range [][]hashing.Digest{
			{NodeHash(ls[2], ls[3])},
			{},
			{ls[2], ls[3]},
		} {
			if Verify(internal, idx, 4, path, tree.Root()) {
				t.Fatalf("internal node accepted as leaf at index %d", idx)
			}
		}
	}
}

// Duplicating an odd trailing node (Bitcoin's approach, CVE-2012-2459) lets
// distinct leaf sets collide to one root. Promotion must not do that.
func TestPromotionDoesNotCollideWithDuplication(t *testing.T) {
	three, _ := New(leaves(3))
	ls4 := append(leaves(3), leaves(3)[2]) // what duplication would have produced
	four, _ := New(ls4)
	if three.Root() == four.Root() {
		t.Fatal("a 3-leaf tree and its duplicated 4-leaf form share a root")
	}
}

func TestTrailingJunkInProofIsRejected(t *testing.T) {
	ls := leaves(8)
	tree, _ := New(ls)
	path, _ := tree.Proof(3)
	if Verify(ls[3], 3, 8, append(path, ls[0]), tree.Root()) {
		t.Fatal("proof with an extra sibling was accepted")
	}
}

func TestShortProofIsRejected(t *testing.T) {
	ls := leaves(8)
	tree, _ := New(ls)
	path, _ := tree.Proof(3)
	if Verify(ls[3], 3, 8, path[:1], tree.Root()) {
		t.Fatal("truncated proof was accepted")
	}
}

// size is part of what the pool commitment binds. A verifier told the wrong
// size walks the wrong level widths and must not land on the root.
func TestWrongSizeIsRejected(t *testing.T) {
	ls := leaves(8)
	tree, _ := New(ls)
	path, _ := tree.Proof(3)
	if Verify(ls[3], 3, 16, path, tree.Root()) {
		t.Fatal("proof verified under a falsified pool size")
	}
}

func TestEmptyTreeRejected(t *testing.T) {
	if _, err := New(nil); err == nil {
		t.Fatal("built a tree with no leaves")
	}
}

// Package roster holds the list of entries a draw selects from, and the
// commitment that freezes it.
//
// The commitment scheme does not change with privacy: every roster is a salted
// Merkle tree. What the organiser chooses is how much of it the published
// bundle discloses. That keeps one code path and one set of test vectors,
// and it means a draw can be audited the same way whether or not the entrant
// list is public.
package roster

import (
	"crypto/rand"
	"errors"
	"fmt"

	"github.com/sharkfin009/verifiable-challenge/hashing"
	"github.com/sharkfin009/verifiable-challenge/merkle"
)

// SaltSize is the per-entry salt length.
//
// Salts are not optional even for a public roster. Entry labels are usually
// low-entropy — an email address, a name, a ticket number — so an unsalted
// leaf can be brute-forced. Without salts, anyone holding the root could test
// "was alice@example.com in this draw?" against a private roster, which is
// exactly the disclosure the private mode exists to prevent.
const SaltSize = 32

// Disclosure is the organiser's choice about what the published bundle shows.
type Disclosure string

const (
	// Public publishes every entry. A verifier rebuilds the whole tree and
	// checks it against the committed root, so inclusion needs no proofs and
	// entrants can confirm themselves by reading the list.
	Public Disclosure = "public"

	// Private publishes only the winners, each with a Merkle proof. Everyone
	// else stays hidden behind the root, and entrants confirm their own
	// inclusion with a receipt instead.
	Private Disclosure = "private"
)

// Valid reports whether d is a known disclosure mode.
func (d Disclosure) Valid() bool { return d == Public || d == Private }

// Entry is one candidate for selection. Label is opaque to this package: a
// name, an email, a ticket number, a wallet address — whatever the organiser
// uses to identify a participant.
type Entry struct {
	Label string `json:"label"`
}

// CanonicalBytes is the deterministic encoding of an entry.
func (e Entry) CanonicalBytes() []byte {
	return hashing.LP([]byte(e.Label))
}

// Sealed is an entry with the salt that hides it in the committed tree.
type Sealed struct {
	Salt  []byte `json:"salt"`
	Entry Entry  `json:"entry"`
}

// Leaf computes this entry's Merkle leaf.
func (s Sealed) Leaf() hashing.Digest {
	return merkle.LeafHash(s.Salt, s.Entry.CanonicalBytes())
}

// Commitment is the public description of a frozen roster. Commit binds Size
// as well as Root, so a tree cannot later be re-padded to a different size in
// order to change which index a given seed selects.
type Commitment struct {
	Root   hashing.Digest `json:"root"`
	Size   int            `json:"size"`
	Commit hashing.Digest `json:"commit"`
}

// CommitValue derives the commitment from a root and size.
func CommitValue(root hashing.Digest, size int) hashing.Digest {
	return hashing.Tagged(hashing.TagRoster, root[:], hashing.U32(uint32(size)))
}

// Roster is the private side: every sealed entry, in committed order.
type Roster struct {
	Sealed []Sealed `json:"sealed"`

	tree *merkle.Tree
}

// Seal salts each label with fresh randomness and builds the tree. Order is
// preserved and is part of the commitment: entry i is entry i forever.
func Seal(labels []string) (*Roster, error) {
	if len(labels) == 0 {
		return nil, errors.New("roster: no entries")
	}
	sealed := make([]Sealed, len(labels))
	for i, l := range labels {
		salt := make([]byte, SaltSize)
		if _, err := rand.Read(salt); err != nil {
			return nil, fmt.Errorf("roster: salt: %w", err)
		}
		sealed[i] = Sealed{Salt: salt, Entry: Entry{Label: l}}
	}
	return Load(sealed)
}

// Load rebuilds a roster from existing sealed entries, preserving salts and order.
func Load(sealed []Sealed) (*Roster, error) {
	if len(sealed) == 0 {
		return nil, errors.New("roster: no entries")
	}
	leaves := make([]hashing.Digest, len(sealed))
	for i, s := range sealed {
		if len(s.Salt) != SaltSize {
			return nil, fmt.Errorf("roster: entry %d has a %d-byte salt, want %d", i, len(s.Salt), SaltSize)
		}
		leaves[i] = s.Leaf()
	}
	t, err := merkle.New(leaves)
	if err != nil {
		return nil, err
	}
	return &Roster{Sealed: sealed, tree: t}, nil
}

// Commitment returns the roster's public commitment.
func (r *Roster) Commitment() Commitment {
	root := r.tree.Root()
	return Commitment{Root: root, Size: r.tree.Size(), Commit: CommitValue(root, r.tree.Size())}
}

// Size returns the number of entries.
func (r *Roster) Size() int { return len(r.Sealed) }

// Open returns the sealed entry at index with its Merkle proof.
func (r *Roster) Open(index int) (Sealed, []hashing.Digest, error) {
	if index < 0 || index >= len(r.Sealed) {
		return Sealed{}, nil, errors.New("roster: index out of range")
	}
	proof, err := r.tree.Proof(index)
	if err != nil {
		return Sealed{}, nil, err
	}
	return r.Sealed[index], proof, nil
}

// Receipt proves to one entrant that they were in the committed roster,
// without showing them anybody else's entry.
//
// It is issued at cutoff, because the root does not exist until the roster is
// frozen. An entrant who holds a receipt can detect being silently dropped;
// they cannot detect the roster being padded with entries that were never
// real, which is a separate problem this scheme does not solve.
type Receipt struct {
	DrawID     string           `json:"draw_id"`
	Root       hashing.Digest   `json:"root"`
	Size       int              `json:"size"`
	Index      int              `json:"index"`
	Salt       string           `json:"salt"`
	Entry      Entry            `json:"entry"`
	MerklePath []hashing.Digest `json:"merkle_path"`
}

// VerifyReceipt checks a receipt against the root it claims membership in.
func VerifyReceipt(r Receipt, salt []byte) bool {
	leaf := merkle.LeafHash(salt, r.Entry.CanonicalBytes())
	return merkle.Verify(leaf, r.Index, r.Size, r.MerklePath, r.Root)
}

// Rebuild recomputes a root from fully disclosed entries. This is how a public
// roster is verified: no per-entry proofs, just recompute the whole tree.
func Rebuild(sealed []Sealed) (hashing.Digest, error) {
	r, err := Load(sealed)
	if err != nil {
		return hashing.Digest{}, err
	}
	return r.tree.Root(), nil
}

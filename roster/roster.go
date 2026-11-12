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
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/sharkfin-909/publicdraw/hashing"
	"github.com/sharkfin-909/publicdraw/merkle"
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

// Salt is a per-entry salt.
//
// It marshals as hex rather than Go's default base64 for []byte, because the
// same salt appears as hex in a receipt and in a winner's reveal. One field
// with two encodings in one published bundle is a trap for any other
// implementation that has to parse it.
type Salt []byte

// MarshalJSON encodes the salt as a hex string.
func (s Salt) MarshalJSON() ([]byte, error) {
	return []byte(`"` + hex.EncodeToString(s) + `"`), nil
}

// UnmarshalJSON decodes a hex-string salt.
func (s *Salt) UnmarshalJSON(b []byte) error {
	if len(b) < 2 || b[0] != '"' || b[len(b)-1] != '"' {
		return errors.New("roster: salt must be a hex string")
	}
	raw, err := hex.DecodeString(string(b[1 : len(b)-1]))
	if err != nil {
		return fmt.Errorf("roster: salt: %w", err)
	}
	*s = raw
	return nil
}

func (s Salt) String() string { return hex.EncodeToString(s) }

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
	Salt  Salt  `json:"salt"`
	Entry Entry `json:"entry"`
}

// Leaf computes this entry's Merkle leaf.
func (s Sealed) Leaf() hashing.Digest { return LeafOf(s.Salt, s.Entry) }

// LeafOf is the committed leaf of an entry under its salt. It is the one
// definition every inclusion check uses — the receipt, the served proof, the
// published leaves and each winner — so they cannot drift apart.
func LeafOf(salt []byte, e Entry) hashing.Digest {
	return merkle.LeafHash(salt, e.CanonicalBytes())
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

// SealOne salts a single label.
//
// Salts are generated when an entrant enters, not when the roster is frozen,
// so the entrant can be handed their salt immediately. That is what lets them
// check their own inclusion in the published leaf list later without anything
// having to be delivered to them at the cutoff — and a receipt that never
// arrived is indistinguishable from one that was never issued.
func SealOne(label string) (Sealed, error) {
	salt := make([]byte, SaltSize)
	if _, err := rand.Read(salt); err != nil {
		return Sealed{}, fmt.Errorf("roster: salt: %w", err)
	}
	return Sealed{Salt: Salt(salt), Entry: Entry{Label: label}}, nil
}

// Seal salts each label with fresh randomness and builds the tree. Order is
// preserved and is part of the commitment: entry i is entry i forever.
func Seal(labels []string) (*Roster, error) {
	if len(labels) == 0 {
		return nil, errors.New("roster: no entries")
	}
	sealed := make([]Sealed, len(labels))
	for i, l := range labels {
		s, err := SealOne(l)
		if err != nil {
			return nil, err
		}
		sealed[i] = s
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

// Proof is the public half of a receipt: everything needed to check one
// entrant's inclusion, and nothing that identifies them.
//
// The salt and the label stay with the entrant, who was given both when they
// entered. Serving them back is what turned the receipt endpoint into a
// roster dump: anyone could count from zero and read every name in a draw
// whose entry list was supposed to be private.
//
// Handing out the path is safe. A path that does not climb to the published
// root fails, so an operator cannot use one to manufacture a false positive
// for a leaf that was never in the tree. They can withhold or corrupt one,
// but that is indistinguishable from "you were not in the roster", which is
// the honest answer to the same question.
type Proof struct {
	DrawID     string           `json:"draw_id"`
	Root       hashing.Digest   `json:"root"`
	Size       int              `json:"size"`
	Index      int              `json:"index"`
	MerklePath []hashing.Digest `json:"merkle_path"`
}

// Proof is the public half of the receipt, without the salt and label the
// entrant holds — so a receipt is checked exactly as a served proof is.
func (r Receipt) Proof() Proof {
	return Proof{DrawID: r.DrawID, Root: r.Root, Size: r.Size, Index: r.Index, MerklePath: r.MerklePath}
}

// VerifyProof checks an entrant's own salt and label against a path, which may
// have been served by the operator. A path that does not climb to the root
// fails, so a served proof cannot vouch for a leaf that was never in the tree.
// A receipt is checked as VerifyProof(r.Proof(), salt, r.Entry.Label).
func VerifyProof(p Proof, salt []byte, label string) bool {
	return Includes(p.Root, p.Size, p.Index, p.MerklePath, LeafOf(salt, Entry{Label: label}))
}

// Includes reports whether leaf sits at index in the tree of this size and root.
func Includes(root hashing.Digest, size, index int, path []hashing.Digest, leaf hashing.Digest) bool {
	return merkle.Verify(leaf, index, size, path, root)
}

// Leaves returns every leaf, in committed order.
//
// Publishing these alongside the root is safe in either disclosure mode: a
// leaf is H(tag, salt, label) with 32 bytes of fresh randomness, so it reveals
// nothing about the label to anyone who does not already hold that salt — and
// only the entrant does. It leaks no more than a Merkle proof already does,
// since a proof discloses sibling leaves anyway.
//
// What it buys: the root becomes independently rebuildable even for a private
// draw, and every entrant can confirm their own inclusion without a receipt
// having reached them.
func (r *Roster) Leaves() []hashing.Digest {
	out := make([]hashing.Digest, len(r.Sealed))
	for i, s := range r.Sealed {
		out[i] = s.Leaf()
	}
	return out
}

// RootFromLeaves rebuilds a root from published leaves.
func RootFromLeaves(leaves []hashing.Digest) (hashing.Digest, error) {
	t, err := merkle.New(leaves)
	if err != nil {
		return hashing.Digest{}, err
	}
	return t.Root(), nil
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

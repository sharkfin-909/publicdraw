package roster

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func labels(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = string(rune('a'+i%26)) + "@example.com"
	}
	return out
}

func TestSaltsHideLowEntropyLabels(t *testing.T) {
	r, err := Seal([]string{"alice@example.com", "alice@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Sealed[0].Leaf() == r.Sealed[1].Leaf() {
		t.Fatal("identical labels produced identical leaves; the roster is brute-forceable")
	}
	if bytes.Equal(r.Sealed[0].Salt, r.Sealed[1].Salt) {
		t.Fatal("salts were reused")
	}
}

func TestCommitmentBindsSize(t *testing.T) {
	r, _ := Seal(labels(5))
	c := r.Commitment()
	if CommitValue(c.Root, c.Size) != c.Commit {
		t.Fatal("commitment is not H(root, size)")
	}
	if CommitValue(c.Root, c.Size+1) == c.Commit {
		t.Fatal("a re-padded size yields the same commitment")
	}
}

func TestOrderIsPartOfTheCommitment(t *testing.T) {
	a, _ := Seal([]string{"x", "y"})
	reversed := []Sealed{a.Sealed[1], a.Sealed[0]}
	b, err := Load(reversed)
	if err != nil {
		t.Fatal(err)
	}
	if a.Commitment().Root == b.Commitment().Root {
		t.Fatal("reordering entries did not change the root")
	}
}

func TestReceiptsProveInclusion(t *testing.T) {
	r, _ := Seal(labels(11))
	c := r.Commitment()
	for i := range r.Sealed {
		sealed, path, err := r.Open(i)
		if err != nil {
			t.Fatal(err)
		}
		rec := Receipt{Root: c.Root, Size: c.Size, Index: i, Entry: sealed.Entry,
			Salt: hex.EncodeToString(sealed.Salt), MerklePath: path}
		if !VerifyReceipt(rec, sealed.Salt) {
			t.Fatalf("receipt for index %d did not verify", i)
		}
	}
}

// A receipt is the entrant's defence against being silently dropped, so it
// must not verify against a roster they were not in.
func TestReceiptFromAnotherRosterIsRejected(t *testing.T) {
	a, _ := Seal(labels(8))
	b, _ := Seal(labels(8))
	sealed, path, _ := a.Open(3)
	rec := Receipt{Root: b.Commitment().Root, Size: 8, Index: 3, Entry: sealed.Entry,
		Salt: hex.EncodeToString(sealed.Salt), MerklePath: path}
	if VerifyReceipt(rec, sealed.Salt) {
		t.Fatal("a receipt verified against a roster it did not belong to")
	}
}

func TestReceiptWithAlteredLabelIsRejected(t *testing.T) {
	r, _ := Seal(labels(8))
	c := r.Commitment()
	sealed, path, _ := r.Open(2)
	rec := Receipt{Root: c.Root, Size: c.Size, Index: 2, Entry: Entry{Label: "someone else"},
		Salt: hex.EncodeToString(sealed.Salt), MerklePath: path}
	if VerifyReceipt(rec, sealed.Salt) {
		t.Fatal("a receipt verified with a substituted label")
	}
}

// Public disclosure is verified by rebuilding rather than by proofs.
func TestPublicRosterRebuildsToTheSameRoot(t *testing.T) {
	r, _ := Seal(labels(13))
	got, err := Rebuild(r.Sealed)
	if err != nil {
		t.Fatal(err)
	}
	if got != r.Commitment().Root {
		t.Fatal("rebuilding the disclosed entries produced a different root")
	}
}

func TestRebuildDetectsASubstitutedEntry(t *testing.T) {
	r, _ := Seal(labels(13))
	tampered := append([]Sealed(nil), r.Sealed...)
	tampered[4] = Sealed{Salt: tampered[4].Salt, Entry: Entry{Label: "inserted@example.com"}}
	got, err := Rebuild(tampered)
	if err != nil {
		t.Fatal(err)
	}
	if got == r.Commitment().Root {
		t.Fatal("a substituted entry rebuilt to the committed root")
	}
}

func TestDisclosureValidation(t *testing.T) {
	for _, d := range []Disclosure{Public, Private} {
		if !d.Valid() {
			t.Errorf("%q should be valid", d)
		}
	}
	if Disclosure("semi").Valid() {
		t.Error("unknown disclosure mode accepted")
	}
}

func TestLoadRejectsShortSalt(t *testing.T) {
	r, _ := Seal(labels(3))
	r.Sealed[1].Salt = Salt([]byte("short"))
	if _, err := Load(r.Sealed); err == nil {
		t.Fatal("accepted a truncated salt")
	}
}

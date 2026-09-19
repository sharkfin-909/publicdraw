package hashing

import "testing"

// Plain concatenation is ambiguous: "ab"+"cdef" and "abc"+"def" are the same
// bytes. Length prefixing is what stops an attacker from splitting fields
// differently to reach a digest that was never legitimately produced.
func TestLengthPrefixingRemovesConcatenationAmbiguity(t *testing.T) {
	a := Tagged("t", []byte("ab"), []byte("cdef"))
	b := Tagged("t", []byte("abc"), []byte("def"))
	if a == b {
		t.Fatal("split-differently inputs collided; length prefixing is not working")
	}
}

func TestEmptyAndAbsentFieldsDiffer(t *testing.T) {
	if Tagged("t", []byte("x"), nil) == Tagged("t", []byte("x")) {
		t.Fatal("trailing empty field is indistinguishable from no field")
	}
}

// A digest produced in one context must never be valid in another, or a
// selection digest could be replayed as a leaf digest.
func TestDomainSeparation(t *testing.T) {
	in := []byte("identical input")
	seen := map[Digest]string{}
	for _, tag := range []string{TagLeaf, TagNode, TagCommit, TagChallenge, TagSelect, TagSession} {
		d := Tagged(tag, in)
		if prev, dup := seen[d]; dup {
			t.Fatalf("tags %q and %q produce the same digest", prev, tag)
		}
		seen[d] = tag
	}
}

func TestDigestJSONRoundTrip(t *testing.T) {
	d := Tagged("t", []byte("x"))
	b, err := d.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 66 { // 64 hex chars + 2 quotes
		t.Fatalf("expected hex string encoding, got %s", b)
	}
	var out Digest
	if err := out.UnmarshalJSON(b); err != nil || out != d {
		t.Fatalf("round trip failed: %v", err)
	}
}

func TestParseDigestRejectsWrongLength(t *testing.T) {
	if _, err := ParseDigest("abcd"); err == nil {
		t.Fatal("accepted a short digest")
	}
}

// Package hashing defines the domain-separated, length-prefixed hashing that
// every other package in this module is built on.
//
// Two rules are enforced here and must be mirrored exactly by any other
// implementation (see testdata/testvectors.json):
//
//  1. Every hash is prefixed with a context tag, so a digest produced in one
//     part of the protocol can never be replayed as a digest from another.
//  2. Every variable-length input is length-prefixed, so that H(a, b) is
//     unambiguous. Plain concatenation is not: "ab"+"cdef" and "abc"+"def"
//     are the same bytes, and hash to the same value.
package hashing

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
)

// Context tags. Changing any of these is a breaking protocol change.
const (
	TagLeaf      = "vc/pool/leaf/v1"      // a challenge entry as a Merkle leaf
	TagNode      = "vc/pool/node/v1"      // an internal Merkle node
	TagCommit    = "vc/pool/commit/v1"    // root+size, the value that gets timestamped
	TagChallenge = "vc/pool/challenge/v1" // canonical encoding of a challenge
	TagSelect    = "vc/select/v1"         // challenge index derivation
	TagSession   = "vc/session/v1"        // the server's pre-beacon session commitment
)

// Size is the byte length of a Digest.
const Size = sha256.Size

// Digest is a SHA-256 output.
type Digest [Size]byte

func (d Digest) String() string { return hex.EncodeToString(d[:]) }

// Bytes returns a copy of the digest as a slice.
func (d Digest) Bytes() []byte {
	b := make([]byte, Size)
	copy(b, d[:])
	return b
}

// ParseDigest decodes a 64-character hex string into a Digest.
func ParseDigest(s string) (Digest, error) {
	var d Digest
	b, err := hex.DecodeString(s)
	if err != nil {
		return d, err
	}
	if len(b) != Size {
		return d, errors.New("hashing: digest must be 32 bytes")
	}
	copy(d[:], b)
	return d, nil
}

// LP returns b prefixed with its length as a 4-byte big-endian integer.
func LP(b []byte) []byte {
	out := make([]byte, 4+len(b))
	binary.BigEndian.PutUint32(out[:4], uint32(len(b)))
	copy(out[4:], b)
	return out
}

// U32 encodes n as 4 big-endian bytes.
func U32(n uint32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, n)
	return b
}

// U64 encodes n as 8 big-endian bytes.
func U64(n uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, n)
	return b
}

// Tagged computes SHA-256 over the tag followed by each length-prefixed part.
//
//	Tagged(tag, a, b) = SHA256(tag ‖ len(a) ‖ a ‖ len(b) ‖ b)
func Tagged(tag string, parts ...[]byte) Digest {
	h := sha256.New()
	h.Write([]byte(tag))
	for _, p := range parts {
		h.Write(LP(p))
	}
	var d Digest
	copy(d[:], h.Sum(nil))
	return d
}

// MarshalJSON encodes a digest as a hex string. Without this, Go would
// serialise the underlying [32]byte as a JSON array of integers, which is
// unreadable and would not match the TypeScript implementation's wire format.
func (d Digest) MarshalJSON() ([]byte, error) {
	return []byte(`"` + d.String() + `"`), nil
}

// UnmarshalJSON decodes a hex-string digest.
func (d *Digest) UnmarshalJSON(b []byte) error {
	if len(b) < 2 || b[0] != '"' || b[len(b)-1] != '"' {
		return errors.New("hashing: digest must be a hex string")
	}
	parsed, err := ParseDigest(string(b[1 : len(b)-1]))
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

package pool

import (
	"github.com/sharkfin009/verifiable-challenge/hashing"
	"github.com/sharkfin009/verifiable-challenge/merkle"
)

func verifyEntry(e Entry, index, size int, path []hashing.Digest, c Commitment) bool {
	leaf := merkle.LeafHash(e.Salt, e.Challenge.CanonicalBytes())
	return merkle.Verify(leaf, index, size, path, c.Root)
}

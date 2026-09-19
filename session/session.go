// Package session implements the two-phase selection that makes a challenge
// unpredictable to the candidate and unsteerable by the operator.
//
// The operator names a *future* drand round and signs that commitment before
// the round's value exists. Without this, an operator who assigns session IDs
// after seeing the beacon can simply generate IDs in a loop until the derived
// index lands on the challenge they want — and the resulting proof still
// verifies perfectly. The wait is the guarantee.
package session

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/sharkfin009/verifiable-challenge/beacon"
	"github.com/sharkfin009/verifiable-challenge/hashing"
	"github.com/sharkfin009/verifiable-challenge/pool"
)

// DefaultLead is how many rounds ahead the target is placed. At quicknet's
// 3-second period, 5 rounds is ~15 seconds: long enough that the round is
// unambiguously unpublished when the commitment is signed, short enough to
// show the candidate as a countdown.
const DefaultLead = 5

// Commitment is what the operator signs and hands the candidate before the
// target round exists.
type Commitment struct {
	SessionID   string         `json:"session_id"`
	TargetRound uint64         `json:"target_round"`
	ChainHash   string         `json:"chain_hash"`
	PoolRoot    hashing.Digest `json:"pool_root"`
	PoolSize    int            `json:"pool_size"`
	PoolCommit  hashing.Digest `json:"pool_commit"`
	IssuedAt    int64          `json:"issued_at"`
}

// SigningBytes is the exact preimage covered by the operator's signature.
func (c Commitment) SigningBytes() []byte {
	d := hashing.Tagged(hashing.TagSession,
		[]byte(c.SessionID),
		hashing.U64(c.TargetRound),
		[]byte(c.ChainHash),
		c.PoolRoot[:],
		hashing.U32(uint32(c.PoolSize)),
		c.PoolCommit[:],
		hashing.U64(uint64(c.IssuedAt)),
	)
	return d[:]
}

// NewSessionID returns a fresh random session identifier.
//
// It must be generated *before* the target round publishes. Generating it
// afterwards is the grinding attack this package exists to prevent.
func NewSessionID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Begin issues a signed commitment naming a round `lead` rounds in the future.
func Begin(chain beacon.Chain, c pool.Commitment, priv ed25519.PrivateKey, lead uint64, now time.Time) (Commitment, []byte, error) {
	if lead < 1 {
		return Commitment{}, nil, errors.New("session: lead must be at least 1 round")
	}
	id, err := NewSessionID()
	if err != nil {
		return Commitment{}, nil, err
	}
	com := Commitment{
		SessionID:   id,
		TargetRound: chain.RoundAt(now) + lead,
		ChainHash:   chain.Hash,
		PoolRoot:    c.Root,
		PoolSize:    c.Size,
		PoolCommit:  c.Commit,
		IssuedAt:    now.Unix(),
	}
	return com, ed25519.Sign(priv, com.SigningBytes()), nil
}

// VerifySignature checks the operator's signature over a commitment.
func VerifySignature(pub ed25519.PublicKey, c Commitment, sig []byte) error {
	if len(pub) != ed25519.PublicKeySize {
		return errors.New("session: bad public key size")
	}
	if !ed25519.Verify(pub, c.SigningBytes(), sig) {
		return errors.New("session: operator signature does not verify")
	}
	return nil
}

// Index derives which challenge a session receives.
//
// Every input is load-bearing:
//   - poolRoot binds the selection to one specific committed pool, so a
//     different pool cannot be substituted while keeping the same index.
//   - poolSize is committed alongside the root and fixes the modulus.
//   - randomness is the beacon value nobody could predict.
//   - sessionID separates candidates drawing in the same round.
//
// The modulo introduces a bias of roughly poolSize/2^256. For any real pool
// that is on the order of 1e-74; rejection sampling would be theatre.
func Index(poolRoot hashing.Digest, poolSize int, randomness hashing.Digest, sessionID string) (int, error) {
	if poolSize <= 0 {
		return 0, errors.New("session: pool size must be positive")
	}
	d := hashing.Tagged(hashing.TagSelect,
		poolRoot[:],
		hashing.U32(uint32(poolSize)),
		randomness[:],
		[]byte(sessionID),
	)
	n := new(big.Int).SetBytes(d[:])
	return int(n.Mod(n, big.NewInt(int64(poolSize))).Int64()), nil
}

// GenerateKey returns a fresh operator signing keypair.
func GenerateKey() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("session: keygen: %w", err)
	}
	return pub, priv, nil
}

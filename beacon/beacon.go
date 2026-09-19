// Package beacon reads and verifies drand randomness.
//
// Verification is done locally against the chain's group public key. Trusting
// the HTTP response instead would reintroduce exactly the trusted third party
// this protocol exists to remove: an endpoint that can be asked to lie about
// what the beacon said is indistinguishable from an operator choosing the
// challenge.
package beacon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	bls12381 "github.com/drand/kyber-bls12381"
	blssign "github.com/drand/kyber/sign/bls"
	"github.com/sharkfin009/verifiable-challenge/hashing"
)

// Quicknet is the drand chain this protocol uses: 3-second rounds, unchained,
// signatures on G1. Unchained matters — a round's message depends only on its
// own number, so a future round can be named before it exists.
var Quicknet = Chain{
	Name:      "quicknet",
	Hash:      "52db9ba70e0cc0f6eaf7803dd07447a1f5477735fd3f661792ba94600c84e971",
	PublicKey: "83cf0f2896adee7eb8b5f01fcad3912212c437e0073e911fb90022d3e760183c8c4b450b6a0a6c3ac6a5776a2d1064510d1fec758c921cc22b0e17e63aaf4bcb5ed66304de9cf809bd274ca73bab4af5a6e9c76a4bc09e76eae8991ef5ece45a",
	GenesisTS: 1692803367,
	Period:    3 * time.Second,
}

// Chain describes a drand beacon chain.
type Chain struct {
	Name      string
	Hash      string
	PublicKey string
	GenesisTS int64
	Period    time.Duration
}

// TimeOf returns the wall-clock time at which a round is published. This is
// what makes a round number usable as a timestamp: the mapping is fixed by the
// chain's genesis and period, so it needs no separate attestation.
func (c Chain) TimeOf(round uint64) time.Time {
	return time.Unix(c.GenesisTS, 0).Add(time.Duration(round-1) * c.Period)
}

// RoundAt returns the latest round published at or before t.
func (c Chain) RoundAt(t time.Time) uint64 {
	d := t.Unix() - c.GenesisTS
	if d < 0 {
		return 1
	}
	return uint64(d/int64(c.Period.Seconds())) + 1
}

// Round is a single beacon output.
type Round struct {
	Round     uint64 `json:"round"`
	Signature string `json:"signature"`
}

// Randomness is SHA-256 of the signature, matching drand's own derivation.
func (r Round) Randomness() (hashing.Digest, error) {
	sig, err := hex.DecodeString(r.Signature)
	if err != nil {
		return hashing.Digest{}, fmt.Errorf("beacon: signature hex: %w", err)
	}
	return hashing.Digest(sha256.Sum256(sig)), nil
}

// Verify checks the round's BLS signature against the chain's group key.
//
// The message is SHA-256 of the big-endian round number — that and only that,
// because the chain is unchained.
//
// kyber marks plain BLS as deprecated in favour of BDN. That warning is about
// rogue-key attacks during *aggregation*; here the signature is already
// aggregated by the drand network and checked against the published group key,
// which is what drand itself does.
func (c Chain) Verify(r Round) error {
	sig, err := hex.DecodeString(r.Signature)
	if err != nil {
		return fmt.Errorf("beacon: signature hex: %w", err)
	}
	pkb, err := hex.DecodeString(c.PublicKey)
	if err != nil {
		return fmt.Errorf("beacon: public key hex: %w", err)
	}
	suite := bls12381.NewBLS12381Suite()
	pk := suite.G2().Point()
	if err := pk.UnmarshalBinary(pkb); err != nil {
		return fmt.Errorf("beacon: public key: %w", err)
	}
	msg := sha256.Sum256(hashing.U64(r.Round))
	if err := blssign.NewSchemeOnG1(suite).Verify(pk, msg[:], sig); err != nil {
		return fmt.Errorf("beacon: round %d signature invalid: %w", r.Round, err)
	}
	return nil
}

// Endpoints are public drand HTTP mirrors. They serve identical values; any
// one of them is enough because the signature is verified locally.
var Endpoints = []string{
	"https://api.drand.sh",
	"https://drand.cloudflare.com",
}

// Fetch retrieves a round, trying each mirror in turn, and verifies it before
// returning. Pass 0 for the latest round.
func (c Chain) Fetch(ctx context.Context, round uint64) (Round, error) {
	which := "latest"
	if round > 0 {
		which = fmt.Sprintf("%d", round)
	}
	var lastErr error
	for _, base := range Endpoints {
		url := fmt.Sprintf("%s/%s/public/%s", base, c.Hash, which)
		r, err := fetchOne(ctx, url)
		if err != nil {
			lastErr = err
			continue
		}
		if err := c.Verify(r); err != nil {
			return Round{}, err // a bad signature is fatal, not worth retrying elsewhere
		}
		return r, nil
	}
	return Round{}, fmt.Errorf("beacon: all endpoints failed: %w", lastErr)
}

func fetchOne(ctx context.Context, url string) (Round, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Round{}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Round{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Round{}, fmt.Errorf("beacon: %s: status %d", url, resp.StatusCode)
	}
	var r Round
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return Round{}, err
	}
	if r.Signature == "" {
		return Round{}, errors.New("beacon: response has no signature")
	}
	return r, nil
}

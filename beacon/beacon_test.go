package beacon

import (
	"context"
	"os"
	"testing"
	"time"
)

// A real quicknet round, captured so the signature path is exercised without
// network access. randomness is drand's own published value for this round.
var vector = struct {
	round      uint64
	signature  string
	randomness string
}{
	round:      20000000,
	signature:  "96892582a33552a7b67ba44ef09c3ccd535bbebe760c93ecf45be8958d0c0f06390c6d19d7bf492eb806af7eef6b125c",
	randomness: "f39c98df5525968e6622e0c63edb3d1f9c5607cf780db78abdded36e4b3dba95",
}

func TestVerifiesARealRound(t *testing.T) {
	if err := Quicknet.Verify(Round{Round: vector.round, Signature: vector.signature}); err != nil {
		t.Fatalf("genuine round rejected: %v", err)
	}
}

func TestRandomnessMatchesDrandsOwnDerivation(t *testing.T) {
	r, err := Round{Round: vector.round, Signature: vector.signature}.Randomness()
	if err != nil {
		t.Fatal(err)
	}
	if r.String() != vector.randomness {
		t.Fatalf("randomness = %s, drand publishes %s", r, vector.randomness)
	}
}

func TestTamperedSignatureIsRejected(t *testing.T) {
	bad := vector.signature[:len(vector.signature)-1] + "d"
	if err := Quicknet.Verify(Round{Round: vector.round, Signature: bad}); err == nil {
		t.Fatal("a corrupted signature verified")
	}
}

// The round number is the signed message. Replaying a valid signature under a
// different round would let an operator reuse a known-favourable value.
func TestSignatureIsBoundToItsRoundNumber(t *testing.T) {
	if err := Quicknet.Verify(Round{Round: vector.round + 1, Signature: vector.signature}); err == nil {
		t.Fatal("signature verified against the wrong round number")
	}
}

func TestRoundTimeMappingIsInvertible(t *testing.T) {
	for _, r := range []uint64{1, 2, 1000, vector.round, 30_000_000} {
		if got := Quicknet.RoundAt(Quicknet.TimeOf(r)); got != r {
			t.Errorf("round %d -> %s -> round %d", r, Quicknet.TimeOf(r), got)
		}
	}
}

func TestGenesisRoundIsRoundOne(t *testing.T) {
	if got := Quicknet.RoundAt(time.Unix(Quicknet.GenesisTS, 0)); got != 1 {
		t.Fatalf("genesis maps to round %d, want 1", got)
	}
}

func TestRoundAtClampsBeforeGenesis(t *testing.T) {
	if got := Quicknet.RoundAt(time.Unix(Quicknet.GenesisTS-10_000, 0)); got != 1 {
		t.Fatalf("pre-genesis time maps to round %d, want 1", got)
	}
}

// Opt-in: hits the public drand mirrors.
func TestFetchLive(t *testing.T) {
	if os.Getenv("VC_LIVE") == "" {
		t.Skip("set VC_LIVE=1 to exercise the network path")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	r, err := Quicknet.Fetch(ctx, vector.round)
	if err != nil {
		t.Fatal(err)
	}
	if r.Signature != vector.signature {
		t.Fatalf("mirror served a different signature for round %d", vector.round)
	}
	if _, err := Quicknet.Fetch(ctx, 0); err != nil {
		t.Fatalf("latest round: %v", err)
	}
}

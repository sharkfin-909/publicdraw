package session

import (
	"fmt"
	"testing"
	"time"

	"github.com/sharkfin009/verifiable-challenge/beacon"
	"github.com/sharkfin009/verifiable-challenge/hashing"
	"github.com/sharkfin009/verifiable-challenge/pool"
)

var (
	testRoot = hashing.Tagged("test", []byte("root"))
	testRand = hashing.Tagged("test", []byte("randomness"))
)

func TestIndexIsDeterministic(t *testing.T) {
	a, _ := Index(testRoot, 100, testRand, "session-1")
	b, _ := Index(testRoot, 100, testRand, "session-1")
	if a != b {
		t.Fatal("derivation is not deterministic")
	}
}

func TestIndexIsInRange(t *testing.T) {
	for size := 1; size <= 50; size++ {
		for i := 0; i < 20; i++ {
			idx, err := Index(testRoot, size, testRand, fmt.Sprintf("s-%d", i))
			if err != nil {
				t.Fatal(err)
			}
			if idx < 0 || idx >= size {
				t.Fatalf("size=%d produced index %d", size, idx)
			}
		}
	}
}

// Each input must actually affect the outcome. A derivation that ignores the
// pool root would let an operator swap in a different pool and keep the index.
func TestEveryInputChangesTheIndex(t *testing.T) {
	const size = 1000
	base, _ := Index(testRoot, size, testRand, "s")

	otherRoot, _ := Index(hashing.Tagged("test", []byte("other root")), size, testRand, "s")
	otherRand, _ := Index(testRoot, size, hashing.Tagged("test", []byte("other")), "s")
	otherSess, _ := Index(testRoot, size, testRand, "s2")

	if base == otherRoot && base == otherRand && base == otherSess {
		t.Fatal("index appears insensitive to its inputs")
	}
	if otherRoot == base {
		t.Error("pool root does not affect the index")
	}
	if otherRand == base {
		t.Error("beacon randomness does not affect the index")
	}
	if otherSess == base {
		t.Error("session id does not affect the index")
	}
}

func TestIndexSpreadsAcrossThePool(t *testing.T) {
	const size = 16
	seen := map[int]bool{}
	for i := 0; i < 2000; i++ {
		idx, _ := Index(testRoot, size, testRand, fmt.Sprintf("session-%d", i))
		seen[idx] = true
	}
	if len(seen) != size {
		t.Fatalf("only %d of %d challenges were ever selected", len(seen), size)
	}
}

// THE ATTACK THIS PACKAGE EXISTS TO PREVENT.
//
// If the operator learns the beacon value before fixing the session ID, they
// can simply try IDs until the index lands where they want. Nothing about the
// resulting bundle looks wrong: every hash is correct and every signature
// verifies. The only defence is ordering — committing to the session before
// the round exists — which is why Begin targets a future round.
func TestOperatorCanGrindTheIndexIfSessionIDIsChosenAfterTheBeacon(t *testing.T) {
	const size = 1000
	const wanted = 42

	attempts := 0
	var forged string
	for i := 0; i < 200_000; i++ {
		attempts++
		id := fmt.Sprintf("looks-perfectly-normal-%d", i)
		if idx, _ := Index(testRoot, size, testRand, id); idx == wanted {
			forged = id
			break
		}
	}
	if forged == "" {
		t.Fatal("expected to find a steering session id")
	}
	got, _ := Index(testRoot, size, testRand, forged)
	if got != wanted {
		t.Fatalf("grind produced %d, wanted %d", got, wanted)
	}
	t.Logf("steered a 1000-challenge pool to index %d after %d tries (~%d expected)", wanted, attempts, size)
}

func TestBeginTargetsAFutureRound(t *testing.T) {
	_, priv, _ := GenerateKey()
	chain := beacon.Quicknet
	now := time.Now()

	com, _, err := Begin(chain, pool.Commitment{Root: testRoot, Size: 10, Commit: pool.CommitValue(testRoot, 10)}, priv, DefaultLead, now)
	if err != nil {
		t.Fatal(err)
	}
	target := chain.TimeOf(com.TargetRound)
	if !target.After(now) {
		t.Fatalf("target round %d publishes at %s, not after issue time %s", com.TargetRound, target, now)
	}
	if com.IssuedAt >= target.Unix() {
		t.Fatal("commitment was not issued strictly before its target round")
	}
}

func TestBeginRejectsZeroLead(t *testing.T) {
	_, priv, _ := GenerateKey()
	if _, _, err := Begin(beacon.Quicknet, pool.Commitment{Root: testRoot, Size: 1}, priv, 0, time.Now()); err == nil {
		t.Fatal("accepted a lead of zero, which allows targeting an already-published round")
	}
}

func TestSignatureCoversEveryCommitmentField(t *testing.T) {
	pub, priv, _ := GenerateKey()
	c := pool.Commitment{Root: testRoot, Size: 10, Commit: pool.CommitValue(testRoot, 10)}
	com, sig, _ := Begin(beacon.Quicknet, c, priv, DefaultLead, time.Now())

	if err := VerifySignature(pub, com, sig); err != nil {
		t.Fatalf("honest signature rejected: %v", err)
	}

	for name, mutate := range map[string]func(*Commitment){
		"session id":   func(c *Commitment) { c.SessionID = "different" },
		"target round": func(c *Commitment) { c.TargetRound++ },
		"chain hash":   func(c *Commitment) { c.ChainHash = "other-chain" },
		"pool root":    func(c *Commitment) { c.PoolRoot = hashing.Tagged("test", []byte("swapped")) },
		"pool size":    func(c *Commitment) { c.PoolSize++ },
		"pool commit":  func(c *Commitment) { c.PoolCommit = hashing.Tagged("test", []byte("swapped")) },
		"issued at":    func(c *Commitment) { c.IssuedAt -= 3600 },
	} {
		tampered := com
		mutate(&tampered)
		if err := VerifySignature(pub, tampered, sig); err == nil {
			t.Errorf("signature still verified after altering %s", name)
		}
	}
}

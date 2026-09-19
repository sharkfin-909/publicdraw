package bundle

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sharkfin009/verifiable-challenge/beacon"
	"github.com/sharkfin009/verifiable-challenge/hashing"
	"github.com/sharkfin009/verifiable-challenge/pool"
	"github.com/sharkfin009/verifiable-challenge/session"
)

const (
	vecRound = 20000000
	vecSig   = "96892582a33552a7b67ba44ef09c3ccd535bbebe760c93ecf45be8958d0c0f06390c6d19d7bf492eb806af7eef6b125c"
)

// build assembles an honest bundle around the captured beacon round, running
// the real selection so the index and proof are genuine rather than asserted.
func build(t *testing.T) (Bundle, ed25519.PrivateKey) {
	t.Helper()
	chain := beacon.Quicknet

	challenges := make([]pool.Challenge, 64)
	for i := range challenges {
		challenges[i] = pool.Challenge{
			ID: string(rune('a'+i%26)) + "-" + string(rune('0'+i/26)),
			Language: "go", Title: "challenge", Prompt: "solve it", StarterCode: "func main() {}",
		}
	}
	p, err := pool.New(challenges)
	if err != nil {
		t.Fatal(err)
	}
	pc := p.Commitment()

	pub, priv, _ := session.GenerateKey()
	com := session.Commitment{
		SessionID:   "3f1c9a77b2e04d18",
		TargetRound: vecRound,
		ChainHash:   chain.Hash,
		PoolRoot:    pc.Root,
		PoolSize:    pc.Size,
		PoolCommit:  pc.Commit,
		IssuedAt:    chain.TimeOf(vecRound).Unix() - 15, // committed before the round existed
	}
	sig := ed25519.Sign(priv, com.SigningBytes())

	r := beacon.Round{Round: vecRound, Signature: vecSig}
	rnd, err := r.Randomness()
	if err != nil {
		t.Fatal(err)
	}
	idx, err := session.Index(pc.Root, pc.Size, rnd, com.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	entry, path, err := p.Open(idx)
	if err != nil {
		t.Fatal(err)
	}

	return Bundle{
		Version: 1,
		Pool:    PoolRef{Root: pc.Root, Size: pc.Size, Commit: pc.Commit},
		Session: SessionRef{Commitment: com, OperatorPK: hex.EncodeToString(pub), Signature: hex.EncodeToString(sig)},
		Beacon:  r,
		Reveal:  Reveal{Index: idx, Salt: hex.EncodeToString(entry.Salt), Challenge: entry.Challenge, MerklePath: path},
	}, priv
}

// failed reports whether the named check ran and failed.
func failed(rep Report, substr string) bool {
	for _, c := range rep.Checks {
		if strings.Contains(c.Name, substr) {
			return !c.OK && !c.Skipped
		}
	}
	return false
}

func TestHonestBundleVerifies(t *testing.T) {
	b, _ := build(t)
	rep := Verify(b, beacon.Quicknet)
	if !rep.OK {
		for _, c := range rep.Checks {
			t.Logf("ok=%v skipped=%v %s — %s", c.OK, c.Skipped, c.Name, c.Detail)
		}
		t.Fatal("honest bundle failed verification")
	}
}

// Timestamping is not verified in-process, and must be reported as skipped
// rather than quietly counted as a pass.
func TestTimestampCheckIsReportedAsSkipped(t *testing.T) {
	b, _ := build(t)
	rep := Verify(b, beacon.Quicknet)
	if rep.Skipped != 1 {
		t.Fatalf("expected exactly one skipped check, got %d", rep.Skipped)
	}
	for _, c := range rep.Checks {
		if strings.Contains(c.Name, "timestamped") && !c.Skipped {
			t.Fatal("OTS check claimed to have run")
		}
	}
}

// The core property: an operator who waits for the beacon and then grinds a
// session id must be caught, because the commitment would postdate the round.
func TestCommitmentIssuedAfterTheRoundIsRejected(t *testing.T) {
	b, priv := build(t)
	b.Session.Commitment.IssuedAt = beacon.Quicknet.TimeOf(vecRound).Unix() + 1
	b.Session.Signature = hex.EncodeToString(ed25519.Sign(priv, b.Session.Commitment.SigningBytes()))

	rep := Verify(b, beacon.Quicknet)
	if rep.OK {
		t.Fatal("a bundle committed after its beacon round verified")
	}
	if !failed(rep, "predates") {
		t.Fatal("the ordering check did not fire")
	}
}

func TestSwappedChallengeIsRejected(t *testing.T) {
	b, _ := build(t)
	b.Reveal.Challenge.Prompt = "a much easier problem"

	rep := Verify(b, beacon.Quicknet)
	if rep.OK {
		t.Fatal("a substituted challenge verified")
	}
	if !failed(rep, "merkle proof") {
		t.Fatal("the merkle check did not fire")
	}
}

func TestClaimingADifferentIndexIsRejected(t *testing.T) {
	b, _ := build(t)
	b.Reveal.Index = (b.Reveal.Index + 1) % b.Pool.Size

	rep := Verify(b, beacon.Quicknet)
	if rep.OK {
		t.Fatal("a mismatched index verified")
	}
	if !failed(rep, "index derives") {
		t.Fatal("the derivation check did not fire")
	}
}

func TestSubstitutedBeaconRoundIsRejected(t *testing.T) {
	b, _ := build(t)
	b.Beacon.Round = vecRound + 1

	rep := Verify(b, beacon.Quicknet)
	if rep.OK {
		t.Fatal("a bundle carrying the wrong round verified")
	}
	if !failed(rep, "committed target") {
		t.Fatal("the round-match check did not fire")
	}
}

func TestForgedBeaconSignatureIsRejected(t *testing.T) {
	b, _ := build(t)
	b.Beacon.Signature = vecSig[:len(vecSig)-1] + "d"

	rep := Verify(b, beacon.Quicknet)
	if rep.OK {
		t.Fatal("a forged beacon signature verified")
	}
	if !failed(rep, "drand signature") {
		t.Fatal("the drand check did not fire")
	}
}

func TestUnsignedCommitmentIsRejected(t *testing.T) {
	b, _ := build(t)
	other, _, _ := session.GenerateKey()
	b.Session.OperatorPK = hex.EncodeToString(other)

	rep := Verify(b, beacon.Quicknet)
	if rep.OK {
		t.Fatal("a commitment verified under the wrong operator key")
	}
	if !failed(rep, "operator signature") {
		t.Fatal("the signature check did not fire")
	}
}

// A pool swapped after the fact must break, even if everything else is
// internally consistent.
func TestRepaddedPoolSizeIsRejected(t *testing.T) {
	b, _ := build(t)
	b.Pool.Size = b.Pool.Size * 2

	rep := Verify(b, beacon.Quicknet)
	if rep.OK {
		t.Fatal("a falsified pool size verified")
	}
	if !failed(rep, "binds root and size") {
		t.Fatal("the commitment-binding check did not fire")
	}
}

func TestCommitmentForADifferentPoolIsRejected(t *testing.T) {
	b, priv := build(t)
	other := hashing.Tagged("test", []byte("a different pool entirely"))
	b.Session.Commitment.PoolRoot = other
	b.Session.Signature = hex.EncodeToString(ed25519.Sign(priv, b.Session.Commitment.SigningBytes()))

	rep := Verify(b, beacon.Quicknet)
	if rep.OK {
		t.Fatal("a commitment naming a different pool verified")
	}
	if !failed(rep, "matches this pool") {
		t.Fatal("the pool-match check did not fire")
	}
}

// Verification must never stop at the first failure — a report that hides
// later checks makes a broken bundle harder to diagnose than it needs to be.
func TestVerifyReportsEveryCheck(t *testing.T) {
	b, _ := build(t)
	b.Beacon.Signature = vecSig[:len(vecSig)-1] + "d"
	b.Reveal.Index = 0

	rep := Verify(b, beacon.Quicknet)
	if len(rep.Checks) < 8 {
		t.Fatalf("only %d checks reported", len(rep.Checks))
	}
	if !failed(rep, "drand signature") || !failed(rep, "index derives") {
		t.Fatal("verification short-circuited instead of reporting both failures")
	}
}

func TestBundleSurvivesJSONRoundTrip(t *testing.T) {
	b, _ := build(t)
	raw, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	back, err := Read(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	if !Verify(back, beacon.Quicknet).OK {
		t.Fatal("bundle stopped verifying after a JSON round trip")
	}
}

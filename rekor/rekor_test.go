package rekor

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"testing"
	"time"
)

func fixture(t *testing.T) *Anchor {
	t.Helper()
	a, err := loadFixture()
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	return a
}

func TestInclusionProofVerifies(t *testing.T) {
	if err := fixture(t).VerifyInclusion(); err != nil {
		t.Fatalf("a genuine anchor failed verification: %v", err)
	}
}

// Every part of the proof has to be load-bearing, or the check is decoration.
func TestTamperedProofIsRejected(t *testing.T) {
	for name, mutate := range map[string]func(*Anchor){
		"body":      func(a *Anchor) { a.Body = base64.StdEncoding.EncodeToString([]byte("a different entry")) },
		"root hash": func(a *Anchor) { a.RootHash = base64.StdEncoding.EncodeToString(make([]byte, 32)) },
		"log index": func(a *Anchor) { a.LogIndex++ },
		"tree size": func(a *Anchor) { a.TreeSize++ },
		"a hash":    func(a *Anchor) { a.Hashes[0] = base64.StdEncoding.EncodeToString(make([]byte, 32)) },
		"dropped hash": func(a *Anchor) {
			if len(a.Hashes) > 1 {
				a.Hashes = a.Hashes[1:]
			}
		},
	} {
		a := fixture(t)
		mutate(a)
		if err := a.VerifyInclusion(); err == nil {
			t.Errorf("anchor still verified after altering %s", name)
		}
	}
}

// The checkpoint is what makes the proof mean something: a root nobody signed
// is just a number.
func TestCheckpointMustAgreeWithTheProof(t *testing.T) {
	a := fixture(t)
	cp, err := a.ParseCheckpoint()
	if err != nil {
		t.Fatal(err)
	}
	if cp.RootHash != a.RootHash {
		t.Fatal("fixture is inconsistent")
	}
	if cp.Origin == "" || cp.TreeSize == 0 {
		t.Fatalf("checkpoint did not parse: %+v", cp)
	}

	// A proof whose root is not the one the log signed must not pass.
	a.Checkpoint = fmt.Sprintf("%s\n%d\n%s\n\n— %s AAAA\n",
		cp.Origin, cp.TreeSize, base64.StdEncoding.EncodeToString(make([]byte, 32)), cp.Origin)
	if err := a.VerifyInclusion(); err == nil {
		t.Fatal("a proof verified against a checkpoint for a different root")
	}
}

func TestBindsPayload(t *testing.T) {
	a := fixture(t)
	payload, err := base64.StdEncoding.DecodeString(a.Payload)
	if err != nil {
		t.Fatal(err)
	}
	if !a.BindsPayload(payload) {
		t.Fatal("anchor does not bind its own payload")
	}
	if a.BindsPayload([]byte("something else")) {
		t.Fatal("anchor claims to bind a payload it does not")
	}
}

// Witness times are the only timestamp Rekor v2 offers — integratedTime is
// always zero — so they have to parse, and they must never claim to be
// verified.
func TestWitnessCosignaturesCarryTimeAndAreNotClaimedVerified(t *testing.T) {
	cp, err := fixture(t).ParseCheckpoint()
	if err != nil {
		t.Fatal(err)
	}
	cos := cp.Cosignatures()
	if len(cos) == 0 {
		t.Fatal("no witness cosignatures parsed")
	}
	for _, c := range cos {
		if c.Verified {
			t.Errorf("cosignature from %s claims to be verified", c.Witness)
		}
		if c.Witness == cp.Origin {
			t.Errorf("the log's own signature was counted as a witness")
		}
		if c.At.Before(time.Unix(1_600_000_000, 0)) || c.At.After(time.Now().Add(24*time.Hour)) {
			t.Errorf("witness %s has an implausible time %s", c.Witness, c.At)
		}
	}
	if _, ok := cp.EarliestCosignature(); !ok {
		t.Fatal("no earliest cosignature")
	}
}

func TestMalformedCheckpointIsRejected(t *testing.T) {
	for _, bad := range []string{"", "no blank line", "origin\nnotanumber\nroot\n\n— x y\n"} {
		if _, err := ParseCheckpoint(bad); err == nil {
			t.Errorf("parsed a malformed checkpoint: %q", bad)
		}
	}
}

// Opt-in: submits to Sigstore's real public log.
func TestSubmitLive(t *testing.T) {
	if os.Getenv("VC_LIVE") == "" {
		t.Skip("set VC_LIVE=1 to submit to the public Rekor instance")
	}
	key, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	payload := sha256.Sum256([]byte(fmt.Sprintf("truedraw test %d", time.Now().UnixNano())))

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	a, err := New("").Submit(ctx, payload[:], key)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.VerifyInclusion(); err != nil {
		t.Fatalf("a freshly submitted anchor does not verify: %v", err)
	}
	if !a.BindsPayload(payload[:]) {
		t.Fatal("anchor does not bind what was submitted")
	}
}

// Command vc builds committed challenge pools, runs the two-phase selection,
// and verifies the resulting bundles.
//
// It deliberately has no CLI framework dependency: the verifier is meant to be
// a small static binary someone can audit and run without trusting anything
// about where it came from.
package main

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/sharkfin009/verifiable-challenge/beacon"
	"github.com/sharkfin009/verifiable-challenge/bundle"
	"github.com/sharkfin009/verifiable-challenge/pool"
	"github.com/sharkfin009/verifiable-challenge/session"
)

const usage = `vc — verifiable challenge selection

  vc keygen  -out FILE                       generate an operator signing key
  vc commit  -in FILE -pool FILE -out FILE   commit a challenge pool
  vc begin   -commit FILE -key FILE -out FILE [-lead N]
                                             issue a pre-beacon session commitment
  vc reveal  -session FILE -pool FILE -out FILE [-wait]
                                             open the challenge once the round publishes
  vc verify  FILE                            verify a bundle  [-json]
  vc beacon  [ROUND]                         fetch and verify a drand round
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "keygen":
		err = cmdKeygen(os.Args[2:])
	case "commit":
		err = cmdCommit(os.Args[2:])
	case "begin":
		err = cmdBegin(os.Args[2:])
	case "reveal":
		err = cmdReveal(os.Args[2:])
	case "verify":
		err = cmdVerify(os.Args[2:])
	case "beacon":
		err = cmdBeacon(os.Args[2:])
	case "-h", "--help", "help":
		fmt.Print(usage)
		return
	default:
		fmt.Fprintf(os.Stderr, "vc: unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "vc:", err)
		os.Exit(1)
	}
}

type keyFile struct {
	PublicKey  string `json:"public_key"`
	PrivateKey string `json:"private_key"`
}

func cmdKeygen(args []string) error {
	fs := flag.NewFlagSet("keygen", flag.ExitOnError)
	out := fs.String("out", "operator.key.json", "where to write the keypair")
	fs.Parse(args)

	pub, priv, err := session.GenerateKey()
	if err != nil {
		return err
	}
	if err := writeJSON(*out, keyFile{hex.EncodeToString(pub), hex.EncodeToString(priv)}, 0o600); err != nil {
		return err
	}
	fmt.Printf("operator public key  %s\n", hex.EncodeToString(pub))
	fmt.Printf("written to           %s (mode 0600 — this file signs commitments)\n", *out)
	return nil
}

func cmdCommit(args []string) error {
	fs := flag.NewFlagSet("commit", flag.ExitOnError)
	in := fs.String("in", "", "JSON array of challenges")
	poolOut := fs.String("pool", "pool.secret.json", "salted pool (secret)")
	out := fs.String("out", "commitment.json", "public commitment")
	fs.Parse(args)
	if *in == "" {
		return fmt.Errorf("commit: -in is required")
	}

	var challenges []pool.Challenge
	if err := readJSON(*in, &challenges); err != nil {
		return err
	}
	p, err := pool.New(challenges)
	if err != nil {
		return err
	}
	if err := writeJSON(*poolOut, p, 0o600); err != nil {
		return err
	}
	c := p.Commitment()
	if err := writeJSON(*out, c, 0o644); err != nil {
		return err
	}
	fmt.Printf("challenges  %d\n", c.Size)
	fmt.Printf("root        %s\n", c.Root)
	fmt.Printf("commit      %s\n", c.Commit)
	fmt.Printf("\nTimestamp the commit value now, before any session runs:\n")
	fmt.Printf("  echo -n %s | xxd -r -p > pool.commit && ots stamp pool.commit\n", c.Commit)
	return nil
}

type sessionFile struct {
	Commitment session.Commitment `json:"commitment"`
	OperatorPK string             `json:"operator_pubkey"`
	Signature  string             `json:"signature"`
}

func cmdBegin(args []string) error {
	fs := flag.NewFlagSet("begin", flag.ExitOnError)
	commit := fs.String("commit", "commitment.json", "public pool commitment")
	key := fs.String("key", "operator.key.json", "operator signing key")
	out := fs.String("out", "session.json", "signed session commitment")
	lead := fs.Uint64("lead", session.DefaultLead, "rounds ahead to target")
	fs.Parse(args)

	var c pool.Commitment
	if err := readJSON(*commit, &c); err != nil {
		return err
	}
	var kf keyFile
	if err := readJSON(*key, &kf); err != nil {
		return err
	}
	privb, err := hex.DecodeString(kf.PrivateKey)
	if err != nil {
		return fmt.Errorf("begin: private key hex: %w", err)
	}

	chain := beacon.Quicknet
	now := time.Now()
	com, sig, err := session.Begin(chain, c, ed25519.PrivateKey(privb), *lead, now)
	if err != nil {
		return err
	}
	if err := writeJSON(*out, sessionFile{com, kf.PublicKey, hex.EncodeToString(sig)}, 0o644); err != nil {
		return err
	}
	target := chain.TimeOf(com.TargetRound)
	fmt.Printf("session       %s\n", com.SessionID)
	fmt.Printf("target round  %d\n", com.TargetRound)
	fmt.Printf("publishes at  %s (in %s)\n", target.Format(time.RFC3339), time.Until(target).Truncate(time.Second))
	fmt.Printf("\nThe challenge is now fixed but unknowable — by the candidate and by you.\n")
	return nil
}

func cmdReveal(args []string) error {
	fs := flag.NewFlagSet("reveal", flag.ExitOnError)
	sessF := fs.String("session", "session.json", "signed session commitment")
	poolF := fs.String("pool", "pool.secret.json", "salted pool")
	out := fs.String("out", "bundle.json", "verification bundle")
	wait := fs.Bool("wait", true, "wait for the target round to publish")
	fs.Parse(args)

	var sf sessionFile
	if err := readJSON(*sessF, &sf); err != nil {
		return err
	}
	var p pool.Pool
	if err := readJSON(*poolF, &p); err != nil {
		return err
	}
	loaded, err := pool.Load(p.Entries)
	if err != nil {
		return err
	}

	chain := beacon.Quicknet
	target := chain.TimeOf(sf.Commitment.TargetRound)
	if d := time.Until(target); d > 0 {
		if !*wait {
			return fmt.Errorf("reveal: round %d does not publish for another %s", sf.Commitment.TargetRound, d.Truncate(time.Second))
		}
		fmt.Printf("waiting %s for round %d...\n", d.Truncate(time.Second), sf.Commitment.TargetRound)
		time.Sleep(d + 2*time.Second)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	r, err := chain.Fetch(ctx, sf.Commitment.TargetRound)
	if err != nil {
		return err
	}
	rnd, err := r.Randomness()
	if err != nil {
		return err
	}
	idx, err := session.Index(sf.Commitment.PoolRoot, sf.Commitment.PoolSize, rnd, sf.Commitment.SessionID)
	if err != nil {
		return err
	}
	entry, path, err := loaded.Open(idx)
	if err != nil {
		return err
	}

	b := bundle.Bundle{
		Version: 1,
		Pool:    bundle.PoolRef{Root: sf.Commitment.PoolRoot, Size: sf.Commitment.PoolSize, Commit: sf.Commitment.PoolCommit},
		Session: bundle.SessionRef{Commitment: sf.Commitment, OperatorPK: sf.OperatorPK, Signature: sf.Signature},
		Beacon:  r,
		Reveal: bundle.Reveal{
			Index:      idx,
			Salt:       hex.EncodeToString(entry.Salt),
			Challenge:  entry.Challenge,
			MerklePath: path,
		},
	}
	if err := writeJSON(*out, b, 0o644); err != nil {
		return err
	}
	fmt.Printf("round      %d\n", r.Round)
	fmt.Printf("randomness %s\n", rnd)
	fmt.Printf("index      %d of %d\n", idx, sf.Commitment.PoolSize)
	fmt.Printf("challenge  %s — %s\n", entry.Challenge.ID, entry.Challenge.Title)
	fmt.Printf("bundle     %s\n", *out)
	return nil
}

func cmdVerify(args []string) error {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "emit the report as JSON")
	fs.Parse(args)
	if fs.NArg() != 1 {
		return fmt.Errorf("verify: expected exactly one bundle file")
	}

	f, err := os.Open(fs.Arg(0))
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := bundle.Read(f)
	if err != nil {
		return err
	}
	rep := bundle.Verify(b, beacon.Quicknet)

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rep); err != nil {
			return err
		}
	} else {
		for _, c := range rep.Checks {
			mark := "FAIL"
			switch {
			case c.Skipped:
				mark = "SKIP"
			case c.OK:
				mark = "ok  "
			}
			fmt.Printf("  %s  %-48s  %s\n", mark, c.Name, c.Detail)
		}
		fmt.Println()
		if rep.OK {
			fmt.Printf("VERIFIED — challenge %q was fixed in advance and selected unpredictably\n", b.Reveal.Challenge.ID)
			if rep.Skipped > 0 {
				fmt.Printf("%d check(s) skipped; see SKIP lines above\n", rep.Skipped)
			}
		} else {
			fmt.Println("FAILED — this bundle does not prove what it claims")
		}
	}
	if !rep.OK {
		os.Exit(1)
	}
	return nil
}

func cmdBeacon(args []string) error {
	fs := flag.NewFlagSet("beacon", flag.ExitOnError)
	fs.Parse(args)

	var round uint64
	if fs.NArg() == 1 {
		if _, err := fmt.Sscanf(fs.Arg(0), "%d", &round); err != nil {
			return fmt.Errorf("beacon: bad round number %q", fs.Arg(0))
		}
	}
	chain := beacon.Quicknet
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	r, err := chain.Fetch(ctx, round)
	if err != nil {
		return err
	}
	rnd, err := r.Randomness()
	if err != nil {
		return err
	}
	fmt.Printf("round      %d\n", r.Round)
	fmt.Printf("published  %s\n", chain.TimeOf(r.Round).Format(time.RFC3339))
	fmt.Printf("randomness %s\n", rnd)
	fmt.Printf("signature  verified against %s group key\n", chain.Name)
	return nil
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func writeJSON(path string, v any, mode os.FileMode) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), mode)
}

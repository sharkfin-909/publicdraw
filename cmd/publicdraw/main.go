// Command publicdraw runs and verifies public draws.
//
// It has no CLI framework dependency on purpose: the verifier is meant to be a
// small static binary someone can audit and run without trusting where it came
// from.
package main

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/sharkfin-909/publicdraw/beacon"
	"github.com/sharkfin-909/publicdraw/bundle"
	"github.com/sharkfin-909/publicdraw/httpapi"
	"github.com/sharkfin-909/publicdraw/rekor"
	"github.com/sharkfin-909/publicdraw/roster"
	"github.com/sharkfin-909/publicdraw/service"
	"github.com/sharkfin-909/publicdraw/translog"
)

const (
	organiserTokenEnv = "PUBLICDRAW_ORGANISER_TOKEN"
	minOrganiserToken = 32
)

const usage = `publicdraw — publicly verifiable draws

organiser:
  publicdraw keygen  [-out FILE]
  publicdraw create  -title T [-winners N] [-disclosure public|private] -open 10m
  publicdraw enter   -draw ID (-label L | -file FILE) [-salts FILE]
  publicdraw leaves  -draw ID
  publicdraw anchor  -draw ID                        retry a frozen draw's Rekor anchor
  publicdraw commit  -draw ID
  publicdraw run     -draw ID [-wait]
  publicdraw receipt -draw ID -index N [-out FILE]
  publicdraw serve   [-addr :8080] [-no-auth]   needs PUBLICDRAW_ORGANISER_TOKEN

anyone:
  publicdraw verify  BUNDLE [-json] [-timeline]
  publicdraw check-receipt RECEIPT
  publicdraw log     [-abandoned]
  publicdraw beacon  [ROUND]

global:
  -dir DIR   service directory (default ./vcdata)
  -key FILE  operator key (default DIR/operator.key.json)
`

var (
	dirFlag   = flag.String("dir", "vcdata", "service directory")
	keyFlag   = flag.String("key", "", "operator key file")
	rekorFlag = flag.String("rekor", rekor.DefaultEndpoint, "Rekor endpoint, or empty to disable anchoring")
)

func main() {
	// Global flags are accepted before the subcommand as well as after it,
	// because the help advertises them as global and flag.Parse stops at the
	// first non-flag argument, which is the subcommand.
	global := flag.NewFlagSet("publicdraw", flag.ContinueOnError)
	global.SetOutput(io.Discard)
	global.StringVar(dirFlag, "dir", *dirFlag, "service directory")
	global.StringVar(keyFlag, "key", *keyFlag, "operator key file")
	global.StringVar(rekorFlag, "rekor", *rekorFlag, "Rekor endpoint")
	_ = global.Parse(os.Args[1:])

	args := global.Args()
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	cmd := args[0]
	rest := args[1:]

	var err error
	switch cmd {
	case "keygen":
		err = cmdKeygen(rest)
	case "create":
		err = cmdCreate(rest)
	case "enter":
		err = cmdEnter(rest)
	case "anchor":
		err = cmdAnchor(rest)
	case "leaves":
		err = cmdLeaves(rest)
	case "commit":
		err = cmdCommit(rest)
	case "run":
		err = cmdRun(rest)
	case "receipt":
		err = cmdReceipt(rest)
	case "serve":
		err = cmdServe(rest)
	case "verify":
		err = cmdVerify(rest)
	case "check-receipt":
		err = cmdCheckReceipt(rest)
	case "log":
		err = cmdLog(rest)
	case "beacon":
		err = cmdBeacon(rest)
	case "-h", "--help", "help":
		fmt.Print(usage)
		return
	default:
		fmt.Fprintf(os.Stderr, "publicdraw: unknown command %q\n\n%s", cmd, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "publicdraw:", err)
		os.Exit(1)
	}
}

type keyFile struct {
	PublicKey  string `json:"public_key"`
	PrivateKey string `json:"private_key"`

	// AnchorKey is an ECDSA P-256 key used only for Rekor submissions. It is
	// a second key rather than the operator key because Rekor's hashedrekord
	// type signs a digest and rejects Ed25519.
	AnchorKey string `json:"anchor_key,omitempty"`
}

func keyPath() string {
	if *keyFlag != "" {
		return *keyFlag
	}
	return *dirFlag + "/operator.key.json"
}

func openService() (*service.Service, error) {
	var kf keyFile
	if err := readJSON(keyPath(), &kf); err != nil {
		return nil, fmt.Errorf("%w (run: publicdraw keygen)", err)
	}
	priv, err := hex.DecodeString(kf.PrivateKey)
	if err != nil {
		return nil, err
	}
	opts := []service.Option{}
	// An empty -rekor turns anchoring off. rekor.New("") would quietly fall
	// back to the public instance, so it must not be reached.
	if kf.AnchorKey != "" && *rekorFlag != "" {
		der, err := hex.DecodeString(kf.AnchorKey)
		if err != nil {
			return nil, fmt.Errorf("anchor key hex: %w", err)
		}
		ak, err := x509.ParseECPrivateKey(der)
		if err != nil {
			return nil, fmt.Errorf("anchor key: %w", err)
		}
		opts = append(opts, service.WithRekor(rekor.New(*rekorFlag), ak))
	}
	return service.New(*dirFlag, "vc/log/"+kf.PublicKey[:16], ed25519.PrivateKey(priv), opts...)
}

func cmdKeygen(args []string) error {
	fs := newFlags("keygen", args)
	out := fs.String("out", "", "where to write the keypair")
	parse(fs, args)

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return err
	}
	anchor, err := rekor.GenerateKey()
	if err != nil {
		return err
	}
	anchorDER, err := x509.MarshalECPrivateKey(anchor)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(*dirFlag, 0o755); err != nil {
		return err
	}
	path := *out
	if path == "" {
		path = keyPath()
	}
	kf := keyFile{
		PublicKey:  hex.EncodeToString(pub),
		PrivateKey: hex.EncodeToString(priv),
		AnchorKey:  hex.EncodeToString(anchorDER),
	}
	if err := writeJSON(path, kf, 0o600); err != nil {
		return err
	}
	fmt.Printf("operator public key  %s\n", hex.EncodeToString(pub))
	fmt.Printf("anchor key           ecdsa p-256, for Rekor submissions\n")
	fmt.Printf("written to           %s (mode 0600)\n", path)
	return nil
}

func cmdCreate(args []string) error {
	fs := newFlags("create", args)
	title := fs.String("title", "", "what this draw is for")
	winners := fs.Int("winners", 1, "how many entries to select")
	disc := fs.String("disclosure", "private", "public (publish every entry) or private (winners only)")
	open := fs.Duration("open", 0, "how long entries stay open")
	cutoff := fs.Int64("cutoff", 0, "cutoff as a unix timestamp")
	parse(fs, args)

	if *title == "" {
		return fmt.Errorf("create: -title is required")
	}
	if *cutoff == 0 && *open == 0 {
		return fmt.Errorf("create: pass -open DURATION or -cutoff UNIXTIME")
	}
	svc, err := openService()
	if err != nil {
		return err
	}
	var st *service.Stored
	if *cutoff == 0 {
		st, err = svc.CreateOpenFor(*title, *winners, roster.Disclosure(*disc), *open)
	} else {
		st, err = svc.Create(*title, *winners, roster.Disclosure(*disc), time.Unix(*cutoff, 0))
	}
	if err != nil {
		return err
	}
	chain := svc.Chain()
	fmt.Printf("draw         %s\n", st.Params.DrawID)
	fmt.Printf("title        %s\n", st.Params.Title)
	fmt.Printf("winners      %d\n", st.Params.Winners)
	fmt.Printf("disclosure   %s\n", st.Params.Disclosure)
	fmt.Printf("entries close %s\n", time.Unix(st.Params.CutoffAt, 0).Format(time.RFC3339))
	fmt.Printf("target round %d at %s\n", st.Params.TargetRound, chain.TimeOf(st.Params.TargetRound).Format(time.RFC3339))
	fmt.Printf("\nPublish this draw id now. The parameters are logged and signed;\n")
	fmt.Printf("changing any of them later produces a different draw.\n")
	return nil
}

func cmdEnter(args []string) error {
	fs := newFlags("enter", args)
	id := fs.String("draw", "", "draw id")
	label := fs.String("label", "", "a single entry")
	file := fs.String("file", "", "file of entries, one per line")
	salts := fs.String("salts", "", "append index/salt/label rows here as entries are admitted")
	parse(fs, args)

	var labels []string
	if *label != "" {
		labels = append(labels, *label)
	}
	if *file != "" {
		f, err := os.Open(*file)
		if err != nil {
			return err
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			if l := strings.TrimSpace(sc.Text()); l != "" {
				labels = append(labels, l)
			}
		}
		if err := sc.Err(); err != nil {
			return err
		}
	}
	if len(labels) == 0 {
		return fmt.Errorf("enter: pass -label or -file")
	}
	svc, err := openService()
	if err != nil {
		return err
	}
	admitted, err := svc.Enter(*id, labels...)
	if err != nil {
		return err
	}

	// Each entrant's salt is printed once, here. Holding it from the moment
	// they enter is what lets them check their own inclusion against the
	// published leaves later, with nothing needing to reach them at the
	// cutoff — so this is worth handing on rather than discarding.
	if *salts != "" {
		f, err := os.OpenFile(*salts, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		defer f.Close()
		for _, a := range admitted {
			if _, err := fmt.Fprintf(f, "%d\t%s\t%s\n", a.Index, a.Salt, a.Label); err != nil {
				return err
			}
		}
	}

	fmt.Printf("added %d entries\n\n", len(admitted))
	for _, a := range admitted {
		fmt.Printf("  #%-4d %s  %s\n", a.Index, a.Salt, a.Label)
	}
	fmt.Printf("\nGive each entrant their index and salt. It is what lets them prove\n")
	fmt.Printf("their own inclusion later, whether or not a receipt ever reaches them.\n")
	if *salts != "" {
		fmt.Printf("Appended to %s\n", *salts)
	}
	return nil
}

// cmdLeaves prints every committed leaf. An entrant with the salt they were
// given at entry can hash their own label and find it here — no receipt, and
// no cooperation from the organiser beyond publishing this list.
func cmdLeaves(args []string) error {
	fs := newFlags("leaves", args)
	id := fs.String("draw", "", "draw id")
	parse(fs, args)

	svc, err := openService()
	if err != nil {
		return err
	}
	ll, err := svc.Leaves(*id)
	if err != nil {
		return err
	}
	for i, l := range ll.Leaves {
		fmt.Printf("%d\t%s\n", i, l)
	}
	fmt.Fprintf(os.Stderr, "\n%d leaves, root %s\n", ll.Size, ll.Root)
	return nil
}

// cmdAnchor anchors a frozen draw's commit head into Sigstore's Rekor again,
// for when Rekor was unreachable at the freeze. Freezing anchors on its own;
// this is only the retry. It anchors the head the log had right after the
// draw's roster record, so the anchor still says what the roster was — but a
// verifier credits it only if Rekor's witnesses saw it before the target round.
func cmdAnchor(args []string) error {
	fs := newFlags("anchor", args)
	id := fs.String("draw", "", "draw id")
	parse(fs, args)

	svc, err := openService()
	if err != nil {
		return err
	}
	if !svc.AnchorsEnabled() {
		return fmt.Errorf("anchor: anchoring is off (no anchor key in %s, or -rekor is empty)", keyPath())
	}
	f, err := svc.Reanchor(context.Background(), *id)
	if err != nil {
		return err
	}
	printAnchor(f)
	return nil
}

// printAnchor describes a freeze's anchor, or why there is none.
func printAnchor(f service.Frozen) {
	switch {
	case f.AnchorErr != nil:
		// Not fatal. A draw with no anchor is still a valid draw; it just
		// rests more on the organiser's own word, and verification says so.
		fmt.Printf("rekor      anchoring failed: %v\n", f.AnchorErr)
	case f.Anchor != nil:
		cp, err := f.Anchor.ParseCheckpoint()
		if err != nil {
			fmt.Printf("rekor      index %d (checkpoint unreadable: %v)\n", f.Anchor.LogIndex, err)
			return
		}
		fmt.Printf("rekor      %s index %d of %d, inclusion proof verifies locally", cp.Origin, f.Anchor.LogIndex, f.Anchor.TreeSize)
		if at, ok := cp.EarliestCosignature(); ok {
			fmt.Printf(", witnessed %s", at.Format(time.RFC3339))
		}
		fmt.Println()
	}
}

func cmdCommit(args []string) error {
	fs := newFlags("commit", args)
	id := fs.String("draw", "", "draw id")
	parse(fs, args)

	svc, err := openService()
	if err != nil {
		return err
	}
	f, err := svc.Freeze(context.Background(), *id)
	if err != nil {
		return err
	}
	st := f.Draw
	target := svc.Chain().TimeOf(st.Params.TargetRound)
	fmt.Printf("entries    %d\n", st.RosterCommit.Size)
	fmt.Printf("root       %s\n", st.RosterCommit.Root)
	fmt.Printf("commit     %s\n", st.RosterCommit.Commit)
	fmt.Printf("log head   %s\n", f.Head)
	fmt.Printf("draws at   round %d, in %s\n", st.Params.TargetRound, time.Until(target).Truncate(time.Second))
	printAnchor(f)

	fmt.Printf("\nThe entry list is now frozen and the randomness does not exist yet.\n")
	fmt.Printf("Timestamp the log head to prove that to someone who distrusts your clock:\n")
	fmt.Printf("  echo -n %s | xxd -r -p > log.head && ots stamp log.head\n", f.Head)
	return nil
}

func cmdRun(args []string) error {
	fs := newFlags("run", args)
	id := fs.String("draw", "", "draw id")
	out := fs.String("out", "", "write the bundle here (default DIR/bundle-ID.json)")
	wait := fs.Bool("wait", true, "wait for the target round")
	parse(fs, args)

	svc, err := openService()
	if err != nil {
		return err
	}
	st, err := svc.Get(*id)
	if err != nil {
		return err
	}
	target := svc.Chain().TimeOf(st.Params.TargetRound)
	if d := time.Until(target); d > 0 {
		if !*wait {
			return fmt.Errorf("run: round %d publishes in %s", st.Params.TargetRound, d.Truncate(time.Second))
		}
		fmt.Printf("waiting %s for round %d...\n", d.Truncate(time.Second), st.Params.TargetRound)
		time.Sleep(d + 2*time.Second)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	b, err := svc.Run(ctx, *id)
	if err != nil {
		return err
	}
	path := *out
	if path == "" {
		path = fmt.Sprintf("%s/bundle-%s.json", *dirFlag, *id)
	}
	if err := writeJSON(path, b, 0o644); err != nil {
		return err
	}
	fmt.Printf("round      %d\n", b.Beacon.Round)
	fmt.Printf("seed       %s\n", b.Result.Seed)
	for _, w := range b.Result.Winners {
		fmt.Printf("winner %-2d  #%d  %s\n", w.Rank, w.Index, w.Entry.Label)
	}
	if n := len(b.Result.Waitlist); n > 0 {
		fmt.Printf("waitlist   %d more, in order\n", n)
	}
	fmt.Printf("bundle     %s\n", path)
	return nil
}

func cmdReceipt(args []string) error {
	fs := newFlags("receipt", args)
	id := fs.String("draw", "", "draw id")
	index := fs.Int("index", -1, "entry index")
	out := fs.String("out", "", "write the receipt here")
	parse(fs, args)

	svc, err := openService()
	if err != nil {
		return err
	}
	rec, err := svc.Receipt(*id, *index)
	if err != nil {
		return err
	}
	if *out != "" {
		if err := writeJSON(*out, rec, 0o644); err != nil {
			return err
		}
	}
	fmt.Printf("entry %d of %d — %s\n", rec.Index, rec.Size, rec.Entry.Label)
	fmt.Printf("root  %s\n", rec.Root)
	if *out != "" {
		fmt.Printf("saved %s\n", *out)
	}
	return nil
}

func cmdServe(args []string) error {
	fs := newFlags("serve", args)
	addr := fs.String("addr", ":8080", "listen address")
	noAuth := fs.Bool("no-auth", false, "leave organiser routes open, for local development")
	parse(fs, args)

	token := os.Getenv(organiserTokenEnv)
	if !*noAuth && len(token) < minOrganiserToken {
		return fmt.Errorf("serve: set %s to at least %d characters, or pass -no-auth for local development",
			organiserTokenEnv, minOrganiserToken)
	}
	if *noAuth {
		token = ""
	}

	svc, err := openService()
	if err != nil {
		return err
	}
	srv := &http.Server{
		Addr:              *addr,
		Handler:           httpapi.New(svc, httpapi.WithOrganiserToken(token)),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      2 * time.Minute,
		IdleTimeout:       2 * time.Minute,
	}
	// Each draw is frozen, and anchored, as its cutoff passes. Checked every
	// second: a draw has about 15 seconds to freeze in.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go svc.KeepFreezing(ctx, time.Second, func(frozen []service.Frozen, err error) {
		for _, f := range frozen {
			fmt.Printf("froze %s\n", f.Draw.Params.DrawID)
			printAnchor(f)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "publicdraw:", err)
		}
	})
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()

	fmt.Printf("operator  %s\n", svc.OperatorKey())
	fmt.Printf("log head  %s\n", svc.Log().Head())
	fmt.Printf("anchoring %v\n", svc.AnchorsEnabled())
	fmt.Printf("listening on %s\n", *addr)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func cmdVerify(args []string) error {
	fs := newFlags("verify", args)
	asJSON := fs.Bool("json", false, "emit the report as JSON")
	timeline := fs.Bool("timeline", false, "also show the asserted ordering")
	parse(fs, args)
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
		if !rep.OK {
			os.Exit(1)
		}
		return nil
	}

	if *timeline {
		fmt.Printf("\n%s\n", bundle.Timeline(b, beacon.Quicknet))
	}
	for _, c := range rep.Checks {
		mark := "FAIL"
		switch {
		case c.Skipped:
			mark = "SKIP"
		case c.OK:
			mark = "ok  "
		}
		fmt.Printf("  %s  %-52s  %s\n", mark, c.Name, c.Detail)
	}
	fmt.Println()
	if !rep.OK {
		fmt.Println("FAILED — this bundle does not prove what it claims")
		os.Exit(1)
	}
	fmt.Printf("VERIFIED — %q, %d winner(s) of %d entries\n",
		b.Draw.Params.Title, len(b.Result.Winners), b.Roster.Commitment.Size)
	for _, w := range b.Result.Winners {
		fmt.Printf("  %d. #%d  %s\n", w.Rank, w.Index, w.Entry.Label)
	}
	if rep.Skipped > 0 {
		fmt.Printf("\n%d check(s) skipped — see SKIP lines above\n", rep.Skipped)
	}
	return nil
}

func cmdCheckReceipt(args []string) error {
	fs := newFlags("check-receipt", args)
	parse(fs, args)
	if fs.NArg() != 1 {
		return fmt.Errorf("check-receipt: expected exactly one receipt file")
	}
	var rec roster.Receipt
	if err := readJSON(fs.Arg(0), &rec); err != nil {
		return err
	}
	salt, err := hex.DecodeString(rec.Salt)
	if err != nil {
		return err
	}
	if !roster.VerifyProof(rec.Proof(), salt, rec.Entry.Label) {
		fmt.Printf("FAILED — %q is not entry %d of the roster rooted at %s\n", rec.Entry.Label, rec.Index, rec.Root)
		os.Exit(1)
	}
	fmt.Printf("VERIFIED — %q is entry %d of %d in the roster rooted at\n  %s\n",
		rec.Entry.Label, rec.Index, rec.Size, rec.Root)
	fmt.Printf("\nCheck that root against the draw's published bundle to confirm it is\n")
	fmt.Printf("the roster the winners were drawn from.\n")
	return nil
}

func cmdLog(args []string) error {
	fs := newFlags("log", args)
	abandoned := fs.Bool("abandoned", false, "list draws logged at creation with no published result")
	parse(fs, args)

	svc, err := openService()
	if err != nil {
		return err
	}
	l := svc.Log()
	if *abandoned {
		ids, err := svc.Abandoned()
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			fmt.Println("no abandoned draws")
			return nil
		}
		fmt.Printf("%d draw(s) whose round has published but whose result has not:\n", len(ids))
		for _, id := range ids {
			fmt.Printf("  %s\n", id)
		}
		fmt.Printf("\nThis is not proof of misconduct — draws get cancelled for honest\n")
		fmt.Printf("reasons — but a retry attack can only show up here.\n")
		return nil
	}
	for _, r := range l.Records() {
		fmt.Printf("  %4d  %-17s  %s  %s\n", r.Seq, r.Kind, r.DrawID, time.Unix(r.LoggedAt, 0).Format(time.RFC3339))
	}
	fmt.Printf("\n%d record(s), head %s\n", l.Len(), l.Head())
	if _, err := translog.Verify(l.Records(), "vc/log/"+svc.OperatorKey()[:16]); err != nil {
		return err
	}
	fmt.Println("chain verified from genesis")
	return nil
}

func cmdBeacon(args []string) error {
	fs := newFlags("beacon", args)
	parse(fs, args)

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
	fmt.Printf("signature  verified against the %s group key\n", chain.Name)
	return nil
}

// newFlags builds a flag set that also carries the global options, so -dir and
// -key work after the subcommand name where people expect them.
func newFlags(name string, _ []string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	fs.StringVar(dirFlag, "dir", *dirFlag, "service directory")
	fs.StringVar(keyFlag, "key", *keyFlag, "operator key file")
	fs.StringVar(rekorFlag, "rekor", *rekorFlag, "Rekor endpoint")
	return fs
}

func parse(fs *flag.FlagSet, args []string) { _ = fs.Parse(args) }

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

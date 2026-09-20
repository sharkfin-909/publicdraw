package vc_test

// testdata/testvectors.json is the contract between this implementation and
// any other. A cross-language disagreement on a single byte breaks the
// protocol silently — one side computes a digest, the other computes a
// different one, and each is internally consistent. Pinning the values here
// makes that disagreement fail loudly instead.
//
// Regenerate after an intentional protocol change:  go test -run TestVectors -update .

import (
	"encoding/hex"
	"encoding/json"
	"flag"
	"os"
	"testing"

	"github.com/sharkfin009/verifiable-challenge/beacon"
	"github.com/sharkfin009/verifiable-challenge/draw"
	"github.com/sharkfin009/verifiable-challenge/hashing"
	"github.com/sharkfin009/verifiable-challenge/merkle"
	"github.com/sharkfin009/verifiable-challenge/roster"
	"github.com/sharkfin009/verifiable-challenge/translog"
)

var update = flag.Bool("update", false, "rewrite testdata/testvectors.json")

const path = "testdata/testvectors.json"

type vectors struct {
	Note   string            `json:"_note"`
	Tags   map[string]string `json:"tags"`
	LP     []lp              `json:"length_prefix"`
	Tagged []tagged          `json:"tagged_hash"`
	Leaf   []leaf            `json:"roster_leaf"`
	Node   node              `json:"merkle_node"`
	Roots  []root            `json:"merkle_roots"`
	Entry  []entryVec        `json:"entry_canonical"`
	Commit []commit          `json:"roster_commit"`
	Seed   []seedVec         `json:"draw_seed"`
	Rank   []rankVec         `json:"draw_ranking"`
	Log    []logVec          `json:"log_chain"`
	Beacon beaconVec         `json:"beacon"`
}

type lp struct {
	In  string `json:"in_hex"`
	Out string `json:"out_hex"`
}
type tagged struct {
	Tag   string   `json:"tag"`
	Parts []string `json:"parts_hex"`
	Out   string   `json:"out"`
}
type leaf struct {
	Salt  string `json:"salt_hex"`
	Entry string `json:"entry_hex"`
	Out   string `json:"leaf"`
}
type node struct {
	Left, Right, Out string
}
type root struct {
	Size int    `json:"size"`
	Root string `json:"root"`
}
type entryVec struct {
	Label     string `json:"label"`
	Canonical string `json:"canonical_hex"`
	Leaf      string `json:"leaf_with_zero_salt"`
}
type commit struct {
	Root string `json:"root"`
	Size int    `json:"size"`
	Out  string `json:"commit"`
}
type seedVec struct {
	Root       string `json:"roster_root"`
	Size       int    `json:"roster_size"`
	Randomness string `json:"randomness"`
	DrawID     string `json:"draw_id"`
	Seed       string `json:"seed"`
}
type rankVec struct {
	Seed     string `json:"seed"`
	Size     int    `json:"size"`
	Winners  int    `json:"winners"`
	Won      []int  `json:"won"`
	Waitlist []int  `json:"waitlist"`
}
type logVec struct {
	LogID    string `json:"log_id"`
	Seq      uint64 `json:"seq"`
	Kind     string `json:"kind"`
	DrawID   string `json:"draw_id"`
	Payload  string `json:"payload"`
	LoggedAt int64  `json:"logged_at"`
	Prev     string `json:"prev"`
	Head     string `json:"head"`
}
type beaconVec struct {
	Round       uint64 `json:"round"`
	Signature   string `json:"signature"`
	Randomness  string `json:"randomness"`
	PublishedAt int64  `json:"published_unix"`
}

func h(s string) []byte { b, _ := hex.DecodeString(s); return b }

func generate() vectors {
	v := vectors{
		Note: "Cross-implementation contract. Every value must be reproduced byte-for-byte by any other implementation.",
		Tags: map[string]string{
			"leaf": hashing.TagLeaf, "node": hashing.TagNode, "entry": hashing.TagEntry,
			"roster": hashing.TagRoster, "draw": hashing.TagDraw, "roster_sig": hashing.TagRosterSig,
			"seed": hashing.TagSeed, "ticket": hashing.TagTicket,
			"log_entry": hashing.TagLogEntry, "log_genesis": hashing.TagLogGenesis,
		},
	}

	for _, in := range []string{"", "00", "616263", "ff00ff"} {
		v.LP = append(v.LP, lp{in, hex.EncodeToString(hashing.LP(h(in)))})
	}

	for _, tc := range []struct {
		tag   string
		parts []string
	}{
		{hashing.TagLeaf, []string{"616263"}},
		{hashing.TagSeed, []string{"6162", "63646566"}},
		{hashing.TagSeed, []string{"616263", "646566"}},
		{hashing.TagNode, []string{}},
	} {
		parts := make([][]byte, len(tc.parts))
		for i, p := range tc.parts {
			parts[i] = h(p)
		}
		v.Tagged = append(v.Tagged, tagged{tc.tag, tc.parts, hashing.Tagged(tc.tag, parts...).String()})
	}

	zero := make([]byte, roster.SaltSize)
	for _, e := range []string{"", "68656c6c6f"} {
		v.Leaf = append(v.Leaf, leaf{hex.EncodeToString(zero), e, merkle.LeafHash(zero, h(e)).String()})
	}

	l := hashing.Tagged("vc/testvec", []byte("left"))
	r := hashing.Tagged("vc/testvec", []byte("right"))
	v.Node = node{l.String(), r.String(), merkle.NodeHash(l, r).String()}

	for _, n := range []int{1, 2, 3, 4, 5, 7, 8, 9, 16, 17} {
		ls := make([]hashing.Digest, n)
		for i := range ls {
			ls[i] = hashing.Tagged("vc/testvec", hashing.U32(uint32(i)))
		}
		tr, _ := merkle.New(ls)
		v.Roots = append(v.Roots, root{n, tr.Root().String()})
	}

	for _, label := range []string{"", "alice@example.com", "ab", "abc"} {
		e := roster.Entry{Label: label}
		cb := e.CanonicalBytes()
		v.Entry = append(v.Entry, entryVec{label, hex.EncodeToString(cb), merkle.LeafHash(zero, cb).String()})
	}

	rroot := hashing.Tagged("vc/testvec", []byte("roster root"))
	for _, n := range []int{1, 5, 1000} {
		v.Commit = append(v.Commit, commit{rroot.String(), n, roster.CommitValue(rroot, n).String()})
	}

	rnd := hashing.Tagged("vc/testvec", []byte("randomness"))
	for _, tc := range []struct {
		size int
		id   string
	}{{1, "d"}, {5, "3f1c9a77b2e04d18"}, {1000, "draw-0"}, {64, ""}} {
		s := draw.Seed(rroot, tc.size, rnd, tc.id)
		v.Seed = append(v.Seed, seedVec{rroot.String(), tc.size, rnd.String(), tc.id, s.String()})
	}

	for _, tc := range []struct{ size, winners int }{{5, 1}, {10, 3}, {17, 5}, {64, 2}} {
		s := draw.Seed(rroot, tc.size, rnd, "draw-fixed")
		won, wait, err := draw.Select(s, tc.size, tc.winners)
		if err != nil {
			panic(err)
		}
		if len(wait) > 8 {
			wait = wait[:8]
		}
		v.Rank = append(v.Rank, rankVec{s.String(), tc.size, tc.winners, won, wait})
	}

	const logID = "vc/testvec/log"
	head := translog.Genesis(logID)
	v.Log = append(v.Log, logVec{LogID: logID, Seq: 0, Kind: "genesis", Head: head.String()})
	for i, k := range []translog.Kind{translog.KindDrawCreated, translog.KindRosterCommitted, translog.KindResultPublished} {
		payload := hashing.Tagged("vc/testvec", []byte(k))
		prev := head
		head = translog.HeadAfter(prev, uint64(i), k, "draw-fixed", payload, int64(1700000000+i))
		v.Log = append(v.Log, logVec{
			LogID: logID, Seq: uint64(i), Kind: string(k), DrawID: "draw-fixed",
			Payload: payload.String(), LoggedAt: int64(1700000000 + i),
			Prev: prev.String(), Head: head.String(),
		})
	}

	br := beacon.Round{Round: 20000000, Signature: "96892582a33552a7b67ba44ef09c3ccd535bbebe760c93ecf45be8958d0c0f06390c6d19d7bf492eb806af7eef6b125c"}
	brnd, _ := br.Randomness()
	v.Beacon = beaconVec{br.Round, br.Signature, brnd.String(), beacon.Quicknet.TimeOf(br.Round).Unix()}

	return v
}

func TestVectors(t *testing.T) {
	got := generate()
	if *update {
		b, err := json.MarshalIndent(got, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", path)
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run: go test -run TestVectors -update .)", err)
	}
	var want vectors
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	g, _ := json.Marshal(got)
	w, _ := json.Marshal(want)
	if string(g) != string(w) {
		t.Fatal("implementation no longer matches testdata/testvectors.json — a breaking protocol change; if intended, regenerate with -update and update every other implementation")
	}
}

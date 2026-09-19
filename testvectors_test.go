package vc_test

// The vectors in testdata/testvectors.json are the contract between this
// implementation and any other (the browser client is written in TypeScript).
// A cross-language disagreement on a single byte breaks the protocol silently,
// so both sides reproduce this file rather than trusting each other.
//
// Regenerate with: go test -run TestVectors -update

import (
	"encoding/hex"
	"encoding/json"
	"flag"
	"os"
	"testing"

	"github.com/sharkfin009/verifiable-challenge/beacon"
	"github.com/sharkfin009/verifiable-challenge/hashing"
	"github.com/sharkfin009/verifiable-challenge/merkle"
	"github.com/sharkfin009/verifiable-challenge/pool"
	"github.com/sharkfin009/verifiable-challenge/session"
)

var update = flag.Bool("update", false, "rewrite testdata/testvectors.json")

const vectorPath = "testdata/testvectors.json"

type vectors struct {
	Note      string            `json:"_note"`
	Tags      map[string]string `json:"tags"`
	LP        []lpCase          `json:"length_prefix"`
	Tagged    []taggedCase      `json:"tagged_hash"`
	Leaf      []leafCase        `json:"pool_leaf"`
	Node      nodeCase          `json:"merkle_node"`
	Roots     []rootCase        `json:"merkle_roots"`
	Canonical []canonCase       `json:"challenge_canonical"`
	Commit    []commitCase      `json:"pool_commit"`
	Index     []indexCase       `json:"select_index"`
	Beacon    beaconCase        `json:"beacon"`
}

type lpCase struct {
	In  string `json:"in_hex"`
	Out string `json:"out_hex"`
}
type taggedCase struct {
	Tag   string   `json:"tag"`
	Parts []string `json:"parts_hex"`
	Out   string   `json:"out"`
}
type leafCase struct {
	Salt  string `json:"salt_hex"`
	Entry string `json:"entry_hex"`
	Out   string `json:"leaf"`
}
type nodeCase struct {
	Left  string `json:"left"`
	Right string `json:"right"`
	Out   string `json:"node"`
}
type rootCase struct {
	Size int    `json:"size"`
	Root string `json:"root"`
}
type canonCase struct {
	Challenge pool.Challenge `json:"challenge"`
	Canonical string         `json:"canonical_hex"`
	Leaf      string         `json:"leaf_with_zero_salt"`
}
type commitCase struct {
	Root string `json:"root"`
	Size int    `json:"size"`
	Out  string `json:"commit"`
}
type indexCase struct {
	Root       string `json:"pool_root"`
	Size       int    `json:"pool_size"`
	Randomness string `json:"randomness"`
	SessionID  string `json:"session_id"`
	Index      int    `json:"index"`
}
type beaconCase struct {
	Round      uint64 `json:"round"`
	Signature  string `json:"signature"`
	Randomness string `json:"randomness"`
	TimeUnix   int64  `json:"published_unix"`
}

func h(s string) []byte { b, _ := hex.DecodeString(s); return b }

func generate() vectors {
	v := vectors{
		Note: "Cross-implementation contract. Every value here must be reproduced byte-for-byte by any other implementation of this protocol.",
		Tags: map[string]string{
			"leaf": hashing.TagLeaf, "node": hashing.TagNode, "commit": hashing.TagCommit,
			"challenge": hashing.TagChallenge, "select": hashing.TagSelect, "session": hashing.TagSession,
		},
	}

	for _, in := range []string{"", "00", "616263", "ff00ff"} {
		v.LP = append(v.LP, lpCase{In: in, Out: hex.EncodeToString(hashing.LP(h(in)))})
	}

	for _, tc := range []struct {
		tag   string
		parts []string
	}{
		{hashing.TagLeaf, []string{"616263"}},
		{hashing.TagSelect, []string{"6162", "63646566"}},
		{hashing.TagSelect, []string{"616263", "646566"}},
		{hashing.TagNode, []string{}},
	} {
		parts := make([][]byte, len(tc.parts))
		for i, p := range tc.parts {
			parts[i] = h(p)
		}
		v.Tagged = append(v.Tagged, taggedCase{tc.tag, tc.parts, hashing.Tagged(tc.tag, parts...).String()})
	}

	zeroSalt := make([]byte, pool.SaltSize)
	for _, e := range []string{"", "68656c6c6f"} {
		v.Leaf = append(v.Leaf, leafCase{
			Salt: hex.EncodeToString(zeroSalt), Entry: e,
			Out: merkle.LeafHash(zeroSalt, h(e)).String(),
		})
	}

	l := hashing.Tagged("vc/testvec", []byte("left"))
	r := hashing.Tagged("vc/testvec", []byte("right"))
	v.Node = nodeCase{l.String(), r.String(), merkle.NodeHash(l, r).String()}

	for _, n := range []int{1, 2, 3, 4, 5, 7, 8, 9, 16, 17} {
		ls := make([]hashing.Digest, n)
		for i := range ls {
			ls[i] = hashing.Tagged("vc/testvec", hashing.U32(uint32(i)))
		}
		tr, _ := merkle.New(ls)
		v.Roots = append(v.Roots, rootCase{n, tr.Root().String()})
	}

	for _, c := range []pool.Challenge{
		{},
		{ID: "lru-cache", Language: "go", Title: "LRU cache", Prompt: "Implement an LRU cache.", StarterCode: "package main\n"},
		{ID: "ab", Language: "cd"},
		{ID: "abc", Language: "d"},
	} {
		cb := c.CanonicalBytes()
		v.Canonical = append(v.Canonical, canonCase{c, hex.EncodeToString(cb), merkle.LeafHash(zeroSalt, cb).String()})
	}

	root := hashing.Tagged("vc/testvec", []byte("pool root"))
	for _, n := range []int{1, 5, 1000} {
		v.Commit = append(v.Commit, commitCase{root.String(), n, pool.CommitValue(root, n).String()})
	}

	rnd := hashing.Tagged("vc/testvec", []byte("randomness"))
	for _, tc := range []struct {
		size int
		sid  string
	}{{1, "s"}, {5, "3f1c9a77b2e04d18"}, {1000, "session-0"}, {1000, "session-1"}, {64, ""}} {
		idx, _ := session.Index(root, tc.size, rnd, tc.sid)
		v.Index = append(v.Index, indexCase{root.String(), tc.size, rnd.String(), tc.sid, idx})
	}

	br := beacon.Round{Round: 20000000, Signature: "96892582a33552a7b67ba44ef09c3ccd535bbebe760c93ecf45be8958d0c0f06390c6d19d7bf492eb806af7eef6b125c"}
	brnd, _ := br.Randomness()
	v.Beacon = beaconCase{br.Round, br.Signature, brnd.String(), beacon.Quicknet.TimeOf(br.Round).Unix()}

	return v
}

func TestVectors(t *testing.T) {
	got := generate()
	if *update {
		b, err := json.MarshalIndent(got, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(vectorPath, append(b, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", vectorPath)
		return
	}

	raw, err := os.ReadFile(vectorPath)
	if err != nil {
		t.Fatalf("%v (run: go test -run TestVectors -update)", err)
	}
	var want vectors
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}

	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		t.Fatal("implementation no longer matches testdata/testvectors.json — this is a breaking protocol change; if intended, regenerate with -update and update the TypeScript client too")
	}
}

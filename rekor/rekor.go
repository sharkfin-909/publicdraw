// Package rekor anchors this service's transparency log into Sigstore's Rekor,
// a public append-only log run by people who are not the organiser.
//
// What it fixes, and what it does not:
//
// The local log in package translog is a hash chain. It is tamper-evident only
// against someone who already saw an earlier head — if nobody wrote one down,
// an organiser can rebuild the chain from scratch and there is nothing to
// compare against. Anchoring each head into Rekor removes that, because Rekor
// is append-only, publicly readable, and outside the organiser's control. An
// abandoned draw anchored at the cutoff cannot be un-anchored.
//
// Rekor v2 reports integratedTime as 0 — it is not a timestamp authority. Time
// comes instead from the witness cosignatures on the checkpoint, where several
// independent operators countersign the tree head together with the moment
// they saw it. Those timestamps are parsed here but reported as observed
// rather than verified, because verifying them needs the witnesses' own keys
// and a policy for which witnesses count. See Cosignature.
package rekor

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// DefaultEndpoint is Sigstore's current public Rekor v2 instance.
//
// Sigstore rotates log instances, so this is configurable rather than wired
// in: a bundle records the origin its checkpoint actually came from, and a
// verifier checks that rather than assuming this one.
const DefaultEndpoint = "https://log2025-1.rekor.sigstore.dev"

// Anchor is one submitted log head and everything needed to check it offline.
type Anchor struct {
	// Payload is what was submitted — this service's transparency-log head.
	Payload string `json:"payload"`

	LogIndex uint64   `json:"log_index"`
	TreeSize uint64   `json:"tree_size"`
	RootHash string   `json:"root_hash"`
	Hashes   []string `json:"hashes"`

	// Checkpoint is the C2SP signed note covering the tree the proof lands in.
	Checkpoint string `json:"checkpoint"`

	// Body is the canonicalised entry as Rekor stored it. The leaf hash is
	// derived from exactly these bytes, so it has to be kept.
	Body string `json:"body"`

	// AnchorKey is the public key that signed the submission, DER, base64.
	AnchorKey string `json:"anchor_key"`

	SubmittedAt int64 `json:"submitted_at"`
}

// Client submits anchors.
type Client struct {
	Endpoint string
	HTTP     *http.Client
}

// New returns a client for the given endpoint, or the default if empty.
func New(endpoint string) *Client {
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	return &Client{Endpoint: endpoint, HTTP: &http.Client{Timeout: 60 * time.Second}}
}

// GenerateKey returns a fresh ECDSA P-256 anchoring key.
//
// P-256 rather than the Ed25519 used everywhere else in this protocol, because
// Rekor's hashedrekord type signs a digest and rejects Ed25519 outright. The
// two keys have genuinely different jobs: the Ed25519 operator key signs what
// the organiser commits to, and this one only says "these bytes were published".
func GenerateKey() (*ecdsa.PrivateKey, error) {
	return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
}

type submitResponse struct {
	LogIndex       string `json:"logIndex"`
	IntegratedTime string `json:"integratedTime"`
	InclusionProof struct {
		LogIndex   string   `json:"logIndex"`
		RootHash   string   `json:"rootHash"`
		TreeSize   string   `json:"treeSize"`
		Hashes     []string `json:"hashes"`
		Checkpoint struct {
			Envelope string `json:"envelope"`
		} `json:"checkpoint"`
	} `json:"inclusionProof"`
	CanonicalizedBody string `json:"canonicalizedBody"`
}

// Submit publishes a payload to Rekor and returns its inclusion proof.
//
// Rekor batches, so this takes a couple of seconds. Callers on the critical
// path of a draw should treat it as best-effort and attach the anchor when it
// lands, rather than holding up a cutoff for it.
func (c *Client) Submit(ctx context.Context, payload []byte, key *ecdsa.PrivateKey) (*Anchor, error) {
	digest := sha256.Sum256(payload)
	sig, err := ecdsa.SignASN1(rand.Reader, key, digest[:])
	if err != nil {
		return nil, fmt.Errorf("rekor: sign: %w", err)
	}
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("rekor: marshal key: %w", err)
	}

	b64 := base64.StdEncoding
	body, err := json.Marshal(map[string]any{
		"hashedRekordRequestV002": map[string]any{
			"digest": b64.EncodeToString(digest[:]),
			"signature": map[string]any{
				"content": b64.EncodeToString(sig),
				"verifier": map[string]any{
					"publicKey":  map[string]any{"rawBytes": b64.EncodeToString(der)},
					"keyDetails": "PKIX_ECDSA_P256_SHA_256",
				},
			},
		},
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint+"/api/v2/log/entries", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rekor: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		if len(raw) > 300 {
			raw = raw[:300]
		}
		return nil, fmt.Errorf("rekor: status %d: %s", resp.StatusCode, raw)
	}

	var out submitResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("rekor: decode: %w", err)
	}
	idx, err := strconv.ParseUint(out.InclusionProof.LogIndex, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("rekor: log index: %w", err)
	}
	size, err := strconv.ParseUint(out.InclusionProof.TreeSize, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("rekor: tree size: %w", err)
	}

	return &Anchor{
		Payload:     base64.StdEncoding.EncodeToString(payload),
		LogIndex:    idx,
		TreeSize:    size,
		RootHash:    out.InclusionProof.RootHash,
		Hashes:      out.InclusionProof.Hashes,
		Checkpoint:  out.InclusionProof.Checkpoint.Envelope,
		Body:        out.CanonicalizedBody,
		AnchorKey:   b64.EncodeToString(der),
		SubmittedAt: time.Now().Unix(),
	}, nil
}

// --- verification --------------------------------------------------------

// RFC 6962 hashing: leaves and internal nodes are prefixed differently, for
// the same second-preimage reason as the roster tree.
func leafHash(b []byte) []byte {
	h := sha256.Sum256(append([]byte{0x00}, b...))
	return h[:]
}

func nodeHash(l, r []byte) []byte {
	buf := make([]byte, 0, 1+len(l)+len(r))
	buf = append(buf, 0x01)
	buf = append(buf, l...)
	buf = append(buf, r...)
	h := sha256.Sum256(buf)
	return h[:]
}

// VerifyInclusion recomputes the tree root from the anchored entry and its
// proof, and checks it against both the proof's stated root and the root in
// the signed checkpoint.
func (a *Anchor) VerifyInclusion() error {
	b64 := base64.StdEncoding
	body, err := b64.DecodeString(a.Body)
	if err != nil {
		return fmt.Errorf("rekor: body: %w", err)
	}
	want, err := b64.DecodeString(a.RootHash)
	if err != nil {
		return fmt.Errorf("rekor: root hash: %w", err)
	}
	if a.TreeSize == 0 || a.LogIndex >= a.TreeSize {
		return fmt.Errorf("rekor: index %d outside tree of %d", a.LogIndex, a.TreeSize)
	}

	proof := make([][]byte, 0, len(a.Hashes))
	for i, h := range a.Hashes {
		d, err := b64.DecodeString(h)
		if err != nil {
			return fmt.Errorf("rekor: proof hash %d: %w", i, err)
		}
		proof = append(proof, d)
	}

	h := leafHash(body)
	fn, sn := a.LogIndex, a.TreeSize-1
	for _, p := range proof {
		if fn%2 == 1 || fn == sn {
			h = nodeHash(p, h)
			for fn%2 == 0 {
				fn >>= 1
				sn >>= 1
			}
		} else {
			h = nodeHash(h, p)
		}
		fn >>= 1
		sn >>= 1
	}
	if !bytes.Equal(h, want) {
		return errors.New("rekor: inclusion proof does not reach the stated root")
	}

	cp, err := a.ParseCheckpoint()
	if err != nil {
		return err
	}
	if cp.RootHash != a.RootHash {
		return errors.New("rekor: checkpoint root differs from the proof root")
	}
	if cp.TreeSize != a.TreeSize {
		return fmt.Errorf("rekor: checkpoint covers %d entries, proof claims %d", cp.TreeSize, a.TreeSize)
	}
	return nil
}

// BindsPayload reports whether this anchor is for exactly these bytes.
func (a *Anchor) BindsPayload(payload []byte) bool {
	got, err := base64.StdEncoding.DecodeString(a.Payload)
	return err == nil && bytes.Equal(got, payload)
}

// Checkpoint is a parsed C2SP signed note.
type Checkpoint struct {
	Origin   string
	TreeSize uint64
	RootHash string
	// Body is the exact text the signatures cover.
	Body       string
	Signatures []SignatureLine
}

// SignatureLine is one signer's line from a note.
type SignatureLine struct {
	Name string
	Raw  string
}

// Cosignature is a witness's countersignature over a checkpoint, carrying the
// moment that witness saw the tree.
//
// Verified reports whether this cosignature's signature was checked. It is
// false here: doing so needs each witness's public key and a policy for how
// many independent witnesses must agree. The timestamp is therefore evidence
// worth surfacing, not a proven fact, and callers must not present it as one.
type Cosignature struct {
	Witness  string
	At       time.Time
	Verified bool
}

// ParseCheckpoint splits a note into its body and signature lines.
func (a *Anchor) ParseCheckpoint() (Checkpoint, error) {
	return ParseCheckpoint(a.Checkpoint)
}

// ParseCheckpoint parses a C2SP signed note.
func ParseCheckpoint(envelope string) (Checkpoint, error) {
	// A note is: body, a blank line, then one signature line per signer.
	i := strings.Index(envelope, "\n\n")
	if i < 0 {
		return Checkpoint{}, errors.New("rekor: checkpoint has no signature block")
	}
	body, sigs := envelope[:i+1], envelope[i+2:]

	lines := strings.Split(strings.TrimRight(body, "\n"), "\n")
	if len(lines) < 3 {
		return Checkpoint{}, errors.New("rekor: checkpoint body is too short")
	}
	size, err := strconv.ParseUint(lines[1], 10, 64)
	if err != nil {
		return Checkpoint{}, fmt.Errorf("rekor: checkpoint tree size: %w", err)
	}

	cp := Checkpoint{Origin: lines[0], TreeSize: size, RootHash: lines[2], Body: body}
	for _, l := range strings.Split(sigs, "\n") {
		if !strings.HasPrefix(l, "— ") {
			continue
		}
		f := strings.Fields(l)
		if len(f) < 3 {
			continue
		}
		cp.Signatures = append(cp.Signatures, SignatureLine{Name: f[1], Raw: f[2]})
	}
	return cp, nil
}

// Cosignatures returns the witness countersignatures and the times they carry.
//
// A C2SP cosignature blob is a 4-byte key hash followed by an 8-byte
// big-endian unix time; the log's own signature has no such time, so it is
// skipped. Every result is marked unverified — see Cosignature.
func (c Checkpoint) Cosignatures() []Cosignature {
	var out []Cosignature
	for _, s := range c.Signatures {
		if s.Name == c.Origin {
			continue // the log signing its own tree, not a witness
		}
		blob, err := base64.StdEncoding.DecodeString(s.Raw)
		if err != nil || len(blob) < 12 {
			continue
		}
		ts := binary.BigEndian.Uint64(blob[4:12])
		// Reject anything outside a plausible range rather than surfacing a
		// misparse as a confident date.
		if ts < 1_600_000_000 || ts > 4_000_000_000 {
			continue
		}
		out = append(out, Cosignature{Witness: s.Name, At: time.Unix(int64(ts), 0).UTC()})
	}
	return out
}

// EarliestCosignature returns the earliest witness time, which is the tightest
// upper bound on when the anchored payload existed.
func (c Checkpoint) EarliestCosignature() (time.Time, bool) {
	cos := c.Cosignatures()
	if len(cos) == 0 {
		return time.Time{}, false
	}
	best := cos[0].At
	for _, x := range cos[1:] {
		if x.At.Before(best) {
			best = x.At
		}
	}
	return best, true
}

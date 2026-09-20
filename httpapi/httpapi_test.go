package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sharkfin009/verifiable-challenge/beacon"
	"github.com/sharkfin009/verifiable-challenge/bundle"
	"github.com/sharkfin009/verifiable-challenge/draw"
	"github.com/sharkfin009/verifiable-challenge/httpapi"
	"github.com/sharkfin009/verifiable-challenge/roster"
	"github.com/sharkfin009/verifiable-challenge/service"
)

const (
	vecRound = 20000000
	vecSig   = "96892582a33552a7b67ba44ef09c3ccd535bbebe760c93ecf45be8958d0c0f06390c6d19d7bf492eb806af7eef6b125c"
	tCreate  = 1752803300
	tCutoff  = 1752803347
	tRound   = 1752803364
)

type env struct {
	srv *httptest.Server
	now *time.Time
}

func newEnv(t *testing.T) *env {
	t.Helper()
	_, priv, _ := draw.GenerateKey()
	now := time.Unix(tCreate, 0)
	svc, err := service.New(t.TempDir(), "test-log", priv,
		service.WithClock(func() time.Time { return now }),
		service.WithFetcher(func(context.Context, uint64) (beacon.Round, error) {
			return beacon.Round{Round: vecRound, Signature: vecSig}, nil
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	s := httptest.NewServer(httpapi.New(svc))
	t.Cleanup(s.Close)
	return &env{srv: s, now: &now}
}

func (e *env) do(t *testing.T, method, path string, body any, out any) int {
	t.Helper()
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, e.srv.URL+path, rd)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("%s %s: decode: %v", method, path, err)
		}
	}
	return resp.StatusCode
}

func TestFullLifecycleOverHTTP(t *testing.T) {
	e := newEnv(t)

	var created struct {
		Params draw.Params `json:"params"`
	}
	if code := e.do(t, "POST", "/v1/draws", map[string]any{
		"title": "community grant", "winners": 2,
		"disclosure": "private", "cutoff_at": tCutoff,
	}, &created); code != http.StatusCreated {
		t.Fatalf("create returned %d", code)
	}
	id := created.Params.DrawID

	var entered struct {
		Count int `json:"entry_count"`
	}
	if code := e.do(t, "POST", "/v1/draws/"+id+"/entries",
		map[string]any{"labels": []string{"a", "b", "c", "d", "e"}}, &entered); code != http.StatusOK {
		t.Fatalf("entries returned %d", code)
	}
	if entered.Count != 5 {
		t.Fatalf("entry count = %d, want 5", entered.Count)
	}

	*e.now = time.Unix(tCutoff, 0)
	if code := e.do(t, "POST", "/v1/draws/"+id+"/commit", nil, nil); code != http.StatusOK {
		t.Fatalf("commit returned %d", code)
	}

	var rec roster.Receipt
	if code := e.do(t, "GET", "/v1/draws/"+id+"/receipt/3", nil, &rec); code != http.StatusOK {
		t.Fatalf("receipt returned %d", code)
	}
	salt := mustHex(t, rec.Salt)
	if !roster.VerifyReceipt(rec, salt) {
		t.Fatal("receipt served over HTTP does not verify")
	}

	*e.now = time.Unix(tRound+6, 0)
	var b bundle.Bundle
	if code := e.do(t, "POST", "/v1/draws/"+id+"/run", nil, &b); code != http.StatusOK {
		t.Fatalf("run returned %d", code)
	}
	if !bundle.Verify(b, beacon.Quicknet).OK {
		t.Fatal("bundle returned over HTTP does not verify")
	}
	if rec.Root != b.Roster.Commitment.Root {
		t.Fatal("the receipt's root is not the roster the winners came from")
	}
}

// Ordering rules must be enforced by the service, not re-implemented by the
// transport — so the API has to surface them rather than allow them.
func TestHTTPRefusesEntriesAfterCutoff(t *testing.T) {
	e := newEnv(t)
	var created struct {
		Params draw.Params `json:"params"`
	}
	e.do(t, "POST", "/v1/draws", map[string]any{"title": "t", "winners": 1, "cutoff_at": tCutoff}, &created)

	*e.now = time.Unix(tCutoff, 0)
	if code := e.do(t, "POST", "/v1/draws/"+created.Params.DrawID+"/entries",
		map[string]any{"labels": []string{"late"}}, nil); code != http.StatusConflict {
		t.Fatalf("late entry returned %d, want 409", code)
	}
}

func TestHTTPRefusesRunBeforeTheRound(t *testing.T) {
	e := newEnv(t)
	var created struct {
		Params draw.Params `json:"params"`
	}
	e.do(t, "POST", "/v1/draws", map[string]any{"title": "t", "winners": 1, "cutoff_at": tCutoff}, &created)
	id := created.Params.DrawID
	e.do(t, "POST", "/v1/draws/"+id+"/entries", map[string]any{"labels": []string{"a", "b"}}, nil)

	*e.now = time.Unix(tCutoff, 0)
	e.do(t, "POST", "/v1/draws/"+id+"/commit", nil, nil)

	*e.now = time.Unix(tRound-1, 0)
	if code := e.do(t, "POST", "/v1/draws/"+id+"/run", nil, nil); code != http.StatusConflict {
		t.Fatalf("early run returned %d, want 409", code)
	}
}

// A private draw's salts are what hide its entrants. The API must never serve
// them, or private disclosure is decorative.
func TestHTTPNeverServesSaltsForAPrivateDraw(t *testing.T) {
	e := newEnv(t)
	var created struct {
		Params draw.Params `json:"params"`
	}
	e.do(t, "POST", "/v1/draws", map[string]any{
		"title": "t", "winners": 1, "disclosure": "private", "cutoff_at": tCutoff,
	}, &created)
	id := created.Params.DrawID
	e.do(t, "POST", "/v1/draws/"+id+"/entries", map[string]any{"labels": []string{"secret-person", "b"}}, nil)
	*e.now = time.Unix(tCutoff, 0)
	e.do(t, "POST", "/v1/draws/"+id+"/commit", nil, nil)

	var raw map[string]any
	e.do(t, "GET", "/v1/draws/"+id, nil, &raw)
	body, _ := json.Marshal(raw)
	for _, leak := range []string{"secret-person", "salt", "sealed"} {
		if bytes.Contains(bytes.ToLower(body), []byte(leak)) {
			t.Fatalf("private draw response leaked %q: %s", leak, body)
		}
	}
}

func TestHTTPVerifyEndpointRejectsATamperedBundle(t *testing.T) {
	e := newEnv(t)
	var created struct {
		Params draw.Params `json:"params"`
	}
	e.do(t, "POST", "/v1/draws", map[string]any{"title": "t", "winners": 1, "cutoff_at": tCutoff}, &created)
	id := created.Params.DrawID
	e.do(t, "POST", "/v1/draws/"+id+"/entries", map[string]any{"labels": []string{"a", "b", "c"}}, nil)
	*e.now = time.Unix(tCutoff, 0)
	e.do(t, "POST", "/v1/draws/"+id+"/commit", nil, nil)
	*e.now = time.Unix(tRound+6, 0)

	var b bundle.Bundle
	e.do(t, "POST", "/v1/draws/"+id+"/run", nil, &b)

	var rep bundle.Report
	if code := e.do(t, "POST", "/v1/verify", b, &rep); code != http.StatusOK || !rep.OK {
		t.Fatalf("honest bundle: code=%d ok=%v", code, rep.OK)
	}

	b.Result.Winners[0].Entry.Label = "someone else"
	if code := e.do(t, "POST", "/v1/verify", b, &rep); code != http.StatusUnprocessableEntity || rep.OK {
		t.Fatalf("tampered bundle: code=%d ok=%v, want 422 and not ok", code, rep.OK)
	}
}

func TestHTTPUnknownDrawIs404(t *testing.T) {
	e := newEnv(t)
	if code := e.do(t, "GET", "/v1/draws/nope", nil, nil); code != http.StatusNotFound {
		t.Fatalf("unknown draw returned %d, want 404", code)
	}
}

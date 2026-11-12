package httpapi_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/sharkfin-909/publicdraw/hashing"
	"github.com/sharkfin-909/publicdraw/rekor"
	"github.com/sharkfin-909/publicdraw/service"
)

// Freezing over HTTP used to commit the roster and stop there: only the CLI
// anchored, so draws run through the server never reached Rekor.
func TestCommitOverHTTPAnchorsTheRoster(t *testing.T) {
	var anchored []hashing.Digest
	e := newEnv(t, service.WithAnchorer(func(_ context.Context, h hashing.Digest) (*rekor.Anchor, error) {
		anchored = append(anchored, h)
		return &rekor.Anchor{}, nil
	}))

	var created struct {
		Params struct {
			DrawID string `json:"draw_id"`
		} `json:"params"`
	}
	if code := e.do(t, "POST", "/v1/draws", map[string]any{"title": "t", "winners": 1, "cutoff_at": tCutoff}, &created); code != http.StatusCreated {
		t.Fatalf("create: %d", code)
	}
	id := created.Params.DrawID
	if code := e.do(t, "POST", "/v1/draws/"+id+"/entries", map[string]any{"labels": []string{"a", "b"}}, nil); code != http.StatusOK {
		t.Fatalf("enter: %d", code)
	}
	*e.now = time.Unix(tCutoff, 0)

	var out struct {
		Anchored bool           `json:"anchored"`
		LogHead  hashing.Digest `json:"log_head"`
	}
	if code := e.do(t, "POST", "/v1/draws/"+id+"/commit", nil, &out); code != http.StatusOK {
		t.Fatalf("commit: %d", code)
	}
	if !out.Anchored || len(anchored) != 1 || anchored[0] != out.LogHead {
		t.Fatalf("anchored = %v (%v), want the commit head %s", out.Anchored, anchored, out.LogHead)
	}
}

func TestAMissingDrawIs404AndARefusalIs409(t *testing.T) {
	e := newEnv(t)
	if code := e.do(t, "GET", "/v1/draws/nope", nil, nil); code != http.StatusNotFound {
		t.Fatalf("missing draw: %d, want 404", code)
	}
	var created struct {
		Params struct {
			DrawID string `json:"draw_id"`
		} `json:"params"`
	}
	e.do(t, "POST", "/v1/draws", map[string]any{"title": "t", "winners": 1, "cutoff_at": tCutoff}, &created)
	if code := e.do(t, "GET", "/v1/draws/"+created.Params.DrawID+"/leaves", nil, nil); code != http.StatusConflict {
		t.Fatalf("leaves before the freeze: %d, want 409", code)
	}
}

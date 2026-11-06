// Package httpapi exposes the draw service over HTTP.
//
// The handlers are deliberately thin. Every rule that makes a draw meaningful
// — ordering, cutoffs, disclosure — lives in the service layer, so the API
// cannot accidentally permit something the CLI forbids.
package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sharkfin-909/publicdraw/bundle"
	"github.com/sharkfin-909/publicdraw/roster"
	"github.com/sharkfin-909/publicdraw/service"
)

// Server wraps a service with an HTTP mux.
type Server struct {
	svc *service.Service
	mux *http.ServeMux
}

// New builds the HTTP surface.
func New(svc *service.Service) *Server {
	s := &Server{svc: svc, mux: http.NewServeMux()}

	s.mux.HandleFunc("GET /v1/health", s.health)
	s.mux.HandleFunc("GET /v1/operator", s.operator)
	s.mux.HandleFunc("POST /v1/draws", s.createDraw)
	s.mux.HandleFunc("GET /v1/draws", s.listDraws)
	s.mux.HandleFunc("GET /v1/draws/{id}", s.getDraw)
	s.mux.HandleFunc("GET /v1/public/draws", s.listPublic)
	s.mux.HandleFunc("PUT /v1/draws/{id}/visibility", s.setVisibility)
	s.mux.HandleFunc("POST /v1/draws/{id}/entries", s.addEntries)
	s.mux.HandleFunc("POST /v1/draws/{id}/commit", s.commit)
	s.mux.HandleFunc("GET /v1/draws/{id}/leaves", s.leaves)
	s.mux.HandleFunc("GET /v1/draws/{id}/proof/{index}", s.proof)
	s.mux.HandleFunc("POST /v1/draws/{id}/run", s.run)
	s.mux.HandleFunc("GET /v1/draws/{id}/bundle", s.getBundle)
	s.mux.HandleFunc("GET /v1/log", s.getLog)
	s.mux.HandleFunc("GET /v1/log/abandoned", s.abandoned)
	s.mux.HandleFunc("POST /v1/verify", s.verify)

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }

func write(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, code int, err error) {
	write(w, code, map[string]string{"error": err.Error()})
}

// badRequest is returned for anything the caller could have avoided, which
// here means every ordering violation: entering after cutoff, committing
// twice, drawing before the round.
func svcErr(w http.ResponseWriter, err error) {
	if strings.Contains(err.Error(), "no draw") {
		fail(w, http.StatusNotFound, err)
		return
	}
	fail(w, http.StatusConflict, err)
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	write(w, http.StatusOK, map[string]any{"ok": true, "log_length": s.svc.Log().Len()})
}

func (s *Server) operator(w http.ResponseWriter, _ *http.Request) {
	c := s.svc.Chain()
	write(w, http.StatusOK, map[string]any{
		"public_key": s.svc.OperatorKey(),
		"chain":      map[string]any{"name": c.Name, "hash": c.Hash, "period_seconds": int(c.Period.Seconds())},
		"log_head":   s.svc.Log().Head(),
	})
}

type createReq struct {
	Title      string `json:"title"`
	Winners    int    `json:"winners"`
	Disclosure string `json:"disclosure"`
	CutoffAt   int64  `json:"cutoff_at"`
	OpenFor    int    `json:"open_for_seconds"`
}

func (s *Server) createDraw(w http.ResponseWriter, r *http.Request) {
	var req createReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	cutoff := time.Unix(req.CutoffAt, 0)
	if req.CutoffAt == 0 && req.OpenFor > 0 {
		cutoff = time.Now().Add(time.Duration(req.OpenFor) * time.Second)
	}
	if req.Winners == 0 {
		req.Winners = 1
	}
	disc := roster.Disclosure(req.Disclosure)
	if disc == "" {
		disc = roster.Private
	}
	st, err := s.svc.Create(req.Title, req.Winners, disc, cutoff)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	write(w, http.StatusCreated, st)
}

// listPublic backs the sitemap. It returns only what an organiser opted in,
// with just enough to build a URL and a listing — never entries or salts.
func (s *Server) listPublic(w http.ResponseWriter, _ *http.Request) {
	sts, err := s.svc.ListPublic()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]map[string]any, 0, len(sts))
	for _, st := range sts {
		row := map[string]any{
			"draw_id": st.Params.DrawID, "title": st.Params.Title,
			"state": st.State, "winners": st.Params.Winners,
			"disclosure": st.Params.Disclosure, "created_at": st.Params.CreatedAt,
			"target_round": st.Params.TargetRound, "entry_count": len(st.Sealed),
		}
		if st.RosterCommit != nil {
			row["committed_at"] = st.RosterCommit.CommittedAt
		}
		out = append(out, row)
	}
	write(w, http.StatusOK, map[string]any{"draws": out})
}

type visibilityReq struct {
	Visibility string `json:"visibility"`
}

func (s *Server) setVisibility(w http.ResponseWriter, r *http.Request) {
	var req visibilityReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	st, err := s.svc.SetVisibility(r.PathValue("id"), service.Visibility(req.Visibility))
	if err != nil {
		svcErr(w, err)
		return
	}
	write(w, http.StatusOK, map[string]any{"draw_id": st.Params.DrawID, "visibility": st.Visible()})
}

func (s *Server) listDraws(w http.ResponseWriter, _ *http.Request) {
	ids, err := s.svc.List()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	write(w, http.StatusOK, map[string]any{"draws": ids})
}

// getDraw never returns the sealed entries or their salts: those are the
// roster's hiding, and handing them out would make private disclosure a lie.
func (s *Server) getDraw(w http.ResponseWriter, r *http.Request) {
	st, err := s.svc.Get(r.PathValue("id"))
	if err != nil {
		svcErr(w, err)
		return
	}
	out := map[string]any{
		"params": st.Params, "params_sig": st.ParamsSig,
		"state": st.State, "entry_count": len(st.Sealed),
		"visibility": st.Visible(),
	}
	if st.RosterCommit != nil {
		out["roster_commit"] = st.RosterCommit
		out["roster_sig"] = st.RosterSig
	}
	if st.Params.Disclosure == roster.Public && st.State != service.StateOpen {
		labels := make([]string, len(st.Sealed))
		for i, e := range st.Sealed {
			labels[i] = e.Entry.Label
		}
		out["labels"] = labels
	}
	write(w, http.StatusOK, out)
}

type entriesReq struct {
	Labels []string `json:"labels"`
}

func (s *Server) addEntries(w http.ResponseWriter, r *http.Request) {
	var req entriesReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	admitted, err := s.svc.Enter(r.PathValue("id"), req.Labels...)
	if err != nil {
		svcErr(w, err)
		return
	}
	// The salts come back here and only here. An entrant who keeps theirs can
	// prove their own inclusion later from the published leaves alone, with
	// nothing needing to be sent to them at the cutoff.
	write(w, http.StatusOK, map[string]any{
		"admitted": admitted,
		"note":     "keep the salt for each entry: it is what lets that entrant check their own inclusion after the cutoff",
	})
}

func (s *Server) commit(w http.ResponseWriter, r *http.Request) {
	st, err := s.svc.CommitRoster(r.PathValue("id"))
	if err != nil {
		svcErr(w, err)
		return
	}
	write(w, http.StatusOK, map[string]any{
		"roster_commit": st.RosterCommit, "roster_sig": st.RosterSig, "state": st.State,
	})
}

// leaves publishes every committed leaf. Safe in either disclosure mode, and
// the only thing an entrant needs — besides the salt they were given at entry
// — to confirm they were in the roster the winners came from.
func (s *Server) leaves(w http.ResponseWriter, r *http.Request) {
	st, err := s.svc.Get(r.PathValue("id"))
	if err != nil {
		svcErr(w, err)
		return
	}
	if st.RosterCommit == nil {
		fail(w, http.StatusConflict, errors.New("roster is not committed yet"))
		return
	}
	ros, err := roster.Load(st.Sealed)
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	write(w, http.StatusOK, map[string]any{
		"root":   st.RosterCommit.Root,
		"size":   st.RosterCommit.Size,
		"leaves": ros.Leaves(),
	})
}

// proof serves the public half of a receipt only.
//
// This replaces a /receipt/{index} endpoint that returned the salt and the
// label as well, so anyone could walk the indices of a private draw and read
// the whole entry list. The entrant already holds both — they were handed
// them when they entered, once.
func (s *Server) proof(w http.ResponseWriter, r *http.Request) {
	idx, err := strconv.Atoi(r.PathValue("index"))
	if err != nil {
		fail(w, http.StatusBadRequest, errors.New("index must be an integer"))
		return
	}
	p, err := s.svc.Proof(r.PathValue("id"), idx)
	if err != nil {
		svcErr(w, err)
		return
	}
	write(w, http.StatusOK, p)
}

func (s *Server) run(w http.ResponseWriter, r *http.Request) {
	b, err := s.svc.Run(r.Context(), r.PathValue("id"))
	if err != nil {
		svcErr(w, err)
		return
	}
	write(w, http.StatusOK, b)
}

func (s *Server) getBundle(w http.ResponseWriter, r *http.Request) {
	b, err := s.svc.Run(r.Context(), r.PathValue("id"))
	if err != nil {
		svcErr(w, err)
		return
	}
	write(w, http.StatusOK, b)
}

func (s *Server) getLog(w http.ResponseWriter, _ *http.Request) {
	l := s.svc.Log()
	write(w, http.StatusOK, map[string]any{
		"records": l.Records(), "head": l.Head(), "length": l.Len(),
	})
}

func (s *Server) abandoned(w http.ResponseWriter, _ *http.Request) {
	ids, err := s.svc.Abandoned()
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	if ids == nil {
		ids = []string{}
	}
	write(w, http.StatusOK, map[string]any{
		"abandoned": ids,
		"note":      "draws whose target round has published but whose result never was",
	})
}

// verify runs the public verifier over a posted bundle. It exists so a browser
// can check a bundle without shipping a verifier, but it is the weakest way to
// verify: the answer comes from this server. The CLI and the WASM verifier
// exist so nobody has to take it.
func (s *Server) verify(w http.ResponseWriter, r *http.Request) {
	b, err := bundle.Read(r.Body)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	rep := bundle.Verify(b, s.svc.Chain())
	code := http.StatusOK
	if !rep.OK {
		code = http.StatusUnprocessableEntity
	}
	write(w, code, rep)
}

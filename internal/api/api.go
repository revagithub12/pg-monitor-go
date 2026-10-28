// Package api exposes the REST and WebSocket endpoints.
package api

import (
	"embed"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/YOUR_GITHUB_USERNAME/pg-monitor-go/internal/alerts"
	"github.com/YOUR_GITHUB_USERNAME/pg-monitor-go/internal/auth"
	"github.com/YOUR_GITHUB_USERNAME/pg-monitor-go/internal/hub"
	"github.com/YOUR_GITHUB_USERNAME/pg-monitor-go/internal/monitor"
)

//go:embed static/index.html
var static embed.FS

type Server struct {
	Auth    *auth.Service
	Monitor *monitor.Monitor
	Rules   *alerts.Store
	Hub     *hub.Hub
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	both := []string{auth.RoleAdmin, auth.RoleViewer}

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		page, _ := static.ReadFile("static/index.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(page)
	})

	mux.HandleFunc("POST /auth/login", s.login)

	mux.HandleFunc("GET /metrics/latest", s.Auth.Require(s.latest, both...))
	mux.HandleFunc("GET /metrics/history", s.Auth.Require(s.history, both...))

	mux.HandleFunc("GET /alerts/rules", s.Auth.Require(s.listRules, both...))
	mux.HandleFunc("POST /alerts/rules", s.Auth.Require(s.createRule, auth.RoleAdmin))
	mux.HandleFunc("DELETE /alerts/rules/{id}", s.Auth.Require(s.deleteRule, auth.RoleAdmin))

	// Browsers cannot set headers on WebSocket connections, so the token is read from the query string.
	mux.HandleFunc("GET /ws", s.websocket)

	return mux
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	token, err := s.Auth.Login(body.Username, body.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "incorrect username or password")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

func (s *Server) latest(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.Monitor.Latest()
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "no metrics collected yet")
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

func (s *Server) history(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.Monitor.History())
}

func (s *Server) listRules(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.Rules.List())
}

func (s *Server) createRule(w http.ResponseWriter, r *http.Request) {
	var rule alerts.Rule
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&rule); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	created, err := s.Rules.Add(rule)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) deleteRule(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid rule id")
		return
	}
	if !s.Rules.Delete(id) {
		writeError(w, http.StatusNotFound, "rule not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) websocket(w http.ResponseWriter, r *http.Request) {
	if _, err := s.Auth.Parse(r.URL.Query().Get("token")); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or missing token")
		return
	}
	s.Hub.ServeWS(w, r)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

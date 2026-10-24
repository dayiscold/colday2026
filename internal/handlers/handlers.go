package handlers

import (
	"encoding/json"
	"io"
	"net/http"

	"colday2026/internal/auth"
	"colday2026/internal/store"
)

type Handler struct {
	store      *store.Store
	auth       *auth.Auth
	uploadsDir string
	uploadsURL string
	staticDir  string
}

func New(s *store.Store, a *auth.Auth, uploadsDir, uploadsURL, staticDir string) *Handler {
	return &Handler{
		store:      s,
		auth:       a,
		uploadsDir: uploadsDir,
		uploadsURL: uploadsURL,
		staticDir:  staticDir,
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func readJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

package handlers

import (
	"net/http"
	"time"
)

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
		return
	}
	if !h.auth.CheckPassword(body.Password) {
		time.Sleep(150 * time.Millisecond)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Неверный код"})
		return
	}
	h.auth.SetSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	h.auth.ClearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"authed": h.auth.IsAuthed(r)})
}

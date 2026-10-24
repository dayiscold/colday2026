package handlers

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"colday2026/internal/models"
	"colday2026/internal/store"
)

func (h *Handler) ListReleases(w http.ResponseWriter, r *http.Request) {
	rels := h.store.List()
	writeJSON(w, http.StatusOK, rels)
}

func (h *Handler) CreateRelease(w http.ResponseWriter, r *http.Request) {
	var rel models.Release
	if err := readJSON(r, &rel); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if strings.TrimSpace(rel.Title) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title required"})
		return
	}
	rel, err := h.store.Create(rel)
	if err != nil {
		h.storeError(w, "createRelease", err)
		return
	}
	writeJSON(w, http.StatusCreated, rel)
}

func (h *Handler) GetRelease(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad id"})
		return
	}
	rel, err := h.store.Get(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, rel)
}

func (h *Handler) UpdateRelease(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad id"})
		return
	}
	var rel models.Release
	if err := readJSON(r, &rel); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	rel.ID = id
	if strings.TrimSpace(rel.Title) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title required"})
		return
	}
	if err := h.store.Update(rel); err != nil {
		h.storeError(w, "updateRelease", err)
		return
	}
	writeJSON(w, http.StatusOK, rel)
}

func (h *Handler) DeleteRelease(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad id"})
		return
	}
	if err := h.store.Delete(id); err != nil {
		h.storeError(w, "deleteRelease", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) storeError(w http.ResponseWriter, operation string, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	log.Printf("%s: %v", operation, err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Не удалось сохранить данные. Проверьте доступ к хранилищу."})
}

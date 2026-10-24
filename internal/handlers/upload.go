package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const maxCoverSize = 8 << 20

func (h *Handler) UploadCover(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxCoverSize+(1<<20))
	if err := r.ParseMultipartForm(maxCoverSize); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "Обложка должна быть не больше 8 МБ"})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad form"})
		return
	}
	defer r.MultipartForm.RemoveAll()
	f, hdr, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no file"})
		return
	}
	defer f.Close()
	if hdr.Size > maxCoverSize {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "Обложка должна быть не больше 8 МБ"})
		return
	}

	ext := strings.ToLower(filepath.Ext(hdr.Filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif":
	default:
		ext = ".jpg"
	}
	idBytes := make([]byte, 12)
	if _, err := rand.Read(idBytes); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "rand"})
		return
	}
	name := hex.EncodeToString(idBytes) + ext
	if err := os.MkdirAll(h.uploadsDir, 0755); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "mkdir"})
		return
	}
	dst := filepath.Join(h.uploadsDir, name)
	out, err := os.Create(dst)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "create"})
		return
	}
	defer out.Close()
	complete := false
	defer func() {
		if !complete {
			os.Remove(dst)
		}
	}()
	if _, err := io.Copy(out, f); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "write"})
		return
	}
	if err := out.Sync(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "write"})
		return
	}
	if err := out.Close(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "write"})
		return
	}
	complete = true
	writeJSON(w, http.StatusOK, map[string]string{"url": h.uploadsURL + name})
}

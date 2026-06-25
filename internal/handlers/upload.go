package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func (h *Handler) UploadCover(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad form"})
		return
	}
	f, hdr, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no file"})
		return
	}
	defer f.Close()

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
	if _, err := io.Copy(out, io.LimitReader(f, 8<<20)); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "write"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"url": h.uploadsURL + name})
}

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"colday2026/internal/auth"
	"colday2026/internal/handlers"
	"colday2026/internal/models"
	"colday2026/internal/store"
)

func TestPublicAppRoutes(t *testing.T) {
	dir := t.TempDir()
	cfg := &config{
		staticDir:  filepath.Join(dir, "static"),
		uploadsDir: filepath.Join(dir, "external-uploads"),
		uploadsURL: "/static/uploads/",
	}
	files := map[string]string{
		filepath.Join(cfg.staticDir, "app", "index.html"):       "<!doctype html><title>React app</title>",
		filepath.Join(cfg.staticDir, "app", "assets", "app.js"): "console.log('app')",
		filepath.Join(cfg.staticDir, "goatadmin.html"):          "<!doctype html><title>Admin</title>",
		filepath.Join(cfg.uploadsDir, "cover.png"):              "cover",
	}
	for name, contents := range files {
		if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(contents), 0644); err != nil {
			t.Fatal(err)
		}
	}
	s, err := store.NewStore(filepath.Join(dir, "data.json"))
	if err != nil {
		t.Fatal(err)
	}
	a := auth.New("testpass", bytes.Repeat([]byte{1}, 32), false)
	router := newRouter(cfg, handlers.New(s, a, cfg.uploadsDir, cfg.uploadsURL, cfg.staticDir))
	for _, tc := range []struct {
		path        string
		status      int
		body        string
		contentType string
	}{
		{"/", 200, "React app", "text/html"},
		{"/static/colday.html", 200, "React app", "text/html"},
		{"/static/app/assets/app.js", 200, "console.log", "javascript"},
		{"/admin", 200, "Admin", "text/html"},
		{"/admin/", 200, "Admin", "text/html"},
		{"/static/uploads/cover.png", 200, "cover", "image/png"},
		{"/missing", 404, "404", "text/plain"},
		{"/static/app/assets/missing.js", 404, "404", "text/plain"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			r := httptest.NewRequest("GET", tc.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)
			if w.Code != tc.status || !strings.Contains(w.Body.String(), tc.body) || !strings.Contains(w.Header().Get("Content-Type"), tc.contentType) {
				t.Fatalf("%s: status=%d, type=%s, body=%s", tc.path, w.Code, w.Header().Get("Content-Type"), w.Body.String())
			}
		})
	}
}

func TestAdminSaveAndUploadSurviveRestart(t *testing.T) {
	dir := t.TempDir()
	cfg := &config{
		dataFile:   filepath.Join(dir, "data", "data.json"),
		staticDir:  "static",
		uploadsDir: filepath.Join(dir, "external-uploads"),
		uploadsURL: "/static/uploads/",
	}
	a := auth.New("testpass", bytes.Repeat([]byte{1}, 32), false)
	start := func() *httptest.Server {
		s, err := store.NewStore(cfg.dataFile)
		if err != nil {
			t.Fatal(err)
		}
		return httptest.NewServer(newRouter(cfg, handlers.New(s, a, cfg.uploadsDir, cfg.uploadsURL, cfg.staticDir)))
	}
	server := start()
	defer func() { server.Close() }()
	request := func(method, path, contentType string, body io.Reader, cookie *http.Cookie, status int) *http.Response {
		t.Helper()
		req, err := http.NewRequest(method, server.URL+path, body)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", contentType)
		if cookie != nil {
			req.AddCookie(cookie)
		}
		res, err := server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { res.Body.Close() })
		if res.StatusCode != status {
			body, _ := io.ReadAll(res.Body)
			t.Fatalf("%s %s: %d, %s", method, path, res.StatusCode, body)
		}
		return res
	}
	request("GET", "/admin", "", nil, nil, 200)
	request("GET", "/admin/", "", nil, nil, 200)
	request("POST", "/api/releases", "application/json", bytes.NewBufferString(`{"title":"Unauthorized"}`), nil, 401)
	login := request("POST", "/api/login", "application/json", bytes.NewBufferString(`{"password":"testpass"}`), nil, 200)
	if len(login.Cookies()) != 1 {
		t.Fatal("login must set a session cookie")
	}
	cookie := login.Cookies()[0]
	var upload bytes.Buffer
	w := multipart.NewWriter(&upload)
	f, err := w.CreateFormFile("file", "cover.png")
	if err != nil {
		t.Fatal(err)
	}
	const cover = "\x89PNG\r\n\x1a\ncover-content"
	if _, err := io.WriteString(f, cover); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	res := request("POST", "/api/upload-cover", w.FormDataContentType(), &upload, cookie, 200)
	var uploaded struct{ URL string }
	if err := json.NewDecoder(res.Body).Decode(&uploaded); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(models.Release{Title: "Before restart", Cover: uploaded.URL})
	request("POST", "/api/releases", "application/json", bytes.NewReader(payload), cookie, 201)
	payload, _ = json.Marshal(models.Release{Title: "After edit", Cover: uploaded.URL, Featured: true})
	request("PUT", "/api/releases/1", "application/json", bytes.NewReader(payload), cookie, 200)
	server.Close()
	server = start()
	res = request("GET", "/api/releases/1", "", nil, nil, 200)
	var release models.Release
	if err := json.NewDecoder(res.Body).Decode(&release); err != nil {
		t.Fatal(err)
	}
	if release.Title != "After edit" || release.Cover != uploaded.URL || !release.Featured {
		t.Fatalf("release not persisted: %+v", release)
	}
	res = request("GET", uploaded.URL, "", nil, nil, 200)
	content, _ := io.ReadAll(res.Body)
	if string(content) != cover {
		t.Fatal("uploaded cover is unavailable after restart")
	}
	request("DELETE", "/api/releases/1", "", nil, cookie, 200)
	server.Close()
	server = start()
	request("GET", "/api/releases/1", "", nil, nil, 404)
	if _, err := os.Stat(cfg.dataFile); err != nil {
		t.Fatal(err)
	}
}

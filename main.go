package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"colday2026/internal/auth"
	"colday2026/internal/handlers"
	"colday2026/internal/store"
)

func loadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			continue
		}
		k := strings.TrimSpace(line[:eq])
		v := strings.TrimSpace(line[eq+1:])
		if len(v) >= 2 {
			if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
				v = v[1 : len(v)-1]
			}
		}
		if _, exists := os.LookupEnv(k); exists {
			continue
		}
		_ = os.Setenv(k, v)
	}
	return sc.Err()
}

type config struct {
	addr          string
	adminPassword string
	dataFile      string
	sessionKey    []byte
	staticDir     string
	uploadsDir    string
	uploadsURL    string
	cookieSecure  bool
}

func loadConfig() (*config, error) {
	addr := os.Getenv("COLDAY_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	pw := os.Getenv("COLDAY_ADMIN_PASSWORD")
	if pw == "" {
		return nil, errors.New("COLDAY_ADMIN_PASSWORD required")
	}
	dataFile := os.Getenv("COLDAY_DATA_FILE")
	if dataFile == "" {
		dataFile = "data/data.json"
	}
	keyHex := os.Getenv("COLDAY_SESSION_KEY")
	var key []byte
	if keyHex == "" {
		key = make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}
		log.Println("WARN: COLDAY_SESSION_KEY not set; using an ephemeral key (sessions reset on every restart)")
	} else {
		var err error
		key, err = hex.DecodeString(keyHex)
		if err != nil {
			return nil, fmt.Errorf("COLDAY_SESSION_KEY must be hex: %w", err)
		}
		if len(key) < 32 {
			return nil, errors.New("COLDAY_SESSION_KEY must be at least 32 bytes (64 hex chars)")
		}
	}
	return &config{
		addr:          addr,
		adminPassword: pw,
		dataFile:      dataFile,
		sessionKey:    key,
		staticDir:     envOr("COLDAY_STATIC_DIR", "static"),
		uploadsDir:    envOr("COLDAY_UPLOADS_DIR", filepath.Join("static", "uploads")),
		uploadsURL:    "/static/uploads/",
		cookieSecure:  os.Getenv("COLDAY_COOKIE_SECURE") == "1",
	}, nil
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func main() {
	if err := loadDotEnv(".env"); err != nil {
		log.Printf("WARN: .env: %v", err)
	}
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if err := os.MkdirAll(cfg.uploadsDir, 0755); err != nil {
		log.Fatalf("mkdir uploads: %v", err)
	}

	st, err := store.NewStore(cfg.dataFile)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	log.Printf("store: %s", cfg.dataFile)

	a := auth.New(cfg.adminPassword, cfg.sessionKey, cfg.cookieSecure)
	h := handlers.New(st, a, cfg.uploadsDir, cfg.uploadsURL, cfg.staticDir)

	srv := &http.Server{
		Addr:              cfg.addr,
		Handler:           newRouter(cfg, h),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
	}
	log.Printf("colday listening on %s", cfg.addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("listen: %v", err)
	}
}

func newRouter(cfg *config, h *handlers.Handler) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(cfg.staticDir, "app", "index.html"))
	})
	// Preserve the former public HTML URL after moving to a compiled React app.
	mux.HandleFunc("/static/colday.html", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(cfg.staticDir, "app", "index.html"))
	})
	mux.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(cfg.staticDir, "goatadmin.html"))
	})
	mux.HandleFunc("/admin/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(cfg.staticDir, "goatadmin.html"))
	})
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(cfg.staticDir, "favicon.svg"))
	})
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(cfg.staticDir))))
	// Uploads may live outside the read-only static directory.
	mux.Handle(cfg.uploadsURL, http.StripPrefix(cfg.uploadsURL, http.FileServer(http.Dir(cfg.uploadsDir))))

	mux.HandleFunc("POST /api/login", h.Login)
	mux.HandleFunc("POST /api/logout", h.Logout)
	mux.HandleFunc("GET /api/me", h.Me)

	mux.HandleFunc("GET /api/releases", h.ListReleases)
	mux.HandleFunc("POST /api/releases", h.RequireAuth(h.CreateRelease))
	mux.HandleFunc("GET /api/releases/{id}", h.GetRelease)
	mux.HandleFunc("PUT /api/releases/{id}", h.RequireAuth(h.UpdateRelease))
	mux.HandleFunc("DELETE /api/releases/{id}", h.RequireAuth(h.DeleteRelease))

	mux.HandleFunc("POST /api/upload-cover", h.RequireAuth(h.UploadCover))

	return mux
}

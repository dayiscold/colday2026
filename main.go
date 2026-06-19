package main

import (
	"bufio"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// loadDotEnv reads KEY=VALUE pairs from path and sets them in the process env.
// Existing env vars are NOT overridden. Lines starting with # or blank are skipped.
// Values may be wrapped in single or double quotes.
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

const (
	sessionCookie = "colday_session"
	sessionMaxAge = 7 * 24 * time.Hour
)

type config struct {
	addr          string
	dsn           string
	adminPassword string
	sessionKey    []byte
	staticDir     string
	uploadsDir    string
	uploadsURL    string
	cookieSecure  bool
}

type Release struct {
	ID           int64  `json:"id"`
	Title        string `json:"title"`
	ReleaseType  string `json:"release_type"`
	Cover        string `json:"cover"`
	Bandlink     string `json:"bandlink"`
	UPC          string `json:"upc"`
	OriginalDate string `json:"original_date"`
	CounterSongs int    `json:"counter_songs"`
	SongDuration string `json:"song_duration"`
	Featured     bool   `json:"featured"`
}

func loadConfig() (*config, error) {
	addr := os.Getenv("COLDAY_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	dsn := os.Getenv("COLDAY_DB_DSN")
	if dsn == "" {
		return nil, errors.New("COLDAY_DB_DSN required (e.g. user:pass@tcp(127.0.0.1:3306)/colday?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci)")
	}
	pw := os.Getenv("COLDAY_ADMIN_PASSWORD")
	if pw == "" {
		return nil, errors.New("COLDAY_ADMIN_PASSWORD required")
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
		dsn:           dsn,
		adminPassword: pw,
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

func openDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, err
	}
	return db, nil
}

func migrate(db *sql.DB) error {
	var n int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE()
		  AND TABLE_NAME = 'releases'
		  AND COLUMN_NAME = 'featured'`).Scan(&n)
	if err != nil {
		return fmt.Errorf("inspect releases.featured: %w", err)
	}
	if n == 0 {
		if _, err := db.Exec(`ALTER TABLE releases ADD COLUMN featured TINYINT(1) NOT NULL DEFAULT 0`); err != nil {
			return fmt.Errorf("add releases.featured: %w", err)
		}
		log.Println("migration: added releases.featured")
	}
	return nil
}

// ---- DB ops ----

const releaseCols = `id, title, release_type,
	IFNULL(cover,'') AS cover,
	IFNULL(bandlink,'') AS bandlink,
	IFNULL(upc,'') AS upc,
	IFNULL(DATE_FORMAT(original_date,'%Y-%m-%d'),'') AS original_date,
	IFNULL(counter_songs,0) AS counter_songs,
	IFNULL(TIME_FORMAT(song_duration,'%H:%i:%s'),'') AS song_duration,
	featured`

func scanRelease(rows interface{ Scan(...any) error }) (Release, error) {
	var r Release
	var feat int
	err := rows.Scan(&r.ID, &r.Title, &r.ReleaseType, &r.Cover, &r.Bandlink, &r.UPC, &r.OriginalDate, &r.CounterSongs, &r.SongDuration, &feat)
	r.Featured = feat != 0
	return r, err
}

func listReleases(db *sql.DB) ([]Release, error) {
	rows, err := db.Query(`SELECT ` + releaseCols + ` FROM releases ORDER BY featured DESC, original_date DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Release, 0)
	for rows.Next() {
		r, err := scanRelease(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func getRelease(db *sql.DB, id int64) (*Release, error) {
	row := db.QueryRow(`SELECT `+releaseCols+` FROM releases WHERE id = ?`, id)
	r, err := scanRelease(row)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func clearOtherFeatured(tx *sql.Tx, exceptID int64) error {
	_, err := tx.Exec(`UPDATE releases SET featured = 0 WHERE id <> ?`, exceptID)
	return err
}

func nullableDate(s string) any {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return s
}
func nullableTime(s string) any {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return s
}

func createRelease(db *sql.DB, r *Release) (int64, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`INSERT INTO releases
		(title, release_type, cover, bandlink, upc, original_date, counter_songs, song_duration, featured)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.Title, r.ReleaseType, r.Cover, r.Bandlink, r.UPC, nullableDate(r.OriginalDate),
		r.CounterSongs, nullableTime(r.SongDuration), boolToInt(r.Featured))
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if r.Featured {
		if err := clearOtherFeatured(tx, id); err != nil {
			return 0, err
		}
	}
	return id, tx.Commit()
}

func updateRelease(db *sql.DB, r *Release) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.Exec(`UPDATE releases SET
		title=?, release_type=?, cover=?, bandlink=?, upc=?, original_date=?,
		counter_songs=?, song_duration=?, featured=? WHERE id=?`,
		r.Title, r.ReleaseType, r.Cover, r.Bandlink, r.UPC, nullableDate(r.OriginalDate),
		r.CounterSongs, nullableTime(r.SongDuration), boolToInt(r.Featured), r.ID)
	if err != nil {
		return err
	}
	if r.Featured {
		if err := clearOtherFeatured(tx, r.ID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func deleteRelease(db *sql.DB, id int64) error {
	_, err := db.Exec(`DELETE FROM releases WHERE id = ?`, id)
	return err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// ---- Auth ----

func signSession(key []byte, expires time.Time) string {
	expStr := strconv.FormatInt(expires.Unix(), 10)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(expStr))
	sig := mac.Sum(nil)
	return base64.RawURLEncoding.EncodeToString([]byte(expStr)) + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func verifySession(key []byte, value string) bool {
	parts := strings.SplitN(value, ".", 2)
	if len(parts) != 2 {
		return false
	}
	expBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(expBytes)
	if subtle.ConstantTimeCompare(sig, mac.Sum(nil)) != 1 {
		return false
	}
	exp, err := strconv.ParseInt(string(expBytes), 10, 64)
	if err != nil {
		return false
	}
	return time.Now().Unix() < exp
}

func isAuthed(cfg *config, r *http.Request) bool {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return false
	}
	return verifySession(cfg.sessionKey, c.Value)
}

func requireAuth(cfg *config, w http.ResponseWriter, r *http.Request) bool {
	if !isAuthed(cfg, r) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return false
	}
	return true
}

// ---- HTTP helpers ----

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func readJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// ---- Handlers ----

func handleIndex(cfg *config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(cfg.staticDir, "colday.html"))
	}
}

func handleAdmin(cfg *config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(cfg.staticDir, "goatadmin.html"))
	}
}

func handleLogin(cfg *config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method"})
			return
		}
		var body struct {
			Password string `json:"password"`
		}
		if err := readJSON(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
			return
		}
		if subtle.ConstantTimeCompare([]byte(body.Password), []byte(cfg.adminPassword)) != 1 {
			time.Sleep(150 * time.Millisecond)
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Неверный код"})
			return
		}
		token := signSession(cfg.sessionKey, time.Now().Add(sessionMaxAge))
		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookie,
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   cfg.cookieSecure,
			MaxAge:   int(sessionMaxAge.Seconds()),
		})
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

func handleLogout(cfg *config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookie,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   cfg.cookieSecure,
			MaxAge:   -1,
		})
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

func handleMe(cfg *config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]bool{"authed": isAuthed(cfg, r)})
	}
}

func handleReleasesCollection(cfg *config, db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			rel, err := listReleases(db)
			if err != nil {
				log.Printf("listReleases: %v", err)
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db"})
				return
			}
			writeJSON(w, http.StatusOK, rel)
		case http.MethodPost:
			if !requireAuth(cfg, w, r) {
				return
			}
			var rel Release
			if err := readJSON(r, &rel); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			if strings.TrimSpace(rel.Title) == "" {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title required"})
				return
			}
			id, err := createRelease(db, &rel)
			if err != nil {
				log.Printf("createRelease: %v", err)
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db"})
				return
			}
			rel.ID = id
			writeJSON(w, http.StatusCreated, rel)
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method"})
		}
	}
}

func handleReleaseByID(cfg *config, db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := strings.TrimPrefix(r.URL.Path, "/api/releases/")
		idStr = strings.Trim(idStr, "/")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad id"})
			return
		}
		switch r.Method {
		case http.MethodGet:
			rel, err := getRelease(db, id)
			if errors.Is(err, sql.ErrNoRows) {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
				return
			}
			if err != nil {
				log.Printf("getRelease: %v", err)
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db"})
				return
			}
			writeJSON(w, http.StatusOK, rel)
		case http.MethodPut:
			if !requireAuth(cfg, w, r) {
				return
			}
			var rel Release
			if err := readJSON(r, &rel); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			rel.ID = id
			if strings.TrimSpace(rel.Title) == "" {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title required"})
				return
			}
			if err := updateRelease(db, &rel); err != nil {
				log.Printf("updateRelease: %v", err)
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db"})
				return
			}
			writeJSON(w, http.StatusOK, rel)
		case http.MethodDelete:
			if !requireAuth(cfg, w, r) {
				return
			}
			if err := deleteRelease(db, id); err != nil {
				log.Printf("deleteRelease: %v", err)
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db"})
				return
			}
			writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method"})
		}
	}
}

func handleCoverUpload(cfg *config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method"})
			return
		}
		if !requireAuth(cfg, w, r) {
			return
		}
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
		if err := os.MkdirAll(cfg.uploadsDir, 0o755); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "mkdir"})
			return
		}
		dst := filepath.Join(cfg.uploadsDir, name)
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
		writeJSON(w, http.StatusOK, map[string]string{"url": cfg.uploadsURL + name})
	}
}

// ---- main ----

func main() {
	if err := loadDotEnv(".env"); err != nil {
		log.Printf("WARN: .env: %v", err)
	}
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if err := os.MkdirAll(cfg.uploadsDir, 0o755); err != nil {
		log.Fatalf("mkdir uploads: %v", err)
	}

	db, err := openDB(cfg.dsn)
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	defer db.Close()

	if err := migrate(db); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIndex(cfg))
	mux.HandleFunc("/admin", handleAdmin(cfg))
	mux.HandleFunc("/admin/", handleAdmin(cfg))
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(cfg.staticDir, "favicon.svg"))
	})
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(cfg.staticDir))))

	mux.HandleFunc("/api/login", handleLogin(cfg))
	mux.HandleFunc("/api/logout", handleLogout(cfg))
	mux.HandleFunc("/api/me", handleMe(cfg))
	mux.HandleFunc("/api/releases", handleReleasesCollection(cfg, db))
	mux.HandleFunc("/api/releases/", handleReleaseByID(cfg, db))
	mux.HandleFunc("/api/upload-cover", handleCoverUpload(cfg))

	srv := &http.Server{
		Addr:              cfg.addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
	}
	log.Printf("colday listening on %s", cfg.addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("listen: %v", err)
	}
}

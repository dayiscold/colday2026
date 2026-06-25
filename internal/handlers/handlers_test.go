package handlers

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
	"colday2026/internal/models"
	"colday2026/internal/store"
)

func newTestHarness(t *testing.T) (*Handler, *store.Store, *auth.Auth, string) {
	t.Helper()
	dir := t.TempDir()

	s, err := store.NewStore(filepath.Join(dir, "data.json"))
	if err != nil {
		t.Fatal(err)
	}

	a := auth.New("testpass", nil, false)
	uploadsDir := filepath.Join(dir, "uploads")
	h := New(s, a, uploadsDir, "/uploads/", dir)
	return h, s, a, dir
}

func readResponse(t *testing.T, rr *httptest.ResponseRecorder) (int, map[string]any) {
	t.Helper()
	result := make(map[string]any)
	if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return rr.Code, result
}

func readResponseArr(t *testing.T, rr *httptest.ResponseRecorder) (int, []any) {
	t.Helper()
	var result []any
	if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return rr.Code, result
}

func authedCookie(a *auth.Auth) *http.Cookie {
	return &http.Cookie{
		Name:  "colday_session",
		Value: a.SessionCookieValue(),
	}
}

func bodyReader(v any) io.Reader {
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(v)
	return &buf
}

// ---- Releases ----

func TestListReleases_Empty(t *testing.T) {
	h, _, _, _ := newTestHarness(t)
	rr := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/releases", nil)

	h.ListReleases(rr, r)

	code, arr := readResponseArr(t, rr)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if len(arr) != 0 {
		t.Fatalf("expected empty array, got %d items", len(arr))
	}
}

func TestListReleases(t *testing.T) {
	h, s, _, _ := newTestHarness(t)
	s.Create(models.Release{Title: "Alpha", ReleaseType: "album"})
	s.Create(models.Release{Title: "Beta", ReleaseType: "single"})

	rr := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/releases", nil)
	h.ListReleases(rr, r)

	code, arr := readResponseArr(t, rr)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if len(arr) != 2 {
		t.Fatalf("expected 2 releases, got %d", len(arr))
	}
}

func TestListReleases_FeaturedFirst(t *testing.T) {
	h, s, _, _ := newTestHarness(t)
	s.Create(models.Release{Title: "A"})
	s.Create(models.Release{Title: "B", Featured: true})
	s.Create(models.Release{Title: "C"})

	rr := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/releases", nil)
	h.ListReleases(rr, r)

	_, arr := readResponseArr(t, rr)
	if len(arr) != 3 {
		t.Fatalf("expected 3 releases, got %d", len(arr))
	}
	first := arr[0].(map[string]any)
	if first["title"] != "B" {
		t.Fatalf("expected featured release first, got %v", first["title"])
	}
}

func TestCreateRelease_Unauthorized(t *testing.T) {
	h, _, _, _ := newTestHarness(t)
	body := bodyReader(models.Release{Title: "Test"})
	rr := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/releases", body)
	r.Header.Set("Content-Type", "application/json")

	h.RequireAuth(h.CreateRelease)(rr, r)

	code, _ := readResponse(t, rr)
	if code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", code)
	}
}

func TestCreateRelease(t *testing.T) {
	h, _, a, _ := newTestHarness(t)
	body := bodyReader(models.Release{
		Title:       "New Release",
		ReleaseType: "album",
		Bandlink:    "https://example.com",
		UPC:         "123456",
	})
	rr := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/releases", body)
	r.Header.Set("Content-Type", "application/json")
	r.AddCookie(authedCookie(a))

	h.CreateRelease(rr, r)

	code, res := readResponse(t, rr)
	if code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %v", code, res)
	}
	if res["title"] != "New Release" {
		t.Fatalf("expected title 'New Release', got %v", res["title"])
	}
	id, ok := res["id"].(float64)
	if !ok || id == 0 {
		t.Fatalf("expected valid id, got %v", res["id"])
	}
}

func TestCreateRelease_EmptyTitle(t *testing.T) {
	h, _, a, _ := newTestHarness(t)
	body := bodyReader(models.Release{Title: ""})
	rr := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/releases", body)
	r.Header.Set("Content-Type", "application/json")
	r.AddCookie(authedCookie(a))

	h.CreateRelease(rr, r)

	code, _ := readResponse(t, rr)
	if code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", code)
	}
}

func TestCreateRelease_Featured(t *testing.T) {
	h, s, a, _ := newTestHarness(t)
	s.Create(models.Release{Title: "Old Featured", Featured: true})

	body := bodyReader(models.Release{Title: "New Featured", Featured: true})
	rr := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/releases", body)
	r.Header.Set("Content-Type", "application/json")
	r.AddCookie(authedCookie(a))

	h.CreateRelease(rr, r)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}
	rels := s.List()
	featuredCount := 0
	for _, rel := range rels {
		if rel.Featured {
			featuredCount++
			if rel.Title != "New Featured" {
				t.Fatalf("expected 'New Featured' as featured, got %s", rel.Title)
			}
		}
	}
	if featuredCount != 1 {
		t.Fatalf("expected exactly 1 featured, got %d", featuredCount)
	}
}

func TestCreateRelease_InvalidJSON(t *testing.T) {
	h, _, a, _ := newTestHarness(t)
	rr := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/releases", strings.NewReader("not json"))
	r.Header.Set("Content-Type", "application/json")
	r.AddCookie(authedCookie(a))

	h.CreateRelease(rr, r)

	code, _ := readResponse(t, rr)
	if code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", code)
	}
}

func TestCreateRelease_UnknownFields(t *testing.T) {
	h, _, a, _ := newTestHarness(t)
	rr := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/releases", strings.NewReader(`{"title":"T","unknown":"field"}`))
	r.Header.Set("Content-Type", "application/json")
	r.AddCookie(authedCookie(a))

	h.CreateRelease(rr, r)

	code, _ := readResponse(t, rr)
	if code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown fields, got %d", code)
	}
}

func TestGetRelease(t *testing.T) {
	h, s, _, _ := newTestHarness(t)
	created := s.Create(models.Release{Title: "Found Me"})

	rr := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/releases/1", nil)
	r.SetPathValue("id", "1")
	h.GetRelease(rr, r)

	code, res := readResponse(t, rr)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if res["title"] != "Found Me" {
		t.Fatalf("expected 'Found Me', got %v", res["title"])
	}
	if res["id"] != float64(created.ID) {
		t.Fatalf("expected id %d, got %v", created.ID, res["id"])
	}
}

func TestGetRelease_NotFound(t *testing.T) {
	h, _, _, _ := newTestHarness(t)
	rr := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/releases/999", nil)
	r.SetPathValue("id", "999")
	h.GetRelease(rr, r)

	code, _ := readResponse(t, rr)
	if code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", code)
	}
}

func TestGetRelease_BadID(t *testing.T) {
	h, _, _, _ := newTestHarness(t)
	rr := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/releases/abc", nil)
	r.SetPathValue("id", "abc")
	h.GetRelease(rr, r)

	code, _ := readResponse(t, rr)
	if code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", code)
	}
}

func TestUpdateRelease_Unauthorized(t *testing.T) {
	h, _, _, _ := newTestHarness(t)
	body := bodyReader(models.Release{Title: "Hacked"})
	rr := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/api/releases/1", body)
	r.Header.Set("Content-Type", "application/json")
	r.SetPathValue("id", "1")

	h.RequireAuth(h.UpdateRelease)(rr, r)

	code, _ := readResponse(t, rr)
	if code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", code)
	}
}

func TestUpdateRelease(t *testing.T) {
	h, s, a, _ := newTestHarness(t)
	s.Create(models.Release{Title: "Original"})

	body := bodyReader(models.Release{
		Title:       "Updated",
		ReleaseType: "single",
		Bandlink:    "https://updated.com",
	})
	rr := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/api/releases/1", body)
	r.Header.Set("Content-Type", "application/json")
	r.SetPathValue("id", "1")
	r.AddCookie(authedCookie(a))

	h.UpdateRelease(rr, r)

	code, res := readResponse(t, rr)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if res["title"] != "Updated" {
		t.Fatalf("expected 'Updated', got %v", res["title"])
	}
	if res["id"] != float64(1) {
		t.Fatalf("expected id 1, got %v", res["id"])
	}
}

func TestUpdateRelease_NotFound(t *testing.T) {
	h, _, a, _ := newTestHarness(t)
	body := bodyReader(models.Release{Title: "Ghost"})
	rr := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/api/releases/999", body)
	r.Header.Set("Content-Type", "application/json")
	r.SetPathValue("id", "999")
	r.AddCookie(authedCookie(a))

	h.UpdateRelease(rr, r)

	code, _ := readResponse(t, rr)
	if code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", code)
	}
}

func TestUpdateRelease_FeaturedClearsOthers(t *testing.T) {
	h, s, a, _ := newTestHarness(t)
	s.Create(models.Release{Title: "A", Featured: true})
	s.Create(models.Release{Title: "B"})

	body := bodyReader(models.Release{Title: "B Updated", Featured: true})
	rr := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/api/releases/2", body)
	r.Header.Set("Content-Type", "application/json")
	r.SetPathValue("id", "2")
	r.AddCookie(authedCookie(a))

	h.UpdateRelease(rr, r)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	rels := s.List()
	for _, rel := range rels {
		if rel.Title == "A" && rel.Featured {
			t.Fatal("release A should no longer be featured")
		}
		if rel.Title == "B Updated" && !rel.Featured {
			t.Fatal("release B should be featured")
		}
	}
}

func TestUpdateRelease_EmptyTitle(t *testing.T) {
	h, s, a, _ := newTestHarness(t)
	s.Create(models.Release{Title: "Original"})

	body := bodyReader(models.Release{Title: ""})
	rr := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/api/releases/1", body)
	r.Header.Set("Content-Type", "application/json")
	r.SetPathValue("id", "1")
	r.AddCookie(authedCookie(a))

	h.UpdateRelease(rr, r)

	code, _ := readResponse(t, rr)
	if code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", code)
	}
}

func TestDeleteRelease_Unauthorized(t *testing.T) {
	h, _, _, _ := newTestHarness(t)
	rr := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE", "/api/releases/1", nil)
	r.SetPathValue("id", "1")

	h.RequireAuth(h.DeleteRelease)(rr, r)

	code, _ := readResponse(t, rr)
	if code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", code)
	}
}

func TestDeleteRelease(t *testing.T) {
	h, s, a, _ := newTestHarness(t)
	s.Create(models.Release{Title: "To Delete"})

	rr := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE", "/api/releases/1", nil)
	r.SetPathValue("id", "1")
	r.AddCookie(authedCookie(a))

	h.DeleteRelease(rr, r)

	code, res := readResponse(t, rr)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if res["ok"] != true {
		t.Fatal("expected ok: true")
	}
	if len(s.List()) != 0 {
		t.Fatal("expected 0 releases after delete")
	}
}

func TestDeleteRelease_NotFound(t *testing.T) {
	h, _, a, _ := newTestHarness(t)
	rr := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE", "/api/releases/999", nil)
	r.SetPathValue("id", "999")
	r.AddCookie(authedCookie(a))

	h.DeleteRelease(rr, r)

	code, _ := readResponse(t, rr)
	if code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", code)
	}
}

// ---- Auth ----

func TestLogin_Success(t *testing.T) {
	h, _, _, _ := newTestHarness(t)
	body := bodyReader(map[string]string{"password": "testpass"})
	rr := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/login", body)
	r.Header.Set("Content-Type", "application/json")

	h.Login(rr, r)

	code, res := readResponse(t, rr)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if res["ok"] != true {
		t.Fatal("expected ok: true")
	}
	cookies := rr.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == "colday_session" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected session cookie")
	}
}

func TestLogin_Failure(t *testing.T) {
	h, _, _, _ := newTestHarness(t)
	body := bodyReader(map[string]string{"password": "wrong"})
	rr := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/login", body)
	r.Header.Set("Content-Type", "application/json")

	h.Login(rr, r)

	code, _ := readResponse(t, rr)
	if code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", code)
	}
}

func TestLogin_BadJSON(t *testing.T) {
	h, _, _, _ := newTestHarness(t)
	rr := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/login", strings.NewReader("not json"))
	r.Header.Set("Content-Type", "application/json")

	h.Login(rr, r)

	code, _ := readResponse(t, rr)
	if code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", code)
	}
}

func TestLogout(t *testing.T) {
	h, _, _, _ := newTestHarness(t)
	rr := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/logout", nil)

	h.Logout(rr, r)

	code, res := readResponse(t, rr)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if res["ok"] != true {
		t.Fatal("expected ok: true")
	}
}

func TestMe_Authed(t *testing.T) {
	h, _, a, _ := newTestHarness(t)
	rr := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/me", nil)
	r.AddCookie(authedCookie(a))

	h.Me(rr, r)

	code, res := readResponse(t, rr)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if res["authed"] != true {
		t.Fatal("expected authed: true")
	}
}

func TestMe_NotAuthed(t *testing.T) {
	h, _, _, _ := newTestHarness(t)
	rr := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/me", nil)

	h.Me(rr, r)

	code, res := readResponse(t, rr)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if res["authed"] != false {
		t.Fatal("expected authed: false")
	}
}

// ---- Upload ----

func TestUploadCover_Unauthorized(t *testing.T) {
	h, _, _, _ := newTestHarness(t)
	rr := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/upload-cover", nil)

	h.RequireAuth(h.UploadCover)(rr, r)

	code, _ := readResponse(t, rr)
	if code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", code)
	}
}

func TestUploadCover(t *testing.T) {
	h, _, a, dir := newTestHarness(t)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", "cover.png")
	if err != nil {
		t.Fatal(err)
	}
	fw.Write([]byte("fake-png-data"))
	w.Close()

	rr := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/upload-cover", &buf)
	r.Header.Set("Content-Type", w.FormDataContentType())
	r.AddCookie(authedCookie(a))

	h.UploadCover(rr, r)

	code, res := readResponse(t, rr)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %v", code, res)
	}
	url, ok := res["url"].(string)
	if !ok {
		t.Fatal("expected url string")
	}
	if !strings.HasPrefix(url, "/uploads/") || !strings.HasSuffix(url, ".png") {
		t.Fatalf("unexpected url: %s", url)
	}
	if _, err := os.Stat(filepath.Join(dir, "uploads", strings.TrimPrefix(url, "/uploads/"))); err != nil {
		t.Fatalf("uploaded file not found: %v", err)
	}
}

func TestUploadCover_NoFile(t *testing.T) {
	h, _, a, _ := newTestHarness(t)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	w.Close()

	rr := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/upload-cover", &buf)
	r.Header.Set("Content-Type", w.FormDataContentType())
	r.AddCookie(authedCookie(a))

	h.UploadCover(rr, r)

	code, _ := readResponse(t, rr)
	if code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", code)
	}
}

func TestUploadCover_UnknownExtensionDefaultsToJpg(t *testing.T) {
	h, _, a, dir := newTestHarness(t)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", "cover.bmp")
	if err != nil {
		t.Fatal(err)
	}
	fw.Write([]byte("data"))
	w.Close()

	rr := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/upload-cover", &buf)
	r.Header.Set("Content-Type", w.FormDataContentType())
	r.AddCookie(authedCookie(a))

	h.UploadCover(rr, r)

	code, res := readResponse(t, rr)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %v", code, res)
	}
	url := res["url"].(string)
	if !strings.HasSuffix(url, ".jpg") {
		t.Fatalf("expected .jpg extension, got %s", url)
	}
	if _, err := os.Stat(filepath.Join(dir, "uploads", strings.TrimPrefix(url, "/uploads/"))); err != nil {
		t.Fatalf("uploaded file not found: %v", err)
	}
}

// ---- Store persistence ----

func TestStore_Persistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.json")

	s1, err := store.NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	s1.Create(models.Release{Title: "Persist Me"})

	s2, err := store.NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	rels := s2.List()
	if len(rels) != 1 || rels[0].Title != "Persist Me" {
		t.Fatalf("expected persisted release, got %+v", rels)
	}
}

// ---- Featured logic ----

func TestFeatured_OnlyOneFeatured(t *testing.T) {
	h, s, a, _ := newTestHarness(t)

	h.store.Create(models.Release{Title: "A", Featured: true})
	h.store.Create(models.Release{Title: "B", Featured: true})
	h.store.Create(models.Release{Title: "C", Featured: true})

	rels := s.List()
	count := 0
	for _, r := range rels {
		if r.Featured {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 featured, got %d", count)
	}
	if rels[0].Title != "C" {
		t.Fatalf("expected last created 'C' to be featured, got %s", rels[0].Title)
	}

	body := bodyReader(models.Release{Title: "A Updated", Featured: true})
	rr := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/api/releases/1", body)
	r.Header.Set("Content-Type", "application/json")
	r.SetPathValue("id", "1")
	r.AddCookie(authedCookie(a))

	h.UpdateRelease(rr, r)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	rels = s.List()
	count = 0
	for _, r := range rels {
		if r.Featured {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 featured after update, got %d", count)
	}
}

// ---- Auth cookie verification ----

func TestAuth_ExpiredCookie(t *testing.T) {
	h, _, _, _ := newTestHarness(t)
	rr := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/me", nil)
	r.AddCookie(&http.Cookie{
		Name:  "colday_session",
		Value: "invalid",
	})
	h.Me(rr, r)

	code, res := readResponse(t, rr)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if res["authed"] != false {
		t.Fatal("expected authed: false for invalid cookie")
	}
}

func TestAuth_TamperedCookie(t *testing.T) {
	h, _, a, _ := newTestHarness(t)
	valid := a.SessionCookieValue()
	tampered := valid[:len(valid)-1] + "x"

	rr := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/me", nil)
	r.AddCookie(&http.Cookie{
		Name:  "colday_session",
		Value: tampered,
	})
	h.Me(rr, r)

	code, res := readResponse(t, rr)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if res["authed"] != false {
		t.Fatal("expected authed: false for tampered cookie")
	}
}

// ---- Edge cases ----

func TestCreateRelease_MinimalFields(t *testing.T) {
	h, _, a, _ := newTestHarness(t)
	body := bodyReader(models.Release{Title: "Minimal"})
	rr := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/releases", body)
	r.Header.Set("Content-Type", "application/json")
	r.AddCookie(authedCookie(a))

	h.CreateRelease(rr, r)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}
}

func TestUpdateRelease_PartialFields(t *testing.T) {
	h, s, a, _ := newTestHarness(t)
	s.Create(models.Release{
		Title:       "Full",
		ReleaseType: "album",
		Bandlink:    "https://example.com",
		UPC:         "123",
		CounterSongs: 10,
		SongDuration: "00:30:00",
	})

	body := bodyReader(models.Release{
		Title:       "Partial Update",
		ReleaseType: "single",
	})
	rr := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/api/releases/1", body)
	r.Header.Set("Content-Type", "application/json")
	r.SetPathValue("id", "1")
	r.AddCookie(authedCookie(a))

	h.UpdateRelease(rr, r)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	rel, _ := s.Get(1)
	if rel.Title != "Partial Update" {
		t.Fatalf("expected 'Partial Update', got %s", rel.Title)
	}
	if rel.Bandlink != "" {
		t.Fatalf("expected Bandlink to be cleared (zero value), got %s", rel.Bandlink)
	}
}

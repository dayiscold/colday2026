package store

import (
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"colday2026/internal/models"
)

func TestFailedWritesPreserveState(t *testing.T) {
	for _, operation := range []string{"create", "update", "delete"} {
		t.Run(operation, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "data.json")
			s, err := NewStore(path)
			if err != nil {
				t.Fatal(err)
			}
			original, err := s.Create(models.Release{Title: "Original", Featured: true})
			if err != nil {
				t.Fatal(err)
			}
			before := s.List()
			// A directory at the destination forces rename to fail even as root.
			if err := os.Rename(path, path+".backup"); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(path, 0755); err != nil {
				t.Fatal(err)
			}
			switch operation {
			case "create":
				_, err = s.Create(models.Release{Title: "New", Featured: true})
			case "update":
				original.Title = "Changed"
				original.Featured = false
				err = s.Update(original)
			case "delete":
				err = s.Delete(original.ID)
			}
			if err == nil {
				t.Fatal("expected persistence error")
			}
			if !reflect.DeepEqual(before, s.List()) {
				t.Fatalf("failed write changed memory: %+v", s.List())
			}
			leftovers, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".colday-*.tmp"))
			if err != nil || len(leftovers) != 0 {
				t.Fatalf("temporary files not cleaned up: %v, %v", leftovers, err)
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(path+".backup", path); err != nil {
				t.Fatal(err)
			}
			created, err := s.Create(models.Release{Title: "Retry"})
			if err != nil || created.ID != 2 {
				t.Fatalf("retry should use next uncommitted ID: %+v, %v", created, err)
			}
		})
	}
}

func TestRecoverNextID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")
	if err := os.WriteFile(path, []byte(`{"releases":[{"id":8,"title":"Existing"}],"next_id":1}`), 0644); err != nil {
		t.Fatal(err)
	}
	s, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	created, err := s.Create(models.Release{Title: "New"})
	if err != nil || created.ID != 9 {
		t.Fatalf("expected ID 9: %+v, %v", created, err)
	}
}

func TestConcurrentWritesSurviveReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")
	s, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			if _, err := s.Create(models.Release{Title: "Concurrent", Featured: true}); err != nil {
				t.Error(err)
			}
			s.List()
		})
	}
	wg.Wait()
	reloaded, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.List(), reloaded.List()) || len(reloaded.List()) != 20 {
		t.Fatalf("writes lost after reload: %+v", reloaded.List())
	}
	ids := make(map[int64]bool)
	featured := 0
	for _, release := range reloaded.List() {
		if ids[release.ID] {
			t.Fatalf("duplicate ID %d", release.ID)
		}
		ids[release.ID] = true
		if release.Featured {
			featured++
		}
	}
	if featured != 1 {
		t.Fatalf("expected one featured release, got %d", featured)
	}
}

package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"sync"

	"colday2026/internal/models"
)

var ErrNotFound = errors.New("release not found")

type dataFile struct {
	Releases []models.Release `json:"releases"`
	NextID   int64            `json:"next_id"`
}

type Store struct {
	mu       sync.RWMutex
	filePath string
	data     dataFile
}

func NewStore(filePath string) (*Store, error) {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	s := &Store{filePath: filePath}

	if raw, err := os.ReadFile(filePath); err == nil {
		if err := json.Unmarshal(raw, &s.data); err != nil {
			return nil, err
		}
		// Older files may have a missing or stale counter. Never reuse an ID.
		if s.data.NextID < 1 {
			s.data.NextID = 1
		}
		for _, r := range s.data.Releases {
			if r.ID >= s.data.NextID {
				s.data.NextID = r.ID + 1
			}
		}
		return s, nil
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	s.data = dataFile{
		Releases: []models.Release{},
		NextID:   1,
	}
	if err := s.save(s.data); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) List() []models.Release {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]models.Release, len(s.data.Releases))
	copy(out, s.data.Releases)

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Featured != out[j].Featured {
			return out[i].Featured
		}
		if out[i].OriginalDate != out[j].OriginalDate {
			return out[i].OriginalDate > out[j].OriginalDate
		}
		return out[i].ID > out[j].ID
	})
	return out
}

func (s *Store) Get(id int64) (models.Release, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, r := range s.data.Releases {
		if r.ID == id {
			return r, nil
		}
	}
	return models.Release{}, ErrNotFound
}

func (s *Store) Create(r models.Release) (models.Release, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	next := s.snapshot()
	r.ID = next.NextID
	next.NextID++

	if r.Featured {
		for i := range next.Releases {
			next.Releases[i].Featured = false
		}
	}

	next.Releases = append(next.Releases, r)
	if err := s.commit(next); err != nil {
		return models.Release{}, err
	}
	return r, nil
}

func (s *Store) Update(r models.Release) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, existing := range s.data.Releases {
		if existing.ID == r.ID {
			next := s.snapshot()
			if r.Featured {
				for j := range next.Releases {
					next.Releases[j].Featured = false
				}
			}
			r.ID = existing.ID
			next.Releases[i] = r
			return s.commit(next)
		}
	}
	return ErrNotFound
}

func (s *Store) Delete(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, r := range s.data.Releases {
		if r.ID == id {
			next := s.snapshot()
			next.Releases = append(next.Releases[:i], next.Releases[i+1:]...)
			return s.commit(next)
		}
	}
	return ErrNotFound
}

// Callers hold the write lock; publish changes only after persisting them.
func (s *Store) snapshot() dataFile {
	return dataFile{Releases: slices.Clone(s.data.Releases), NextID: s.data.NextID}
}

func (s *Store) commit(next dataFile) error {
	if err := s.save(next); err != nil {
		return err
	}
	s.data = next
	return nil
}

func (s *Store) save(data dataFile) error {
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	// Create beside the destination so rename stays on the same filesystem.
	f, err := os.CreateTemp(filepath.Dir(s.filePath), ".colday-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if err := f.Chmod(0644); err != nil {
		return err
	}
	if _, err := f.Write(raw); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), s.filePath)
}

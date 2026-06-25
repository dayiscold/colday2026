package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"colday2026/internal/models"
)

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
		return s, nil
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	s.data = dataFile{
		Releases: []models.Release{},
		NextID:   1,
	}
	if err := s.save(); err != nil {
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
	return models.Release{}, errors.New("release not found")
}

func (s *Store) Create(r models.Release) models.Release {
	s.mu.Lock()
	defer s.mu.Unlock()

	r.ID = s.data.NextID
	s.data.NextID++

	if r.Featured {
		for i := range s.data.Releases {
			s.data.Releases[i].Featured = false
		}
	}

	s.data.Releases = append(s.data.Releases, r)
	s.save()
	return r
}

func (s *Store) Update(r models.Release) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, existing := range s.data.Releases {
		if existing.ID == r.ID {
			if r.Featured {
				for j := range s.data.Releases {
					s.data.Releases[j].Featured = false
				}
			}
			r.ID = existing.ID
			s.data.Releases[i] = r
			return s.save()
		}
	}
	return errors.New("release not found")
}

func (s *Store) Delete(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, r := range s.data.Releases {
		if r.ID == id {
			s.data.Releases = append(s.data.Releases[:i], s.data.Releases[i+1:]...)
			return s.save()
		}
	}
	return errors.New("release not found")
}

func (s *Store) save() error {
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.filePath + ".tmp"
	if err := os.WriteFile(tmp, raw, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, s.filePath)
}

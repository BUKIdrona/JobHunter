package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Record remembers a posting so it is not mailed twice.
type Record struct {
	Key        string    `json:"key"`
	Title      string    `json:"title"`
	Company    string    `json:"company"`
	URL        string    `json:"url"`
	Location   string    `json:"location,omitempty"`
	Source     string    `json:"source,omitempty"`
	Skills     []string  `json:"skills,omitempty"`
	Score      int       `json:"score,omitempty"`
	HREmail    string    `json:"hr_email,omitempty"`
	HRStatus   string    `json:"hr_status,omitempty"`
	HRSentAt   time.Time `json:"hr_sent_at,omitempty"`
	DigestSent bool      `json:"digest_sent"`
	FirstSeen  time.Time `json:"first_seen"`
	Updated    time.Time `json:"updated"`
}

const (
	HRSent   = "sent"
	HRFailed = "failed"
	HRNone   = "none"
)

type fileData struct {
	Jobs map[string]Record `json:"jobs"`
}

// Store is a JSON file of postings already handled.
type Store struct {
	path string
	data fileData
}

// Open loads state from disk. A missing file starts empty.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	s := &Store{path: path, data: fileData{Jobs: map[string]Record{}}}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	if len(raw) == 0 {
		return s, nil
	}
	if err := json.Unmarshal(raw, &s.data); err != nil {
		return nil, fmt.Errorf("state file %s is unreadable: %w", path, err)
	}
	if s.data.Jobs == nil {
		s.data.Jobs = map[string]Record{}
	}
	return s, nil
}

// Get returns a record and whether it exists.
func (s *Store) Get(key string) (Record, bool) {
	rec, ok := s.data.Jobs[key]
	return rec, ok
}

// Put inserts or replaces a record and writes the file.
func (s *Store) Put(rec Record) error {
	if rec.FirstSeen.IsZero() {
		if prev, ok := s.data.Jobs[rec.Key]; ok && !prev.FirstSeen.IsZero() {
			rec.FirstSeen = prev.FirstSeen
		} else {
			rec.FirstSeen = time.Now()
		}
	}
	rec.Updated = time.Now()
	s.data.Jobs[rec.Key] = rec
	return s.save()
}

// HRSentToday counts hiring emails already sent since local midnight.
func (s *Store) HRSentToday(now time.Time) int {
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	n := 0
	for _, rec := range s.data.Jobs {
		if rec.HRStatus == HRSent && !rec.HRSentAt.Before(start) {
			n++
		}
	}
	return n
}

// PendingDigest returns hiring emails that were sent but not yet reported.
func (s *Store) PendingDigest() []Record {
	var out []Record
	for _, rec := range s.data.Jobs {
		if rec.HRStatus == HRSent && !rec.DigestSent {
			out = append(out, rec)
		}
	}
	return out
}

func (s *Store) save() error {
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

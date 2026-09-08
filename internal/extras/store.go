package extras

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/egor-muindor/submix/internal/entry"
)

// Store keeps the last successfully parsed set of entries for each subscription
// in memory and mirrors it to disk (last-good cache).
type Store struct {
	dir string

	mu    sync.RWMutex
	bySub map[string][]entry.Entry
}

// NewStore creates a store with its cache in directory dir.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &Store{dir: dir, bySub: map[string][]entry.Entry{}}, nil
}

// Set replaces the subscription's entries and persists them to disk.
func (s *Store) Set(subscription string, entries []entry.Entry) {
	s.mu.Lock()
	s.bySub[subscription] = entries
	s.mu.Unlock()

	if err := s.persist(subscription, entries); err != nil {
		slog.Warn("store: persist failed", "subscription", subscription, "err", err)
	}
}

// All returns the entries of all subscriptions, sorted by subscription name.
func (s *Store) All() []entry.Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.bySub))
	for name := range s.bySub {
		names = append(names, name)
	}
	sort.Strings(names)

	var all []entry.Entry
	for _, name := range names {
		all = append(all, s.bySub[name]...)
	}
	return all
}

// Restore loads the last-good cache from disk. Corrupt or missing files are
// skipped; the service starts with whatever is available.
func (s *Store) Restore(subscriptions []string) error {
	for _, name := range subscriptions {
		raw, err := os.ReadFile(s.pathFor(name))
		if err != nil {
			if !os.IsNotExist(err) {
				slog.Warn("store: read cache", "subscription", name, "err", err)
			}
			continue
		}
		var entries []entry.Entry
		if err := json.Unmarshal(raw, &entries); err != nil {
			slog.Warn("store: corrupt cache ignored", "subscription", name, "err", err)
			continue
		}
		s.mu.Lock()
		s.bySub[name] = entries
		s.mu.Unlock()
		slog.Info("store: restored from cache", "subscription", name, "entries", len(entries))
	}
	return nil
}

func (s *Store) persist(subscription string, entries []entry.Entry) error {
	raw, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	tmp := s.pathFor(subscription) + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.pathFor(subscription))
}

func (s *Store) pathFor(subscription string) string {
	return filepath.Join(s.dir, subscription+".json")
}

package extras

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/egor-muindor/submix/internal/entry"
)

// UserResolver returns panel user data by shortUuid.
type UserResolver interface {
	UserFor(ctx context.Context, shortUUID string) (PanelUser, error)
}

// Server tells the mixer which entries to mix in for a given user.
type Server struct {
	cfg   *Config
	store *Store
	panel UserResolver
}

// NewServer assembles a server.
func NewServer(cfg *Config, store *Store, panel UserResolver) *Server {
	return &Server{cfg: cfg, store: store, panel: panel}
}

// Handler returns an http.Handler with all of the service's routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/extras", s.handleExtras)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return mux
}

func (s *Server) handleExtras(w http.ResponseWriter, r *http.Request) {
	shortUUID := r.URL.Query().Get("shortUuid")
	if shortUUID == "" {
		http.Error(w, "shortUuid is required", http.StatusBadRequest)
		return
	}
	format := r.URL.Query().Get("format")

	user, err := s.panel.UserFor(r.Context(), shortUUID)
	if err != nil {
		// Without user data we mix in nothing: otherwise an expired or disabled
		// user would get external nodes next to the panel's placeholder.
		// Not a 5xx: the mixer will pass the panel's response through as is.
		slog.Warn("extras: panel user lookup failed, nothing mixed", "subHash", subHash(shortUUID), "err", err)
		writeEntries(w, []entry.Entry{})
		return
	}
	if !user.Active {
		slog.Info("extras: user inactive, nothing mixed",
			"subHash", subHash(shortUUID), "format", format, "status", user.Status)
		writeEntries(w, []entry.Entry{})
		return
	}

	userTags := TagsFor(s.cfg.Users, user.Tag)
	entries := SelectEntries(s.candidates(), userTags)

	slog.Info("extras served",
		"subHash", subHash(shortUUID), "format", format, "panelTag", user.Tag,
		"userTags", userTags, "entries", len(entries))
	writeEntries(w, entries)
}

func writeEntries(w http.ResponseWriter, entries []entry.Entry) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(entry.Response{Entries: entries}); err != nil {
		slog.Error("extras: encode response", "err", err)
	}
}

// subHash returns a truncated hash of shortUuid for logs. shortUuid is the
// subscription secret (the panel serves the config by it with no authentication),
// so it is never written in the clear to logs that settle on the node's disk.
func subHash(shortUUID string) string {
	sum := sha256.Sum256([]byte(shortUUID))
	return fmt.Sprintf("%x", sum)[:8]
}

// candidates returns the entries of all subscriptions from the Store plus the
// static entries from the config.
func (s *Server) candidates() []entry.Entry {
	all := s.store.All()
	for _, static := range s.cfg.StaticEntries {
		all = append(all, entry.Entry{
			Name: static.Name,
			URI:  static.URI,
			Tags: static.Tags,
		})
	}
	return all
}

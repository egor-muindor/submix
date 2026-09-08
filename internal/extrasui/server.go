package extrasui

import (
	"embed"
	"encoding/json"
	"errors"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/egor-muindor/submix/internal/extras"
)

//go:embed static/index.html
var static embed.FS

// maxRequestBody caps the request body: a preview carries every fetched entry.
const maxRequestBody = 32 << 20

// Server holds the editor's HTTP handlers. There is no state between requests:
// the page sends the draft and the fetched entries.
type Server struct {
	configPath string
	client     *http.Client
}

// NewServer creates a server for the file at configPath. External
// subscriptions are downloaded through client (its timeout caps one fetch).
func NewServer(configPath string, client *http.Client) *Server {
	return &Server{configPath: configPath, client: client}
}

// Handler returns an http.Handler with all routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("GET /api/config", s.handleGetConfig)
	mux.HandleFunc("POST /api/import", requireJSON(s.handleImport))
	mux.HandleFunc("POST /api/fetch", requireJSON(s.handleFetch))
	mux.HandleFunc("POST /api/preview", requireJSON(s.handlePreview))
	mux.HandleFunc("POST /api/save", requireJSON(s.handleSave))
	return loopbackOnly(mux)
}

// loopbackOnly rejects requests whose Host does not point at localhost, on
// every route including GET /api/config: with DNS rebinding a foreign name
// resolves to 127.0.0.1 and looks same-origin to the browser, and the config
// holds subscription URLs with tokens, so reading is no safer than writing.
func loopbackOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isLoopbackHost(r.Host) {
			writeError(w, http.StatusForbidden, errors.New("host must be localhost"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// requireJSON lets through only requests with Content-Type application/json
// that did not come from a foreign site. Localhost is unreachable from the
// network but reachable from any tab of the same browser: an HTML form on a
// foreign page sends text/plain without a preflight and, without this check,
// would overwrite extras.yaml. A JSON fetch from a foreign origin hits the
// preflight, which the mux rejects (OPTIONS → 405).
func requireJSON(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
			writeError(w, http.StatusForbidden, errors.New("cross-site request rejected"))
			return
		}
		ct, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if ct != "application/json" {
			writeError(w, http.StatusUnsupportedMediaType, errors.New("content-type must be application/json"))
			return
		}
		next(w, r)
	}
}

type configResponse struct {
	Path   string         `json:"path"`
	Config *extras.Config `json:"config,omitempty"`
	Error  string         `json:"error,omitempty"`
}

type importRequest struct {
	YAML string `json:"yaml"`
}

type fetchRequest struct {
	URL       string `json:"url"`
	UserAgent string `json:"user_agent"`
}

type fetchResponse struct {
	Entries   []FetchedEntry `json:"entries"`
	FetchedAt string         `json:"fetched_at"`
}

type saveRequest struct {
	Config extras.Config `json:"config"`
}

type saveResponse struct {
	Path string `json:"path"`
	YAML string `json:"yaml"`
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	data, err := static.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	raw, err := os.ReadFile(s.configPath)
	if errors.Is(err, os.ErrNotExist) {
		writeJSON(w, http.StatusOK, configResponse{Path: s.configPath, Config: normalize(&extras.Config{})})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, configResponse{Path: s.configPath, Error: err.Error()})
		return
	}
	cfg, err := extras.Decode(raw)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, configResponse{Path: s.configPath, Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, configResponse{Path: s.configPath, Config: normalize(cfg)})
}

func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	var req importRequest
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	cfg, err := extras.Decode([]byte(req.YAML))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, configResponse{Path: s.configPath, Config: normalize(cfg)})
}

func (s *Server) handleFetch(w http.ResponseWriter, r *http.Request) {
	var req fetchRequest
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.URL == "" {
		writeError(w, http.StatusBadRequest, errors.New("url is required"))
		return
	}
	ua := req.UserAgent
	if ua == "" {
		ua = extras.DefaultUserAgent
	}
	entries, err := extras.FetchEntries(r.Context(), s.client, req.URL, ua)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	out := make([]FetchedEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, FetchedEntry{Name: e.Name, URI: e.URI, Errors: ConversionErrors(e)})
	}
	writeJSON(w, http.StatusOK, fetchResponse{Entries: out, FetchedAt: time.Now().UTC().Format(time.RFC3339)})
}

func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request) {
	var req PreviewRequest
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, Preview(req))
}

func (s *Server) handleSave(w http.ResponseWriter, r *http.Request) {
	var req saveRequest
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	text, err := RenderYAML(&req.Config)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if _, err := extras.Parse([]byte(text)); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := writeAtomic(s.configPath, []byte(text)); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, saveResponse{Path: s.configPath, YAML: text})
}

// isLoopbackHost reports whether the Host header points at localhost or a
// loopback address (with or without a port).
func isLoopbackHost(hostport string) bool {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		host = hostport
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// writeAtomic writes to a temporary file next to path and renames it over the
// target: on any error the old file stays untouched.
func writeAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

// normalize replaces nil slices and maps with empty ones so the page receives
// [] and {} instead of null.
func normalize(cfg *extras.Config) *extras.Config {
	if cfg.Subscriptions == nil {
		cfg.Subscriptions = []extras.Subscription{}
	}
	for i := range cfg.Subscriptions {
		s := &cfg.Subscriptions[i]
		if s.Rules == nil {
			s.Rules = []extras.Rule{}
		}
		for j := range s.Rules {
			if s.Rules[j].Tags == nil {
				s.Rules[j].Tags = []string{}
			}
		}
	}
	if cfg.StaticEntries == nil {
		cfg.StaticEntries = []extras.StaticEntry{}
	}
	for i := range cfg.StaticEntries {
		if cfg.StaticEntries[i].Tags == nil {
			cfg.StaticEntries[i].Tags = []string{}
		}
	}
	if cfg.Users.DefaultTags == nil {
		cfg.Users.DefaultTags = []string{}
	}
	if cfg.Users.ByPanelTag == nil {
		cfg.Users.ByPanelTag = map[string][]string{}
	}
	return cfg
}

func readJSON(w http.ResponseWriter, r *http.Request, v any) error {
	return json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBody)).Decode(v)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

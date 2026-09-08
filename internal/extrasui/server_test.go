package extrasui_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egor-muindor/submix/internal/extras"
	"github.com/egor-muindor/submix/internal/extrasui"
)

const validYAML = `defaults:
  refresh: 6h
subscriptions:
  - name: provider-a
    url: https://provider.example/sub/token
    rules:
      - match: "Germany|DE"
        tags: [de]
static_entries:
  - name: "My exit"
    uri: "vless://uuid@my.example.com:443?security=tls#My"
    tags: [premium]
users:
  default_tags: [bulk]
  by_panel_tag:
    PREMIUM: [de, premium]
`

func newTestServer(t *testing.T, configYAML string) (http.Handler, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "extras.yaml")
	if configYAML != "" {
		if err := os.WriteFile(path, []byte(configYAML), 0o600); err != nil {
			t.Fatalf("write config: %v", err)
		}
	}
	client := &http.Client{Timeout: 5 * time.Second}
	return extrasui.NewServer(path, client).Handler(), path
}

func call(t *testing.T, h http.Handler, method, target string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw := []byte(nil)
	if body != nil {
		var err error
		if raw, err = json.Marshal(body); err != nil {
			t.Fatalf("marshal body: %v", err)
		}
	}
	req := httptest.NewRequest(method, target, bytes.NewReader(raw))
	req.Host = "127.0.0.1:3040" // httptest defaults to example.com, but the server admits only localhost
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeConfigResponse(t *testing.T, rec *httptest.ResponseRecorder) (string, json.RawMessage) {
	t.Helper()
	var resp struct {
		Path   string          `json:"path"`
		Config json.RawMessage `json:"config"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v\nbody=%s", err, rec.Body.String())
	}
	return resp.Path, resp.Config
}

func TestConfigRoundTripThroughSave(t *testing.T) {
	h, path := newTestServer(t, validYAML)

	rec := call(t, h, http.MethodGet, "/api/config", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET config: %d %s", rec.Code, rec.Body.String())
	}
	gotPath, before := decodeConfigResponse(t, rec)
	if gotPath != path {
		t.Fatalf("path = %q, want %q", gotPath, path)
	}

	rec = call(t, h, http.MethodPost, "/api/save", map[string]any{"config": before})
	if rec.Code != http.StatusOK {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}

	rec = call(t, h, http.MethodGet, "/api/config", nil)
	_, after := decodeConfigResponse(t, rec)
	if !bytes.Equal(before, after) {
		t.Fatalf("config changed after save round trip:\nbefore=%s\nafter=%s", before, after)
	}

	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved: %v", err)
	}
	if strings.Contains(string(saved), "user_agent") {
		t.Fatalf("saved yaml must omit empty user_agent:\n%s", saved)
	}
	if _, err := extras.Parse(saved); err != nil {
		t.Fatalf("saved yaml is not valid for extras-api: %v", err)
	}
}

func TestSaveRejectsInvalidConfigAndKeepsFile(t *testing.T) {
	h, path := newTestServer(t, validYAML)
	draft := map[string]any{
		"subscriptions": []map[string]any{{"name": "p", "url": "https://p.example/sub", "rules": []any{}}},
		"users":         map[string]any{"default_tags": []string{}},
	}
	rec := call(t, h, http.MethodPost, "/api/save", map[string]any{"config": draft})
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "at least one rule") {
		t.Fatalf("save invalid: %d %s", rec.Code, rec.Body.String())
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != validYAML {
		t.Fatalf("file must stay untouched, got:\n%s", got)
	}
}

func TestConfigMissingFileGivesEmptyTemplate(t *testing.T) {
	h, _ := newTestServer(t, "")
	rec := call(t, h, http.MethodGet, "/api/config", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET config: %d %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{`"subscriptions":[]`, `"static_entries":[]`, `"default_tags":[]`, `"by_panel_tag":{}`} {
		if !strings.Contains(body, want) {
			t.Fatalf("template lacks %s: %s", want, body)
		}
	}
}

func TestConfigBrokenFileGives400WithPath(t *testing.T) {
	h, path := newTestServer(t, "subscriptions: [\n")
	rec := call(t, h, http.MethodGet, "/api/config", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("GET config: %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), path) || !strings.Contains(rec.Body.String(), `"error"`) {
		t.Fatalf("body must carry path and error: %s", rec.Body.String())
	}
}

func TestImportDecodesWithoutValidation(t *testing.T) {
	h, _ := newTestServer(t, "")
	rec := call(t, h, http.MethodPost, "/api/import", map[string]string{
		"yaml": "subscriptions:\n  - name: draft\n    url: https://p.example/sub\n",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("import draft: %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"name":"draft"`) || !strings.Contains(rec.Body.String(), `"rules":[]`) {
		t.Fatalf("import body: %s", rec.Body.String())
	}

	rec = call(t, h, http.MethodPost, "/api/import", map[string]string{"yaml": "unknown_field: 1\n"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("import unknown field: %d %s", rec.Code, rec.Body.String())
	}
}

func TestFetchReturnsEntriesWithConversionErrors(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("vless://11111111-2222-3333-4444-555555555555@de.example.com:443?type=tcp&security=tls&sni=de.example.com#DE-1\n" +
			"hysteria2://pass@h.example.com:443#H2\n"))
	}))
	defer provider.Close()

	h, _ := newTestServer(t, "")
	rec := call(t, h, http.MethodPost, "/api/fetch", map[string]string{"url": provider.URL, "user_agent": ""})
	if rec.Code != http.StatusOK {
		t.Fatalf("fetch: %d %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Entries   []extrasui.FetchedEntry `json:"entries"`
		FetchedAt string                  `json:"fetched_at"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Entries) != 2 || resp.FetchedAt == "" {
		t.Fatalf("resp = %+v", resp)
	}
	if resp.Entries[0].Errors != nil {
		t.Fatalf("DE-1 must convert: %v", resp.Entries[0].Errors)
	}
	if !strings.Contains(resp.Entries[1].Errors["clash"], "unsupported scheme") {
		t.Fatalf("H2 errors = %v", resp.Entries[1].Errors)
	}

	provider.Close()
	rec = call(t, h, http.MethodPost, "/api/fetch", map[string]string{"url": provider.URL})
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("fetch dead provider: %d %s", rec.Code, rec.Body.String())
	}

	rec = call(t, h, http.MethodPost, "/api/fetch", map[string]string{"url": ""})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("fetch without url: %d", rec.Code)
	}
}

func TestPostRejectsNonJSONAndCrossSite(t *testing.T) {
	h, path := newTestServer(t, validYAML)

	// HTML form from a foreign site: text/plain, no preflight.
	req := httptest.NewRequest(http.MethodPost, "/api/save",
		strings.NewReader(`{"config":{"subscriptions":[],"users":{"default_tags":[]}}}`))
	req.Host = "localhost:3040"
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("text/plain save: %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/save", strings.NewReader(`{"config":{}}`))
	req.Host = "127.0.0.1:3040"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("cross-site save: %d %s", rec.Code, rec.Body.String())
	}

	// DNS rebinding: a foreign name resolves to 127.0.0.1, the browser sends same-origin.
	req = httptest.NewRequest(http.MethodPost, "/api/save", strings.NewReader(`{"config":{}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Host = "evil.example:3040"
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("foreign host save: %d %s", rec.Code, rec.Body.String())
	}
	// Reading the config (it holds subscription URLs with tokens) is blocked just like writing.
	req = httptest.NewRequest(http.MethodGet, "/api/config", nil)
	req.Host = "evil.example"
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || strings.Contains(rec.Body.String(), "provider.example") {
		t.Fatalf("foreign host config read: %d %s", rec.Code, rec.Body.String())
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != validYAML {
		t.Fatalf("file must stay untouched, got:\n%s", got)
	}
}

func TestSaveWriteFailureKeepsFile(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	h, path := newTestServer(t, validYAML)
	dir := filepath.Dir(path)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	rec := call(t, h, http.MethodGet, "/api/config", nil)
	_, cfg := decodeConfigResponse(t, rec)
	rec = call(t, h, http.MethodPost, "/api/save", map[string]any{"config": cfg})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("save into read-only dir: %d %s", rec.Code, rec.Body.String())
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != validYAML {
		t.Fatalf("file must stay untouched, got:\n%s", got)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("temp file left behind: %v", entries)
	}
}

func TestSaveRenameFailureRemovesTempFile(t *testing.T) {
	// configPath is a non-empty directory: CreateTemp and the write succeed,
	// only rename fails. This exercises the temp-file removal branch.
	dir := t.TempDir()
	configPath := filepath.Join(dir, "extras.yaml")
	if err := os.MkdirAll(filepath.Join(configPath, "child"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	h := extrasui.NewServer(configPath, &http.Client{Timeout: time.Second}).Handler()

	rec := call(t, h, http.MethodPost, "/api/save", map[string]any{
		"config": map[string]any{"subscriptions": []any{}, "users": map[string]any{"default_tags": []string{}}},
	})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("save over a directory: %d %s", rec.Code, rec.Body.String())
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	if len(entries) != 1 || !entries[0].IsDir() {
		t.Fatalf("temp file left behind: %v", entries)
	}
}

func TestMalformedJSONAndRouting(t *testing.T) {
	h, _ := newTestServer(t, validYAML)

	req := httptest.NewRequest(http.MethodPost, "/api/preview", strings.NewReader(`{"config":`))
	req.Host = "[::1]:3040"
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"error"`) {
		t.Fatalf("malformed json: %d %s", rec.Code, rec.Body.String())
	}

	if rec := call(t, h, http.MethodGet, "/nope", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown path: %d", rec.Code)
	}
	if rec := call(t, h, http.MethodGet, "/api/save", nil); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /api/save: %d", rec.Code)
	}
}

func TestPreviewEndpointAndIndexPage(t *testing.T) {
	h, _ := newTestServer(t, validYAML)
	rec := call(t, h, http.MethodPost, "/api/preview", map[string]any{
		"config":    map[string]any{"subscriptions": []any{}, "users": map[string]any{"default_tags": []string{"x"}}},
		"fetched":   map[string]any{},
		"panel_tag": "",
	})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"tags":["x"]`) {
		t.Fatalf("preview: %d %s", rec.Code, rec.Body.String())
	}

	rec = call(t, h, http.MethodGet, "/", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "<title>extras-ui</title>") {
		t.Fatalf("index: %d", rec.Code)
	}
}

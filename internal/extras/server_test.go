package extras_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/egor-muindor/submix/internal/entry"
	"github.com/egor-muindor/submix/internal/extras"
)

// fakeResolver is an active user with tag, or a panel error, or an inactive
// user (inactive).
type fakeResolver struct {
	tag      string
	err      error
	inactive bool
}

func (f fakeResolver) UserFor(context.Context, string) (extras.PanelUser, error) {
	if f.err != nil {
		return extras.PanelUser{}, f.err
	}
	return extras.PanelUser{Username: "u", Tag: f.tag, Active: !f.inactive, Status: "ACTIVE"}, nil
}

func newTestServer(t *testing.T, resolver extras.UserResolver) http.Handler {
	t.Helper()

	cfg, err := extras.Load("testdata/valid.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	store, err := extras.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	store.Set("provider-a", []entry.Entry{
		{Name: "DE-1", URI: "vless://u@de.example.com:443#DE-1", Tags: []string{"de"}},
		{Name: "GE-1", URI: "vless://u@ge.example.com:443#GE-1", Tags: []string{"ge"}},
	})
	store.Set("provider-b", []entry.Entry{
		{Name: "BULK-1", URI: "vless://u@bulk.example.com:443#BULK-1", Tags: []string{"bulk"}},
	})

	return extras.NewServer(cfg, store, resolver).Handler()
}

func decodeEntries(t *testing.T, rec *httptest.ResponseRecorder) []entry.Entry {
	t.Helper()
	var resp entry.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v\nbody=%s", err, rec.Body.String())
	}
	return resp.Entries
}

func TestServerFiltersByPanelTag(t *testing.T) {
	h := newTestServer(t, fakeResolver{tag: "PREMIUM"}) // -> de, premium (+ default bulk)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/extras?shortUuid=s1&format=clash", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	names := map[string]bool{}
	for _, e := range decodeEntries(t, rec) {
		names[e.Name] = true
	}
	if !names["DE-1"] || !names["BULK-1"] || !names["My exit"] {
		t.Fatalf("names = %#v (want DE-1, BULK-1 and the premium static entry)", names)
	}
	if names["GE-1"] {
		t.Fatalf("GE-1 must not be returned for PREMIUM: %#v", names)
	}
}

func TestServerUnknownTagGetsDefaultsOnly(t *testing.T) {
	h := newTestServer(t, fakeResolver{tag: "NOT_MAPPED"})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/extras?shortUuid=s1", nil))

	entries := decodeEntries(t, rec)
	if len(entries) != 1 || entries[0].Name != "BULK-1" {
		t.Fatalf("entries = %#v, want only default_tags (bulk)", entries)
	}
}

func TestServerPanelErrorMixesNothing(t *testing.T) {
	h := newTestServer(t, fakeResolver{err: context.DeadlineExceeded})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/extras?shortUuid=s1", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, must stay 200 to keep the mixer fail-open", rec.Code)
	}
	// Without user data nothing may be mixed in: the user may be expired.
	if entries := decodeEntries(t, rec); len(entries) != 0 {
		t.Fatalf("entries = %#v, want none when the panel is unavailable", entries)
	}
	if !strings.Contains(rec.Body.String(), `"entries":[]`) {
		t.Fatalf("body must carry an empty array, got %s", rec.Body.String())
	}
}

func TestServerInactiveUserGetsNothing(t *testing.T) {
	h := newTestServer(t, fakeResolver{tag: "PREMIUM", inactive: true})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/extras?shortUuid=s1&format=clash", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if entries := decodeEntries(t, rec); len(entries) != 0 {
		t.Fatalf("expired/disabled user must get no extras, got %#v", entries)
	}
}

func TestServerRequiresShortUuid(t *testing.T) {
	h := newTestServer(t, fakeResolver{tag: "PREMIUM"})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/extras", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestServerHealthz(t *testing.T) {
	h := newTestServer(t, fakeResolver{tag: ""})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
}

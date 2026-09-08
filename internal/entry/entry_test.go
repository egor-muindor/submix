package entry_test

import (
	"encoding/json"
	"testing"

	"github.com/egor-muindor/submix/internal/entry"
)

func TestEntryJSONRoundTrip(t *testing.T) {
	raw := `{"entries":[{"name":"DE-1","uri":"vless://x@h:443","tags":["de"],` +
		`"overrides":{"singbox":{"type":"hysteria2"}}}]}`

	var resp entry.Response
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(resp.Entries))
	}
	e := resp.Entries[0]
	if e.Name != "DE-1" || e.URI != "vless://x@h:443" {
		t.Fatalf("bad entry: %+v", e)
	}
	if got := e.Tags; len(got) != 1 || got[0] != "de" {
		t.Fatalf("bad tags: %v", got)
	}
	if _, ok := e.Overrides["singbox"]; !ok {
		t.Fatalf("singbox override missing: %v", e.Overrides)
	}
}

func TestHasTagIntersection(t *testing.T) {
	e := entry.Entry{Tags: []string{"de", "premium"}}
	if !e.MatchesAny([]string{"premium"}) {
		t.Fatal("want match on premium")
	}
	if e.MatchesAny([]string{"ge"}) {
		t.Fatal("unexpected match on ge")
	}
	if e.MatchesAny(nil) {
		t.Fatal("empty user tags must not match")
	}
}

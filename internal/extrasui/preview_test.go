package extrasui_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/egor-muindor/submix/internal/extras"
	"github.com/egor-muindor/submix/internal/extrasui"
)

func previewConfig() extras.Config {
	return extras.Config{
		Subscriptions: []extras.Subscription{{
			Name: "p",
			URL:  "https://p.example/sub",
			Rules: []extras.Rule{
				{Match: "DE", Tags: []string{"de"}},
				{Match: "(", Tags: []string{"broken"}},
				{Match: "Berlin", Tags: []string{"de", "capital"}},
				{Match: "Paris", Tags: nil},
			},
		}},
		StaticEntries: []extras.StaticEntry{{
			Name: "My exit",
			URI:  "vless://11111111-2222-3333-4444-555555555555@my.example.com:443?type=tcp&security=tls&sni=my.example.com#My",
			Tags: []string{"de"},
		}},
		Users: extras.Users{
			DefaultTags: []string{"bulk"},
			ByPanelTag:  map[string][]string{"PREMIUM": {"de"}},
		},
	}
}

func previewFetched() map[string]extrasui.FetchedSubscription {
	return map[string]extrasui.FetchedSubscription{
		"p": {Entries: []extrasui.FetchedEntry{
			{Name: "DE-Berlin", URI: "vless://u@de.example.com:443?security=tls#DE-Berlin"},
			{Name: "FR-Paris", URI: "vless://u@fr.example.com:443?security=tls#FR-Paris"},
		}},
	}
}

func TestPreviewMatchesRulesAndReportsBadRegexp(t *testing.T) {
	resp := extrasui.Preview(extrasui.PreviewRequest{Config: previewConfig(), Fetched: previewFetched()})

	p, ok := resp.Subscriptions["p"]
	if !ok {
		t.Fatalf("no preview for subscription p: %+v", resp.Subscriptions)
	}
	if p.RuleErrors["1"] == "" {
		t.Fatalf("rule #2 has a bad regexp, want error, got %v", p.RuleErrors)
	}
	if p.RuleErrors["3"] != "tags are required" {
		t.Fatalf("rule #4 has no tags, want 'tags are required', got %v", p.RuleErrors)
	}
	if len(p.Entries) != 2 {
		t.Fatalf("entries = %+v", p.Entries)
	}
	berlin := p.Entries[0]
	if !reflect.DeepEqual(berlin.Matched, []int{0, 2}) || !reflect.DeepEqual(berlin.Tags, []string{"de", "capital"}) {
		t.Fatalf("DE-Berlin = %+v, want matched [0 2] tags [de capital]", berlin)
	}
	paris := p.Entries[1]
	if !reflect.DeepEqual(paris.Matched, []int{3}) || len(paris.Tags) != 0 || paris.Tags == nil {
		t.Fatalf("FR-Paris = %+v, want matched [3] (tagless rule still matches) and empty non-nil tags", paris)
	}
	if !strings.Contains(resp.ValidationError, "bad match regexp") {
		t.Fatalf("validation_error = %q, want bad match regexp", resp.ValidationError)
	}
}

func TestPreviewResolvesUserByPanelTag(t *testing.T) {
	cfg := previewConfig()
	cfg.Subscriptions[0].Rules = cfg.Subscriptions[0].Rules[:1] // only DE -> de

	resp := extrasui.Preview(extrasui.PreviewRequest{Config: cfg, Fetched: previewFetched(), PanelTag: "PREMIUM"})
	if resp.ValidationError != "" {
		t.Fatalf("unexpected validation error: %s", resp.ValidationError)
	}
	if !reflect.DeepEqual(resp.User.Tags, []string{"bulk", "de"}) {
		t.Fatalf("user tags = %v", resp.User.Tags)
	}
	if len(resp.User.Entries) != 2 {
		t.Fatalf("user entries = %+v", resp.User.Entries)
	}
	if resp.User.Entries[0].Name != "DE-Berlin" || resp.User.Entries[0].Source != "p" {
		t.Fatalf("entry 0 = %+v, want DE-Berlin from p", resp.User.Entries[0])
	}
	if resp.User.Entries[1].Name != "My exit" || resp.User.Entries[1].Source != "static" {
		t.Fatalf("entry 1 = %+v, want My exit from static", resp.User.Entries[1])
	}

	none := extrasui.Preview(extrasui.PreviewRequest{Config: cfg, Fetched: previewFetched()})
	if !reflect.DeepEqual(none.User.Tags, []string{"bulk"}) || len(none.User.Entries) != 0 || none.User.Entries == nil {
		t.Fatalf("user without panel tag = %+v, want tags [bulk] and empty non-nil entries", none.User)
	}
}

func TestPreviewOrdersUserEntriesLikeProduction(t *testing.T) {
	rule := []extras.Rule{{Match: "DE", Tags: []string{"de"}}}
	cfg := extras.Config{
		Subscriptions: []extras.Subscription{
			{Name: "zeta", URL: "https://z.example/sub", Rules: rule},
			{Name: "alpha", URL: "https://a.example/sub", Rules: rule},
		},
		Users: extras.Users{DefaultTags: []string{"de"}},
	}
	fetched := map[string]extrasui.FetchedSubscription{
		"zeta":  {Entries: []extrasui.FetchedEntry{{Name: "DE-z", URI: "vless://u@z.example.com:443"}}},
		"alpha": {Entries: []extrasui.FetchedEntry{{Name: "DE-a", URI: "vless://u@a.example.com:443"}}},
	}

	resp := extrasui.Preview(extrasui.PreviewRequest{Config: cfg, Fetched: fetched})
	if len(resp.User.Entries) != 2 || resp.User.Entries[0].Source != "alpha" || resp.User.Entries[1].Source != "zeta" {
		t.Fatalf("user entries = %+v, want alpha before zeta (Store.All sorts by name)", resp.User.Entries)
	}
}

func TestPreviewReportsStaticConversionErrorsAndYAML(t *testing.T) {
	cfg := previewConfig()
	cfg.Subscriptions[0].Rules = cfg.Subscriptions[0].Rules[:1]
	cfg.StaticEntries = append(cfg.StaticEntries, extras.StaticEntry{
		Name: "H2", URI: "hysteria2://pass@h.example.com:443#H2", Tags: []string{"de"},
	})

	resp := extrasui.Preview(extrasui.PreviewRequest{Config: cfg})
	if len(resp.StaticErrors) != 2 {
		t.Fatalf("static_errors = %v", resp.StaticErrors)
	}
	if resp.StaticErrors[0] != nil {
		t.Fatalf("valid static entry must have no errors: %v", resp.StaticErrors[0])
	}
	if !strings.Contains(resp.StaticErrors[1]["clash"], "unsupported scheme") {
		t.Fatalf("static_errors[1] = %v", resp.StaticErrors[1])
	}
	if !strings.Contains(resp.YAML, "name: p") {
		t.Fatalf("yaml not rendered:\n%s", resp.YAML)
	}
	if p := resp.Subscriptions["p"]; p.Entries == nil || len(p.Entries) != 0 {
		t.Fatalf("subscription without fetched data must have empty non-nil entries: %+v", p)
	}
}

func TestPreviewValidationErrorForDraft(t *testing.T) {
	cfg := previewConfig()
	cfg.Subscriptions[0].Rules = nil
	resp := extrasui.Preview(extrasui.PreviewRequest{Config: cfg})
	if !strings.Contains(resp.ValidationError, "at least one rule") {
		t.Fatalf("validation_error = %q", resp.ValidationError)
	}
}

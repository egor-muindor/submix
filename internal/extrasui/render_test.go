package extrasui_test

import (
	"strings"
	"testing"

	"github.com/egor-muindor/submix/internal/extras"
	"github.com/egor-muindor/submix/internal/extrasui"
)

func TestRenderYAMLOmitsEmptyOptionalFields(t *testing.T) {
	cfg := &extras.Config{
		Subscriptions: []extras.Subscription{{
			Name:  "p",
			URL:   "https://p.example/sub",
			Rules: []extras.Rule{{Match: "DE", Tags: []string{"de"}}},
		}},
		Users: extras.Users{DefaultTags: []string{}},
	}
	text, err := extrasui.RenderYAML(cfg)
	if err != nil {
		t.Fatalf("RenderYAML: %v", err)
	}
	for _, absent := range []string{"user_agent", "refresh", "defaults", "static_entries", "by_panel_tag"} {
		if strings.Contains(text, absent) {
			t.Fatalf("rendered yaml must omit empty %q:\n%s", absent, text)
		}
	}
	if !strings.Contains(text, "default_tags: []") {
		t.Fatalf("default_tags must be rendered explicitly:\n%s", text)
	}
}

func TestRenderYAMLRoundTripsEmojiThroughParse(t *testing.T) {
	const match = "🇩🇪|Germany"
	cfg := &extras.Config{
		Subscriptions: []extras.Subscription{{
			Name:  "p",
			URL:   "https://p.example/sub",
			Rules: []extras.Rule{{Match: match, Tags: []string{"de"}}},
		}},
	}
	text, err := extrasui.RenderYAML(cfg)
	if err != nil {
		t.Fatalf("RenderYAML: %v", err)
	}
	back, err := extras.Parse([]byte(text))
	if err != nil {
		t.Fatalf("Parse(rendered): %v\n%s", err, text)
	}
	if got := back.Subscriptions[0].Rules[0].Match; got != match {
		t.Fatalf("match after round trip = %q, want %q", got, match)
	}
}

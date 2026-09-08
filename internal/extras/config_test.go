package extras_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egor-muindor/submix/internal/extras"
)

func TestLoadValidConfig(t *testing.T) {
	cfg, err := extras.Load("testdata/valid.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(cfg.Subscriptions) != 2 {
		t.Fatalf("subscriptions = %d", len(cfg.Subscriptions))
	}

	a := cfg.Subscriptions[0]
	if a.UA != extras.DefaultUserAgent {
		t.Fatalf("provider-a UA = %q, want default", a.UA)
	}
	if a.Interval != 6*time.Hour {
		t.Fatalf("provider-a interval = %v, want defaults.refresh", a.Interval)
	}
	if len(a.Rules) != 2 || a.Rules[0].Re == nil {
		t.Fatalf("rules not compiled: %#v", a.Rules)
	}

	b := cfg.Subscriptions[1]
	if b.UA != "v2rayNG/1.8.5" || b.Interval != time.Hour {
		t.Fatalf("provider-b = %+v", b)
	}

	if len(cfg.StaticEntries) != 1 || cfg.StaticEntries[0].Name != "My exit" {
		t.Fatalf("static entries = %#v", cfg.StaticEntries)
	}
	if got := cfg.Users.ByPanelTag["PREMIUM"]; len(got) != 2 {
		t.Fatalf("by_panel_tag = %#v", cfg.Users.ByPanelTag)
	}
	if got := cfg.Users.DefaultTags; len(got) != 1 || got[0] != "bulk" {
		t.Fatalf("default_tags = %#v", got)
	}
}

func TestLoadDefaultsWhenOmitted(t *testing.T) {
	path := writeTempConfig(t, `
subscriptions:
  - name: only
    url: https://p.example/sub
    rules:
      - match: ".*"
        tags: [all]
users:
  by_panel_tag:
    PREMIUM: [all]
`)
	cfg, err := extras.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Subscriptions[0].Interval != 24*time.Hour {
		t.Fatalf("interval = %v, want 24h default", cfg.Subscriptions[0].Interval)
	}
	if cfg.Subscriptions[0].UA != extras.DefaultUserAgent {
		t.Fatalf("ua = %q", cfg.Subscriptions[0].UA)
	}
}

func TestLoadRejectsBadConfigs(t *testing.T) {
	cases := map[string]string{
		"duplicate name": `
subscriptions:
  - name: dup
    url: https://a.example/sub
    rules: [{match: ".*", tags: [x]}]
  - name: dup
    url: https://b.example/sub
    rules: [{match: ".*", tags: [x]}]
`,
		"no rules": `
subscriptions:
  - name: a
    url: https://a.example/sub
`,
		"bad regex": `
subscriptions:
  - name: a
    url: https://a.example/sub
    rules: [{match: "([", tags: [x]}]
`,
		"rule without tags": `
subscriptions:
  - name: a
    url: https://a.example/sub
    rules: [{match: ".*", tags: []}]
`,
		"rule with empty match": `
subscriptions:
  - name: a
    url: https://a.example/sub
    rules: [{match: "", tags: [x]}]
`,
		"bad url": `
subscriptions:
  - name: a
    url: "not a url"
    rules: [{match: ".*", tags: [x]}]
`,
		"refresh too small": `
defaults:
  refresh: 5s
subscriptions:
  - name: a
    url: https://a.example/sub
    rules: [{match: ".*", tags: [x]}]
`,
		"static entry without tags": `
static_entries:
  - name: x
    uri: "vless://u@h:443"
`,
		"unknown field": `
subscriptionz: []
`,
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := extras.Load(writeTempConfig(t, body)); err == nil {
				t.Fatalf("want error for %q", name)
			}
		})
	}
}

func TestDecodeAcceptsDraftValidateRejectsIt(t *testing.T) {
	draft := []byte(`
subscriptions:
  - name: draft
    url: https://p.example/sub
users:
  default_tags: []
`)
	cfg, err := extras.Decode(draft)
	if err != nil {
		t.Fatalf("Decode must accept a draft without rules: %v", err)
	}
	if len(cfg.Subscriptions) != 1 || cfg.Subscriptions[0].Name != "draft" {
		t.Fatalf("decoded = %+v", cfg.Subscriptions)
	}
	if err := extras.Validate(cfg); err == nil {
		t.Fatal("Validate must reject a subscription without rules")
	}
	if _, err := extras.Parse(draft); err == nil {
		t.Fatal("Parse must reject a subscription without rules")
	}
}

func TestDecodeEmptyDocumentGivesEmptyConfig(t *testing.T) {
	for name, raw := range map[string]string{"empty": "", "comment only": "# nothing here\n"} {
		cfg, err := extras.Decode([]byte(raw))
		if err != nil {
			t.Fatalf("%s: Decode = %v, want empty config", name, err)
		}
		if len(cfg.Subscriptions) != 0 || cfg.Users.ByPanelTag != nil {
			t.Fatalf("%s: cfg = %+v, want zero value", name, cfg)
		}
	}
}

func TestValidateClearsStaleRegexpOnFailure(t *testing.T) {
	cfg, err := extras.Parse([]byte(`
subscriptions:
  - name: p
    url: https://p.example/sub
    rules:
      - match: "DE"
        tags: [de]
      - match: "FR"
        tags: [fr]
`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	cfg.Subscriptions[0].Rules[0].Match = "DE("
	cfg.Subscriptions[0].Rules[1].Match = "FR-new"
	if err := extras.Validate(cfg); err == nil {
		t.Fatal("Validate must reject a bad regexp")
	}
	for j, r := range cfg.Subscriptions[0].Rules {
		if r.Re != nil {
			t.Fatalf("rule #%d: Validate left a previously compiled regexp in place", j+1)
		}
	}
}

func TestLoadRejectsEmptyFile(t *testing.T) {
	path := writeTempConfig(t, "\n# only a comment\n")
	_, err := extras.Load(path)
	if err == nil || !strings.Contains(err.Error(), "is empty") {
		t.Fatalf("Load(empty) = %v, want 'is empty' error", err)
	}
}

func TestParseCompilesRulesAndFillsDefaults(t *testing.T) {
	cfg, err := extras.Parse([]byte(`
subscriptions:
  - name: p
    url: https://p.example/sub
    rules:
      - match: "DE"
        tags: [de]
`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	s := cfg.Subscriptions[0]
	if s.Rules[0].Re == nil {
		t.Fatal("rule not compiled")
	}
	if s.UA != extras.DefaultUserAgent || s.Interval != extras.DefaultRefresh {
		t.Fatalf("defaults not applied: UA=%q Interval=%v", s.UA, s.Interval)
	}
}

func TestConfigJSONUsesYAMLFieldNames(t *testing.T) {
	cfg, err := extras.Load("testdata/valid.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	for _, key := range []string{`"static_entries"`, `"by_panel_tag"`, `"default_tags"`, `"user_agent"`, `"subscriptions"`} {
		if !bytes.Contains(raw, []byte(key)) {
			t.Fatalf("json lacks %s: %s", key, raw)
		}
	}
	for _, leak := range []string{`"Re"`, `"UA"`, `"Interval"`} {
		if bytes.Contains(raw, []byte(leak)) {
			t.Fatalf("json leaks internal field %s: %s", leak, raw)
		}
	}
}

func writeTempConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "extras.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

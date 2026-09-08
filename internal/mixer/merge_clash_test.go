package mixer_test

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/egor-muindor/submix/internal/entry"
	"github.com/egor-muindor/submix/internal/mixer"
)

const clashBody = `# comment from panel template
mixed-port: 7890
proxies:
  - name: Panel-1
    type: vless
    server: panel.example.com
    port: 443
proxy-groups:
  - name: Selector
    type: select
    proxies:
      - Panel-1
  - name: Auto
    type: url-test
    proxies:
      - Panel-1
  - name: Direct-Only
    type: select
    proxies:
      - DIRECT
rules:
  - MATCH,Selector
`

func TestMergeClashAddsProxyAndGroups(t *testing.T) {
	entries := []entry.Entry{
		{Name: "DE-1", URI: "vless://uuid@de.example.com:443?type=tcp&security=tls&sni=de.example.com#Provider"},
	}

	out, err := mixer.MergeClash([]byte(clashBody), entries)
	if err != nil {
		t.Fatalf("MergeClash: %v", err)
	}

	var doc struct {
		Proxies     []map[string]any `yaml:"proxies"`
		ProxyGroups []struct {
			Name    string   `yaml:"name"`
			Type    string   `yaml:"type"`
			Proxies []string `yaml:"proxies"`
		} `yaml:"proxy-groups"`
		Rules []string `yaml:"rules"`
	}
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("unmarshal result: %v\n%s", err, out)
	}

	if len(doc.Proxies) != 2 {
		t.Fatalf("proxies = %#v", doc.Proxies)
	}
	if doc.Proxies[1]["name"] != "DE-1" || doc.Proxies[1]["type"] != "vless" {
		t.Fatalf("added proxy = %#v", doc.Proxies[1])
	}
	if len(doc.Rules) != 1 {
		t.Fatalf("rules must be preserved: %#v", doc.Rules)
	}

	for _, g := range doc.ProxyGroups {
		last := g.Proxies[len(g.Proxies)-1]
		switch g.Name {
		case "Selector", "Auto":
			if last != "DE-1" {
				t.Fatalf("group %q proxies = %#v", g.Name, g.Proxies)
			}
		case "Direct-Only":
			if last != "DE-1" {
				t.Fatalf("group %q must also receive the proxy: %#v", g.Name, g.Proxies)
			}
		}
	}
}

func TestMergeClashSkipsUnconvertibleEntries(t *testing.T) {
	entries := []entry.Entry{
		{Name: "H2", URI: "hysteria2://pw@h.example.com:443#H2"},
		{Name: "DE-1", URI: "vless://uuid@de.example.com:443?type=tcp&security=tls#DE"},
	}

	out, err := mixer.MergeClash([]byte(clashBody), entries)
	if err != nil {
		t.Fatalf("MergeClash: %v", err)
	}
	var doc struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(doc.Proxies) != 2 {
		t.Fatalf("want only convertible entry added, got %#v", doc.Proxies)
	}
}

func TestMergeClashSkipsEntryWithEmptyName(t *testing.T) {
	entries := []entry.Entry{{Name: "", URI: "vless://uuid@de.example.com:443?security=tls#DE"}}

	out, err := mixer.MergeClash([]byte(clashBody), entries)
	if err != nil {
		t.Fatalf("MergeClash: %v", err)
	}
	if string(out) != clashBody {
		t.Fatalf("body must be unchanged: an entry with no name must be skipped, not written as name: \"\": %s", out)
	}
}

func TestMergeClashUsesOverride(t *testing.T) {
	entries := []entry.Entry{{
		Name: "H2",
		URI:  "hysteria2://pw@h.example.com:443#H2",
		Overrides: map[string]json.RawMessage{
			entry.OverrideClash: json.RawMessage(`{"type":"hysteria2","server":"h.example.com","port":443,"password":"pw"}`),
		},
	}}

	out, err := mixer.MergeClash([]byte(clashBody), entries)
	if err != nil {
		t.Fatalf("MergeClash: %v", err)
	}
	var doc struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(doc.Proxies) != 2 || doc.Proxies[1]["type"] != "hysteria2" {
		t.Fatalf("proxies = %#v", doc.Proxies)
	}
}

func TestMergeClashNoProxiesKey(t *testing.T) {
	entries := []entry.Entry{{Name: "DE-1", URI: "vless://uuid@de.example.com:443?security=tls#DE"}}
	if _, err := mixer.MergeClash([]byte("rules:\n  - MATCH,DIRECT\n"), entries); err == nil {
		t.Fatal("want error when proxies key is missing")
	}
}

// clashBodyNamedDE1 is a body with a panel proxy named DE-1, used to check renaming
// on name collisions with mixed-in entries.
const clashBodyNamedDE1 = `proxies:
  - name: DE-1
    type: vless
    server: panel.example.com
    port: 443
proxy-groups:
  - name: Selector
    type: select
    proxies:
      - DE-1
`

func TestMergeClashRenamesCollidingNames(t *testing.T) {
	entries := []entry.Entry{
		{Name: "DE-1", URI: "vless://uuid1@a.example.com:443?security=tls#A"},
		{Name: "DE-1", URI: "vless://uuid2@b.example.com:443?security=tls#B"},
	}

	out, err := mixer.MergeClash([]byte(clashBodyNamedDE1), entries)
	if err != nil {
		t.Fatalf("MergeClash: %v", err)
	}

	var doc struct {
		Proxies     []map[string]any `yaml:"proxies"`
		ProxyGroups []struct {
			Name    string   `yaml:"name"`
			Proxies []string `yaml:"proxies"`
		} `yaml:"proxy-groups"`
	}
	if err := yaml.Unmarshal(out, &doc); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}

	if len(doc.Proxies) != 3 {
		t.Fatalf("proxies = %#v", doc.Proxies)
	}
	names := map[string]bool{}
	for _, p := range doc.Proxies {
		name, _ := p["name"].(string)
		if names[name] {
			t.Fatalf("duplicate proxy name in output: %q (mihomo rejects the whole config)", name)
		}
		names[name] = true
	}
	for _, want := range []string{"DE-1", "DE-1 (2)", "DE-1 (3)"} {
		if !names[want] {
			t.Fatalf("missing renamed proxy %q, got %#v", want, names)
		}
	}

	group := doc.ProxyGroups[0].Proxies
	for _, want := range []string{"DE-1", "DE-1 (2)", "DE-1 (3)"} {
		found := false
		for _, g := range group {
			if g == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("group must reference the final renamed name %q, got %#v", want, group)
		}
	}
}

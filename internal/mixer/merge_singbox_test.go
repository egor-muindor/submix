package mixer_test

import (
	"encoding/json"
	"testing"

	"github.com/egor-muindor/submix/internal/entry"
	"github.com/egor-muindor/submix/internal/mixer"
)

const singboxBody = `{
  "log": {"level": "info"},
  "outbounds": [
    {"type": "selector", "tag": "select", "outbounds": ["auto", "Panel-1"], "default": "auto"},
    {"type": "urltest", "tag": "auto", "outbounds": ["Panel-1"]},
    {"type": "vless", "tag": "Panel-1", "server": "panel.example.com", "server_port": 443, "uuid": "u"},
    {"type": "direct", "tag": "direct"}
  ]
}`

func TestMergeSingboxAddsOutboundAndGroups(t *testing.T) {
	entries := []entry.Entry{
		{Name: "DE-1", URI: "vless://uuid@de.example.com:443?type=tcp&security=tls&sni=de.example.com#Provider"},
	}

	out, err := mixer.MergeSingbox([]byte(singboxBody), entries)
	if err != nil {
		t.Fatalf("MergeSingbox: %v", err)
	}

	var doc struct {
		Log       map[string]any `json:"log"`
		Outbounds []struct {
			Type      string   `json:"type"`
			Tag       string   `json:"tag"`
			Outbounds []string `json:"outbounds"`
		} `json:"outbounds"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	if doc.Log["level"] != "info" {
		t.Fatalf("log block must be preserved: %#v", doc.Log)
	}
	if len(doc.Outbounds) != 5 {
		t.Fatalf("outbounds = %#v", doc.Outbounds)
	}
	if doc.Outbounds[4].Tag != "DE-1" || doc.Outbounds[4].Type != "vless" {
		t.Fatalf("appended = %#v", doc.Outbounds[4])
	}
	if last := doc.Outbounds[0].Outbounds; last[len(last)-1] != "DE-1" {
		t.Fatalf("selector = %#v", doc.Outbounds[0].Outbounds)
	}
	if last := doc.Outbounds[1].Outbounds; last[len(last)-1] != "DE-1" {
		t.Fatalf("urltest = %#v", doc.Outbounds[1].Outbounds)
	}
}

func TestMergeSingboxSkipsUnconvertible(t *testing.T) {
	entries := []entry.Entry{{Name: "H2", URI: "hysteria2://pw@h:443#H2"}}

	out, err := mixer.MergeSingbox([]byte(singboxBody), entries)
	if err != nil {
		t.Fatalf("MergeSingbox: %v", err)
	}
	if string(out) != singboxBody {
		t.Fatalf("body must be returned unchanged when nothing was added")
	}
}

func TestMergeSingboxNoOutbounds(t *testing.T) {
	entries := []entry.Entry{{Name: "DE-1", URI: "vless://uuid@h:443?security=tls#DE"}}
	if _, err := mixer.MergeSingbox([]byte(`{"log":{}}`), entries); err == nil {
		t.Fatal("want error when outbounds are missing")
	}
}

// singboxBodyNamedDE1 is a body with a panel outbound tagged DE-1, used to check
// renaming on tag collisions with mixed-in entries.
const singboxBodyNamedDE1 = `{
  "outbounds": [
    {"type": "selector", "tag": "select", "outbounds": ["DE-1"]},
    {"type": "vless", "tag": "DE-1", "server": "panel.example.com", "server_port": 443, "uuid": "u"}
  ]
}`

func TestMergeSingboxRenamesCollidingTags(t *testing.T) {
	entries := []entry.Entry{
		{Name: "DE-1", URI: "vless://uuid1@a.example.com:443?security=tls#A"},
		{Name: "DE-1", URI: "vless://uuid2@b.example.com:443?security=tls#B"},
	}

	out, err := mixer.MergeSingbox([]byte(singboxBodyNamedDE1), entries)
	if err != nil {
		t.Fatalf("MergeSingbox: %v", err)
	}

	var doc struct {
		Outbounds []struct {
			Type      string   `json:"type"`
			Tag       string   `json:"tag"`
			Outbounds []string `json:"outbounds"`
		} `json:"outbounds"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}

	if len(doc.Outbounds) != 4 {
		t.Fatalf("outbounds = %#v", doc.Outbounds)
	}
	tags := map[string]bool{}
	for _, ob := range doc.Outbounds {
		if ob.Tag == "" {
			continue
		}
		if tags[ob.Tag] {
			t.Fatalf("duplicate outbound tag in output: %q (sing-box rejects the whole config)", ob.Tag)
		}
		tags[ob.Tag] = true
	}
	for _, want := range []string{"DE-1", "DE-1 (2)", "DE-1 (3)"} {
		if !tags[want] {
			t.Fatalf("missing renamed outbound %q, got %#v", want, tags)
		}
	}

	group := doc.Outbounds[0].Outbounds
	for _, want := range []string{"DE-1", "DE-1 (2)", "DE-1 (3)"} {
		found := false
		for _, g := range group {
			if g == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("selector must reference the final renamed tag %q, got %#v", want, group)
		}
	}
}

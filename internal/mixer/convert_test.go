package mixer_test

import (
	"encoding/json"
	"testing"

	"github.com/egor-muindor/submix/internal/entry"
	"github.com/egor-muindor/submix/internal/mixer"
)

func TestClashProxyFromURI(t *testing.T) {
	e := entry.Entry{Name: "DE-1", URI: "vless://uuid@de.example.com:443?type=tcp&security=tls&sni=de.example.com#ProviderName"}

	p, err := mixer.ClashProxy(e)
	if err != nil {
		t.Fatalf("ClashProxy: %v", err)
	}
	if p["name"] != "DE-1" {
		t.Fatalf("name = %v, want entry name to win over URI fragment", p["name"])
	}
	if p["type"] != "vless" {
		t.Fatalf("proxy = %#v", p)
	}
}

func TestClashProxyFromOverride(t *testing.T) {
	e := entry.Entry{
		Name: "H2",
		URI:  "hysteria2://pw@h.example.com:443#H2",
		Overrides: map[string]json.RawMessage{
			entry.OverrideClash: json.RawMessage(`{"type":"hysteria2","server":"h.example.com","port":443,"password":"pw"}`),
		},
	}

	p, err := mixer.ClashProxy(e)
	if err != nil {
		t.Fatalf("ClashProxy: %v", err)
	}
	if p["type"] != "hysteria2" || p["name"] != "H2" {
		t.Fatalf("proxy = %#v", p)
	}
}

func TestConvertersRejectUnsupportedWithoutOverride(t *testing.T) {
	e := entry.Entry{Name: "H2", URI: "hysteria2://pw@h.example.com:443#H2"}

	if _, err := mixer.ClashProxy(e); err == nil {
		t.Fatal("ClashProxy: want error")
	}
	if _, err := mixer.SingboxOutbound(e); err == nil {
		t.Fatal("SingboxOutbound: want error")
	}
	if _, err := mixer.XrayOutbound(e); err == nil {
		t.Fatal("XrayOutbound: want error")
	}
}

func TestSingboxAndXrayUseEntryName(t *testing.T) {
	e := entry.Entry{Name: "DE-1", URI: "vless://uuid@de.example.com:443?type=tcp&security=tls#Other"}

	ob, err := mixer.SingboxOutbound(e)
	if err != nil {
		t.Fatalf("SingboxOutbound: %v", err)
	}
	if ob["tag"] != "DE-1" {
		t.Fatalf("tag = %v", ob["tag"])
	}
	if _, err := mixer.XrayOutbound(e); err != nil {
		t.Fatalf("XrayOutbound: %v", err)
	}
}

func TestConvertersRejectEmptyName(t *testing.T) {
	e := entry.Entry{Name: "", URI: "vless://uuid@de.example.com:443?type=tcp&security=tls#Fragment"}

	if _, err := mixer.ClashProxy(e); err == nil {
		t.Fatal("ClashProxy: want error for empty name")
	}
	if _, err := mixer.SingboxOutbound(e); err == nil {
		t.Fatal("SingboxOutbound: want error for empty name")
	}
	if _, err := mixer.XrayOutbound(e); err == nil {
		t.Fatal("XrayOutbound: want error for empty name")
	}
}

func TestConvertersRejectEmptyNameEvenWithOverride(t *testing.T) {
	e := entry.Entry{
		Name: "",
		URI:  "hysteria2://pw@h.example.com:443#H2",
		Overrides: map[string]json.RawMessage{
			entry.OverrideClash: json.RawMessage(`{"type":"hysteria2","server":"h.example.com","port":443,"password":"pw"}`),
		},
	}
	if _, err := mixer.ClashProxy(e); err == nil {
		t.Fatal("ClashProxy: want error for empty name even with an override")
	}
}

func TestURIWithEntryName(t *testing.T) {
	e := entry.Entry{Name: "DE-1", URI: "vless://uuid@h:443#Other"}
	if got := mixer.URIWithEntryName(e); got != "vless://uuid@h:443#DE-1" {
		t.Fatalf("got %q", got)
	}
}

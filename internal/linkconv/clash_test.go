package linkconv_test

import (
	"testing"

	"github.com/egor-muindor/submix/internal/linkconv"
	"github.com/egor-muindor/submix/internal/linkparse"
)

func mustParse(t *testing.T, uri string) linkparse.Node {
	t.Helper()
	n, err := linkparse.Parse(uri)
	if err != nil {
		t.Fatalf("parse %q: %v", uri, err)
	}
	return n
}

func TestToClashVlessReality(t *testing.T) {
	n := mustParse(t, "vless://uuid-1@de.example.com:443?type=tcp&security=reality"+
		"&pbk=PUBKEY&sid=ab12&sni=www.microsoft.com&fp=chrome&flow=xtls-rprx-vision#DE")

	p, err := linkconv.ToClash(n)
	if err != nil {
		t.Fatalf("ToClash: %v", err)
	}
	if p["type"] != "vless" || p["server"] != "de.example.com" || p["port"] != 443 {
		t.Fatalf("proxy = %#v", p)
	}
	if p["uuid"] != "uuid-1" || p["flow"] != "xtls-rprx-vision" || p["tls"] != true {
		t.Fatalf("proxy = %#v", p)
	}
	if p["servername"] != "www.microsoft.com" || p["client-fingerprint"] != "chrome" {
		t.Fatalf("proxy = %#v", p)
	}
	ro, ok := p["reality-opts"].(map[string]any)
	if !ok || ro["public-key"] != "PUBKEY" || ro["short-id"] != "ab12" {
		t.Fatalf("reality-opts = %#v", p["reality-opts"])
	}
}

func TestToClashVlessRealityDefaultsWithoutFpOrSni(t *testing.T) {
	n := mustParse(t, "vless://uuid-1@de.example.com:443?type=tcp&security=reality&pbk=PUBKEY&sid=ab12#DE")

	p, err := linkconv.ToClash(n)
	if err != nil {
		t.Fatalf("ToClash: %v", err)
	}
	if p["client-fingerprint"] != "chrome" {
		t.Fatalf("client-fingerprint = %v, want default chrome for reality", p["client-fingerprint"])
	}
	if _, ok := p["servername"]; ok {
		t.Fatalf("servername must be omitted when sni is empty, got %#v", p["servername"])
	}
}

func TestToClashVlessWS(t *testing.T) {
	n := mustParse(t, "vless://uuid-1@h.example.com:8443?type=ws&security=tls&path=%2Fws&host=cdn.example.com#WS")

	p, err := linkconv.ToClash(n)
	if err != nil {
		t.Fatalf("ToClash: %v", err)
	}
	if p["network"] != "ws" {
		t.Fatalf("network = %v", p["network"])
	}
	opts, ok := p["ws-opts"].(map[string]any)
	if !ok || opts["path"] != "/ws" {
		t.Fatalf("ws-opts = %#v", p["ws-opts"])
	}
	headers, ok := opts["headers"].(map[string]any)
	if !ok || headers["Host"] != "cdn.example.com" {
		t.Fatalf("headers = %#v", opts["headers"])
	}
}

func TestToClashShadowsocksAndTrojan(t *testing.T) {
	ss, err := linkconv.ToClash(mustParse(t, "ss://aes-256-gcm:pass@1.2.3.4:8388#SS"))
	if err != nil {
		t.Fatalf("ToClash ss: %v", err)
	}
	if ss["type"] != "ss" || ss["cipher"] != "aes-256-gcm" || ss["password"] != "pass" {
		t.Fatalf("ss = %#v", ss)
	}

	tj, err := linkconv.ToClash(mustParse(t, "trojan://pw@tj.example.com:443?sni=tj.example.com#TJ"))
	if err != nil {
		t.Fatalf("ToClash trojan: %v", err)
	}
	if tj["type"] != "trojan" || tj["password"] != "pw" || tj["sni"] != "tj.example.com" {
		t.Fatalf("trojan = %#v", tj)
	}
}

func TestToClashUnsupportedNetwork(t *testing.T) {
	n := mustParse(t, "vless://uuid-1@h:443?type=kcp#KCP")
	if _, err := linkconv.ToClash(n); err == nil {
		t.Fatal("want error for kcp")
	}
}

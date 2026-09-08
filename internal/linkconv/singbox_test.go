package linkconv_test

import (
	"testing"

	"github.com/egor-muindor/submix/internal/linkconv"
)

func TestToSingboxVlessReality(t *testing.T) {
	n := mustParse(t, "vless://uuid-1@de.example.com:443?type=tcp&security=reality"+
		"&pbk=PUBKEY&sid=ab12&sni=www.microsoft.com&fp=chrome&flow=xtls-rprx-vision#DE")

	ob, err := linkconv.ToSingbox(n)
	if err != nil {
		t.Fatalf("ToSingbox: %v", err)
	}
	if ob["type"] != "vless" || ob["tag"] != "DE" || ob["server"] != "de.example.com" {
		t.Fatalf("outbound = %#v", ob)
	}
	if ob["server_port"] != 443 || ob["uuid"] != "uuid-1" || ob["flow"] != "xtls-rprx-vision" {
		t.Fatalf("outbound = %#v", ob)
	}
	tls, ok := ob["tls"].(map[string]any)
	if !ok || tls["enabled"] != true || tls["server_name"] != "www.microsoft.com" {
		t.Fatalf("tls = %#v", ob["tls"])
	}
	reality, ok := tls["reality"].(map[string]any)
	if !ok || reality["public_key"] != "PUBKEY" || reality["short_id"] != "ab12" {
		t.Fatalf("reality = %#v", tls["reality"])
	}
	utls, ok := tls["utls"].(map[string]any)
	if !ok || utls["fingerprint"] != "chrome" {
		t.Fatalf("utls = %#v", tls["utls"])
	}
}

func TestToSingboxVlessRealityRequiresUtlsWithoutFp(t *testing.T) {
	// sing-box refuses to start reality without uTLS enabled ("uTLS is
	// required by reality client"); the fp URI param is optional, so a
	// default fingerprint must be set whenever fp is absent.
	n := mustParse(t, "vless://uuid-1@de.example.com:443?type=tcp&security=reality"+
		"&pbk=PUBKEY&sid=ab12&sni=www.microsoft.com#DE")

	ob, err := linkconv.ToSingbox(n)
	if err != nil {
		t.Fatalf("ToSingbox: %v", err)
	}
	tls, ok := ob["tls"].(map[string]any)
	if !ok {
		t.Fatalf("tls = %#v", ob["tls"])
	}
	utls, ok := tls["utls"].(map[string]any)
	if !ok || utls["enabled"] != true {
		t.Fatalf("utls must be enabled for reality even without fp, got %#v", tls["utls"])
	}
	if utls["fingerprint"] != "chrome" {
		t.Fatalf("fingerprint = %v, want default chrome", utls["fingerprint"])
	}
}

func TestToSingboxVlessWS(t *testing.T) {
	n := mustParse(t, "vless://uuid-1@h.example.com:8443?type=ws&security=tls&path=%2Fws&host=cdn.example.com#WS")

	ob, err := linkconv.ToSingbox(n)
	if err != nil {
		t.Fatalf("ToSingbox: %v", err)
	}
	tr, ok := ob["transport"].(map[string]any)
	if !ok || tr["type"] != "ws" || tr["path"] != "/ws" {
		t.Fatalf("transport = %#v", ob["transport"])
	}
	headers, ok := tr["headers"].(map[string]any)
	if !ok || headers["Host"] != "cdn.example.com" {
		t.Fatalf("headers = %#v", tr["headers"])
	}
}

func TestToSingboxShadowsocksAndTrojan(t *testing.T) {
	ss, err := linkconv.ToSingbox(mustParse(t, "ss://aes-256-gcm:pass@1.2.3.4:8388#SS"))
	if err != nil {
		t.Fatalf("ToSingbox ss: %v", err)
	}
	if ss["type"] != "shadowsocks" || ss["method"] != "aes-256-gcm" || ss["password"] != "pass" {
		t.Fatalf("ss = %#v", ss)
	}

	tj, err := linkconv.ToSingbox(mustParse(t, "trojan://pw@tj.example.com:443?sni=tj.example.com#TJ"))
	if err != nil {
		t.Fatalf("ToSingbox trojan: %v", err)
	}
	if tj["type"] != "trojan" || tj["password"] != "pw" {
		t.Fatalf("trojan = %#v", tj)
	}
	tls, ok := tj["tls"].(map[string]any)
	if !ok || tls["server_name"] != "tj.example.com" {
		t.Fatalf("tls = %#v", tj["tls"])
	}
}

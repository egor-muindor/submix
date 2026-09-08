package linkconv_test

import (
	"testing"

	"github.com/egor-muindor/submix/internal/linkconv"
)

func TestToXrayVlessReality(t *testing.T) {
	n := mustParse(t, "vless://uuid-1@de.example.com:443?type=tcp&security=reality"+
		"&pbk=PUBKEY&sid=ab12&sni=www.microsoft.com&fp=chrome&flow=xtls-rprx-vision#DE")

	ob, err := linkconv.ToXray(n)
	if err != nil {
		t.Fatalf("ToXray: %v", err)
	}
	if ob["protocol"] != "vless" {
		t.Fatalf("outbound = %#v", ob)
	}
	settings := ob["settings"].(map[string]any)
	vnext := settings["vnext"].([]any)
	server := vnext[0].(map[string]any)
	if server["address"] != "de.example.com" || server["port"] != 443 {
		t.Fatalf("vnext = %#v", server)
	}
	user := server["users"].([]any)[0].(map[string]any)
	if user["id"] != "uuid-1" || user["flow"] != "xtls-rprx-vision" || user["encryption"] != "none" {
		t.Fatalf("user = %#v", user)
	}
	stream := ob["streamSettings"].(map[string]any)
	if stream["network"] != "tcp" || stream["security"] != "reality" {
		t.Fatalf("stream = %#v", stream)
	}
	reality := stream["realitySettings"].(map[string]any)
	if reality["publicKey"] != "PUBKEY" || reality["shortId"] != "ab12" ||
		reality["serverName"] != "www.microsoft.com" || reality["fingerprint"] != "chrome" {
		t.Fatalf("reality = %#v", reality)
	}
}

func TestToXrayVlessRealityDefaultFingerprint(t *testing.T) {
	n := mustParse(t, "vless://uuid-1@de.example.com:443?type=tcp&security=reality&pbk=PUBKEY&sid=ab12&sni=x.example.com#DE")

	ob, err := linkconv.ToXray(n)
	if err != nil {
		t.Fatalf("ToXray: %v", err)
	}
	stream := ob["streamSettings"].(map[string]any)
	reality := stream["realitySettings"].(map[string]any)
	if reality["fingerprint"] != "chrome" {
		t.Fatalf("fingerprint = %v, want default chrome when fp is absent", reality["fingerprint"])
	}
}

func TestToXrayVlessWS(t *testing.T) {
	n := mustParse(t, "vless://uuid-1@h.example.com:8443?type=ws&security=tls&path=%2Fws&host=cdn.example.com#WS")

	ob, err := linkconv.ToXray(n)
	if err != nil {
		t.Fatalf("ToXray: %v", err)
	}
	stream := ob["streamSettings"].(map[string]any)
	if stream["network"] != "ws" || stream["security"] != "tls" {
		t.Fatalf("stream = %#v", stream)
	}
	ws := stream["wsSettings"].(map[string]any)
	if ws["path"] != "/ws" {
		t.Fatalf("wsSettings = %#v", ws)
	}
	headers := ws["headers"].(map[string]any)
	if headers["Host"] != "cdn.example.com" {
		t.Fatalf("headers = %#v", headers)
	}
}

func TestToXrayShadowsocksAndTrojan(t *testing.T) {
	ss, err := linkconv.ToXray(mustParse(t, "ss://aes-256-gcm:pass@1.2.3.4:8388#SS"))
	if err != nil {
		t.Fatalf("ToXray ss: %v", err)
	}
	if ss["protocol"] != "shadowsocks" {
		t.Fatalf("ss = %#v", ss)
	}
	srv := ss["settings"].(map[string]any)["servers"].([]any)[0].(map[string]any)
	if srv["method"] != "aes-256-gcm" || srv["password"] != "pass" || srv["address"] != "1.2.3.4" {
		t.Fatalf("server = %#v", srv)
	}

	tj, err := linkconv.ToXray(mustParse(t, "trojan://pw@tj.example.com:443?sni=tj.example.com#TJ"))
	if err != nil {
		t.Fatalf("ToXray trojan: %v", err)
	}
	if tj["protocol"] != "trojan" {
		t.Fatalf("trojan = %#v", tj)
	}
	tjSrv := tj["settings"].(map[string]any)["servers"].([]any)[0].(map[string]any)
	if tjSrv["password"] != "pw" || tjSrv["address"] != "tj.example.com" {
		t.Fatalf("server = %#v", tjSrv)
	}
}

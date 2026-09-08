package linkparse_test

import (
	"testing"

	"github.com/egor-muindor/submix/internal/linkparse"
)

func TestParseVlessReality(t *testing.T) {
	uri := "vless://11111111-2222-3333-4444-555555555555@de.example.com:443" +
		"?type=tcp&security=reality&pbk=PUBKEY&sid=ab12&sni=www.microsoft.com" +
		"&fp=chrome&flow=xtls-rprx-vision#%F0%9F%87%A9%F0%9F%87%AA%20Germany-1"

	n, err := linkparse.Parse(uri)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Scheme != "vless" {
		t.Fatalf("scheme = %q", n.Scheme)
	}
	if n.Host != "de.example.com" || n.Port != 443 {
		t.Fatalf("host/port = %s:%d", n.Host, n.Port)
	}
	if n.Credential != "11111111-2222-3333-4444-555555555555" {
		t.Fatalf("credential = %q", n.Credential)
	}
	if n.Name != "🇩🇪 Germany-1" {
		t.Fatalf("name = %q", n.Name)
	}
	if n.Params.Get("pbk") != "PUBKEY" || n.Params.Get("security") != "reality" {
		t.Fatalf("params = %v", n.Params)
	}
}

func TestParseVlessWebsocket(t *testing.T) {
	uri := "vless://uuid-1@h.example.com:8443?type=ws&security=tls&path=%2Fws&host=cdn.example.com#WS"

	n, err := linkparse.Parse(uri)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Network() != "ws" {
		t.Fatalf("network = %q", n.Network())
	}
	if n.Params.Get("path") != "/ws" {
		t.Fatalf("path = %q", n.Params.Get("path"))
	}
}

func TestParseErrors(t *testing.T) {
	for _, uri := range []string{
		"hysteria2://x@h:443#H2",
		"vless://uuid@h:notaport#X",
		"vless://h.example.com:443#NoCreds",
		"not a uri at all",
	} {
		if _, err := linkparse.Parse(uri); err == nil {
			t.Fatalf("want error for %q", uri)
		}
	}
}

func TestNameFallsBackToHost(t *testing.T) {
	if got := linkparse.Name("vless://uuid@h.example.com:443"); got != "h.example.com:443" {
		t.Fatalf("name = %q", got)
	}
	if got := linkparse.Name("vless://uuid@h.example.com:443#Tokyo"); got != "Tokyo" {
		t.Fatalf("name = %q", got)
	}
}

func TestWithFragment(t *testing.T) {
	got := linkparse.WithFragment("vless://uuid@h:443#Old", "New Name")
	if got != "vless://uuid@h:443#New%20Name" {
		t.Fatalf("got %q", got)
	}
}

func TestDecodeBase64(t *testing.T) {
	// std alphabet with padding and url alphabet without padding: both must decode.
	for _, s := range []string{"dmxlc3M6Ly94", "dmxlc3M6Ly94=="} {
		b, err := linkparse.DecodeBase64(s)
		if err != nil {
			t.Fatalf("decode %q: %v", s, err)
		}
		if string(b) != "vless://x" {
			t.Fatalf("decoded %q", b)
		}
	}
	if _, err := linkparse.DecodeBase64("!!!!"); err == nil {
		t.Fatal("want error on non-base64")
	}
}

func TestParseShadowsocksUserinfoBase64(t *testing.T) {
	// ss://base64(aes-256-gcm:secretpass)@ss.example.com:8388#SS-1
	uri := "ss://YWVzLTI1Ni1nY206c2VjcmV0cGFzcw@ss.example.com:8388#SS-1"

	n, err := linkparse.Parse(uri)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Scheme != "ss" || n.Method != "aes-256-gcm" || n.Password != "secretpass" {
		t.Fatalf("node = %+v", n)
	}
	if n.Host != "ss.example.com" || n.Port != 8388 || n.Name != "SS-1" {
		t.Fatalf("node = %+v", n)
	}
}

func TestParseShadowsocksFullBase64(t *testing.T) {
	// ss://base64(aes-256-gcm:secretpass@ss.example.com:8388)#SS-2
	uri := "ss://YWVzLTI1Ni1nY206c2VjcmV0cGFzc0Bzcy5leGFtcGxlLmNvbTo4Mzg4#SS-2"

	n, err := linkparse.Parse(uri)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Method != "aes-256-gcm" || n.Password != "secretpass" {
		t.Fatalf("node = %+v", n)
	}
	if n.Host != "ss.example.com" || n.Port != 8388 || n.Name != "SS-2" {
		t.Fatalf("node = %+v", n)
	}
}

func TestParseShadowsocksPlainUserinfo(t *testing.T) {
	n, err := linkparse.Parse("ss://aes-128-gcm:pass123@1.2.3.4:9000#SS-3")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Method != "aes-128-gcm" || n.Password != "pass123" || n.Host != "1.2.3.4" {
		t.Fatalf("node = %+v", n)
	}
}

func TestParseTrojan(t *testing.T) {
	n, err := linkparse.Parse("trojan://pass@tj.example.com:443?sni=tj.example.com&type=tcp#TJ")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n.Scheme != "trojan" || n.Credential != "pass" || n.Port != 443 {
		t.Fatalf("node = %+v", n)
	}
	if n.Params.Get("sni") != "tj.example.com" {
		t.Fatalf("params = %v", n.Params)
	}
}

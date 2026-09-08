package extras_test

import (
	"encoding/base64"
	"testing"

	"github.com/egor-muindor/submix/internal/extras"
)

func TestParseSubscriptionBodyBase64(t *testing.T) {
	links := "vless://uuid@de.example.com:443?security=tls#%F0%9F%87%A9%F0%9F%87%AA%20Germany-1\n" +
		"ss://YWVzLTI1Ni1nY206cHc@1.2.3.4:8388#SS-1\n"
	body := []byte(base64.StdEncoding.EncodeToString([]byte(links)))

	got, err := extras.ParseSubscriptionBody(body)
	if err != nil {
		t.Fatalf("ParseSubscriptionBody: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("entries = %#v", got)
	}
	if got[0].Name != "🇩🇪 Germany-1" {
		t.Fatalf("name = %q", got[0].Name)
	}
	if got[0].URI != "vless://uuid@de.example.com:443?security=tls#%F0%9F%87%A9%F0%9F%87%AA%20Germany-1" {
		t.Fatalf("uri = %q", got[0].URI)
	}
	if got[1].Name != "SS-1" {
		t.Fatalf("name = %q", got[1].Name)
	}
}

func TestParseSubscriptionBodyPlaintext(t *testing.T) {
	body := []byte("# comment\n\nvless://uuid@h.example.com:443#A\nnot-a-link\ntrojan://pw@t.example.com:443#B\n")

	got, err := extras.ParseSubscriptionBody(body)
	if err != nil {
		t.Fatalf("ParseSubscriptionBody: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("entries = %#v", got)
	}
	if got[0].Name != "A" || got[1].Name != "B" {
		t.Fatalf("entries = %#v", got)
	}
}

func TestParseSubscriptionBodyNameFallback(t *testing.T) {
	body := []byte("vless://uuid@h.example.com:443\n")

	got, err := extras.ParseSubscriptionBody(body)
	if err != nil {
		t.Fatalf("ParseSubscriptionBody: %v", err)
	}
	if got[0].Name != "h.example.com:443" {
		t.Fatalf("name = %q", got[0].Name)
	}
}

func TestParseSubscriptionBodyErrors(t *testing.T) {
	for _, body := range []string{"", "   ", "no links here at all", "<html>error page</html>"} {
		if _, err := extras.ParseSubscriptionBody([]byte(body)); err == nil {
			t.Fatalf("want error for %q", body)
		}
	}
}

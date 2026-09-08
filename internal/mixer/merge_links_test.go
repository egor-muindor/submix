package mixer_test

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/egor-muindor/submix/internal/entry"
	"github.com/egor-muindor/submix/internal/linkparse"
	"github.com/egor-muindor/submix/internal/mixer"
)

func TestMergeBase64(t *testing.T) {
	original := "vless://uuid@panel.example.com:443#Panel-1\n"
	body := []byte(base64.StdEncoding.EncodeToString([]byte(original)))
	entries := []entry.Entry{{Name: "DE-1", URI: "vless://uuid2@de.example.com:443#Provider"}}

	out, err := mixer.MergeBase64(body, entries)
	if err != nil {
		t.Fatalf("MergeBase64: %v", err)
	}

	decoded, err := linkparse.DecodeBase64(string(out))
	if err != nil {
		t.Fatalf("decode result: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(decoded)), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %#v", lines)
	}
	if lines[0] != "vless://uuid@panel.example.com:443#Panel-1" {
		t.Fatalf("original line changed: %q", lines[0])
	}
	if lines[1] != "vless://uuid2@de.example.com:443#DE-1" {
		t.Fatalf("appended line = %q", lines[1])
	}
}

func TestMergeLinksPlaintext(t *testing.T) {
	body := []byte("vless://uuid@panel.example.com:443#Panel-1\n")
	entries := []entry.Entry{{Name: "SS", URI: "ss://aes-256-gcm:pw@1.2.3.4:8388#Provider"}}

	out, err := mixer.MergeLinks(body, entries)
	if err != nil {
		t.Fatalf("MergeLinks: %v", err)
	}
	got := string(out)
	if !strings.Contains(got, "#Panel-1") || !strings.Contains(got, "#SS") {
		t.Fatalf("out = %q", got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Fatalf("out must end with newline: %q", got)
	}
}

func TestMergeLinksOutlineBodyOnlyAddsSSEntries(t *testing.T) {
	// A body that is only ss:// lines is Outline's plain-list output; Outline
	// clients don't understand vless/trojan URIs, so those must be skipped.
	body := []byte("ss://YWVzLTI1Ni1nY206cHc@1.2.3.4:8388#A\nss://YWVzLTI1Ni1nY206cHc@5.6.7.8:8388#B\n")
	entries := []entry.Entry{
		{Name: "SS-1", URI: "ss://YWVzLTI1Ni1nY206cHc@9.9.9.9:8388#SS"},
		{Name: "VLESS-1", URI: "vless://uuid@h:443?security=tls#V"},
	}

	out, err := mixer.MergeLinks(body, entries)
	if err != nil {
		t.Fatalf("MergeLinks: %v", err)
	}
	got := string(out)
	if !strings.Contains(got, "#SS-1") {
		t.Fatalf("ss:// entry must be added: %q", got)
	}
	if strings.Contains(got, "#V") || strings.Contains(got, "vless://") {
		t.Fatalf("non-ss entry must not be added to an Outline (ss-only) body: %q", got)
	}
}

func TestMergeBase64EmptyEntries(t *testing.T) {
	body := []byte(base64.StdEncoding.EncodeToString([]byte("vless://uuid@h:443#A\n")))
	out, err := mixer.MergeBase64(body, nil)
	if err != nil {
		t.Fatalf("MergeBase64: %v", err)
	}
	if string(out) != string(body) {
		t.Fatalf("body must be unchanged: %q", out)
	}
}

func TestMergeBase64BadInput(t *testing.T) {
	if _, err := mixer.MergeBase64([]byte("!!!!"), []entry.Entry{{Name: "A", URI: "vless://u@h:443"}}); err == nil {
		t.Fatal("want error on non-base64 body")
	}
}

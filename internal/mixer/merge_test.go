package mixer_test

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/egor-muindor/submix/internal/entry"
	"github.com/egor-muindor/submix/internal/mixer"
)

func TestMergeDispatch(t *testing.T) {
	entries := []entry.Entry{{Name: "DE-1", URI: "vless://uuid@de.example.com:443?type=tcp&security=tls&sni=de.example.com#P"}}

	t.Run("base64", func(t *testing.T) {
		body := []byte(base64.StdEncoding.EncodeToString([]byte("vless://u@h:443#A\n")))
		out, format, err := mixer.Merge(body, entries)
		if err != nil {
			t.Fatalf("Merge: %v", err)
		}
		if format != entry.FormatBase64 {
			t.Fatalf("format = %q", format)
		}
		decoded, _ := base64.StdEncoding.DecodeString(string(out))
		if !strings.Contains(string(decoded), "#DE-1") {
			t.Fatalf("decoded = %q", decoded)
		}
	})

	t.Run("clash", func(t *testing.T) {
		out, format, err := mixer.Merge([]byte(clashBody), entries)
		if err != nil {
			t.Fatalf("Merge: %v", err)
		}
		if format != entry.FormatClash {
			t.Fatalf("format = %q", format)
		}
		if !strings.Contains(string(out), "DE-1") {
			t.Fatalf("out = %s", out)
		}
	})

	t.Run("singbox", func(t *testing.T) {
		_, format, err := mixer.Merge([]byte(singboxBody), entries)
		if err != nil {
			t.Fatalf("Merge: %v", err)
		}
		if format != entry.FormatSingbox {
			t.Fatalf("format = %q", format)
		}
	})

	t.Run("xray-json", func(t *testing.T) {
		_, format, err := mixer.Merge([]byte(xrayBody), entries)
		if err != nil {
			t.Fatalf("Merge: %v", err)
		}
		if format != entry.FormatXrayJSON {
			t.Fatalf("format = %q", format)
		}
	})

	t.Run("unknown is untouched", func(t *testing.T) {
		body := []byte(`{"servers":[{"id":"1"}]}`)
		out, format, err := mixer.Merge(body, entries)
		if err != nil {
			t.Fatalf("Merge: %v", err)
		}
		if format != entry.FormatUnknown {
			t.Fatalf("format = %q", format)
		}
		if string(out) != string(body) {
			t.Fatalf("unknown format must be returned unchanged: %s", out)
		}
	})
}

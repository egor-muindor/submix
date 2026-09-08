package mixer_test

import (
	"encoding/base64"
	"testing"

	"github.com/egor-muindor/submix/internal/entry"
	"github.com/egor-muindor/submix/internal/mixer"
)

func TestDetect(t *testing.T) {
	links := "vless://uuid@h:443#A\nss://YWVzOnB3@1.2.3.4:8388#B\n"

	cases := []struct {
		name string
		body string
		want entry.Format
	}{
		{"base64", base64.StdEncoding.EncodeToString([]byte(links)), entry.FormatBase64},
		{"links", links, entry.FormatLinks},
		{"xray-json", `[{"remarks":"A","outbounds":[{"tag":"proxy"}]}]`, entry.FormatXrayJSON},
		{"singbox", `{"outbounds":[{"type":"vless","tag":"A"}]}`, entry.FormatSingbox},
		{"clash", "proxies:\n  - name: A\n    type: ss\nproxy-groups: []\n", entry.FormatClash},
		{"empty", "", entry.FormatUnknown},
		{"outline-json-object", `{"servers":[{"id":"1"}]}`, entry.FormatUnknown},
		{"garbage", "just some text without links", entry.FormatUnknown},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := mixer.Detect([]byte(tc.body)); got != tc.want {
				t.Fatalf("Detect(%q) = %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}

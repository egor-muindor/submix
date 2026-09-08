package extrasui_test

import (
	"strings"
	"testing"

	"github.com/egor-muindor/submix/internal/entry"
	"github.com/egor-muindor/submix/internal/extrasui"
)

func TestConversionErrorsNilForConvertibleLink(t *testing.T) {
	e := entry.Entry{
		Name: "DE-1",
		URI:  "vless://11111111-2222-3333-4444-555555555555@de.example.com:443?type=tcp&security=tls&sni=de.example.com#DE-1",
	}
	if errs := extrasui.ConversionErrors(e); errs != nil {
		t.Fatalf("valid vless must convert to every format, got %v", errs)
	}
}

func TestConversionErrorsReportsEveryFormat(t *testing.T) {
	e := entry.Entry{Name: "H2", URI: "hysteria2://pass@h.example.com:443#H2"}
	errs := extrasui.ConversionErrors(e)
	for _, format := range []string{"clash", "singbox", "xray"} {
		if !strings.Contains(errs[format], "unsupported scheme") {
			t.Fatalf("errs[%s] = %q, want unsupported scheme", format, errs[format])
		}
	}
}

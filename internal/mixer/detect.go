// Package mixer detects the format of a subscription response and mixes
// additional connection links into it.
package mixer

import (
	"bytes"
	"encoding/json"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/egor-muindor/submix/internal/entry"
	"github.com/egor-muindor/submix/internal/linkparse"
)

// Detect determines the panel response format from the body.
func Detect(body []byte) entry.Format {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return entry.FormatUnknown
	}

	switch trimmed[0] {
	case '[':
		var arr []json.RawMessage
		if json.Unmarshal(trimmed, &arr) == nil {
			return entry.FormatXrayJSON
		}
	case '{':
		var obj map[string]json.RawMessage
		if json.Unmarshal(trimmed, &obj) == nil {
			if _, ok := obj["outbounds"]; ok {
				return entry.FormatSingbox
			}
			return entry.FormatUnknown
		}
	}

	// Clash/Mihomo/Stash: a YAML document with a proxies key.
	var doc map[string]any
	if yaml.Unmarshal(trimmed, &doc) == nil {
		if _, ok := doc["proxies"]; ok {
			return entry.FormatClash
		}
	}

	// base64-encoded list of links.
	if decoded, err := linkparse.DecodeBase64(string(trimmed)); err == nil {
		if strings.Contains(string(decoded), "://") {
			return entry.FormatBase64
		}
	}

	// Plain list of links.
	if strings.Contains(string(trimmed), "://") {
		return entry.FormatLinks
	}

	return entry.FormatUnknown
}

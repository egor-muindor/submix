package mixer

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/egor-muindor/submix/internal/entry"
	"github.com/egor-muindor/submix/internal/linkparse"
)

// MergeBase64 appends the entries to a base64-encoded list of connection links.
func MergeBase64(body []byte, entries []entry.Entry) ([]byte, error) {
	if len(entries) == 0 {
		return body, nil
	}
	decoded, err := linkparse.DecodeBase64(string(body))
	if err != nil {
		return nil, fmt.Errorf("base64 body: %w", err)
	}
	merged, err := appendLinks(decoded, entries)
	if err != nil {
		return nil, err
	}
	return []byte(base64.StdEncoding.EncodeToString(merged)), nil
}

// MergeLinks appends the entries to a plain list of connection links.
//
// If the body consists entirely of ss:// links, it is an Outline response: the
// Outline client does not understand vless/trojan links, so only entries with
// the ss:// scheme are mixed in and the rest are skipped.
func MergeLinks(body []byte, entries []entry.Entry) ([]byte, error) {
	if len(entries) == 0 {
		return body, nil
	}
	if isOutlineBody(body) {
		entries = filterByScheme(entries, "ss://")
		if len(entries) == 0 {
			return body, nil
		}
	}
	return appendLinks(body, entries)
}

func appendLinks(body []byte, entries []entry.Entry) ([]byte, error) {
	var b strings.Builder
	text := strings.TrimRight(string(body), "\r\n")
	if text != "" {
		b.WriteString(text)
		b.WriteString("\n")
	}
	for _, e := range entries {
		if e.URI == "" {
			continue
		}
		b.WriteString(URIWithEntryName(e))
		b.WriteString("\n")
	}
	return []byte(b.String()), nil
}

// isOutlineBody reports whether the body consists entirely of non-empty ss://
// lines, which is exactly what an Outline response looks like.
func isOutlineBody(body []byte) bool {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return false
	}
	found := false
	for _, line := range strings.Split(trimmed, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "ss://") {
			return false
		}
		found = true
	}
	return found
}

func filterByScheme(entries []entry.Entry, prefix string) []entry.Entry {
	var out []entry.Entry
	for _, e := range entries {
		if strings.HasPrefix(e.URI, prefix) {
			out = append(out, e)
		}
	}
	return out
}

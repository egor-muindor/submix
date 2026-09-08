package extras

import (
	"errors"
	"strings"

	"github.com/egor-muindor/submix/internal/entry"
	"github.com/egor-muindor/submix/internal/linkparse"
)

// ParseSubscriptionBody parses an external subscription body: a base64 blob or a
// flat list of connection links. It returns an error if no links are found; in
// that case the caller keeps the previous successful result.
func ParseSubscriptionBody(body []byte) ([]entry.Entry, error) {
	text := strings.TrimSpace(string(body))
	if text == "" {
		return nil, errors.New("empty subscription body")
	}

	if decoded, err := linkparse.DecodeBase64(text); err == nil && strings.Contains(string(decoded), "://") {
		text = string(decoded)
	}

	var entries []entry.Entry
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "://") {
			continue
		}
		// Schemes we cannot convert (hysteria2 and the like) are kept too: they
		// pass through to the base64/links formats as is, and the merger skips
		// them in the other formats.
		entries = append(entries, entry.Entry{
			Name: linkparse.Name(line),
			URI:  line,
		})
	}

	if len(entries) == 0 {
		return nil, errors.New("no connection links found in subscription body")
	}
	return entries, nil
}

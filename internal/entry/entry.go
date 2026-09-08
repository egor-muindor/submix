// Package entry holds the canonical types exchanged between extras-api and sub-mixer.
package entry

import "encoding/json"

// Format is the subscription response format, detected from the panel response body.
type Format string

const (
	FormatBase64   Format = "base64"
	FormatLinks    Format = "links"
	FormatXrayJSON Format = "xray-json"
	FormatClash    Format = "clash"
	FormatSingbox  Format = "singbox"
	FormatUnknown  Format = "unknown"
)

// Keys in Entry.Overrides.
const (
	OverrideClash   = "clash"
	OverrideSingbox = "singbox"
	OverrideXray    = "xray"
)

// Entry is a single extra connection link.
type Entry struct {
	Name string   `json:"name"`
	URI  string   `json:"uri"`
	Tags []string `json:"tags,omitempty"`
	// Overrides holds ready-made objects for formats the URI cannot be converted to
	// automatically. Keys: clash, singbox, xray.
	Overrides map[string]json.RawMessage `json:"overrides,omitempty"`
}

// MatchesAny reports whether the entry's tags intersect with the user's tags.
func (e Entry) MatchesAny(userTags []string) bool {
	for _, ut := range userTags {
		for _, et := range e.Tags {
			if et == ut {
				return true
			}
		}
	}
	return false
}

// Response is the extras-api response body.
type Response struct {
	Entries []Entry `json:"entries"`
}

// Package extrasui is a local editor for extras.yaml: a JSON API for the page
// and pure preview functions on top of internal/extras and internal/mixer.
package extrasui

import "github.com/egor-muindor/submix/internal/extras"

// FetchedEntry is an entry of an external subscription together with its
// conversion check result. Errors maps a format (clash, singbox, xray) to the
// error text; formats the entry converts to are absent from the map.
type FetchedEntry struct {
	Name   string            `json:"name"`
	URI    string            `json:"uri"`
	Errors map[string]string `json:"errors,omitempty"`
}

// FetchedSubscription holds the fetched entries of one subscription, as the
// page sends them to the preview.
type FetchedSubscription struct {
	Entries []FetchedEntry `json:"entries"`
}

// PreviewRequest is the body of POST /api/preview.
type PreviewRequest struct {
	Config   extras.Config                  `json:"config"`
	Fetched  map[string]FetchedSubscription `json:"fetched"`
	PanelTag string                         `json:"panel_tag"`
}

// EntryMatch is the result of applying the rules to one entry: the indexes of
// the matching rules and the resulting tags (empty if the entry was dropped).
type EntryMatch struct {
	Name    string   `json:"name"`
	Matched []int    `json:"matched"`
	Tags    []string `json:"tags"`
}

// SubscriptionPreview is the preview of one subscription. RuleErrors holds
// regexp compilation errors keyed by rule index (as a string, like a JSON
// object key).
type SubscriptionPreview struct {
	RuleErrors map[string]string `json:"rule_errors"`
	Entries    []EntryMatch      `json:"entries"`
}

// UserEntry is an entry the user being checked would receive.
// Source is the subscription name or "static".
type UserEntry struct {
	Name   string   `json:"name"`
	Source string   `json:"source"`
	Tags   []string `json:"tags"`
}

// UserPreview is what a user with the selected panel tag would receive.
type UserPreview struct {
	Tags    []string    `json:"tags"`
	Entries []UserEntry `json:"entries"`
}

// PreviewResponse is the response body of POST /api/preview.
type PreviewResponse struct {
	ValidationError string                         `json:"validation_error"`
	YAML            string                         `json:"yaml"`
	Subscriptions   map[string]SubscriptionPreview `json:"subscriptions"`
	StaticErrors    []map[string]string            `json:"static_errors"`
	User            UserPreview                    `json:"user"`
}

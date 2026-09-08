package mixer

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/egor-muindor/submix/internal/entry"
	"github.com/egor-muindor/submix/internal/linkconv"
	"github.com/egor-muindor/submix/internal/linkparse"
)

// errEmptyName is returned for an entry without a name: such an entry must not
// be written into a client config (in Clash it would become the field name: "").
var errEmptyName = errors.New("entry name is required")

// ClashProxy returns the proxy object for Clash-like formats.
func ClashProxy(e entry.Entry) (map[string]any, error) {
	if e.Name == "" {
		return nil, errEmptyName
	}
	if m, ok := overrideObject(e, entry.OverrideClash); ok {
		m["name"] = e.Name
		return m, nil
	}
	n, err := parseWithName(e)
	if err != nil {
		return nil, err
	}
	return linkconv.ToClash(n)
}

// SingboxOutbound returns the outbound for sing-box.
func SingboxOutbound(e entry.Entry) (map[string]any, error) {
	if e.Name == "" {
		return nil, errEmptyName
	}
	if m, ok := overrideObject(e, entry.OverrideSingbox); ok {
		m["tag"] = e.Name
		return m, nil
	}
	n, err := parseWithName(e)
	if err != nil {
		return nil, err
	}
	return linkconv.ToSingbox(n)
}

// XrayOutbound returns the outbound for XRAY_JSON (without the tag field).
func XrayOutbound(e entry.Entry) (map[string]any, error) {
	if e.Name == "" {
		return nil, errEmptyName
	}
	if m, ok := overrideObject(e, entry.OverrideXray); ok {
		return m, nil
	}
	n, err := parseWithName(e)
	if err != nil {
		return nil, err
	}
	return linkconv.ToXray(n)
}

// URIWithEntryName returns the connection link with the entry name in the fragment.
func URIWithEntryName(e entry.Entry) string {
	return linkparse.WithFragment(e.URI, e.Name)
}

func parseWithName(e entry.Entry) (linkparse.Node, error) {
	n, err := linkparse.Parse(e.URI)
	if err != nil {
		return linkparse.Node{}, fmt.Errorf("entry %q: %w", e.Name, err)
	}
	n.Name = e.Name
	return n, nil
}

func overrideObject(e entry.Entry, key string) (map[string]any, bool) {
	raw, ok := e.Overrides[key]
	if !ok {
		return nil, false
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, false
	}
	return m, true
}

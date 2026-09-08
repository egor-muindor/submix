package mixer

import (
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/egor-muindor/submix/internal/entry"
)

// MergeSingbox appends outbounds to a sing-box config and adds their tags to
// the selector/urltest groups.
func MergeSingbox(body []byte, entries []entry.Entry) ([]byte, error) {
	if len(entries) == 0 {
		return body, nil
	}

	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	outbounds, ok := doc["outbounds"].([]any)
	if !ok {
		return nil, errors.New("singbox: no outbounds array")
	}

	// sing-box rejects the whole config on duplicate outbounds[].tag: collect the
	// tags taken by the panel up front and rename collisions before adding entries.
	used := map[string]bool{}
	for _, raw := range outbounds {
		if m, ok := raw.(map[string]any); ok {
			if tag, ok := m["tag"].(string); ok {
				used[tag] = true
			}
		}
	}

	var added []string
	for _, e := range entries {
		ob, err := SingboxOutbound(e)
		if err != nil {
			slog.Warn("singbox: skip entry", "name", e.Name, "err", err)
			continue
		}
		name := uniqueName(used, e.Name)
		ob["tag"] = name
		outbounds = append(outbounds, ob)
		added = append(added, name)
	}
	if len(added) == 0 {
		return body, nil
	}

	for _, raw := range outbounds {
		group, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		switch group["type"] {
		case "selector", "urltest":
			list, ok := group["outbounds"].([]any)
			if !ok {
				continue
			}
			for _, name := range added {
				list = append(list, name)
			}
			group["outbounds"] = list
		}
	}

	doc["outbounds"] = outbounds
	return json.Marshal(doc)
}

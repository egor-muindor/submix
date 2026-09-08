package mixer

import (
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/egor-muindor/submix/internal/entry"
)

// MergeXrayJSON clones the first config of the array as a template and appends
// one config per entry, replacing the first (proxy) outbound.
func MergeXrayJSON(body []byte, entries []entry.Entry) ([]byte, error) {
	if len(entries) == 0 {
		return body, nil
	}

	var configs []map[string]any
	if err := json.Unmarshal(body, &configs); err != nil {
		return nil, err
	}
	if len(configs) == 0 {
		return nil, errors.New("xray-json: empty config array")
	}

	template, err := json.Marshal(configs[0])
	if err != nil {
		return nil, err
	}

	added := 0
	for _, e := range entries {
		ob, err := XrayOutbound(e)
		if err != nil {
			slog.Warn("xray-json: skip entry", "name", e.Name, "err", err)
			continue
		}

		var cfg map[string]any
		if err := json.Unmarshal(template, &cfg); err != nil {
			return nil, err
		}
		outbounds, ok := cfg["outbounds"].([]any)
		if !ok || len(outbounds) == 0 {
			return nil, errors.New("xray-json: template has no outbounds")
		}
		if first, ok := outbounds[0].(map[string]any); ok {
			if tag, ok := first["tag"]; ok {
				ob["tag"] = tag
			}
		}
		outbounds[0] = ob
		cfg["outbounds"] = outbounds
		cfg["remarks"] = e.Name

		configs = append(configs, cfg)
		added++
	}
	if added == 0 {
		return body, nil
	}
	return json.Marshal(configs)
}

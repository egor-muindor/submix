package extrasui

import (
	"bytes"

	"gopkg.in/yaml.v3"

	"github.com/egor-muindor/submix/internal/extras"
)

// RenderYAML serializes the config for saving and display. Empty optional
// fields are omitted (omitempty in the Config yaml tags). Characters outside
// the BMP (flag emoji) are written by yaml.v3 as \U0001F1E9: this is valid
// YAML and parses back to the same string.
func RenderYAML(cfg *extras.Config) (string, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(cfg); err != nil {
		return "", err
	}
	if err := enc.Close(); err != nil {
		return "", err
	}
	return buf.String(), nil
}

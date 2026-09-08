package mixer

import (
	"bytes"
	"errors"
	"log/slog"

	"gopkg.in/yaml.v3"

	"github.com/egor-muindor/submix/internal/entry"
)

// MergeClash appends the entries to the proxies field and to every proxy-groups
// group that has a proxies list.
func MergeClash(body []byte, entries []entry.Entry) ([]byte, error) {
	if len(entries) == 0 {
		return body, nil
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil, errors.New("clash: not a yaml document")
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, errors.New("clash: root is not a mapping")
	}

	proxies := mapValue(root, "proxies")
	if proxies == nil || proxies.Kind != yaml.SequenceNode {
		return nil, errors.New("clash: no proxies sequence")
	}

	// mihomo rejects the whole config on duplicate proxies[].name: collect the names
	// taken by the panel up front and rename collisions before adding entries.
	used := map[string]bool{}
	for _, p := range proxies.Content {
		if nameNode := mapValue(p, "name"); nameNode != nil {
			used[nameNode.Value] = true
		}
	}

	var added []string
	for _, e := range entries {
		p, err := ClashProxy(e)
		if err != nil {
			slog.Warn("clash: skip entry", "name", e.Name, "err", err)
			continue
		}
		name := uniqueName(used, e.Name)
		p["name"] = name
		var node yaml.Node
		if err := node.Encode(p); err != nil {
			slog.Warn("clash: encode entry", "name", e.Name, "err", err)
			continue
		}
		proxies.Content = append(proxies.Content, &node)
		added = append(added, name)
	}
	if len(added) == 0 {
		return body, nil
	}

	if groups := mapValue(root, "proxy-groups"); groups != nil && groups.Kind == yaml.SequenceNode {
		for _, g := range groups.Content {
			if g.Kind != yaml.MappingNode {
				continue
			}
			list := mapValue(g, "proxies")
			if list == nil || list.Kind != yaml.SequenceNode {
				continue
			}
			for _, name := range added {
				list.Content = append(list.Content, &yaml.Node{
					Kind:  yaml.ScalarNode,
					Tag:   "!!str",
					Value: name,
				})
			}
		}
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// mapValue returns the value stored under key in the mapping node.
func mapValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

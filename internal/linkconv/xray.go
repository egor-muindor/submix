package linkconv

import (
	"fmt"

	"github.com/egor-muindor/submix/internal/linkparse"
)

// ToXray builds an outbound for an Xray config (XRAY_JSON format).
// The "tag" field is left empty: the merger sets it by copying the tag from the template.
func ToXray(n linkparse.Node) (map[string]any, error) {
	switch n.Scheme {
	case "ss":
		return map[string]any{
			"protocol": "shadowsocks",
			"settings": map[string]any{
				"servers": []any{map[string]any{
					"address":  n.Host,
					"port":     n.Port,
					"method":   n.Method,
					"password": n.Password,
				}},
			},
			"streamSettings": map[string]any{"network": "tcp"},
		}, nil

	case "trojan":
		stream, err := xrayStream(n)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"protocol": "trojan",
			"settings": map[string]any{
				"servers": []any{map[string]any{
					"address":  n.Host,
					"port":     n.Port,
					"password": n.Credential,
				}},
			},
			"streamSettings": stream,
		}, nil

	case "vless":
		stream, err := xrayStream(n)
		if err != nil {
			return nil, err
		}
		user := map[string]any{"id": n.Credential, "encryption": "none"}
		if flow := n.Params.Get("flow"); flow != "" {
			user["flow"] = flow
		}
		return map[string]any{
			"protocol": "vless",
			"settings": map[string]any{
				"vnext": []any{map[string]any{
					"address": n.Host,
					"port":    n.Port,
					"users":   []any{user},
				}},
			},
			"streamSettings": stream,
		}, nil
	}

	return nil, fmt.Errorf("xray: unsupported scheme %q", n.Scheme)
}

func xrayStream(n linkparse.Node) (map[string]any, error) {
	network := n.Network()
	if network == "raw" {
		network = "tcp"
	}
	stream := map[string]any{"network": network}

	switch n.Security() {
	case "reality":
		stream["security"] = "reality"
		fp := n.Params.Get("fp")
		if fp == "" {
			fp = "chrome"
		}
		reality := map[string]any{
			"publicKey":   n.Params.Get("pbk"),
			"shortId":     n.Params.Get("sid"),
			"serverName":  n.Params.Get("sni"),
			"fingerprint": fp,
		}
		if spx := n.Params.Get("spx"); spx != "" {
			reality["spiderX"] = spx
		}
		stream["realitySettings"] = reality
	case "tls":
		stream["security"] = "tls"
		tls := map[string]any{}
		sni := n.Params.Get("sni")
		if sni == "" && n.Scheme == "trojan" {
			sni = n.Host
		}
		if sni != "" {
			tls["serverName"] = sni
		}
		if fp := n.Params.Get("fp"); fp != "" {
			tls["fingerprint"] = fp
		}
		if n.Params.Get("allowInsecure") == "1" {
			tls["allowInsecure"] = true
		}
		stream["tlsSettings"] = tls
	case "none":
		stream["security"] = "none"
	default:
		return nil, fmt.Errorf("xray: unsupported security %q", n.Security())
	}

	switch network {
	case "tcp":
		// no extra settings required
	case "ws":
		ws := map[string]any{}
		if path := n.Params.Get("path"); path != "" {
			ws["path"] = path
		}
		if host := n.Params.Get("host"); host != "" {
			ws["headers"] = map[string]any{"Host": host}
		}
		stream["wsSettings"] = ws
	case "grpc":
		stream["grpcSettings"] = map[string]any{"serviceName": n.Params.Get("serviceName")}
	default:
		return nil, fmt.Errorf("xray: unsupported network %q", network)
	}
	return stream, nil
}

// Package linkconv converts parsed connection links into objects of the
// client config formats (clash, sing-box, xray).
package linkconv

import (
	"fmt"

	"github.com/egor-muindor/submix/internal/linkparse"
)

// ToClash builds a proxy object for a Clash/Mihomo/Stash config.
func ToClash(n linkparse.Node) (map[string]any, error) {
	p := map[string]any{
		"name":   n.Name,
		"server": n.Host,
		"port":   n.Port,
		"udp":    true,
	}

	switch n.Scheme {
	case "ss":
		p["type"] = "ss"
		p["cipher"] = n.Method
		p["password"] = n.Password
		return p, nil

	case "trojan":
		p["type"] = "trojan"
		p["password"] = n.Credential
		if sni := n.Params.Get("sni"); sni != "" {
			p["sni"] = sni
		}
		if n.Params.Get("allowInsecure") == "1" {
			p["skip-cert-verify"] = true
		}
		if err := applyClashTransport(p, n); err != nil {
			return nil, err
		}
		return p, nil

	case "vless":
		p["type"] = "vless"
		p["uuid"] = n.Credential
		if flow := n.Params.Get("flow"); flow != "" {
			p["flow"] = flow
		}
		switch n.Security() {
		case "reality":
			p["tls"] = true
			if sni := n.Params.Get("sni"); sni != "" {
				p["servername"] = sni
			}
			p["reality-opts"] = map[string]any{
				"public-key": n.Params.Get("pbk"),
				"short-id":   n.Params.Get("sid"),
			}
		case "tls":
			p["tls"] = true
			if sni := n.Params.Get("sni"); sni != "" {
				p["servername"] = sni
			}
		case "none":
			// no transport encryption: nothing to add
		default:
			return nil, fmt.Errorf("clash: unsupported security %q", n.Security())
		}
		fp := n.Params.Get("fp")
		if fp == "" && n.Security() == "reality" {
			fp = "chrome"
		}
		if fp != "" {
			p["client-fingerprint"] = fp
		}
		if err := applyClashTransport(p, n); err != nil {
			return nil, err
		}
		return p, nil
	}

	return nil, fmt.Errorf("clash: unsupported scheme %q", n.Scheme)
}

func applyClashTransport(p map[string]any, n linkparse.Node) error {
	network := n.Network()
	p["network"] = network
	switch network {
	case "tcp", "raw":
		p["network"] = "tcp"
	case "ws":
		opts := map[string]any{}
		if path := n.Params.Get("path"); path != "" {
			opts["path"] = path
		}
		if host := n.Params.Get("host"); host != "" {
			opts["headers"] = map[string]any{"Host": host}
		}
		p["ws-opts"] = opts
	case "grpc":
		p["grpc-opts"] = map[string]any{"grpc-service-name": n.Params.Get("serviceName")}
	default:
		return fmt.Errorf("clash: unsupported network %q", network)
	}
	return nil
}

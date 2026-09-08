package linkconv

import (
	"fmt"

	"github.com/egor-muindor/submix/internal/linkparse"
)

// ToSingbox builds an outbound for a sing-box config.
func ToSingbox(n linkparse.Node) (map[string]any, error) {
	ob := map[string]any{
		"tag":         n.Name,
		"server":      n.Host,
		"server_port": n.Port,
	}

	switch n.Scheme {
	case "ss":
		ob["type"] = "shadowsocks"
		ob["method"] = n.Method
		ob["password"] = n.Password
		return ob, nil

	case "trojan":
		ob["type"] = "trojan"
		ob["password"] = n.Credential
		ob["tls"] = singboxTLS(n, true)
		if err := applySingboxTransport(ob, n); err != nil {
			return nil, err
		}
		return ob, nil

	case "vless":
		ob["type"] = "vless"
		ob["uuid"] = n.Credential
		if flow := n.Params.Get("flow"); flow != "" {
			ob["flow"] = flow
		}
		switch n.Security() {
		case "reality", "tls":
			ob["tls"] = singboxTLS(n, false)
		case "none":
			// no TLS
		default:
			return nil, fmt.Errorf("singbox: unsupported security %q", n.Security())
		}
		if err := applySingboxTransport(ob, n); err != nil {
			return nil, err
		}
		return ob, nil
	}

	return nil, fmt.Errorf("singbox: unsupported scheme %q", n.Scheme)
}

// singboxTLS assembles the tls block. fallbackSNIToHost=true is for trojan, where sni
// may be absent from the parameters.
func singboxTLS(n linkparse.Node, fallbackSNIToHost bool) map[string]any {
	tls := map[string]any{"enabled": true}

	sni := n.Params.Get("sni")
	if sni == "" && fallbackSNIToHost {
		sni = n.Host
	}
	if sni != "" {
		tls["server_name"] = sni
	}
	// sing-box refuses to load a config with reality but uTLS disabled
	// ("uTLS is required by reality client"), so reality always gets a
	// fingerprint, defaulting to chrome when the URI didn't specify one.
	if fp := n.Params.Get("fp"); fp != "" {
		tls["utls"] = map[string]any{"enabled": true, "fingerprint": fp}
	} else if n.Security() == "reality" {
		tls["utls"] = map[string]any{"enabled": true, "fingerprint": "chrome"}
	}
	if n.Security() == "reality" {
		tls["reality"] = map[string]any{
			"enabled":    true,
			"public_key": n.Params.Get("pbk"),
			"short_id":   n.Params.Get("sid"),
		}
	}
	if n.Params.Get("allowInsecure") == "1" {
		tls["insecure"] = true
	}
	return tls
}

func applySingboxTransport(ob map[string]any, n linkparse.Node) error {
	switch n.Network() {
	case "tcp", "raw":
		return nil // sing-box: default transport
	case "ws":
		tr := map[string]any{"type": "ws"}
		if path := n.Params.Get("path"); path != "" {
			tr["path"] = path
		}
		if host := n.Params.Get("host"); host != "" {
			tr["headers"] = map[string]any{"Host": host}
		}
		ob["transport"] = tr
		return nil
	case "grpc":
		ob["transport"] = map[string]any{
			"type":         "grpc",
			"service_name": n.Params.Get("serviceName"),
		}
		return nil
	default:
		return fmt.Errorf("singbox: unsupported network %q", n.Network())
	}
}

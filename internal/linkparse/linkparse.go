// Package linkparse parses connection links (vless/ss/trojan) into a Node structure.
package linkparse

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// Node is a parsed connection link.
type Node struct {
	Scheme string // vless | ss | trojan
	Name   string // name from the URI fragment
	Host   string
	Port   int

	// Credential is the id for vless and the password for trojan. Empty for ss.
	Credential string

	// ss only.
	Method   string
	Password string

	Params url.Values
}

// Network returns the transport (tcp by default).
func (n Node) Network() string {
	if v := n.Params.Get("type"); v != "" {
		return v
	}
	return "tcp"
}

// Security returns the security value (none by default).
func (n Node) Security() string {
	if v := n.Params.Get("security"); v != "" {
		return v
	}
	return "none"
}

// Parse parses a connection link.
func Parse(raw string) (Node, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil {
		return Node{}, fmt.Errorf("parse uri: %w", err)
	}
	switch u.Scheme {
	case "vless", "trojan":
		return parseUserHost(u)
	case "ss":
		return parseSS(u)
	default:
		return Node{}, fmt.Errorf("unsupported scheme %q", u.Scheme)
	}
}

func parseUserHost(u *url.URL) (Node, error) {
	if u.User == nil || u.User.Username() == "" {
		return Node{}, fmt.Errorf("%s: missing credential", u.Scheme)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return Node{}, fmt.Errorf("%s: bad port %q", u.Scheme, u.Port())
	}
	return Node{
		Scheme:     u.Scheme,
		Name:       u.Fragment,
		Host:       u.Hostname(),
		Port:       port,
		Credential: u.User.Username(),
		Params:     u.Query(),
	}, nil
}

func parseSS(u *url.URL) (Node, error) {
	n := Node{Scheme: "ss", Name: u.Fragment, Params: u.Query()}

	// Form ss://<userinfo>@host:port
	if u.User != nil && u.Host != "" {
		port, err := strconv.Atoi(u.Port())
		if err != nil {
			return Node{}, fmt.Errorf("ss: bad port %q", u.Port())
		}
		n.Host, n.Port = u.Hostname(), port
		if pw, ok := u.User.Password(); ok {
			n.Method, n.Password = u.User.Username(), pw
			return n, nil
		}
		decoded, err := DecodeBase64(u.User.Username())
		if err != nil {
			return Node{}, fmt.Errorf("ss: userinfo: %w", err)
		}
		method, pass, ok := strings.Cut(string(decoded), ":")
		if !ok {
			return Node{}, errors.New("ss: userinfo has no ':'")
		}
		n.Method, n.Password = method, pass
		return n, nil
	}

	// Form ss://<base64(method:password@host:port)>
	blob := u.Host
	if blob == "" {
		blob = u.Opaque
	}
	decoded, err := DecodeBase64(blob)
	if err != nil {
		return Node{}, fmt.Errorf("ss: body: %w", err)
	}
	inner, err := url.Parse("ss://" + string(decoded))
	if err != nil {
		return Node{}, fmt.Errorf("ss: inner uri: %w", err)
	}
	if inner.User == nil {
		return Node{}, errors.New("ss: inner uri has no userinfo")
	}
	port, err := strconv.Atoi(inner.Port())
	if err != nil {
		return Node{}, fmt.Errorf("ss: inner bad port %q", inner.Port())
	}
	pass, _ := inner.User.Password()
	n.Host, n.Port = inner.Hostname(), port
	n.Method, n.Password = inner.User.Username(), pass
	if n.Method == "" || n.Password == "" {
		return Node{}, errors.New("ss: empty method or password")
	}
	return n, nil
}

// Name returns the name from the fragment; if there is no fragment, host:port.
func Name(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return strings.TrimSpace(raw)
	}
	if u.Fragment != "" {
		return u.Fragment
	}
	if u.Host != "" {
		return u.Host
	}
	return strings.TrimSpace(raw)
}

// WithFragment replaces the name (fragment) in a connection link.
func WithFragment(raw, name string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return raw
	}
	u.Fragment = name
	return u.String()
}

// DecodeBase64 decodes a string in any of the four base64 alphabets.
func DecodeBase64(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimRight(s, "=")
	if s == "" {
		return nil, errors.New("empty base64 input")
	}
	for _, enc := range []*base64.Encoding{base64.RawStdEncoding, base64.RawURLEncoding} {
		if b, err := enc.DecodeString(s); err == nil {
			return b, nil
		}
	}
	return nil, errors.New("not base64")
}

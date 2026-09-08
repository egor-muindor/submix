package mixer

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"regexp"
	"strconv"

	"github.com/egor-muindor/submix/internal/entry"
)

// ProxyConfig holds the mixing proxy parameters.
type ProxyConfig struct {
	PanelURL       *url.URL
	Extras         *ExtrasClient
	InterceptPaths *regexp.Regexp
}

// NewProxy builds a transparent reverse proxy to the panel that mixes entries
// into responses for paths matching InterceptPaths.
func NewProxy(cfg ProxyConfig) (*httputil.ReverseProxy, error) {
	if cfg.PanelURL == nil {
		return nil, errors.New("proxy: PanelURL is required")
	}
	if cfg.Extras == nil {
		return nil, errors.New("proxy: Extras client is required")
	}
	if cfg.InterceptPaths == nil {
		return nil, errors.New("proxy: InterceptPaths is required")
	}

	target := cfg.PanelURL

	proxy := &httputil.ReverseProxy{
		// Director (not Rewrite): Rewrite strips the incoming X-Forwarded-*
		// headers, without which panel 3.x drops the connection.
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			// The panel must see the same Host as when sub-page calls it directly.
			req.Host = target.Host
			if cfg.InterceptPaths.MatchString(req.URL.Path) {
				// The response will be read and modified, so ask for it uncompressed.
				req.Header.Set("Accept-Encoding", "identity")
			}
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			slog.Error("proxy: upstream error", "path", r.URL.Path, "err", err)
			w.WriteHeader(http.StatusBadGateway)
		},
	}

	proxy.ModifyResponse = func(resp *http.Response) error {
		if resp.Request == nil || resp.StatusCode != http.StatusOK {
			return nil
		}
		if resp.Request.Method == http.MethodHead {
			// A HEAD response has no body: there is nothing to touch, and rewriting
			// Content-Length from the (zero) bytes read would corrupt it.
			return nil
		}
		if resp.Header.Get("Content-Encoding") != "" {
			// The upstream ignored Accept-Encoding: identity. The body cannot be
			// safely parsed as text/JSON/YAML, so pass it through as is.
			return nil
		}
		match := cfg.InterceptPaths.FindStringSubmatch(resp.Request.URL.Path)
		if match == nil {
			return nil
		}
		shortUUID := shortUUIDFromMatch(match, resp.Request.URL.Path)

		// io.ReadAll returns both the partially read bytes and the error at once:
		// use whatever was read instead of breaking the client connection with
		// a 502 response (which would violate fail-open).
		original, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			slog.Error("proxy: read upstream body", "subHash", subHash(shortUUID), "err", err)
			resp.Body = io.NopCloser(bytes.NewReader(original))
			return nil
		}

		out := original
		merge := func() ([]byte, error) {
			return mixBody(resp.Request, cfg.Extras, shortUUID, original)
		}
		if merged, err := withRecover(merge); err != nil {
			slog.Warn("proxy: mixing skipped", "subHash", subHash(shortUUID), "err", err)
		} else {
			out = merged
		}

		resp.Body = io.NopCloser(bytes.NewReader(out))
		if !bytes.Equal(out, original) {
			resp.ContentLength = int64(len(out))
			resp.Header.Set("Content-Length", strconv.Itoa(len(out)))
			resp.Header.Del("Content-Encoding")
		}
		return nil
	}

	return proxy, nil
}

// mixBody fetches entries from extras-api and mixes them into the response body.
func mixBody(req *http.Request, extras *ExtrasClient, shortUUID string, body []byte) ([]byte, error) {
	format := Detect(body)
	if format == entry.FormatUnknown {
		return body, nil
	}

	entries, err := extras.Fetch(req.Context(), shortUUID, format)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return body, nil
	}

	merged, _, err := Merge(body, entries)
	if err != nil {
		return nil, err
	}
	slog.Info("proxy: mixed entries",
		"subHash", subHash(shortUUID), "format", string(format), "entries", len(entries))
	return merged, nil
}

func shortUUIDFromMatch(match []string, urlPath string) string {
	if len(match) > 1 && match[1] != "" {
		return match[1]
	}
	return path.Base(urlPath)
}

// subHash returns a truncated hash of shortUuid for logs. shortUuid is the
// subscription secret (the panel serves the config by it without authentication),
// so it is never written in plain text to logs that persist on the node's disk.
func subHash(shortUUID string) string {
	sum := sha256.Sum256([]byte(shortUUID))
	return fmt.Sprintf("%x", sum)[:8]
}

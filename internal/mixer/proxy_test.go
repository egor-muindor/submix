package mixer_test

import (
	"bytes"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/egor-muindor/submix/internal/mixer"
)

// panelStub serves a base64 subscription at /api/sub/<uuid> and arbitrary content on other paths.
func panelStub(t *testing.T, seen *http.Header) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if seen != nil {
			*seen = r.Header.Clone()
		}
		if strings.HasPrefix(r.URL.Path, "/api/sub/") {
			w.Header().Set("Content-Type", "text/plain")
			w.Header().Set("subscription-userinfo", "upload=0; download=0; total=100")
			_, _ = w.Write([]byte(base64.StdEncoding.EncodeToString([]byte("vless://u@panel.example.com:443#Panel-1\n"))))
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html>page</html>"))
	}))
}

func extrasStub(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if status != http.StatusOK {
			http.Error(w, "boom", status)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
}

func newTestProxy(t *testing.T, panelURL, extrasURL string) *httptest.Server {
	t.Helper()
	target, err := url.Parse(panelURL)
	if err != nil {
		t.Fatalf("parse panel url: %v", err)
	}
	proxy, err := mixer.NewProxy(mixer.ProxyConfig{
		PanelURL:       target,
		Extras:         mixer.NewExtrasClient(extrasURL, time.Second),
		InterceptPaths: regexp.MustCompile(`^/api/sub/([^/]+)`),
	})
	if err != nil {
		t.Fatalf("NewProxy: %v", err)
	}
	return httptest.NewServer(proxy)
}

func TestProxyMixesSubscription(t *testing.T) {
	panel := panelStub(t, nil)
	defer panel.Close()
	extras := extrasStub(t, http.StatusOK, `{"entries":[{"name":"DE-1","uri":"vless://u2@de.example.com:443?security=tls#P"}]}`)
	defer extras.Close()

	front := newTestProxy(t, panel.URL, extras.URL)
	defer front.Close()

	resp, err := http.Get(front.URL + "/api/sub/short123")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	decoded, err := base64.StdEncoding.DecodeString(string(body))
	if err != nil {
		t.Fatalf("decode: %v\nbody=%s", err, body)
	}
	if !strings.Contains(string(decoded), "#Panel-1") || !strings.Contains(string(decoded), "#DE-1") {
		t.Fatalf("decoded = %q", decoded)
	}
	if got := resp.Header.Get("subscription-userinfo"); got != "upload=0; download=0; total=100" {
		t.Fatalf("panel headers must pass through, got %q", got)
	}
	if resp.ContentLength != int64(len(body)) {
		t.Fatalf("content-length = %d, body = %d", resp.ContentLength, len(body))
	}
}

func TestProxyFailOpenWhenExtrasDown(t *testing.T) {
	panel := panelStub(t, nil)
	defer panel.Close()
	extras := extrasStub(t, http.StatusInternalServerError, "")
	defer extras.Close()

	front := newTestProxy(t, panel.URL, extras.URL)
	defer front.Close()

	resp, err := http.Get(front.URL + "/api/sub/short123")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	decoded, err := base64.StdEncoding.DecodeString(string(body))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if strings.Contains(string(decoded), "DE-1") {
		t.Fatalf("nothing must be mixed in: %q", decoded)
	}
	if !strings.Contains(string(decoded), "#Panel-1") {
		t.Fatalf("original body must be intact: %q", decoded)
	}
}

func TestProxyPassesThroughOtherPaths(t *testing.T) {
	panel := panelStub(t, nil)
	defer panel.Close()
	extras := extrasStub(t, http.StatusOK, `{"entries":[{"name":"DE-1","uri":"vless://u2@h:443?security=tls#P"}]}`)
	defer extras.Close()

	front := newTestProxy(t, panel.URL, extras.URL)
	defer front.Close()

	resp, err := http.Get(front.URL + "/assets/app.js")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if string(body) != "<html>page</html>" {
		t.Fatalf("body = %q", body)
	}
}

func TestProxyForwardsHeadersToPanel(t *testing.T) {
	var seen http.Header
	panel := panelStub(t, &seen)
	defer panel.Close()
	extras := extrasStub(t, http.StatusOK, `{"entries":[]}`)
	defer extras.Close()

	front := newTestProxy(t, panel.URL, extras.URL)
	defer front.Close()

	req, _ := http.NewRequest(http.MethodGet, front.URL+"/api/sub/short123", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.7")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("User-Agent", "Happ/1.0")
	req.Header.Set("x-remnawave-real-ip", "203.0.113.7")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)

	if !strings.Contains(seen.Get("X-Forwarded-For"), "203.0.113.7") {
		t.Fatalf("X-Forwarded-For = %q", seen.Get("X-Forwarded-For"))
	}
	if seen.Get("X-Forwarded-Proto") != "https" {
		t.Fatalf("X-Forwarded-Proto = %q", seen.Get("X-Forwarded-Proto"))
	}
	if seen.Get("User-Agent") != "Happ/1.0" {
		t.Fatalf("User-Agent = %q", seen.Get("User-Agent"))
	}
	if seen.Get("x-remnawave-real-ip") != "203.0.113.7" {
		t.Fatalf("x-remnawave-real-ip = %q", seen.Get("x-remnawave-real-ip"))
	}
	if seen.Get("Accept-Encoding") != "identity" {
		t.Fatalf("intercepted requests must ask for identity encoding, got %q", seen.Get("Accept-Encoding"))
	}
}

func TestProxySkipsInterceptOnHeadRequests(t *testing.T) {
	panel := panelStub(t, nil)
	defer panel.Close()
	extras := extrasStub(t, http.StatusOK, `{"entries":[{"name":"DE-1","uri":"vless://u2@de.example.com:443?security=tls#P"}]}`)
	defer extras.Close()

	front := newTestProxy(t, panel.URL, extras.URL)
	defer front.Close()

	direct, err := http.Head(panel.URL + "/api/sub/short123")
	if err != nil {
		t.Fatalf("head direct: %v", err)
	}
	direct.Body.Close()

	resp, err := http.Head(front.URL + "/api/sub/short123")
	if err != nil {
		t.Fatalf("head: %v", err)
	}
	defer resp.Body.Close()

	if resp.ContentLength != direct.ContentLength {
		t.Fatalf("Content-Length = %d, want the panel's original %d (not overwritten with 0)",
			resp.ContentLength, direct.ContentLength)
	}
}

func TestProxySkipsInterceptWhenUpstreamSendsContentEncoding(t *testing.T) {
	rawBody := []byte{0x1f, 0x8b, 0x08, 0x00, 0xff, 0xff, 0xff}
	panel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// a stubborn upstream ignores Accept-Encoding: identity and compresses anyway.
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(rawBody)
	}))
	defer panel.Close()
	extras := extrasStub(t, http.StatusOK, `{"entries":[{"name":"DE-1","uri":"vless://u2@de.example.com:443?security=tls#P"}]}`)
	defer extras.Close()

	front := newTestProxy(t, panel.URL, extras.URL)
	defer front.Close()

	req, _ := http.NewRequest(http.MethodGet, front.URL+"/api/sub/short123", nil)
	req.Header.Set("Accept-Encoding", "identity") // disable the test client transport's automatic decompression
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if resp.Header.Get("Content-Encoding") != "gzip" {
		t.Fatalf("Content-Encoding must pass through untouched, got %q", resp.Header.Get("Content-Encoding"))
	}
	if !bytes.Equal(body, rawBody) {
		t.Fatalf("body must pass through untouched when upstream sent Content-Encoding, got %x, want %x", body, rawBody)
	}
}

func TestProxyModifyResponseNeverFailsOnReadError(t *testing.T) {
	panel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("short"))
		if fl, ok := w.(http.Flusher); ok {
			fl.Flush()
		}
		// close the connection before the chunked body completes; this simulates
		// a network error while reading the upstream response.
		if hj, ok := w.(http.Hijacker); ok {
			if conn, _, err := hj.Hijack(); err == nil {
				_ = conn.Close()
			}
		}
	}))
	defer panel.Close()
	extras := extrasStub(t, http.StatusOK, `{"entries":[{"name":"DE-1","uri":"vless://u2@de.example.com:443?security=tls#P"}]}`)
	defer extras.Close()

	front := newTestProxy(t, panel.URL, extras.URL)
	defer front.Close()

	resp, err := http.Get(front.URL + "/api/sub/short123")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 even when reading the upstream body fails (fail-open, no 502)", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("client-side read must not fail either: %v", err)
	}
	if string(body) != "short" {
		t.Fatalf("body = %q, want the bytes read before the upstream error", body)
	}
}

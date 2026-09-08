package mixer_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/egor-muindor/submix/internal/entry"
	"github.com/egor-muindor/submix/internal/mixer"
)

func TestExtrasClientFetch(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"entries":[{"name":"DE-1","uri":"vless://u@h:443"}]}`))
	}))
	defer srv.Close()

	c := mixer.NewExtrasClient(srv.URL, time.Second)
	entries, err := c.Fetch(context.Background(), "short123", entry.FormatClash)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "DE-1" {
		t.Fatalf("entries = %#v", entries)
	}
	if gotQuery != "/v1/extras?format=clash&shortUuid=short123" {
		t.Fatalf("query = %q", gotQuery)
	}
}

func TestExtrasClientErrors(t *testing.T) {
	t.Run("http error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "boom", http.StatusInternalServerError)
		}))
		defer srv.Close()

		c := mixer.NewExtrasClient(srv.URL, time.Second)
		if _, err := c.Fetch(context.Background(), "s", entry.FormatBase64); err == nil {
			t.Fatal("want error on 500")
		}
	})

	t.Run("timeout", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(200 * time.Millisecond)
			_, _ = w.Write([]byte(`{"entries":[]}`))
		}))
		defer srv.Close()

		c := mixer.NewExtrasClient(srv.URL, 20*time.Millisecond)
		if _, err := c.Fetch(context.Background(), "s", entry.FormatBase64); err == nil {
			t.Fatal("want timeout error")
		}
	})

	t.Run("response body over the size limit", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"entries":[{"name":"`))
			_, _ = w.Write(bytes.Repeat([]byte("a"), 9<<20)) // 9 MiB, over the 8 MiB limit
			_, _ = w.Write([]byte(`","uri":"ss://x"}]}`))
		}))
		defer srv.Close()

		c := mixer.NewExtrasClient(srv.URL, 5*time.Second)
		if _, err := c.Fetch(context.Background(), "s", entry.FormatBase64); err == nil {
			t.Fatal("want error when the response body exceeds the read limit")
		}
	})
}

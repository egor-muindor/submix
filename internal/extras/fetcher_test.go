package extras_test

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/egor-muindor/submix/internal/extras"
)

func TestFetchOnceAppliesRulesAndUserAgent(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		links := "vless://u@de.example.com:443?security=tls#DE-Berlin\n" +
			"vless://u@fr.example.com:443?security=tls#FR-Paris\n"
		_, _ = w.Write([]byte(base64.StdEncoding.EncodeToString([]byte(links))))
	}))
	defer srv.Close()

	sub := extras.Subscription{
		Name:     "provider-a",
		URL:      srv.URL,
		UA:       "TestAgent/1.0",
		Interval: time.Hour,
		Rules: []extras.Rule{
			{Match: "DE", Tags: []string{"de"}, Re: regexp.MustCompile("DE")},
		},
	}

	store, err := extras.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	f := extras.NewFetcher(store, 5*time.Second)

	if err := f.FetchOnce(context.Background(), sub); err != nil {
		t.Fatalf("FetchOnce: %v", err)
	}
	if gotUA != "TestAgent/1.0" {
		t.Fatalf("User-Agent = %q", gotUA)
	}

	all := store.All()
	if len(all) != 1 {
		t.Fatalf("stored = %#v", all)
	}
	if all[0].Name != "DE-Berlin" {
		t.Fatalf("name = %q", all[0].Name)
	}
	if len(all[0].Tags) != 1 || all[0].Tags[0] != "de" {
		t.Fatalf("tags = %#v", all[0].Tags)
	}
}

func TestFetchOnceKeepsLastGoodOnFailure(t *testing.T) {
	fail := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte("vless://u@de.example.com:443?security=tls#DE-Berlin\n"))
	}))
	defer srv.Close()

	sub := extras.Subscription{
		Name: "provider-a", URL: srv.URL, UA: "T", Interval: time.Hour,
		Rules: []extras.Rule{{Match: ".*", Tags: []string{"all"}, Re: regexp.MustCompile(".*")}},
	}

	store, err := extras.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	f := extras.NewFetcher(store, 5*time.Second)

	if err := f.FetchOnce(context.Background(), sub); err != nil {
		t.Fatalf("first FetchOnce: %v", err)
	}

	fail = true
	if err := f.FetchOnce(context.Background(), sub); err == nil {
		t.Fatal("want error on 500")
	}

	all := store.All()
	if len(all) != 1 || all[0].Name != "DE-Berlin" {
		t.Fatalf("last-good must survive: %#v", all)
	}
}

func TestFetchOnceRejectsGarbageBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html>maintenance</html>"))
	}))
	defer srv.Close()

	sub := extras.Subscription{
		Name: "provider-a", URL: srv.URL, UA: "T", Interval: time.Hour,
		Rules: []extras.Rule{{Match: ".*", Tags: []string{"all"}, Re: regexp.MustCompile(".*")}},
	}

	store, err := extras.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	f := extras.NewFetcher(store, 5*time.Second)

	if err := f.FetchOnce(context.Background(), sub); err == nil {
		t.Fatal("want error on garbage body")
	}
	if len(store.All()) != 0 {
		t.Fatalf("nothing must be stored: %#v", store.All())
	}
}

func TestFetchEntriesReturnsAllWithoutRules(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte("vless://u@de.example.com:443?security=tls#DE-Berlin\n" +
			"vless://u@fr.example.com:443?security=tls#FR-Paris\n"))
	}))
	defer srv.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	entries, err := extras.FetchEntries(context.Background(), client, srv.URL, "TestAgent/1.0")
	if err != nil {
		t.Fatalf("FetchEntries: %v", err)
	}
	if gotUA != "TestAgent/1.0" {
		t.Fatalf("User-Agent = %q", gotUA)
	}
	if len(entries) != 2 || entries[0].Name != "DE-Berlin" || entries[1].Name != "FR-Paris" {
		t.Fatalf("entries = %#v", entries)
	}
	if entries[0].Tags != nil {
		t.Fatalf("FetchEntries must not apply rules, got tags %#v", entries[0].Tags)
	}
}

func TestFetchEntriesReportsBadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusForbidden)
	}))
	defer srv.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	_, err := extras.FetchEntries(context.Background(), client, srv.URL, "x")
	if err == nil || !strings.Contains(err.Error(), "status 403") {
		t.Fatalf("err = %v, want status 403", err)
	}
}

package extras_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egor-muindor/submix/internal/extras"
)

// fakePanel emulates two panel endpoints: /api/sub/{shortUuid}/info (no token)
// and /api/users/by-username/{username} (with token).
type fakePanel struct {
	infoBody     string
	userBody     string
	userStatus   int
	infoCalls    atomic.Int32
	userCalls    atomic.Int32
	gotAuth      string
	gotForwarded string
}

func (f *fakePanel) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f.gotAuth = r.Header.Get("Authorization")
		f.gotForwarded = r.Header.Get("X-Forwarded-For")
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/sub/short123/info":
			f.infoCalls.Add(1)
			_, _ = w.Write([]byte(f.infoBody))
		case "/api/users/by-username/alice":
			f.userCalls.Add(1)
			if f.userStatus != 0 {
				http.Error(w, "nope", f.userStatus)
				return
			}
			_, _ = w.Write([]byte(f.userBody))
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}
}

const activeInfo = `{"response":{"isFound":true,"user":{"username":"alice","userStatus":"ACTIVE","isActive":true,"expiresAt":"2099-12-31T15:13:22.214Z"}}}`

func TestPanelClientUserForCachesAndSendsHeaders(t *testing.T) {
	panel := &fakePanel{infoBody: activeInfo, userBody: `{"response":{"username":"alice","tag":"PREMIUM"}}`}
	srv := httptest.NewServer(panel.handler())
	defer srv.Close()

	p := extras.NewPanelClient(srv.URL, "token123", time.Minute, 2*time.Second)

	user, err := p.UserFor(context.Background(), "short123")
	if err != nil {
		t.Fatalf("UserFor: %v", err)
	}
	if !user.Active || user.Tag != "PREMIUM" || user.Username != "alice" || user.Status != "ACTIVE" {
		t.Fatalf("user = %+v", user)
	}
	if panel.gotAuth != "Bearer token123" {
		t.Fatalf("Authorization = %q", panel.gotAuth)
	}
	if panel.gotForwarded == "" {
		t.Fatal("panel 3.x requires X-Forwarded-For to be set")
	}

	if _, err := p.UserFor(context.Background(), "short123"); err != nil {
		t.Fatalf("second UserFor: %v", err)
	}
	if panel.infoCalls.Load() != 1 || panel.userCalls.Load() != 1 {
		t.Fatalf("calls info=%d user=%d, want 1/1 (cached)", panel.infoCalls.Load(), panel.userCalls.Load())
	}
}

func TestPanelClientNullTag(t *testing.T) {
	panel := &fakePanel{infoBody: activeInfo, userBody: `{"response":{"username":"alice","tag":null}}`}
	srv := httptest.NewServer(panel.handler())
	defer srv.Close()

	p := extras.NewPanelClient(srv.URL, "t", time.Minute, 2*time.Second)
	user, err := p.UserFor(context.Background(), "short123")
	if err != nil {
		t.Fatalf("UserFor: %v", err)
	}
	if user.Tag != "" || !user.Active {
		t.Fatalf("user = %+v, want active with empty tag", user)
	}
}

func TestPanelClientTagLookupFailureKeepsStatus(t *testing.T) {
	// Token without access to /api/users/*: the status is known, the tag is not.
	panel := &fakePanel{infoBody: activeInfo, userStatus: http.StatusForbidden}
	srv := httptest.NewServer(panel.handler())
	defer srv.Close()

	p := extras.NewPanelClient(srv.URL, "t", time.Minute, 2*time.Second)
	user, err := p.UserFor(context.Background(), "short123")
	if err != nil {
		t.Fatalf("UserFor must not fail when only the tag lookup is forbidden: %v", err)
	}
	if !user.Active || user.Tag != "" {
		t.Fatalf("user = %+v, want active without tag", user)
	}
}

func TestPanelClientInactiveUsers(t *testing.T) {
	cases := map[string]string{
		"expired status":   `{"response":{"isFound":true,"user":{"username":"alice","userStatus":"EXPIRED","isActive":false,"expiresAt":"2026-01-01T00:00:00.000Z"}}}`,
		"disabled status":  `{"response":{"isFound":true,"user":{"username":"alice","userStatus":"DISABLED","isActive":false,"expiresAt":"2099-12-31T00:00:00.000Z"}}}`,
		"date in the past": `{"response":{"isFound":true,"user":{"username":"alice","userStatus":"ACTIVE","isActive":true,"expiresAt":"2026-01-01T00:00:00.000Z"}}}`,
		"not found":        `{"response":{"isFound":false}}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			panel := &fakePanel{infoBody: body, userBody: `{"response":{"tag":"PREMIUM"}}`}
			srv := httptest.NewServer(panel.handler())
			defer srv.Close()

			p := extras.NewPanelClient(srv.URL, "t", time.Minute, 2*time.Second)
			user, err := p.UserFor(context.Background(), "short123")
			if err != nil {
				t.Fatalf("UserFor: %v", err)
			}
			if user.Active {
				t.Fatalf("user must be inactive: %+v", user)
			}
		})
	}
}

func TestPanelClientServesStaleOnError(t *testing.T) {
	fail := false
	panel := &fakePanel{infoBody: activeInfo, userBody: `{"response":{"tag":"PREMIUM"}}`}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		panel.handler()(w, r)
	}))
	defer srv.Close()

	// Zero TTL: every call goes to the panel, but on error the stale value is served.
	p := extras.NewPanelClient(srv.URL, "t", 0, 2*time.Second)

	if _, err := p.UserFor(context.Background(), "short123"); err != nil {
		t.Fatalf("first UserFor: %v", err)
	}

	fail = true
	user, err := p.UserFor(context.Background(), "short123")
	if err != nil {
		t.Fatalf("stale value must be served without error: %v", err)
	}
	if user.Tag != "PREMIUM" || !user.Active {
		t.Fatalf("user = %+v", user)
	}
}

func TestPanelClientErrorWithoutCache(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	p := extras.NewPanelClient(srv.URL, "t", time.Minute, 2*time.Second)
	if _, err := p.UserFor(context.Background(), "short123"); err == nil {
		t.Fatal("want error when there is no cached value")
	}
}

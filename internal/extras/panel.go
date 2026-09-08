package extras

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// PanelUser is what extras-api knows about a panel user.
type PanelUser struct {
	Username string
	// Tag is the user's tag field in the panel; empty if there is no tag or it
	// could not be fetched (the user then gets only default_tags).
	Tag string
	// Active means status ACTIVE and the subscription has not expired. Inactive
	// users get nothing mixed in: the panel serves them a placeholder such as
	// "Subscription expired", and external nodes next to it would be a bug.
	Active bool
	// Status is the panel's userStatus (ACTIVE, EXPIRED, DISABLED, LIMITED), for logs.
	Status string
}

// PanelClient fetches user data from the Remnawave panel and caches it.
//
// Two requests instead of a single /api/users/by-short-uuid: the subscription-page
// token gets a 403 there, whereas /api/sub/{shortUuid}/info returns status,
// expiry and username with no token at all, and /api/users/by-username/{username}
// works with that token and returns the tag.
type PanelClient struct {
	baseURL string
	token   string
	ttl     time.Duration
	http    *http.Client

	mu    sync.Mutex
	cache map[string]cachedUser
}

type cachedUser struct {
	user      PanelUser
	fetchedAt time.Time
}

// NewPanelClient creates a panel client.
func NewPanelClient(baseURL, token string, ttl, timeout time.Duration) *PanelClient {
	return &PanelClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		ttl:     ttl,
		http:    &http.Client{Timeout: timeout},
		cache:   map[string]cachedUser{},
	}
}

// UserFor returns the user data for shortUuid. On a panel error it serves the
// last cached value (if there is one), otherwise it returns the error.
func (p *PanelClient) UserFor(ctx context.Context, shortUUID string) (PanelUser, error) {
	p.mu.Lock()
	cached, hasCached := p.cache[shortUUID]
	p.mu.Unlock()

	if hasCached && time.Since(cached.fetchedAt) < p.ttl {
		return cached.user, nil
	}

	user, err := p.fetchUser(ctx, shortUUID)
	if err != nil {
		if hasCached {
			slog.Warn("panel: serving stale user", "subHash", subHash(shortUUID), "err", err)
			return cached.user, nil
		}
		return PanelUser{}, err
	}

	p.mu.Lock()
	p.cache[shortUUID] = cachedUser{user: user, fetchedAt: time.Now()}
	p.mu.Unlock()

	return user, nil
}

// fetchUser: status and username from /api/sub/{shortUuid}/info, then tag from
// /api/users/by-username/{username}. A failure of the second request is not
// fatal: the status is already known, the user simply gets only default_tags.
func (p *PanelClient) fetchUser(ctx context.Context, shortUUID string) (PanelUser, error) {
	var info struct {
		Response struct {
			IsFound bool `json:"isFound"`
			User    struct {
				Username   string `json:"username"`
				UserStatus string `json:"userStatus"`
				IsActive   bool   `json:"isActive"`
				ExpiresAt  string `json:"expiresAt"`
			} `json:"user"`
		} `json:"response"`
	}
	if err := p.getJSON(ctx, "/api/sub/"+url.PathEscape(shortUUID)+"/info", &info); err != nil {
		return PanelUser{}, err
	}
	if !info.Response.IsFound {
		return PanelUser{Status: "NOT_FOUND"}, nil
	}

	u := info.Response.User
	user := PanelUser{
		Username: u.Username,
		Status:   u.UserStatus,
		Active:   u.IsActive && u.UserStatus == "ACTIVE" && !expired(u.ExpiresAt, time.Now()),
	}

	var full struct {
		Response struct {
			Tag *string `json:"tag"`
		} `json:"response"`
	}
	if err := p.getJSON(ctx, "/api/users/by-username/"+url.PathEscape(u.Username), &full); err != nil {
		slog.Warn("panel: tag lookup failed, default_tags only", "subHash", subHash(shortUUID), "err", err)
		return user, nil
	}
	if full.Response.Tag != nil {
		user.Tag = *full.Response.Tag
	}
	return user, nil
}

// expired reports whether expiresAt (RFC 3339) has passed. An unparsable or
// empty value does not count as an expiry: the panel's status and isActive decide.
func expired(expiresAt string, now time.Time) bool {
	if expiresAt == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339Nano, expiresAt)
	if err != nil {
		return false
	}
	return t.Before(now)
}

func (p *PanelClient) getJSON(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+p.token)
	// Panel 3.x drops the connection without X-Forwarded-*.
	req.Header.Set("X-Forwarded-For", "127.0.0.1")
	req.Header.Set("X-Forwarded-Proto", "https")

	resp, err := p.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("panel: status %d for %s", resp.StatusCode, path)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("panel: decode %s: %w", path, err)
	}
	return nil
}

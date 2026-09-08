package extras

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/egor-muindor/submix/internal/entry"
)

// maxSubscriptionBody caps how much of an external subscription body is read.
const maxSubscriptionBody = 8 << 20 // 8 MiB

// Fetcher polls external subscriptions and puts the result into the Store.
type Fetcher struct {
	store *Store
	http  *http.Client
}

// NewFetcher creates a fetcher with a per-request timeout.
func NewFetcher(store *Store, timeout time.Duration) *Fetcher {
	return &Fetcher{store: store, http: &http.Client{Timeout: timeout}}
}

// FetchEntries downloads the subscription via client and parses the body. It
// does not apply rules: this is the provider's raw entry list, which extras-ui
// needs for previews. A package-level function rather than a Fetcher method:
// extras-ui has no need for a Store.
func FetchEntries(ctx context.Context, client *http.Client, url, ua string) ([]entry.Entry, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", ua)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSubscriptionBody))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	return ParseSubscriptionBody(body)
}

// FetchOnce downloads the subscription, applies the rules and stores the result.
// On any error the Store keeps the previous successful result.
func (f *Fetcher) FetchOnce(ctx context.Context, sub Subscription) error {
	parsed, err := FetchEntries(ctx, f.http, sub.URL, sub.UA)
	if err != nil {
		return fmt.Errorf("subscription %q: %w", sub.Name, err)
	}

	var kept []entry.Entry
	for _, e := range parsed {
		tags, ok := ApplyRules(sub.Rules, e.Name)
		if !ok {
			continue
		}
		e.Tags = tags
		kept = append(kept, e)
	}

	f.store.Set(sub.Name, kept)
	slog.Info("subscription fetched",
		"subscription", sub.Name, "parsed", len(parsed), "kept", len(kept))
	return nil
}

// Run polls the subscription immediately and then every sub.Interval while the
// context is alive.
func (f *Fetcher) Run(ctx context.Context, sub Subscription) {
	if err := f.FetchOnce(ctx, sub); err != nil {
		slog.Warn("subscription fetch failed", "subscription", sub.Name, "err", err)
	}

	ticker := time.NewTicker(sub.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := f.FetchOnce(ctx, sub); err != nil {
				slog.Warn("subscription fetch failed", "subscription", sub.Name, "err", err)
			}
		}
	}
}

package mixer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/egor-muindor/submix/internal/entry"
)

// maxExtrasResponseBody is the read limit for an extras-api response body.
const maxExtrasResponseBody = 8 << 20 // 8 MiB

// ExtrasClient fetches additional entries from extras-api.
type ExtrasClient struct {
	baseURL string
	http    *http.Client
}

// NewExtrasClient creates a client with a per-request timeout.
func NewExtrasClient(baseURL string, timeout time.Duration) *ExtrasClient {
	return &ExtrasClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: timeout},
	}
}

// Fetch returns the entries for the given user and format.
func (c *ExtrasClient) Fetch(ctx context.Context, shortUUID string, format entry.Format) ([]entry.Entry, error) {
	q := url.Values{}
	q.Set("shortUuid", shortUUID)
	q.Set("format", string(format))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v1/extras?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("extras-api: status %d", resp.StatusCode)
	}

	var parsed entry.Response
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxExtrasResponseBody)).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("extras-api: decode: %w", err)
	}
	return parsed.Entries, nil
}

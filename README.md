# submix

An extension for the [Remnawave](https://remna.st) panel (3.x, developed and
run against 3.2.1) that mixes extra connection entries into the subscriptions
the panel serves.

Remnawave builds each user's subscription from the nodes it manages. submix
sits between `remnawave-subscription-page` and the panel backend and appends
additional entries to every subscription response: nodes parsed from external
subscriptions (other providers) and hand-written static links, targeted per
user through the user's tag in the panel. Nothing in the panel changes except
one environment variable of the subscription page, and the panel itself is
never modified: submix is a separate pair of containers on the same Docker
network.

submix is an independent third-party project. It is not affiliated with or
endorsed by the Remnawave project.

Two services ship in one Docker image, plus a local config editor:

- `cmd/sub-mixer` — transparent reverse proxy between `remnawave-subscription-page`
  and the panel backend; appends entries to `/api/sub/*` responses in every
  subscription format.
- `cmd/extras-api` — source of the entries: fetches and filters external
  subscriptions on a schedule, holds static entries, resolves the user's
  panel tag and answers "which entries does this user get".
- `cmd/extras-ui` — local web editor for `extras.yaml` with a live preview of
  the rules against real provider entries. Not part of the Docker image.

## How it works

```
subscription-page :3010  (REMNAWAVE_PANEL_URL=http://sub-mixer:3020)
   │
   ▼
sub-mixer :3020 ──────────────► extras-api :3030 ──► panel  /api/sub/*/info, /api/users/by-username/*
   │  transparent reverse proxy       │               (user status, expiry and tag)
   ▼                                  └──────────────► external subscriptions (on a schedule)
remnawave :3000
```

### sub-mixer

- Proxies every request to the panel unchanged. Responses whose path matches
  `INTERCEPT_PATHS` (default `^/api/sub/([^/]+)`, i.e. `/api/sub/<shortUuid>`
  and its `/clash`, `/singbox`, `/xray-json`, `/info` variants) are inspected.
- The subscription format is detected from the response body, not the URL:
  base64 link list, plaintext link list, XRAY_JSON, Clash / Mihomo / Stash
  YAML, sing-box JSON. Unknown bodies are passed through untouched.
- Entries are fetched from extras-api with the user's `shortUuid` and the
  detected format, converted to the target format (`internal/linkconv`) and
  appended: to the link list, to `proxies` plus every `proxy-group`, to
  `outbounds` plus every selector / urltest outbound, or as XRAY_JSON
  outbounds. Names that collide with panel nodes are renamed `<name> (2)`,
  `(3)`, … because mihomo and sing-box reject configs with duplicate names.
- Outline's plain `ss://` list only receives `ss://` entries: Outline clients
  cannot parse anything else.
- Fail-open is the core invariant. Any error (extras-api down or slow,
  conversion failure, a panic inside a merger, an unreadable body) results in
  the original panel response with the original headers. HEAD requests and
  already-compressed responses are never modified. `Content-Length` and
  `Content-Encoding` are rewritten only when the body actually changed.
- Subscription headers the clients rely on (`subscription-userinfo`,
  `profile-title`, `profile-update-interval`, `announce`, …) are passed
  through as-is.
- Logs never contain the `shortUuid` (it is the subscription secret); a
  truncated SHA-256 (`subHash`) is logged instead.

### extras-api

- Loads `extras.yaml` (see [Configuration](#configuration)), fetches every
  external subscription on its own interval with a configurable `User-Agent`,
  parses base64 / plaintext link lists and keeps the entries whose name
  matches at least one rule. Each matched rule adds its tags to the entry.
- The last good result of every subscription is persisted to `CACHE_DIR`, so a
  restart or an upstream outage does not empty the mix.
- The user is resolved through two panel endpoints: `/api/sub/{shortUuid}/info`
  gives status, expiry and username (no token required) and
  `/api/users/by-username/{username}` gives the tag (token required; if the
  token lacks permission the tag is treated as empty). Results are cached for
  `PANEL_CACHE_TTL` with a stale fallback while the panel is unreachable.
- Inactive users get nothing: status `EXPIRED`, `DISABLED`, `LIMITED`, a past
  `expiresAt` or an unknown `shortUuid`. The panel serves them a placeholder
  and extra nodes next to it would be wrong. A panel error with no cached
  answer also yields nothing; the subscription itself is still served by the
  mixer.
- The panel tag maps to "mix tags" through `users.by_panel_tag`;
  `users.default_tags` apply to every active user. An entry is served when its
  tags intersect the user's mix tags. User entries come from external
  subscriptions first, then static entries.

HTTP API:

```
GET /v1/extras?shortUuid=<id>&format=<base64|links|xray-json|clash|singbox>
→ {"entries":[{"name":"…","uri":"…","tags":["…"],"overrides":{…}}]}
GET /healthz → ok
```

### extras-ui

Local page for editing `extras.yaml` with a live preview. Binds to localhost
only, has no authentication by design and is not built into the Docker image.

```bash
go run ./cmd/extras-ui -config deploy/extras.yaml
# open http://127.0.0.1:3040
```

- Edits every section: `defaults`, `subscriptions` with rules,
  `static_entries`, `users`.
- "Fetch" downloads an external subscription and lists the provider's entry
  names: which rules matched, which tags the entry received, and what cannot
  be converted to Clash / sing-box / xray (⚠ with the error text).
- Shows what a user with a given panel tag would receive.
- Imports arbitrary YAML from a file, reloads the file from disk, shows and
  copies the generated YAML.
- Draft, fetched subscriptions and the selected tag live in the browser's
  localStorage and survive a reload; "Reset to file" replaces the draft with
  the file contents and keeps the fetched subscriptions.
- Saving runs the same validation as extras-api (`extras.Parse`) and writes
  the file atomically. Regular expressions are evaluated server-side (Go RE2),
  so the preview matches production.
- Requests must carry a localhost `Host`; POST handlers accept only
  `Content-Type: application/json` with no cross-site `Sec-Fetch-Site`, so
  another tab in the same browser can neither read nor overwrite the file.

Limitations: the file is regenerated on save, so comments in it are lost
(the reference comments live in `deploy/extras.example.yaml`); emoji in `match`
and names are written as `\U0001F1E9` escapes (valid YAML, parses to the same
string); the tool does not talk to the panel, the panel tag is picked by hand.

Flags: `-config` (default `deploy/extras.yaml`), `-listen` (`127.0.0.1:3040`).

## Configuration

`extras.yaml` (see `deploy/extras.example.yaml` for a commented example):

```yaml
defaults:
  user_agent: "Shadowrocket/3378 CFNetwork/3860.700.1 Darwin/25.6.0 iPhone15,2"
  refresh: 24h                      # per-subscription poll interval, minimum 1m

subscriptions:
  - name: provider-a
    url: https://provider.example/sub/TOKEN
    # user_agent / refresh override the defaults
    rules:                          # regexp on the entry name → tags
      - match: "Germany|DE|🇩🇪"
        tags: [de]
      - match: "Georgia|GE|🇬🇪"
        tags: [ge]

static_entries:
  - name: "My exit"
    uri: "vless://uuid@my.example.com:443?type=tcp&security=reality&…#My"
    tags: [premium]

users:
  default_tags: []                  # mix tags every active user gets
  by_panel_tag:                     # panel user tag → mix tags
    PREMIUM: [de, premium]
    FRIENDS_GE: [ge]
```

An entry passes a subscription when at least one rule matches its name; the
tags of all matching rules are united. Entries matching no rule are dropped.
A user in Remnawave has a single tag, so combinations are expressed as a
dedicated panel tag (for example `PREMIUM_GE`) with its own mapping.

The config file is read at start-up only. Restarting extras-api takes about a
second and the mixer serves plain panel responses meanwhile (fail-open), so
there is deliberately no hot reload.

### Environment variables

**sub-mixer**

| Variable          | Default               | Meaning                                   |
|-------------------|-----------------------|-------------------------------------------|
| `LISTEN`          | `:3020`               | listen address                            |
| `PANEL_URL`       | required              | panel backend, e.g. `http://remnawave:3000` |
| `EXTRAS_URL`      | required              | extras-api base URL                       |
| `EXTRAS_TIMEOUT`  | `500ms`               | budget for one extras-api call            |
| `INTERCEPT_PATHS` | `^/api/sub/([^/]+)`   | regexp of paths whose responses are mixed |

**extras-api**

| Variable          | Default                   | Meaning                                  |
|-------------------|---------------------------|------------------------------------------|
| `LISTEN`          | `:3030`                   | listen address                           |
| `CONFIG`          | `/etc/submix/extras.yaml` | config file                              |
| `CACHE_DIR`       | `/var/lib/submix`         | last-good cache of parsed subscriptions  |
| `PANEL_URL`       | required                  | panel backend                            |
| `PANEL_TOKEN`     | required                  | panel API token (the subscription-page token is sufficient) |
| `PANEL_CACHE_TTL` | `5m`                      | user resolution cache                    |
| `PANEL_TIMEOUT`   | `3s`                      | timeout of one panel request             |
| `FETCH_TIMEOUT`   | `30s`                     | timeout of one external subscription fetch |

**extras-ui** has no environment variables, only the `-config` and `-listen` flags.

## Deployment

See [`deploy/README.md`](deploy/README.md): building the image, the compose
file, wiring the subscription page through the mixer, verifying passthrough
before enabling any mixing, rollback, and updating the config with
`scripts/deploy-extras-config.sh`.

## Compatibility

| Component | Version | Notes |
|-----------|---------|-------|
| Remnawave panel (`remnawave/backend`) | 3.2.1 | developed and run against this release; 3.x API |
| `remnawave/subscription-page` | current image | only `REMNAWAVE_PANEL_URL` is changed |
| Go | 1.25+ | see `go.mod`; Docker image builds with `golang:1.26-alpine` |

Panel endpoints used: `/api/sub/{shortUuid}` and its `/clash`, `/singbox`,
`/xray-json`, `/info` variants (proxied), `/api/sub/{shortUuid}/info` and
`/api/users/by-username/{username}` (user resolution). Panel 3.x drops the
connection without `X-Forwarded-*` headers, so the proxy forwards them
unchanged. Other 3.x releases should work as long as these endpoints and the
response formats stay the same; earlier major versions are untested.

## Development

```bash
git clone https://github.com/egor-muindor/submix.git
cd submix
go build ./...
go test ./...
```

`scripts/compare-passthrough.sh <shortUuid>` compares panel and mixer
responses (bodies and headers) for a set of client user agents; run it on the
panel host before switching the subscription page over.

## Known limitations

- The HTML subscription page and its QR codes do not show mixed entries: the
  subscription page renders them from `/api/sub/<uuid>/info`, whose body is not
  a subscription format.
- Clash YAML rebuilt by the mixer escapes non-ASCII characters (`"🇳🇱"` becomes
  `"\U0001F1F3\U0001F1F1"`). This is valid YAML and mihomo / clash-verge read
  it; other Clash-compatible clients have not been verified.
- Only `ss://`, `vless://` and `trojan://` links are converted to Clash,
  sing-box and XRAY_JSON. Other protocols can be supplied as static entries
  with per-format `overrides`.

## License

Copyright (C) 2026 Egor Fadeev.

GNU General Public License v3.0 or later. See [LICENSE](LICENSE).

submix contains no Remnawave code and talks to the panel only through its
HTTP API. Remnawave itself is licensed under the AGPL-3.0; the two licenses
are explicitly compatible (section 13 of each).

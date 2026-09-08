#!/usr/bin/env bash
#
# Compares subscription responses served directly by the panel and through
# sub-mixer: bodies and headers. Headers matter as much as bodies: that is
# where the panel sends subscription-userinfo, profile-title,
# profile-update-interval and announce. Unstable headers (Date, Alt-Svc, ETag)
# and hop-by-hop ones (Connection, Keep-Alive, Transfer-Encoding), which a
# reverse proxy must strip, are excluded; header names are compared
# case-insensitively because Go canonicalises them (ETag -> Etag), which is
# equivalent for clients. The panel returns nodes in random order on every
# request, so the direct response is fetched twice: if the two direct bodies
# differ from each other the body is treated as volatile and compared by size.
#
# Run on the panel host; the sub-mixer and remnawave containers must be on the
# same Docker network (remnawave-network by default).
#
# Usage:
#   ./compare-passthrough.sh <shortUuid>
#   PANEL=http://remnawave:3000 MIXER=http://sub-mixer:3020 ./compare-passthrough.sh <shortUuid>
#
# Exit code: 0 — all responses match, 1 — differences found.

set -euo pipefail

SHORT_UUID="${1:-}"
if [[ -z "$SHORT_UUID" ]]; then
	echo "Usage: $0 <shortUuid>" >&2
	exit 2
fi

PANEL="${PANEL:-http://remnawave:3000}"
MIXER="${MIXER:-http://sub-mixer:3020}"
NETWORK="${NETWORK:-remnawave-network}"
CURL_IMAGE="${CURL_IMAGE:-curlimages/curl:latest}"

USER_AGENTS=(
	"Happ/1.0"
	"v2rayNG/1.8.5"
	"clash-verge/v1.5.1"
	"ClashMetaForAndroid/2.9.0"
	"Stash/2.5.0 Clash/1.0"
	"SFA/1.8.0 (sing-box 1.8.0)"
	"Streisand/1.5"
	"Shadowrocket/2.2"
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 Chrome/120.0 Safari/537.36"
	"curl/8.0"
)

# Paths requested by the subscription page. /info returns the subscription
# metadata as a separate request (used by the HTML page, among others).
PATHS=(
	"/api/sub/$SHORT_UUID"
	"/api/sub/$SHORT_UUID/info"
)

# fetch_headers <base> <ua> <path> <method> — response headers on stdout.
fetch_headers() {
	local base="$1" ua="$2" path="$3" method="$4"
	# HEAD needs -I: with -X HEAD curl waits for a body and exits with code 18.
	local method_opt=(-X "$method")
	[[ "$method" == "HEAD" ]] && method_opt=(-I)
	docker run --rm --network "$NETWORK" "$CURL_IMAGE" \
		-s --compressed -o /dev/null -D - "${method_opt[@]}" \
		-H "User-Agent: $ua" \
		-H "X-Forwarded-For: 127.0.0.1" \
		-H "X-Forwarded-Proto: https" \
		-H "x-remnawave-real-ip: 127.0.0.1" \
		"$base$path"
}

# fetch_body <base> <ua> <path> <method> — response body on stdout.
fetch_body() {
	local base="$1" ua="$2" path="$3" method="$4"
	docker run --rm --network "$NETWORK" "$CURL_IMAGE" \
		-s --compressed -X "$method" \
		-H "User-Agent: $ua" \
		-H "X-Forwarded-For: 127.0.0.1" \
		-H "X-Forwarded-Proto: https" \
		-H "x-remnawave-real-ip: 127.0.0.1" \
		"$base$path"
}

# normalize_headers drops unstable (Date, Alt-Svc, ETag) and hop-by-hop
# (Connection, Keep-Alive, Transfer-Encoding) headers, lowercases the names,
# strips \r and empty lines and sorts: the comparison is on the set of
# headers, not their order, which Go handlers do not guarantee.
normalize_headers() {
	tr -d '\r' \
		| grep -viE '^(Date|Alt-Svc|ETag|Connection|Keep-Alive|Transfer-Encoding):' \
		| grep -v '^[[:space:]]*$' \
		| awk -F: 'NF>1 { name=tolower($1); sub(/^[^:]*:/, ""); print name ":" $0; next } { print }' \
		| sort
}

failed=0

for path in "${PATHS[@]}"; do
	for ua in "${USER_AGENTS[@]}"; do
		direct_headers="$(fetch_headers "$PANEL" "$ua" "$path" GET | normalize_headers)"
		mixed_headers="$(fetch_headers "$MIXER" "$ua" "$path" GET | normalize_headers)"
		direct_body="$(fetch_body "$PANEL" "$ua" "$path" GET)"
		direct_body2="$(fetch_body "$PANEL" "$ua" "$path" GET)"
		mixed_body="$(fetch_body "$MIXER" "$ua" "$path" GET)"

		ok=1
		note=""
		if [[ "$direct_headers" != "$mixed_headers" ]]; then
			printf 'DIFF headers  GET %s  %s\n' "$path" "$ua"
			diff <(printf '%s' "$direct_headers") <(printf '%s' "$mixed_headers") || true
			ok=0
		fi
		if [[ "$direct_body" != "$direct_body2" ]]; then
			# The panel itself returns different bodies for identical requests
			# (random node order): nothing to compare byte by byte, check the size.
			note=" [volatile body, compared by size]"
			if [[ "${#direct_body}" -ne "${#mixed_body}" ]]; then
				printf 'DIFF body     GET %s  %s (direct %d bytes, mixer %d bytes; volatile body)\n' \
					"$path" "$ua" "${#direct_body}" "${#mixed_body}"
				ok=0
			fi
		elif [[ "$direct_body" != "$mixed_body" ]]; then
			printf 'DIFF body     GET %s  %s (direct %d bytes, mixer %d bytes)\n' \
				"$path" "$ua" "${#direct_body}" "${#mixed_body}"
			diff <(printf '%s' "$direct_body") <(printf '%s' "$mixed_body") | head -20 || true
			ok=0
		fi

		if [[ $ok -eq 1 ]]; then
			printf 'OK    GET %s  %s (%d bytes)%s\n' "$path" "$ua" "${#direct_body}" "$note"
		else
			failed=1
		fi
	done
done

# HEAD has no body: compare headers only (including Content-Length, which the
# mixer must pass through from the panel untouched).
for ua in "${USER_AGENTS[@]}"; do
	path="/api/sub/$SHORT_UUID"
	direct_headers="$(fetch_headers "$PANEL" "$ua" "$path" HEAD | normalize_headers)"
	mixed_headers="$(fetch_headers "$MIXER" "$ua" "$path" HEAD | normalize_headers)"

	if [[ "$direct_headers" == "$mixed_headers" ]]; then
		printf 'OK    HEAD %s  %s\n' "$path" "$ua"
	else
		printf 'DIFF headers  HEAD %s  %s\n' "$path" "$ua"
		diff <(printf '%s' "$direct_headers") <(printf '%s' "$mixed_headers") || true
		failed=1
	fi
done

if [[ $failed -eq 0 ]]; then
	echo "PASSTHROUGH OK: all responses match"
else
	echo "PASSTHROUGH FAILED: differences found" >&2
fi
exit $failed

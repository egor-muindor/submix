#!/usr/bin/env bash
#
# Uploads extras.yaml to the panel host, restarts extras-api and checks that the
# service came up and loaded its subscriptions. If the new config fails
# validation (extras-api dies on start-up), the previous file is restored and
# the service restarted once more. The mixer is fail-open the whole time:
# users keep receiving their subscriptions, just without extra entries.
#
# Usage:
#   ./scripts/deploy-extras-config.sh                 # deploy/extras.yaml
#   ./scripts/deploy-extras-config.sh path/to/extras.yaml
#   HOST=user@other.host ./scripts/deploy-extras-config.sh
#
# Settings: HOST (required), REMOTE_DIR (default /opt/submix), WAIT_SECONDS
# (default 20). They are read from the environment or from deploy/deploy.env
# (git-ignored, see deploy/deploy.example.env); the environment wins.
#
# Requirements: an SSH key for HOST and passwordless sudo on the host (needed
# for chgrp: the container reads the file as uid 10001).
#
# Exit code: 0 — config applied, 1 — rolled back to the previous one,
# 2 — usage error.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

env_host="${HOST:-}"
env_dir="${REMOTE_DIR:-}"
env_wait="${WAIT_SECONDS:-}"
ENV_FILE="$SCRIPT_DIR/../deploy/deploy.env"
if [[ -f "$ENV_FILE" ]]; then
	# shellcheck disable=SC1090
	. "$ENV_FILE"
fi
HOST="${env_host:-${HOST:-}}"
REMOTE_DIR="${env_dir:-${REMOTE_DIR:-/opt/submix}}"
WAIT_SECONDS="${env_wait:-${WAIT_SECONDS:-20}}"

if [[ -z "$HOST" ]]; then
	echo "HOST is not set: export HOST=user@panel-host or put it in deploy/deploy.env" >&2
	exit 2
fi

LOCAL_FILE="${1:-$SCRIPT_DIR/../deploy/extras.yaml}"
if [[ ! -f "$LOCAL_FILE" ]]; then
	echo "no such file: $LOCAL_FILE" >&2
	exit 2
fi

echo "==> $LOCAL_FILE -> $HOST:$REMOTE_DIR/extras.yaml"
scp -q "$LOCAL_FILE" "$HOST:$REMOTE_DIR/extras.yaml.new"

# Everything else happens on the server in a single session. Local variables
# are interpolated here, hence the unquoted heredoc; server-side $-variables
# are escaped as \$.
ssh "$HOST" bash -s <<REMOTE
set -euo pipefail
cd "$REMOTE_DIR"

# The container reads the file as uid 10001: the owner stays the deploy user
# (so it can be edited by hand), group 10001, mode 640. cp -p cannot preserve
# group 10001 (the deploy user is not a member), so permissions are reapplied
# after every file swap, including the rollback.
fix_perms() {
	sudo -n chgrp 10001 extras.yaml
	chmod 640 extras.yaml
}

if [[ -f extras.yaml ]]; then
	cp -p extras.yaml extras.yaml.bak
fi
mv extras.yaml.new extras.yaml
fix_perms

restart_and_wait() {
	local since
	since=\$(date -u +%Y-%m-%dT%H:%M:%SZ)
	docker compose restart extras-api >/dev/null
	local i
	for i in \$(seq 1 $WAIT_SECONDS); do
		sleep 1
		local log
		log=\$(docker compose logs --no-color --since "\$since" extras-api 2>/dev/null)
		if grep -q '"msg":"fatal"' <<<"\$log"; then
			# err may contain escaped quotes (subscription \"x\"), hence
			# (\\.|[^"\\])* instead of [^"]*.
			echo "FATAL: \$(grep -oE '"err":"(\\\\.|[^"\\\\])*"' <<<"\$log" | tail -1)"
			return 1
		fi
		if grep -q '"msg":"extras-api started"' <<<"\$log"; then
			# Give the fetcher a moment to download the subscriptions, then print
			# the summary.
			sleep 3
			log=\$(docker compose logs --no-color --since "\$since" extras-api 2>/dev/null)
			grep -E '"msg":"(subscription fetched|subscription fetch failed)"' <<<"\$log" \
				| sed -E 's/.*"msg":"([^"]*)".*"subscription":"([^"]*)"(.*"parsed":([0-9]+),"kept":([0-9]+))?.*"err":"([^"]*)".*/  \1: \2 err=\6/; s/.*"msg":"([^"]*)".*"subscription":"([^"]*)".*"parsed":([0-9]+),"kept":([0-9]+).*/  \1: \2 parsed=\3 kept=\4/' \
				|| echo "  no subscriptions (static_entries only or empty config)"
			return 0
		fi
	done
	echo "TIMEOUT: extras-api did not report start-up within $WAIT_SECONDS s"
	return 1
}

echo "==> restarting extras-api"
if restart_and_wait; then
	echo "==> OK: config applied"
	exit 0
fi

echo "==> new config did not come up, rolling back to the previous one" >&2
if [[ -f extras.yaml.bak ]]; then
	cp -p extras.yaml extras.yaml.rejected
	mv extras.yaml.bak extras.yaml
	fix_perms
	if restart_and_wait; then
		echo "==> rolled back; the rejected file is kept as extras.yaml.rejected" >&2
	else
		echo "==> WARNING: the previous config did not come up either, extras-api is down (mixer is fail-open)" >&2
	fi
else
	echo "==> no previous file to roll back to; extras-api is down (mixer is fail-open)" >&2
fi
exit 1
REMOTE

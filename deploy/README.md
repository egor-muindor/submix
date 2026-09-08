# Deploying submix

submix runs next to the Remnawave panel, on the same Docker network as
`remnawave` and `remnawave-subscription-page` (the compose file joins the
external network `remnawave-network`). Paths below assume the compose project
lives in `/opt/submix` on the panel host; adjust to taste.

## Initial installation

The compose file uses the prebuilt multi-arch image
`ghcr.io/egor-muindor/submix:latest` (see the Docker image section of the main
README for tags). Nothing has to be built on the host.

```bash
# --- on the panel host ---
mkdir -p /opt/submix/cache
cd /opt/submix
curl -fsSLO https://raw.githubusercontent.com/egor-muindor/submix/main/deploy/docker-compose.yml

# extras-api runs as uid 10001 inside the container (see Dockerfile). Without
# the chown Store.persist fails with EACCES, the last-good cache never reaches
# disk and Restore finds nothing after a restart.
sudo chown -R 10001:10001 cache

# Panel token. The subscription-page token (REMNAWAVE_API_TOKEN in the panel
# compose file) is sufficient: extras-api reads the user status from
# /api/sub/*/info without a token and the tag from /api/users/by-username/*,
# which that token is allowed to call. If the token lacks by-username access
# the tag is treated as empty and only users.default_tags apply (the log says
# "panel: tag lookup failed").
echo "PANEL_TOKEN=<token>" > .env
chmod 600 .env

# First start with an empty config (pure passthrough), see the next section.
printf 'users:\n  default_tags: []\n' > extras.yaml
sudo chgrp 10001 extras.yaml && chmod 640 extras.yaml   # you edit, the container reads

docker compose pull
docker compose up -d
docker compose ps
docker compose logs -f
```

### Building the image yourself

Build on the host (`git clone https://github.com/egor-muindor/submix.git`,
then `docker build -t ghcr.io/egor-muindor/submix:latest .`; needs
`golang:1.26-alpine` plus build cache, roughly 300 MB), or build elsewhere and
ship the image with `docker save | docker load`:

```bash
# --- on the build machine, in the repository root ---
docker buildx build --platform linux/amd64 -t ghcr.io/egor-muindor/submix:latest --load .
docker save ghcr.io/egor-muindor/submix:latest | gzip -1 | ssh user@panel-host 'gunzip | docker load'
# then on the host: docker compose up -d --no-build --pull never
```

Health checks:

```bash
docker compose exec sub-mixer wget -qO- http://127.0.0.1:3020/healthz
docker compose exec extras-api wget -qO- http://127.0.0.1:3030/healthz
```

Before switching the subscription page over, compare panel and mixer
responses on the panel host with any user's `shortUuid`. The panel returns
nodes in random order, so volatile bodies are compared by size only:

```bash
scripts/compare-passthrough.sh <shortUuid>
# expected: PASSTHROUGH OK: all responses match
```

## Wiring into the subscription chain

In the panel's `docker-compose.yml`, service `remnawave-subscription-page`,
replace (back the file up first, e.g.
`sudo cp -p docker-compose.yml docker-compose.yml.bak-$(date +%Y%m%d)`):

```yaml
          - REMNAWAVE_PANEL_URL=http://remnawave:3000
```

with:

```yaml
          - REMNAWAVE_PANEL_URL=http://sub-mixer:3020
```

and apply:

```bash
docker compose up -d remnawave-subscription-page
```

## Rollback

Restore `REMNAWAVE_PANEL_URL=http://remnawave:3000` and run
`docker compose up -d remnawave-subscription-page`. submix itself may keep
running.

## Updating the external subscriptions config

The usual path is `scripts/deploy-extras-config.sh`, run from the repository
root on your workstation. It uploads the file, sets permissions (group 10001,
mode 640), restarts extras-api and waits for `extras-api started`. If the new
config fails validation and the service dies, the script restores the previous
file, restarts again and exits with code 1 (the rejected file stays on the
server as `extras.yaml.rejected`). The mixer serves plain panel responses the
whole time.

The target host is taken from the `HOST` environment variable or from
`deploy/deploy.env` (git-ignored; see `deploy/deploy.example.env`):

```bash
cp deploy/deploy.example.env deploy/deploy.env   # set HOST and REMOTE_DIR
./scripts/deploy-extras-config.sh                # deploy/extras.yaml
./scripts/deploy-extras-config.sh path/to/extras.yaml
```

The script ends with `parsed=N kept=M` per subscription. `kept=0` means no
rule matched; use extras-ui (see the main README) to tune the rules.

The same by hand on the server:

```bash
cd /opt/submix
vim extras.yaml            # owned by you, group 10001, mode 640
docker compose restart extras-api
docker compose logs --since 1m extras-api | grep "subscription fetched"
```

There is deliberately no hot reload: a restart takes about a second and the
mixer is fail-open meanwhile.

`deploy/extras.yaml`, `deploy/.env` and `deploy/deploy.env` are git-ignored:
subscription URLs with tokens, the panel token and your host names never
enter the repository.

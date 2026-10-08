# Server deployment

The server is one static binary, one SQLite file and one secret (the master key). This page shows how to run it as a system service or a container, put TLS in front of it, and keep it backed up.

## Checklist

- [ ] Master key generated with `goran-server keygen` and stored in a secret manager, separately from the database.
- [ ] Database on a persistent disk in a directory owned by a dedicated user.
- [ ] Server bound to localhost or a private interface, with a TLS-terminating reverse proxy in front.
- [ ] A backup of the database file taken with SQLite's backup API, plus the master key.
- [ ] `bootstrap` run once; the printed token stored safely and the `bootstrap` token deleted once personal tokens exist.

## systemd

```ini title="/etc/goran/server.env"
GORAN_ADDR=127.0.0.1:8080
GORAN_DB=/var/lib/goran/goran.sqlite
GORAN_MASTER_KEY=…
GORAN_LEASE_SECONDS=60
```

```ini title="/etc/systemd/system/goran-server.service"
[Unit]
Description=Goran control plane
After=network-online.target
Wants=network-online.target

[Service]
User=goran
Group=goran
EnvironmentFile=/etc/goran/server.env
ExecStart=/usr/local/bin/goran-server serve
Restart=always
RestartSec=2
StateDirectory=goran
WorkingDirectory=/var/lib/goran
NoNewPrivileges=yes
PrivateTmp=yes
ProtectSystem=strict
ProtectHome=yes
ReadWritePaths=/var/lib/goran

[Install]
WantedBy=multi-user.target
```

```sh
useradd --system --home /var/lib/goran --shell /usr/sbin/nologin goran
install -m 0755 bin/goran-server /usr/local/bin/
install -d -m 0750 -o goran -g goran /var/lib/goran /etc/goran
chmod 0640 /etc/goran/server.env && chown root:goran /etc/goran/server.env
systemctl enable --now goran-server
sudo -u goran GORAN_DB=/var/lib/goran/goran.sqlite goran-server bootstrap --user admin --workspace client-a
```

The server stops cleanly on `SIGTERM` (five second grace period), so plain `systemctl restart` is safe: agents retry their calls and leases are long enough to cover a restart.

## Docker

The image built from `build/Dockerfile.server` runs as user `goran`, stores the database in the `/data` volume (`GORAN_DB=/data/goran.sqlite`) and listens on `8080`. Its entrypoint is `goran-server`, so the command is the subcommand.

```sh
docker build -f build/Dockerfile.server -t goran-server .
docker volume create goran-data

docker run -d --name goran-server --restart unless-stopped \
  -p 127.0.0.1:8080:8080 \
  -v goran-data:/data \
  -e GORAN_MASTER_KEY=… \
  goran-server

docker exec goran-server goran-server bootstrap --user admin --workspace client-a
```

Pass the master key through your orchestrator's secret mechanism rather than a plain `-e` on a shared host. All other settings are environment variables as documented in [Configuration](configuration.md).

## Docker Compose with TLS

Caddy obtains and renews certificates automatically and proxies to the server:

```yaml title="docker-compose.yml"
services:
  server:
    image: goran-server
    build:
      context: .
      dockerfile: build/Dockerfile.server
    restart: unless-stopped
    environment:
      GORAN_MASTER_KEY: ${GORAN_MASTER_KEY}
      GORAN_LEASE_SECONDS: "60"
    volumes:
      - goran-data:/data

  caddy:
    image: caddy:2
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile:ro
      - caddy-data:/data

volumes:
  goran-data:
  caddy-data:
```

```text title="Caddyfile"
goran.example.net {
    reverse_proxy server:8080
}
```

Run `docker compose exec server goran-server bootstrap --workspace client-a` once to get the first token.

## TLS with a reverse proxy

Goran speaks plain HTTP. Terminate TLS in front of it; both users and agents then use `https://goran.example.net`. Requirements for the proxy are modest: pass requests through unchanged, allow request bodies of at least 64 KiB (log chunks) and keep timeouts above the agent's 20 to 30 second request timeouts.

=== "Caddy"

    ```text
    goran.example.net {
        reverse_proxy 127.0.0.1:8080
    }
    ```

=== "nginx"

    ```nginx
    server {
        listen 443 ssl http2;
        server_name goran.example.net;
        ssl_certificate     /etc/letsencrypt/live/goran.example.net/fullchain.pem;
        ssl_certificate_key /etc/letsencrypt/live/goran.example.net/privkey.pem;

        client_max_body_size 1m;

        location / {
            proxy_pass         http://127.0.0.1:8080;
            proxy_http_version 1.1;
            proxy_set_header   Host $host;
            proxy_set_header   X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header   X-Forwarded-Proto https;
            proxy_read_timeout 60s;
        }
    }
    ```

Agents verify the certificate with the host's CA store. For private CAs, install the CA certificate on every agent host (or mount it into the agent container at `/etc/ssl/certs/`).

## Backups

Two things make up an installation: the database file and the master key. Either one alone is useless.

Use SQLite's online backup for a consistent copy while the server runs:

```sh
sqlite3 /var/lib/goran/goran.sqlite ".backup '/var/backups/goran/goran-$(date +%F).sqlite'"
```

Copying the file with `cp` is only safe when the server is stopped, because WAL mode keeps recent writes in `goran.sqlite-wal`. Restore by stopping the server, replacing the file, deleting any stale `-wal` and `-shm` files and starting again. Full procedure in [Upgrading and backups](../operations/upgrading.md).

## Monitoring

| Signal | How |
| --- | --- |
| liveness | `GET /healthz` answers `{"ok":true}` without authentication |
| stale agents | `GET /api/workspaces/:ws/agents` and compare `last_seen_at` with now |
| failing work | `GET /api/workspaces/:ws/tasks?status=error` |
| waiting approvals | `GET /api/workspaces/:ws/tasks?status=awaiting_approval` |
| lost leases | `reaper: requeued N task(s)` lines in the server log |

There are no metrics endpoints yet.

## Capacity

A single server comfortably handles tens of agents polling every two seconds and a few concurrent tasks streaming logs. The limiting factors are SQLite's single writer and the lack of log pruning; keep an eye on the database size if tasks produce very large outputs. There is no clustering or hot standby: run one instance and restore from backup if the host fails. Agents reconnect on their own and tasks whose leases expired during the outage are requeued.

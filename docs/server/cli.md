# Server CLI reference

```text
goran-server [command] [flags]

commands:
  serve       start the API server and console (default)
  bootstrap   create the first admin user, a workspace and print a token
  keygen      print a fresh master key for GORAN_MASTER_KEY
  help        print usage
```

When no command is given, or the first argument starts with `-`, the command is `serve`. Flags always override environment variables; environment variables override built-in defaults. Unknown commands print the usage and exit with status `1`; invalid flags exit with status `2`.

## `serve`

Starts the HTTP server, the embedded console and the lease reaper.

| Flag | Environment | Default | Meaning |
| --- | --- | --- | --- |
| `--addr` | `GORAN_ADDR` | `:8080` | listen address (`host:port`); use `127.0.0.1:8080` behind a reverse proxy on the same host |
| `--db` | `GORAN_DB` | `local.sqlite` | SQLite database file; created and migrated on start |
| `--master-key` | `GORAN_MASTER_KEY` | required | 64 hexadecimal characters (32 bytes) used to encrypt secrets at rest |
| `--lease-seconds` | `GORAN_LEASE_SECONDS` | `60` | how long an agent may stay silent before its task is requeued |
| `--max-attempts` | `GORAN_MAX_ATTEMPTS` | `3` | default `max_attempts` for new tasks |
| `--task-timeout-seconds` | `GORAN_TASK_TIMEOUT_SECONDS` | `3600` | default `timeout_seconds` for new tasks |
| `--log-sql` | `GORAN_LOG_SQL=1` | off | log every SQL statement to stdout |

```sh
export GORAN_MASTER_KEY=…   # from keygen
goran-server serve --addr 127.0.0.1:8080 --db /var/lib/goran/goran.sqlite
# goran-server listening on 127.0.0.1:8080 (db /var/lib/goran/goran.sqlite, lease 60s)
```

What happens on start:

1. The master key is parsed. Without one the server refuses to start: `GORAN_MASTER_KEY is required so secrets can be encrypted at rest; run goran-server keygen to create one`.
2. The database is opened in WAL mode and the schema is migrated (new tables and columns are added; nothing is dropped).
3. The reaper starts, running every half lease.
4. The listener starts. Requests are logged to stdout, one line each.

`SIGINT` or `SIGTERM` stops accepting connections, waits up to five seconds for in-flight requests and exits `0` with `goran-server stopped`.

## `bootstrap`

Creates the first global admin, a workspace, the admin's membership in it and prints a user token. It is the only built-in way to obtain the first credential; there is no default password or master token.

| Flag | Environment | Default | Meaning |
| --- | --- | --- | --- |
| `--db` | `GORAN_DB` | `local.sqlite` | database file to bootstrap |
| `--user` | | `admin` | admin user name |
| `--email` | | empty | admin email |
| `--workspace` | | `default` | first workspace name |

```sh
goran-server bootstrap --db /var/lib/goran/goran.sqlite --user admin --email ops@example.com --workspace client-a
```

```text
user:      admin (admin)
workspace: client-a
token:     gu_…

export GORAN_TOKEN=gu_…
```

The command is idempotent and safe to repeat:

- an existing user with that name is reused and promoted to global admin if it was not one,
- an existing workspace is reused,
- the membership is created only if missing,
- a **new** token named `bootstrap` is issued every time. Delete old ones through `DELETE /api/tokens/:id` if you no longer need them.

It can run while the server is up; both use the same file with SQLite's busy timeout. Names must match `^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$` and must not be purely numeric. The master key is not needed for bootstrap.

## `keygen`

Prints a new random master key in the form the server expects:

```sh
goran-server keygen
# export GORAN_MASTER_KEY=9c0e…(64 hex characters)
```

Generate it once, store it in your secret manager, and give it to every `serve` process that uses the same database. See [Security model](../concepts/security.md#secrets-at-rest) for rotation and loss.

## Exit status

| Status | When |
| --- | --- |
| `0` | clean shutdown, or `bootstrap` and `keygen` succeeded |
| `1` | any error; the message is printed to stderr |
| `2` | invalid flags |

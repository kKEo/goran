# Server configuration

The server is configured entirely through flags and environment variables; there is no configuration file. Flags override environment variables, which override the defaults. An environment variable set to the empty string counts as unset.

## Settings

| Environment | Flag | Default | Notes |
| --- | --- | --- | --- |
| `GORAN_ADDR` | `--addr` | `:8080` | Bind to `127.0.0.1:8080` when a reverse proxy on the same host terminates TLS. |
| `GORAN_DB` | `--db` | `local.sqlite` | Path to the SQLite file. The directory must exist and be writable. `:memory:` gives a throwaway database that disappears on exit (tests use it). |
| `GORAN_MASTER_KEY` | `--master-key` | required | 64 hex characters. Changing it makes existing secrets unreadable. |
| `GORAN_LEASE_SECONDS` | `--lease-seconds` | `60` | See [Leases](#leases) below. |
| `GORAN_MAX_ATTEMPTS` | `--max-attempts` | `3` | Default `max_attempts` for new tasks; each task may override it. |
| `GORAN_TASK_TIMEOUT_SECONDS` | `--task-timeout-seconds` | `3600` | Default `timeout_seconds` for new tasks; each task may override it. |
| `GORAN_LOG_SQL` | `--log-sql` | unset | Set to `1` to log every SQL statement. Very verbose; for debugging only. |

Settings are read once at start. Restart the server to apply changes. Lease and defaults apply to tasks created or claimed after the restart; a task's own `timeout_seconds` and `max_attempts` are fixed when it is created.

## Leases

`GORAN_LEASE_SECONDS` is the single knob that controls failure detection:

| Derived value | Formula | Default |
| --- | --- | --- |
| agent heartbeat interval | lease / 3 | 20 s |
| reaper interval | lease / 2 | 30 s |
| time an agent has to pick up an approved apply | lease | 60 s |
| worst case until a dead agent's task is requeued | lease + reaper interval | 90 s |

Lower values give faster failover at the cost of more heartbeat traffic and a higher risk of false positives on slow links: an agent that cannot reach the server for a full lease loses its task even if the task is fine, and the task is then run again elsewhere. Sixty seconds is a sensible default for agents on the public internet; go lower only inside one network. The value is sent to agents with each task, so agents need no matching setting.

## Database

Goran uses one SQLite file through a pure-Go driver:

- **WAL mode** is enabled, so `<db>-wal` and `<db>-shm` files appear next to the database while the server runs. Keep all three together, and read [Upgrading and backups](../operations/upgrading.md#backups) before copying them.
- **Busy timeout** is five seconds. A second process (such as `bootstrap`) can use the file while the server runs.
- **One connection** is used, matching SQLite's single-writer model. Requests queue on it; this is fine for tens of agents.
- **Migrations** run automatically on start and are additive. There is no down migration.

The file inherits the umask of the server process. Keep it in a directory only the server's user can read: it holds token hashes, encrypted secrets and every task log.

## Logging

Everything goes to stdout:

- one access log line per request from Gin (method, path, status, latency, client IP). Agents poll every two seconds, so expect a steady stream of `GET /agent/next` lines per agent; filter them out in your log pipeline if they are noise,
- `reaper: requeued N task(s) with expired leases` whenever a lease expires,
- GORM warnings and queries slower than one second,
- every SQL statement when `GORAN_LOG_SQL=1`.

There is no log file option; use your service manager or container runtime to capture stdout.

## Limits worth knowing

| Limit | Value | Where |
| --- | --- | --- |
| log chunk per agent upload | 64 KiB | `POST /agent/tasks/:id/logs` answers `413` above this |
| tasks returned by a list | 200 newest | `GET …/tasks` |
| candidates examined per claim | 50 oldest `new` tasks | `GET /agent/next` |
| header read timeout | 10 s | HTTP server |
| graceful shutdown | 5 s | on `SIGINT`/`SIGTERM` |

There is no request rate limiting and no cap on total log size; see [Known limitations](../operations/limitations.md).

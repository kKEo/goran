# Troubleshooting

Symptoms are grouped by where you see them. Error texts are quoted exactly as the server, the agent or the console prints them.

## Server does not start

| Message | Cause | Fix |
| --- | --- | --- |
| `GORAN_MASTER_KEY is required so secrets can be encrypted at rest; run goran-server keygen to create one` | no master key | `goran-server keygen` and export the result; use the same key for the same database from then on |
| `master key must be hex: …` | quotes, spaces or a truncated value in the variable | paste the 64 hex characters only |
| `master key must be 32 bytes (64 hex characters), got N bytes` | wrong length | use the key from `keygen` |
| `open database /path/goran.sqlite: …` | directory missing or not writable by the server's user | create it, fix ownership |
| `listen tcp :8080: bind: address already in use` | port taken | change `GORAN_ADDR` |

## Authentication and access

| Response | Cause | Fix |
| --- | --- | --- |
| `401 missing Authorization: Bearer <token>` | no header | send `Authorization: Bearer gu_…` |
| `401 invalid token` | typo, revoked, or a token for a different server | `GET /api/tokens` as that user (or any admin) to compare prefixes; issue a new one |
| `403 admin only` | the route needs a global admin | ask a global admin, or have one set `admin: true` on your user |
| `403 workspace admin only` | your role in this workspace is `member` | ask a workspace admin to run the action or promote you with `POST …/members` |
| `404 workspace not found` although it exists | you are not a member; the server hides foreign workspaces | ask a workspace admin to add you |
| console: `Cannot connect: invalid token` | as `401 invalid token` | |
| console: `You are not a member of any workspace yet` | valid user, no memberships | be added to one, or create one with `POST /api/workspaces` |

## Agent registration and polling

| Symptom | Cause | Fix |
| --- | --- | --- |
| `register: unauthorized: credential rejected by the server` | registration token already used, expired, or minted on another server | mint a new one; default lifetime is 60 minutes, single use |
| `register: server answered 400: token and a valid name are required` | invalid `--name` (bad characters, purely numeric) | pick a name matching `^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$` |
| `poll: unauthorized: credential rejected by the server` every interval | the agent was revoked | stop it; register again with a new token if it should come back |
| `poll: Get "https://…/agent/next": dial tcp …` | network, DNS, proxy | check `HTTPS_PROXY`/`NO_PROXY` and firewall egress from the agent host |
| `poll: Get "…": x509: certificate signed by unknown authority` | private CA | install the CA certificate on the host (or `/etc/ssl/certs/` in the container) |
| agent's `last_seen_at` is old | agent process stopped or cannot reach the server | check the service and its log |

## Tasks that never start

A task that stays `new` has no eligible agent. Check, in order:

1. **Is an agent polling in this workspace?** The **Agents** tab shows `last_seen_at`; it should be a few seconds old. Agents belong to the workspace of their registration token, so an agent registered for `client-a` never sees `client-b` tasks.
2. **Labels.** The agent must carry **every** label the task requires. Compare the task's `labels` with the agent's.
3. **Queue head.** The server looks at the fifty oldest `new` tasks; if more than fifty unroutable tasks sit in front, cancel them.

A task stuck in `pending` was claimed but the agent never sent a heartbeat or log. It is requeued when its lease expires (default 60 seconds plus up to 30 seconds of reaper interval). Look at that agent's log for what went wrong.

A task stuck in `approved` is waiting for the agent that planned it. If that agent is gone, the lease expires, the task returns to `new` and is planned again elsewhere.

## Tasks that fail

| `error` | Cause | Fix |
| --- | --- | --- |
| `lease expired after N attempt(s); the agent stopped reporting` | the agent died, hung or lost connectivity for longer than the lease on every attempt | read the agent's log; raise `GORAN_LEASE_SECONDS` on flaky links; check the host is not overloaded |
| `exec: "terraform": executable file not found in $PATH` | the tool is missing or not on the service's `PATH` (systemd units start with a minimal `PATH`) | install it, set `Environment=PATH=…` in the unit, or give an absolute `binary` in `params` |
| `fork/exec /bin/sh: no such file or directory` (the binary exists) | the working directory does not exist on that agent: a shell `cwd`, or a `source.dir` | fix the path or restrict labels; the message names the binary, not the directory |
| `source dir "/srv/x" is not a directory on this agent` | the `dir` source does not exist where the task landed | fix the path or restrict labels so the task lands on the right host |
| `git clone exited with status 128` | authentication failed, repository or `ref` does not exist, host key unknown | run `sudo -u goran git ls-remote <url>` on the agent host; `ref` must be a branch or tag, not a commit |
| `secret "x" not found` | the secret was deleted after the task was created | recreate it and queue the task again |
| `secret "x" cannot be decrypted (master key changed?)` | the server runs with a different master key than the one the secret was stored under | restore the original key, or re-enter the secret |
| `timed out after 1h0m0s` | the task exceeded `timeout_seconds` | raise it on the task, or find out why the run hangs (a held Terraform state lock is a common reason) |
| `agent shut down while the task was running` | the agent was stopped or restarted mid-task | wait for running tasks before restarting agents; queue again |
| `no saved plan in …; re-run the task` | the agent lost its work directory between plan and apply (restart without a persistent volume, cleanup, different `--workdir`) | queue the task again |
| `plan file … is missing; re-run the task` | same as above, partial | queue the task again |
| `terraform plan exited 0 but wrote no plan file at …` | `binary` is not Terraform compatible | fix `binary` |
| `exit status N` | the command failed | the log has the details |
| `terraform params: json: cannot unmarshal number into Go struct field … of type string` | a value in `vars` is not a string | quote the value |

A task that failed is not retried automatically. Attempts only count lost leases.

## Logs

| Symptom | Cause | Fix |
| --- | --- | --- |
| log is empty while the task runs | output is shipped every second or 8 KiB; some tools buffer when not on a terminal | wait a moment; for Python use `PYTHONUNBUFFERED=1` in the agent's environment |
| `[goran] log truncated: server unreachable for too long` in the log | the agent could not upload for long enough to buffer more than 1 MiB | investigate the outage; the task itself was not affected |
| agent log: `could not ship the last log chunk: server answered 413: chunk exceeds 65536 bytes` | an outage left more than 64 KiB pending in one upload, which the server refuses | the task's result is still reported; the missing output is lost. Rerun with `keep_workdir` if you need it |
| agent log: `task N: heartbeat: …` repeated | transient connectivity problems | if they persist for a whole lease the task is requeued |
| agent log: `task N: lease lost, stopping` | the server requeued the task (lease expired) while the agent was still working | the task runs again elsewhere; raise the lease if this is routine |

## State conflicts

| Response | Meaning |
| --- | --- |
| `409 task is pending, expected awaiting_approval` | someone approved first, or the task moved on; reload |
| `409 task changed state concurrently, reload and retry` | the task changed between your read and your write |
| `409 only new or awaiting_approval tasks can be canceled (task is running)` | running tasks cannot be canceled in this release |

## Database

| Symptom | Cause | Fix |
| --- | --- | --- |
| `database is locked` in the server log | another process (an `sqlite3` shell with an open transaction, a copy in progress) held the write lock for more than five seconds | close it; use `.backup` for copies |
| `goran.sqlite-wal` keeps growing | normal in WAL mode with constant writes; it is checkpointed automatically | nothing, unless the disk is short; a restart checkpoints it |
| disk full | logs are never pruned | free space; see [Known limitations](limitations.md) |

## Getting more detail

- `GORAN_LOG_SQL=1` logs every statement the server runs.
- The server's access log shows every request with status and latency; look for `409` and `401` from agents.
- `"keep_workdir": true` in a task's `params` keeps the task directory on the agent for inspection.
- `GET …/tasks/:id` returns `attempts`, `agent_id`, `lease_expires_at` and `error`, which together explain most stuck tasks.

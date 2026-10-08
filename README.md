# Goran

Goran is a small, self-hosted job runner built for one job: **run Terraform,
Ansible and shell tasks inside a client's network from a central control
plane you own.** Agents dial out to the server and poll for work, so they run
behind NAT and firewalls without any inbound port. Every client lives in its
own workspace with its own agents and encrypted secrets, and Terraform runs
stop for a human approval between `plan` and `apply`.

It is the engine that products like Spacelift private workers or Pulumi
customer-managed runners sell on enterprise tiers, as two static Go binaries
and one SQLite file.

```
┌────────────┐  Bearer gu_…   ┌─────────────────────┐  Bearer ga_…  ┌─────────────┐
│ console /  │ ─────────────▶ │ goran-server        │ ◀──────────── │ goran-agent │
│ curl / CI  │                │ workspaces, tasks,  │   GET /next   │ client A's  │
└────────────┘                │ secrets, approvals  │   POST logs   │ network     │
                              │ SQLite + AES-GCM    │   POST result └─────────────┘
                              └─────────────────────┘
```

## Quick start

```sh
make build                                  # bin/goran-server, bin/goran-agent

# 1. one-time server setup
export $(bin/goran-server keygen)            # GORAN_MASTER_KEY=… (keep it; it encrypts secrets at rest)
bin/goran-server bootstrap --user admin --workspace client-a
#   prints: token: gu_…
export GORAN_TOKEN=gu_…

# 2. run the server (console at http://localhost:8080)
bin/goran-server serve

# 3. register an agent on a machine inside the client's network
curl -s -X POST -H "Authorization: Bearer $GORAN_TOKEN" \
     localhost:8080/api/workspaces/client-a/registration-tokens | jq -r .token
#   on the agent host:
bin/goran-agent register --server http://your-server:8080 --token gr_… --name runner-1 --labels client-a,eu
bin/goran-agent run

# 4. store a secret and queue work
curl -s -X PUT -H "Authorization: Bearer $GORAN_TOKEN" -H 'Content-Type: application/json' \
     localhost:8080/api/workspaces/client-a/secrets/linode-token -d '{"value":"…"}'

curl -s -X POST -H "Authorization: Bearer $GORAN_TOKEN" -H 'Content-Type: application/json' \
     localhost:8080/api/workspaces/client-a/tasks -d '{
       "name": "provision vpc",
       "kind": "terraform",
       "params": {"source": {"git": "https://github.com/org/infra.git", "ref": "main", "path": "envs/client-a"},
                  "vars": {"region": "eu-central"}},
       "secrets": {"LINODE_TOKEN": "linode-token"},
       "labels": ["client-a"]
     }'
```

The agent clones the repo, runs `terraform init` and `terraform plan`, streams
the output to the server and parks the task as `awaiting_approval`. Open the
console, read the plan, click **Approve & apply**, and the same agent applies
the saved plan. Pass `"auto_approve": true` to skip the stop.

## Task kinds

| kind        | params                                                                                                                      |
|-------------|-----------------------------------------------------------------------------------------------------------------------------|
| `shell`     | `script` (required), `shell` (default `/bin/sh`), `cwd`                                                                     |
| `terraform` | `source` (required), `vars` (sent as `TF_VAR_*`, never logged), `var_files`, `init_args`, `auto_approve`, `destroy`, `binary` (e.g. `tofu`) |
| `ansible`   | `source` (required), `playbook` (required), `inventory`, `extra_vars` (written to a file, never logged), `args`, `binary`   |

`source` is either `{"git": "<url>", "ref": "<branch or tag>", "path": "<subdir>"}`
or `{"dir": "<path already on the agent host>"}`.

Common fields on every task: `secrets` (`{"ENV_NAME": "secret-name"}`, injected
as environment variables only on the agent that holds the task), `labels`
(the agent must carry every listed label), `timeout_seconds`, `max_attempts`,
`keep_workdir` inside `params` to keep the task directory for debugging.

## How a task moves

```
new ──claim──▶ pending ──heartbeat/log──▶ running ──result──▶ done | error
                                             │
                                             └─▶ awaiting_approval ──approve──▶ approved ──claim (same agent)──▶ pending ──▶ running ──▶ done | error
                                                          └──reject──▶ rejected
new | awaiting_approval ──cancel──▶ canceled
```

A task in `pending`, `running` or `approved` holds a **lease** (default 60 s,
`GORAN_LEASE_SECONDS`). Log chunks and heartbeats extend it. When an agent dies
the lease expires and the task returns to `new` (and is planned again) until
`max_attempts` (default 3) is used up, after which it ends in `error`.

## Security model

- **No master token.** The only credential is what `bootstrap` prints. All user
  tokens (`gu_`), agent keys (`ga_`) and registration tokens (`gr_`) are stored as
  SHA-256 hashes and shown exactly once.
- **Secrets are encrypted at rest** with AES-256-GCM under `GORAN_MASTER_KEY`.
  No `/api` endpoint ever returns a value; values are decrypted into the task
  payload only for the agent that just claimed it.
- **Workspaces are the tenancy boundary.** Agents only see their workspace's
  queue; users only see workspaces they are members of; global admins see all.
  Workspace `admin`s manage members, secrets, agents and approvals; `member`s
  queue and watch tasks.
- **Agents are identified individually** and registered with single-use,
  expiring tokens. Revoking an agent invalidates its key immediately.

Put the server behind TLS (a reverse proxy is fine) before exposing it: tokens
travel as bearer headers.

## Configuration

Server (`goran-server serve`, flags override environment):

| variable                     | default        | meaning                                   |
|------------------------------|----------------|-------------------------------------------|
| `GORAN_ADDR`                 | `:8080`        | listen address                            |
| `GORAN_DB`                   | `local.sqlite` | SQLite file                               |
| `GORAN_MASTER_KEY`           | required       | 64 hex chars from `goran-server keygen`   |
| `GORAN_LEASE_SECONDS`        | `60`           | silence tolerated before a task is requeued |
| `GORAN_MAX_ATTEMPTS`         | `3`            | default per task                          |
| `GORAN_TASK_TIMEOUT_SECONDS` | `3600`         | default per task                          |

Agent (`goran-agent run`; `register` writes `agent.json` with the key):

| variable               | default      | meaning                               |
|------------------------|--------------|---------------------------------------|
| `GORAN_AGENT_CONFIG`   | `agent.json` | file written by `register`            |
| `GORAN_SERVER`         |              | server URL (overrides the file)       |
| `GORAN_AGENT_KEY`      |              | agent key (overrides the file)        |
| `GORAN_WORKDIR`        | `work`       | checkouts and saved plans             |
| `GORAN_POLL_SECONDS`   | `2`          | poll interval                         |

## API

All `/api` routes take `Authorization: Bearer <user token>`; `/agent` routes
take the agent key. Bodies are JSON.

```
GET    /api/me
POST   /api/users                                   admin
POST   /api/users/:name/tokens                      admin
GET    /api/tokens            POST /api/tokens       DELETE /api/tokens/:id
GET    /api/workspaces        POST /api/workspaces
GET    /api/workspaces/:ws    GET  …/members        POST …/members           {user, role}
GET    …/secrets              PUT  …/secrets/:name  DELETE …/secrets/:name   {value}
POST   …/registration-tokens  GET  …/agents         DELETE …/agents/:id
GET    …/tasks[?status=a,b]   POST …/tasks          GET …/tasks/:id          GET …/tasks/:id/log
POST   …/tasks/:id/approve    POST …/tasks/:id/reject   POST …/tasks/:id/cancel

POST   /agent/register                              {token, name, labels}
GET    /agent/next                                  200 task | 204 empty
POST   /agent/tasks/:id/heartbeat
POST   /agent/tasks/:id/logs                        {chunk}
POST   /agent/tasks/:id/result                      {status, exit_code, error}
```

## Development

```sh
make test       # unit tests, API tests and an in-process end-to-end run
make vet fmt
make docker     # build/Dockerfile.server and build/Dockerfile.agent
```

The agent image ships git, bash, Terraform and Ansible; pass
`--build-arg INSTALL_ANSIBLE=0` for a smaller image. The server is a single
static binary with its SQLite driver compiled in (no CGO).

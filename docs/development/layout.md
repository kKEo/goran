# Repository layout

```text
.
├── webapp/                 goran-server
│   ├── main.go             CLI: serve, bootstrap, keygen
│   ├── api/                HTTP handlers, one file per resource
│   │   ├── server.go       Config, router, reaper
│   │   ├── tasks.go        create/list/get/log/approve/reject/cancel
│   │   ├── agent_tasks.go  claim, leases, logs, results (the agent side)
│   │   ├── agent.go        registration tokens and agents
│   │   ├── secret.go       encrypted secrets
│   │   ├── workspace.go    workspaces and members
│   │   ├── user.go, token.go
│   │   ├── bootstrap.go    first admin, workspace and token
│   │   ├── helpers.go      validation and error helpers
│   │   └── api_test.go     endpoint tests with an in-memory database
│   ├── middleware/auth.go  user tokens, agent keys, membership, roles
│   ├── model/              GORM models and custom column types
│   ├── db/db.go            SQLite open and migrate
│   ├── util/               token generation and hashing, AES-GCM box
│   └── ui/                 index.html embedded into the binary
├── agent/                  goran-agent
│   ├── main.go             CLI: register, run
│   ├── config/             agent.json, environment, precedence
│   ├── client/             typed HTTP client, ErrLeaseLost, ErrUnauthorized
│   ├── worker/             the loop: claim, run, stream, heartbeat, report
│   ├── runner/             shell.go, terraform.go, ansible.go, source.go
│   ├── process/            one child process, process group, interleaved output
│   └── logsink/            buffered, retrying log uploads
├── wire/wire.go            JSON types shared by server and agent
├── integration/            end-to-end tests (real worker against a real server)
├── build/                  Dockerfile.server, Dockerfile.agent
├── docs/, mkdocs.yml       this documentation
├── .github/workflows/      go.yml (build, test, images), docs.yml (GitHub Pages)
├── Makefile
└── README.md
```

## Dependencies

The module keeps its dependency list short on purpose:

| Dependency | Used for |
| --- | --- |
| `github.com/gin-gonic/gin` | HTTP routing and JSON binding |
| `gorm.io/gorm` | models, migrations, queries |
| `github.com/glebarez/sqlite` | pure-Go SQLite driver, so the binaries build with `CGO_ENABLED=0` |
| `github.com/stretchr/testify` | assertions in tests |

The agent uses only the standard library plus `wire`.

## Where things are decided

| Behaviour | File |
| --- | --- |
| which routes exist and which role each needs | `webapp/api/server.go` |
| how a task is claimed and requeued | `webapp/api/agent_tasks.go` |
| task validation at creation | `webapp/api/tasks.go` (`validateParams`) |
| the status machine | `webapp/model/task.go` |
| name and environment variable rules | `webapp/api/helpers.go` |
| token format and hashing | `webapp/util/tokengen.go` |
| encryption of secrets | `webapp/util/box.go` |
| console behaviour | `webapp/ui/index.html` |
| what the agent runs for each kind | `agent/runner/*.go` |
| timeouts, heartbeats, result mapping | `agent/worker/worker.go` |
| log batching limits | `agent/logsink/sink.go` |
| server and agent CLI flags | `webapp/main.go`, `agent/main.go` |

# Building and testing

## Prerequisites

- Go 1.21 or newer (`go.mod` declares `go 1.21.5`; CI installs the version from `go.mod`, the Dockerfiles build with Go 1.23).
- `make`.
- For end-to-end tests nothing else: they use a fake `terraform` script, in-memory SQLite and an in-process HTTP server.
- Docker, only for `make docker`.

The module path is `github.com/kkEo/g-mk8s`; the repository is `github.com/kKEo/goran`. Clone the repository and work inside it.

## Make targets

| Target | What it does |
| --- | --- |
| `make build` | `bin/goran-server` and `bin/goran-agent`, static (`CGO_ENABLED=0`), trimmed and stripped |
| `make test` | `go test -timeout 300s ./...` |
| `make vet` | `go vet ./...` |
| `make fmt` | `gofmt -w .` |
| `make keygen` | prints a master key (`go run ./webapp keygen`) |
| `make bootstrap` | creates the first admin and workspace (`USER_NAME`, `EMAIL`, `WORKSPACE` variables) |
| `make serve` | runs the server from source; needs `GORAN_MASTER_KEY` |
| `make agent-register TOKEN=gr_… NAME=runner-1 LABELS=local` | registers an agent against `SERVER` (default `http://localhost:8080`) |
| `make agent-run` | runs the agent from source |
| `make docker` | builds `goran-server` and `goran-agent` images |
| `make docs` | builds the documentation site strictly into `site/` |
| `make docs-serve` | serves the documentation with live reload |
| `make clean` | removes `bin/` and `work/` |

A development loop from scratch:

```sh
eval "$(make -s keygen)"                    # GORAN_MASTER_KEY
make bootstrap WORKSPACE=dev                 # prints a gu_ token
export GORAN_TOKEN=gu_…
make serve                                   # terminal 1
curl -s -X POST -H "Authorization: Bearer $GORAN_TOKEN" localhost:8080/api/workspaces/dev/registration-tokens
make agent-register TOKEN=gr_… NAME=dev LABELS=local   # terminal 2
make agent-run
```

Both `serve` and `agent-run` use the current directory for `local.sqlite`, `agent.json` and `work/`; all three are ignored by git.

## Tests

`make test` runs four kinds of tests, all without external services:

| Package | Covers | Notes |
| --- | --- | --- |
| `webapp/api` | every endpoint, roles, tenancy, claiming, leases, approvals, secrets | in-memory SQLite, an injectable fake clock to expire leases without sleeping |
| `webapp/model`, `webapp/util`, `webapp` | column types, token hashing, the AES-GCM box, CLI commands | |
| `agent/config`, `agent/logsink`, `agent/process`, `agent/runner` | config precedence, buffered log shipping, process-group kill on timeout, each runner | the Terraform runner test uses a fake `terraform` shell script that records its arguments |
| `integration` | a real `worker.Worker` against a real `api.Server` over `httptest`: shell task with a secret, failure and timeout, Terraform plan → approve → apply | lease shortened to 5 seconds |

Useful invocations:

```sh
go test ./webapp/api -run TestApprovalFlow -v
go test ./integration -v
go test -count=1 ./...          # bypass the test cache
```

Tests never touch `local.sqlite`.

## Continuous integration

Two GitHub Actions workflows live in `.github/workflows/`:

- **`go.yml`** on every push and pull request to `master`: `go vet`, `gofmt -l` (the build fails on unformatted files), `go test`, static builds of the server and the agent for linux/amd64 and the agent for linux/arm64 (uploaded as the `goran-binaries` artifact), then both Docker images.
- **`docs.yml`** on pushes and pull requests that touch `docs/`, `mkdocs.yml` or the workflow itself: a strict MkDocs build, and on `master` a deployment to GitHub Pages. See [Working on the docs](docs.md).

Run `make fmt vet test` before pushing to get the same result locally.

## Docker images

```sh
make docker
docker run --rm goran-server keygen
docker run --rm goran-agent help
```

See [Installation](../getting-started/installation.md#docker-images) for the build arguments and image contents.

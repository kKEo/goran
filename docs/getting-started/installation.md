# Installation

Goran ships as two statically linked binaries:

| Binary | Role | Runs where |
| --- | --- | --- |
| `goran-server` | API, console, queue, secrets, approvals | one machine you control |
| `goran-agent` | polls the server and executes tasks | inside each client's network, one or more per workspace |

Both are built with `CGO_ENABLED=0`, so the SQLite driver is pure Go and the binaries have no runtime dependencies of their own. There are no published releases or container images yet; you build from source, take the artifacts of the CI workflow, or build the Docker images from the repository.

## Build from source

Requirements: Go 1.21 or newer (the CI and Docker builds use Go 1.23) and `make`.

```sh
git clone git@github.com:kKEo/goran.git
cd goran
make build
ls bin/
# goran-agent  goran-server
```

`make build` produces trimmed, stripped binaries for the machine you build on. To build for another platform, cross-compile the way the Makefile does:

```sh
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o goran-agent-linux-amd64 ./agent
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o goran-agent-linux-arm64 ./agent
```

!!! note "Module path"
    The Go module is still named `github.com/kkEo/g-mk8s`, the repository's former name. Clone the repository and build inside it; `go install` by module path does not work.

## CI artifacts

Every push to `master` runs the `Go` workflow, which vets, formats, tests and then uploads an artifact named `goran-binaries` containing:

- `goran-server` (linux/amd64)
- `goran-agent` (linux/amd64)
- `goran-agent-linux-arm64`

Download it from the workflow run page on GitHub.

## Docker images

Two Dockerfiles live in `build/`:

```sh
make docker
# or explicitly
docker build -f build/Dockerfile.server -t goran-server .
docker build -f build/Dockerfile.agent  -t goran-agent  .
```

| Image | Base | Contents |
| --- | --- | --- |
| `goran-server` | `alpine:3.20` | the server binary, CA certificates, tzdata; runs as user `goran` (uid 10001); data in the `/data` volume; listens on `8080` |
| `goran-agent` | `alpine:3.20` | the agent binary plus `git`, `bash`, `openssh-client`, `curl`, `unzip`, Terraform (`TERRAFORM_VERSION`, default `1.9.8`) and Ansible (`INSTALL_ANSIBLE=1`); runs as user `goran`; config at `/home/goran/agent.json`, work directory `/home/goran/work` |

Build arguments for the agent image:

```sh
docker build -f build/Dockerfile.agent --build-arg TERRAFORM_VERSION=1.9.8 --build-arg INSTALL_ANSIBLE=0 -t goran-agent:slim .
```

How to run the images is described in [Server deployment](../server/deployment.md) and [Agent deployment](../agent/deployment.md).

## What the agent host needs

The agent itself needs nothing but the binary. The tasks you send it need their tools on the `PATH` of the agent's user:

| Task kind | Needs |
| --- | --- |
| `shell` | the shell named in the task (default `/bin/sh`) |
| `terraform` | `terraform` (or OpenTofu via `"binary": "tofu"`), and `git` when the source is a git repository |
| `ansible` | `ansible-playbook`, and `git` when the source is a git repository; usually also `ssh` and the keys to reach the managed hosts |

The agent runs tasks as the OS user it was started as. See [Agent deployment](../agent/deployment.md) for the recommended setup.

## Supported platforms

The server and agent are tested on Linux and macOS. The agent compiles on Windows but is not supported there: it cannot kill a task's whole process tree on timeout, and the default shell `/bin/sh` does not exist. See [Known limitations](../operations/limitations.md).

# Shell tasks

A shell task runs one script through a shell on the agent host. It is the escape hatch for anything that is not Terraform or Ansible: health checks, one-off maintenance, calling a CLI with credentials from the secret store, or driving another tool from a repository the script clones itself.

## Params

| Key | Type | Required | Default | Meaning |
| --- | --- | --- | --- | --- |
| `script` | string | yes | | the script text, passed as a single argument to `<shell> -c` |
| `shell` | string | no | `/bin/sh` | interpreter, e.g. `/bin/bash`, `/usr/bin/env python3` is **not** valid (one executable, no arguments) |
| `cwd` | string | no | the task's work directory | directory to run in; must exist on the agent host |
| `keep_workdir` | boolean | no | `false` | keep `<workdir>/<task id>/` after the task ends |

The agent runs exactly:

```sh
<shell> -c "<script>"
```

in `cwd`, with the task environment described in [Task anatomy](index.md#environment-of-a-task). The first line of the log is the script itself, prefixed with `[goran] $`.

## Examples

A multi-line script with strict mode, using a secret:

```json
{
  "name": "rotate backup token",
  "kind": "shell",
  "params": {
    "shell": "/bin/bash",
    "script": "set -euo pipefail\ncurl -fsS -H \"Authorization: Bearer $API_TOKEN\" https://backup.example.net/rotate\necho rotated at $(date -u +%FT%TZ)"
  },
  "secrets": { "API_TOKEN": "backup-api-token" },
  "labels": ["client-a"],
  "timeout_seconds": 120
}
```

A kubectl call with a kubeconfig stored as a secret. Secrets are environment variables, so a script that needs a file writes one itself in the task directory, which is deleted afterwards:

```json
{
  "name": "restart ingress",
  "kind": "shell",
  "params": {
    "script": "umask 077; printf '%s' \"$KUBECONFIG_CONTENT\" > kubeconfig; export KUBECONFIG=$PWD/kubeconfig; kubectl -n ingress rollout restart deploy/ingress-nginx"
  },
  "secrets": { "KUBECONFIG_CONTENT": "client-a-kubeconfig" },
  "labels": ["client-a"]
}
```

Running a script that lives in a repository (shell tasks have no `source`; clone it yourself):

```json
{
  "name": "nightly cleanup",
  "kind": "shell",
  "params": {
    "script": "git clone --depth 1 git@github.com:org/ops.git src && cd src && ./scripts/cleanup.sh"
  },
  "labels": ["client-a"]
}
```

## Behaviour to know

- **Exit code is the result.** `0` ends the task as `done`; anything else as `error` with `exit status N`. With `/bin/sh -c`, the exit code is that of the last command unless you use `set -e`.
- **No pseudo-terminal.** Tools that change behaviour without a TTY (colour, progress bars, prompts) see a pipe. Pass their non-interactive flags.
- **Timeout kills the whole process group**, including background processes the script started, after `timeout_seconds` (default one hour).
- **The script is visible** to every member of the workspace in the task's `params`, and is echoed into the log. Put sensitive values in secrets, not in the script.
- **`cwd` must exist**; otherwise the task fails before starting with `fork/exec /bin/sh: no such file or directory`. The message names the shell, but it is the directory that is missing.
- **Windows** is not supported: the default shell does not exist there and process-group termination is not implemented.

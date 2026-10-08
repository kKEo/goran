# Task anatomy

A task is created with `POST /api/workspaces/:ws/tasks` (or the console's **New task** form) and consists of a few common fields plus kind-specific `params`. This page covers the common parts; [Shell](shell.md), [Terraform](terraform.md) and [Ansible](ansible.md) cover each kind's `params`.

## Request fields

```json
{
  "name": "client-a network",
  "kind": "terraform",
  "params": { "source": { "git": "git@github.com:org/infra.git", "ref": "main", "path": "envs/client-a" } },
  "secrets": { "AWS_ACCESS_KEY_ID": "aws-access-key", "AWS_SECRET_ACCESS_KEY": "aws-secret-key" },
  "labels": ["client-a", "terraform"],
  "timeout_seconds": 1800,
  "max_attempts": 2
}
```

| Field | Type | Required | Default | Rules |
| --- | --- | --- | --- | --- |
| `name` | string | yes | | trimmed; 1 to 128 characters; free text |
| `kind` | string | yes | | `shell`, `terraform` or `ansible` |
| `params` | object | depends on kind | `{}` | validated per kind at creation, see below |
| `secrets` | object | no | `{}` | `{"ENV_NAME": "secret-name"}`; `ENV_NAME` must match `^[A-Z_][A-Z0-9_]*$`; every secret must exist in the workspace |
| `labels` | array of strings | no | `[]` | the agent must carry all of them; trimmed, de-duplicated, sorted |
| `timeout_seconds` | integer | no | server `GORAN_TASK_TIMEOUT_SECONDS` (3600) | values `<= 0` mean "use the default" |
| `max_attempts` | integer | no | server `GORAN_MAX_ATTEMPTS` (3) | how many times the task may be handed out after lost leases; `<= 0` means default |

The server validates the shape of `params` before queueing so an agent never claims something it cannot run:

| Kind | Checked at creation |
| --- | --- |
| `shell` | `script` is a non-empty string |
| `terraform` | `source` is an object with a non-empty `git` or `dir` |
| `ansible` | `source` as above, and `playbook` is a non-empty string |

Anything else inside `params` is passed through untouched and interpreted by the agent. Unknown keys are ignored.

Validation failures answer `400` with a message such as `shell task needs a non-empty "script"`, `secret "aws-key" does not exist in this workspace` or `unknown task kind "bash" (use shell, terraform or ansible)`.

## Secrets

`secrets` maps environment variable names to workspace secret names. When an agent claims the task, the server decrypts those secrets and the agent sets them in the environment of every process the task starts. The secret must exist when the task is created, and again when it is claimed; a secret deleted in between fails the task at claim time with `secret "name" not found`.

Values never appear in the task object, the console or the API. See [Security model](../concepts/security.md#secret-delivery).

## Sources

Terraform and Ansible tasks take their files from a `source` object:

```json
{ "git": "git@github.com:org/infra.git", "ref": "v1.4.0", "path": "envs/client-a" }
```

```json
{ "dir": "/srv/infra", "path": "envs/client-a" }
```

| Key | Meaning |
| --- | --- |
| `git` | repository URL. Cloned with `git clone --depth 1` into `<workdir>/<task id>/src` on the agent |
| `ref` | branch or tag, passed as `--branch`. Commit hashes are not accepted by `git clone --branch`; use a tag |
| `path` | subdirectory inside the repository or `dir` to run in |
| `dir` | an absolute directory that already exists on the agent host. When both `dir` and `git` are set, `dir` wins |

Git authentication is handled by the agent host: an SSH key for the agent's user, or a git credential helper. Do not embed tokens in the URL; the clone command line is echoed into the log. A clone made during the Terraform `plan` phase is reused by the `apply` phase.

With `dir`, the agent runs directly inside that directory. Nothing is copied, so files the task writes (such as a local Terraform state) persist on the host.

## Environment of a task

Every process a task starts inherits the agent's environment, plus:

| Variable | Value |
| --- | --- |
| `GORAN_TASK_ID` | numeric task id |
| `GORAN_TASK_NAME` | the task's `name` |
| `GORAN_TASK_KIND` | `shell`, `terraform` or `ansible` |
| `GORAN_TASK_PHASE` | `run`, `plan` or `apply` |
| the task's `secrets` | one variable per entry; these override anything with the same name |

Kind-specific additions: Terraform tasks get `TF_IN_AUTOMATION=1`, `TF_INPUT=0` and one `TF_VAR_<name>` per entry in `vars`; Ansible tasks get `ANSIBLE_NOCOLOR=1` and `ANSIBLE_FORCE_COLOR=0`.

Per-task plain (non-secret) environment variables are not supported yet. Put static settings into the agent's environment, or use `vars` for Terraform.

## Working directory

The agent creates `<workdir>/<task id>/` for every task (default `work/<id>/` relative to where the agent runs). Shell tasks run in it unless `cwd` is set; git sources are cloned into its `src/` subdirectory; Terraform saves `tfplan` and `goran-plan.json` in it; Ansible writes `extra_vars.json` there.

The directory is removed when the task reaches a result, except:

- when the task parks as `awaiting_approval` (the plan has to survive until apply), or
- when `params` contains `"keep_workdir": true`, which is honoured by every kind and is the easiest way to inspect what a failed task left behind.

## Output and exit codes

Standard output and standard error are captured together, in arrival order, and shipped to the server while the task runs. Lines starting with `[goran]` are written by the agent itself (command lines it runs, notes such as `plan saved to …; waiting for approval`). `GET …/tasks/:id/log` returns the whole log as `text/plain`; the console shows it under the task and refreshes every three seconds while the task is active.

| Outcome | Status | `exit_code` | `error` |
| --- | --- | --- | --- |
| process exited `0` | `done` | `0` | empty |
| process exited non-zero | `error` | the code | `exit status N` |
| process could not start (missing binary, bad `cwd`) | `error` | `null` | the operating system error: `exec: "terraform": executable file not found in $PATH` for a missing binary, `fork/exec /bin/sh: no such file or directory` for a missing directory |
| timeout | `error` | `null` | `timed out after 30m0s` |
| agent stopped while running | `error` | `null` | `agent shut down while the task was running` |
| Terraform plan waiting | `awaiting_approval` | `null` | empty |

A Terraform task that fails during `init` or `plan` ends as `error` with Terraform's exit code; the log has the details.

## Reading a task back

The task object returned by `GET …/tasks/:id` carries `status`, `phase`, `agent_id`, `attempts`, `max_attempts`, `lease_expires_at`, `exit_code`, `error`, `created_by_id`, `approved_by_id`, `started_at` and `finished_at` in addition to what you sent. The full field list is in the [API reference](../api/tasks.md#the-task-object).

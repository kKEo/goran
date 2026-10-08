# Tasks

Tasks live inside a workspace. Any member can create, list, read and cancel them; approving and rejecting Terraform plans needs the workspace admin role. The meaning of the fields and the kind-specific `params` are explained in [Task anatomy](../tasks/index.md); the status machine in [Task lifecycle](../concepts/task-lifecycle.md).

## The task object

```json
{
  "id": 42,
  "created_at": "2026-10-08T17:03:11.482Z",
  "updated_at": "2026-10-08T17:05:40.019Z",
  "workspace_id": 2,
  "name": "client-a network",
  "kind": "terraform",
  "params": { "source": { "git": "git@github.com:org/infra.git", "ref": "main", "path": "envs/client-a" } },
  "secrets": { "AWS_ACCESS_KEY_ID": "aws-access-key", "AWS_SECRET_ACCESS_KEY": "aws-secret-key" },
  "labels": ["client-a", "terraform"],
  "timeout_seconds": 3600,
  "status": "awaiting_approval",
  "phase": "plan",
  "agent_id": 7,
  "lease_expires_at": null,
  "attempts": 1,
  "max_attempts": 3,
  "exit_code": null,
  "error": "",
  "created_by_id": 1,
  "approved_by_id": null,
  "started_at": "2026-10-08T17:03:13.001Z",
  "finished_at": null
}
```

| Field | Type | Meaning |
| --- | --- | --- |
| `id` | integer | task id, unique across workspaces |
| `created_at`, `updated_at` | timestamp | row times; `updated_at` changes on every transition, heartbeat and log upload |
| `workspace_id` | integer | owning workspace |
| `name` | string | as given |
| `kind` | string | `shell`, `terraform`, `ansible` |
| `params` | object | as given (kind-specific) |
| `secrets` | object | environment variable name to secret name; never values |
| `labels` | array | required agent labels, normalised |
| `timeout_seconds` | integer | effective timeout (the default was filled in if you sent none) |
| `status` | string | see [statuses](../concepts/task-lifecycle.md#statuses) |
| `phase` | string | `run`, `plan`, `apply`, or empty while `new` or after a requeue |
| `agent_id` | integer or null | agent that holds or last held the task; cleared on requeue |
| `lease_expires_at` | timestamp or null | set while `pending`, `running` or `approved` |
| `attempts` | integer | how many times the task was claimed from `new` |
| `max_attempts` | integer | effective limit |
| `exit_code` | integer or null | `0` for `done`, the process exit code for a non-zero exit, `null` otherwise |
| `error` | string | failure reason, empty when none |
| `created_by_id` | integer | user who queued it |
| `approved_by_id` | integer or null | user who approved or rejected |
| `started_at` | timestamp or null | first claim |
| `finished_at` | timestamp or null | set on `done`, `error`, `rejected`, `canceled` |

## `POST /api/workspaces/:ws/tasks`

Queues a task. The request fields are described in [Task anatomy](../tasks/index.md#request-fields).

```sh
curl -s -X POST -H "Authorization: Bearer $GORAN_TOKEN" -H 'Content-Type: application/json' \
  $GORAN/api/workspaces/client-a/tasks -d '{
    "name": "hello",
    "kind": "shell",
    "params": {"script": "echo hello"},
    "labels": ["client-a"]
  }'
```

Answers `201` with the task object in status `new`. Validation errors are `400` with one of:

| Message | Fix |
| --- | --- |
| `invalid JSON body: …` | malformed JSON or wrong types |
| `name is required (max 128 characters)` | add a name |
| `unknown task kind "x" (use shell, terraform or ansible)` | fix `kind` |
| `params must be a JSON object: …` | `params` must be an object |
| `shell task needs a non-empty "script"` | add `params.script` |
| `"source" must be an object with "git" (URL) or "dir" (path on the agent)` | add `params.source` |
| `"source" needs "git" (URL) or "dir" (path on the agent)` | fill one of them |
| `ansible task needs "playbook"` | add `params.playbook` |
| `secrets: "x" is not a valid environment variable name` | keys must match `^[A-Z_][A-Z0-9_]*$` |
| `secret "x" does not exist in this workspace` | create it first |

## `GET /api/workspaces/:ws/tasks`

The 200 newest tasks of the workspace, newest first. `status` filters by a comma separated list.

```sh
curl -s -H "Authorization: Bearer $GORAN_TOKEN" "$GORAN/api/workspaces/client-a/tasks?status=running,awaiting_approval"
```

Calling this endpoint (and `GET …/tasks/:id`) also requeues tasks with expired leases, so the statuses you see are current even when the reaper has not run yet.

## `GET /api/workspaces/:ws/tasks/:id`

One task. `404 task not found` when the id is unknown or belongs to another workspace.

## `GET /api/workspaces/:ws/tasks/:id/log`

The task's combined output as `text/plain; charset=utf-8`, in the order it was received. Empty body when nothing has been uploaded yet. The log grows while the task runs; poll it to follow progress.

```sh
curl -s -H "Authorization: Bearer $GORAN_TOKEN" $GORAN/api/workspaces/client-a/tasks/42/log
```

## `POST /api/workspaces/:ws/tasks/:id/approve`

Workspace admin only. Moves an `awaiting_approval` task to `approved`, records `approved_by_id` and starts a lease so the planning agent can pick up the apply phase. Answers `200` with the task. `409 task is <status>, expected awaiting_approval` when the task is in any other state.

## `POST /api/workspaces/:ws/tasks/:id/reject`

Workspace admin only. Moves an `awaiting_approval` task to `rejected` (terminal) and records who rejected it in `approved_by_id`. `200` with the task, or `409` as above.

## `POST /api/workspaces/:ws/tasks/:id/cancel`

Any member. Moves a `new` or `awaiting_approval` task to `canceled`. `200` with the task; `409 only new or awaiting_approval tasks can be canceled (task is running)` otherwise. A task that another agent claimed between your read and your cancel answers `409 task changed state concurrently, reload and retry`.

Running tasks cannot be stopped through the API in this release.

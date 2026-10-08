# Agent protocol

The protocol between an agent and the server is five JSON endpoints. `goran-agent` is the reference implementation; this page specifies the contract so you can debug it, or write an agent of your own (for example in a language with better access to a platform's SDK).

All routes except registration require `Authorization: Bearer ga_…`. The reference client sends `User-Agent: goran-agent`, uses a 30 second request timeout and reads at most 4 MiB of any response.

## `POST /agent/register`

Exchanges a registration token for an agent key. No authentication header; the token is the credential.

```json
{ "token": "gr_e2b7…", "name": "runner-1", "labels": ["client-a", "terraform"] }
```

| Field | Required | Rules |
| --- | --- | --- |
| `token` | yes | a registration token from `POST …/registration-tokens` |
| `name` | yes | `^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`, not purely numeric; not required to be unique |
| `labels` | no | strings; normalised (trimmed, de-duplicated, sorted) |

Response `201`:

```json
{ "agent_id": 7, "workspace_id": 2, "name": "runner-1", "key": "ga_0c41e8a…" }
```

The key is shown once. Errors: `400 token and a valid name are required`, `401 invalid registration token`, `401 registration token already used or expired`. The token is consumed atomically, so two concurrent registrations with the same token yield one agent.

## `GET /agent/next`

Claims the next task for this agent. See [Claiming](../concepts/task-lifecycle.md#claiming) for the selection rules.

- `204 No Content`: nothing to do. Poll again after your interval.
- `200 OK` with the task:

```json
{
  "id": 42,
  "name": "client-a network",
  "kind": "terraform",
  "phase": "plan",
  "params": { "source": { "git": "git@github.com:org/infra.git", "ref": "main", "path": "envs/client-a" } },
  "env": { "AWS_ACCESS_KEY_ID": "AKIA…", "AWS_SECRET_ACCESS_KEY": "…" },
  "timeout_seconds": 3600,
  "lease_seconds": 60,
  "attempt": 1,
  "max_attempts": 3
}
```

| Field | Meaning |
| --- | --- |
| `id` | task id; used in the other endpoints |
| `kind`, `phase` | what to run: `shell`/`run`, `ansible`/`run`, `terraform`/`plan`, `terraform`/`apply` |
| `params` | the task's params, verbatim |
| `env` | decrypted secrets, keyed by environment variable name; absent when the task has none. **This is the only place secret values ever appear.** |
| `timeout_seconds` | the agent must stop the task after this long and report `error` |
| `lease_seconds` | the agent must heartbeat (or upload logs) more often than this |
| `attempt`, `max_attempts` | informational |

On `200` the task is already `pending` and leased to you. Start working at once; the first heartbeat or log upload moves it to `running`.

## `POST /agent/tasks/:id/heartbeat`

Extends the lease. The body is ignored (the reference client sends `{}`).

```json
{ "status": "running", "lease_expires_at": "2026-10-08T17:06:40Z" }
```

`409 task is not assigned to this agent (lease lost or task finished)` means stop working on the task immediately and discard its result.

## `POST /agent/tasks/:id/logs`

Appends output and extends the lease.

```json
{ "chunk": "[goran] $ terraform init -input=false -no-color\nInitializing the backend...\n" }
```

- `chunk` is at most 64 KiB (`413 chunk exceeds 65536 bytes` otherwise). An empty chunk is accepted and acts as a heartbeat.
- Chunks are stored in arrival order and concatenated verbatim; include your own newlines.
- Response and `409` semantics as for heartbeat.

## `POST /agent/tasks/:id/result`

Finishes or parks the task. Must come from the agent that holds the lease while the task is `pending` or `running`.

```json
{ "status": "done", "exit_code": 0 }
```

```json
{ "status": "error", "exit_code": 7, "error": "exit status 7" }
```

```json
{ "status": "awaiting_approval" }
```

| Field | Meaning |
| --- | --- |
| `status` | `done`, `error` or `awaiting_approval`; anything else is `400 status must be done, error or awaiting_approval` |
| `exit_code` | optional integer; stored as given (the reference agent sends `0` for `done` and the process code for non-zero exits) |
| `error` | optional text, shown to users |

Answers `200` with the full task object, or `409` if the lease was lost. `awaiting_approval` is only meaningful for Terraform tasks in the `plan` phase: it clears the lease and waits for an admin; after approval the same agent receives the task again from `GET /agent/next` with `phase: "apply"`.

## Writing your own agent

An agent that honours these rules behaves correctly with the server:

1. Poll `GET /agent/next`; on `204` sleep and retry, on `200` run the task.
2. Inject `env` into the task's environment, and never write those values anywhere else.
3. Send a heartbeat or a log chunk at least every `lease_seconds / 2`; the reference agent uses a third.
4. Treat any `409` as "lease lost": kill the task, discard the result, go back to polling.
5. Enforce `timeout_seconds` yourself; the server does not know whether your process is alive.
6. Report exactly one result. If the result call fails for network reasons, retry; if it answers `409`, stop.
7. For `terraform`, implement both phases: save the plan on `plan`, report `awaiting_approval` (unless the task's `params.auto_approve` is set), and on `apply` apply the saved plan from the same place. The server routes `apply` to the agent key that planned.
8. Treat `401` as revocation: stop and tell someone.

Everything else (work directories, cleanup, how commands are run) is the agent's own business.

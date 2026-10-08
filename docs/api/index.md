# API overview

Everything the console does, and everything the agent does, goes through one JSON API on the server. This section documents every endpoint. Examples use `curl` with two variables:

```sh
export GORAN=https://goran.example.net
export GORAN_TOKEN=gu_…
```

## Conventions

- **Authentication.** `/api/...` routes need a user token, `/agent/...` routes an agent key, both in `Authorization: Bearer <credential>`. The bare credential without `Bearer` is accepted too. `POST /agent/register` and `GET /healthz` need no header.
- **Bodies** are JSON (`Content-Type: application/json`). Responses are JSON except the task log, which is `text/plain`.
- **Workspace routes** are rooted at `/api/workspaces/:ws`, where `:ws` is the workspace name or its numeric id.
- **Identifiers** are integers. **Timestamps** are RFC 3339 strings; `null` means "not yet".
- **No pagination and no versioning** in this release. The task list returns the 200 newest tasks.
- **Snake case** everywhere: `lease_expires_at`, `max_attempts`, `key_prefix`.

## Errors

Every non-2xx response has the same body:

```json
{ "error": "task is running, expected awaiting_approval" }
```

| Status | Meaning |
| --- | --- |
| `400` | invalid JSON, failed validation (`name is required (max 128 characters)`, `shell task needs a non-empty "script"`, …) |
| `401` | missing, invalid or revoked token or key; invalid, used or expired registration token |
| `403` | the caller lacks the role: `admin only` (global admin routes) or `workspace admin only` |
| `404` | the object does not exist, or the workspace is one the caller is not a member of |
| `409` | the task is not in the state the operation expects, or (agent routes) the task is not assigned to this agent |
| `413` | a log chunk is larger than 64 KiB |
| `503` | secrets are disabled because the server has no master key (cannot happen with `goran-server serve`, which requires one) |
| `500` | database error; the message is included |

## Endpoint index

| Method and path | Role | Page |
| --- | --- | --- |
| `GET /healthz` | none | liveness probe, `{"ok":true}` |
| `GET /` | none | the [web console](../server/console.md) |
| `GET /api/me` | user | [Users and tokens](users-and-tokens.md) |
| `GET /api/users` | global admin | [Users and tokens](users-and-tokens.md) |
| `POST /api/users` | global admin | [Users and tokens](users-and-tokens.md) |
| `POST /api/users/:name/tokens` | global admin | [Users and tokens](users-and-tokens.md) |
| `GET /api/tokens` | user | [Users and tokens](users-and-tokens.md) |
| `POST /api/tokens` | user | [Users and tokens](users-and-tokens.md) |
| `DELETE /api/tokens/:id` | owner or global admin | [Users and tokens](users-and-tokens.md) |
| `GET /api/workspaces` | user | [Workspaces and members](workspaces.md) |
| `POST /api/workspaces` | user | [Workspaces and members](workspaces.md) |
| `GET /api/workspaces/:ws` | member | [Workspaces and members](workspaces.md) |
| `GET /api/workspaces/:ws/members` | member | [Workspaces and members](workspaces.md) |
| `POST /api/workspaces/:ws/members` | workspace admin | [Workspaces and members](workspaces.md) |
| `GET /api/workspaces/:ws/secrets` | member | [Secrets](secrets.md) |
| `PUT /api/workspaces/:ws/secrets/:name` | workspace admin | [Secrets](secrets.md) |
| `DELETE /api/workspaces/:ws/secrets/:name` | workspace admin | [Secrets](secrets.md) |
| `POST /api/workspaces/:ws/registration-tokens` | workspace admin | [Agents and registration](agents.md) |
| `GET /api/workspaces/:ws/agents` | member | [Agents and registration](agents.md) |
| `DELETE /api/workspaces/:ws/agents/:id` | workspace admin | [Agents and registration](agents.md) |
| `GET /api/workspaces/:ws/tasks` | member | [Tasks](tasks.md) |
| `POST /api/workspaces/:ws/tasks` | member | [Tasks](tasks.md) |
| `GET /api/workspaces/:ws/tasks/:id` | member | [Tasks](tasks.md) |
| `GET /api/workspaces/:ws/tasks/:id/log` | member | [Tasks](tasks.md) |
| `POST /api/workspaces/:ws/tasks/:id/approve` | workspace admin | [Tasks](tasks.md) |
| `POST /api/workspaces/:ws/tasks/:id/reject` | workspace admin | [Tasks](tasks.md) |
| `POST /api/workspaces/:ws/tasks/:id/cancel` | member | [Tasks](tasks.md) |
| `POST /agent/register` | registration token | [Agent protocol](agent-protocol.md) |
| `GET /agent/next` | agent | [Agent protocol](agent-protocol.md) |
| `POST /agent/tasks/:id/heartbeat` | agent | [Agent protocol](agent-protocol.md) |
| `POST /agent/tasks/:id/logs` | agent | [Agent protocol](agent-protocol.md) |
| `POST /agent/tasks/:id/result` | agent | [Agent protocol](agent-protocol.md) |

"Member" means any role in the workspace; global admins pass every check.

## A complete session in curl

```sh
# who am I, which workspaces
curl -s -H "Authorization: Bearer $GORAN_TOKEN" $GORAN/api/me
curl -s -H "Authorization: Bearer $GORAN_TOKEN" $GORAN/api/workspaces

# store a secret, queue a task, follow it
curl -s -X PUT -H "Authorization: Bearer $GORAN_TOKEN" -H 'Content-Type: application/json' \
  $GORAN/api/workspaces/client-a/secrets/aws-secret-key -d '{"value":"…"}'
curl -s -X POST -H "Authorization: Bearer $GORAN_TOKEN" -H 'Content-Type: application/json' \
  $GORAN/api/workspaces/client-a/tasks -d @task.json
curl -s -H "Authorization: Bearer $GORAN_TOKEN" "$GORAN/api/workspaces/client-a/tasks?status=awaiting_approval"
curl -s -H "Authorization: Bearer $GORAN_TOKEN" $GORAN/api/workspaces/client-a/tasks/42/log
curl -s -X POST -H "Authorization: Bearer $GORAN_TOKEN" $GORAN/api/workspaces/client-a/tasks/42/approve
```

Waiting for a task from a script: poll `GET …/tasks/:id` until `status` is one of `done`, `error`, `rejected`, `canceled`, then read `exit_code`, `error` and the log. There are no webhooks yet.

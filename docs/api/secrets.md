# Secrets

Secrets are workspace-scoped named values, encrypted at rest and injected into tasks as environment variables. The API never returns a value. See [Security model](../concepts/security.md#secrets-at-rest) for how they are protected.

## `GET /api/workspaces/:ws/secrets`

Names and update times, sorted by name.

```sh
curl -s -H "Authorization: Bearer $GORAN_TOKEN" $GORAN/api/workspaces/client-a/secrets
```

```json
[
  { "name": "aws-access-key", "updated_at": "2026-10-08T16:40:12Z" },
  { "name": "aws-secret-key", "updated_at": "2026-10-08T16:40:19Z" }
]
```

## `PUT /api/workspaces/:ws/secrets/:name`

Workspace admin only. Creates the secret or replaces its value.

```sh
curl -s -X PUT -H "Authorization: Bearer $GORAN_TOKEN" -H 'Content-Type: application/json' \
  $GORAN/api/workspaces/client-a/secrets/aws-secret-key -d '{"value":"wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"}'
```

| Part | Rules |
| --- | --- |
| `:name` | `^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`, not purely numeric; unique within the workspace |
| `value` | non-empty string; multi-line values (PEM keys, kubeconfigs) are fine as JSON strings |

Answers `201` when created and `200` when updated, both with `{"name": …, "updated_at": …}`. Errors: `400` for an invalid name or empty value, `503` when the server has no master key.

Updating a value affects tasks claimed after the update. A task already running keeps the value it received.

## `DELETE /api/workspaces/:ws/secrets/:name`

Workspace admin only. `204` on success, `404 secret not found`. Queued tasks that reference the secret fail at claim time with `secret "name" not found`.

## Using secrets in tasks

Reference secrets by name in the task's `secrets` map; the key is the environment variable the agent will set:

```json
"secrets": { "AWS_ACCESS_KEY_ID": "aws-access-key", "AWS_SECRET_ACCESS_KEY": "aws-secret-key" }
```

Variable names must match `^[A-Z_][A-Z0-9_]*$`. The referenced secrets must exist when the task is created (`400 secret "x" does not exist in this workspace` otherwise). Details in [Task anatomy](../tasks/index.md#secrets).

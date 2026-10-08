# Workspaces and members

## Objects

**Workspace**

```json
{ "id": 2, "created_at": "…", "updated_at": "…", "name": "client-a" }
```

**Member** (as listed by `GET …/members`)

```json
{ "user_id": 2, "name": "alice", "email": "alice@example.com", "role": "member" }
```

## `GET /api/workspaces`

Workspaces the caller is a member of, oldest first. Global admins get every workspace.

```sh
curl -s -H "Authorization: Bearer $GORAN_TOKEN" $GORAN/api/workspaces
```

## `POST /api/workspaces`

Any user may create a workspace and becomes its first `admin`.

```sh
curl -s -X POST -H "Authorization: Bearer $GORAN_TOKEN" -H 'Content-Type: application/json' \
  $GORAN/api/workspaces -d '{"name":"client-b"}'
```

| Field | Rules |
| --- | --- |
| `name` | `^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`, not purely numeric, unique |

Answers `201` with the workspace, `400` for an invalid name, `409 workspace already exists`.

## `GET /api/workspaces/:ws`

The workspace and the caller's membership in it.

```json
{
  "workspace": { "id": 2, "name": "client-a", "created_at": "…", "updated_at": "…" },
  "membership": { "id": 5, "workspace_id": 2, "user_id": 1, "role": "admin", "created_at": "…", "updated_at": "…" }
}
```

A global admin who is not an explicit member receives a synthetic membership with `id: 0` and `role: admin`. Non-members get `404 workspace not found`.

## `GET /api/workspaces/:ws/members`

Members with their user name, email and role, in the order they were added. Global admins who were never added do not appear.

## `POST /api/workspaces/:ws/members`

Workspace admin only. Adds a user to the workspace or changes an existing member's role.

```sh
curl -s -X POST -H "Authorization: Bearer $GORAN_TOKEN" -H 'Content-Type: application/json' \
  $GORAN/api/workspaces/client-a/members -d '{"user":"alice","role":"admin"}'
```

| Field | Required | Values |
| --- | --- | --- |
| `user` | yes | an existing user name |
| `role` | no | `member` (default) or `admin` |

Answers `200` with the membership object, `400 role must be admin or member`, `404 user not found`.

There is no endpoint to remove a member in this release. To cut someone's access, a global admin deletes their tokens (`DELETE /api/tokens/:id`); see [Known limitations](../operations/limitations.md).

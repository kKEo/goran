# Users and tokens

## Objects

**User**

```json
{ "id": 1, "created_at": "…", "updated_at": "…", "name": "admin", "email": "ops@example.com", "admin": true }
```

**Token** (the secret part is never included; `prefix` is the first ten characters)

```json
{ "id": 3, "created_at": "…", "updated_at": "…", "name": "laptop", "user_id": 1, "prefix": "gu_7d1a9c0", "last_used_at": "…" }
```

**Membership**

```json
{ "id": 1, "created_at": "…", "updated_at": "…", "workspace_id": 1, "user_id": 1, "role": "admin" }
```

## `GET /api/me`

The caller's user and all of their memberships.

```sh
curl -s -H "Authorization: Bearer $GORAN_TOKEN" $GORAN/api/me
```

```json
{
  "user": { "id": 1, "name": "admin", "email": "ops@example.com", "admin": true, "created_at": "…", "updated_at": "…" },
  "memberships": [ { "id": 1, "workspace_id": 1, "user_id": 1, "role": "admin", "created_at": "…", "updated_at": "…" } ]
}
```

## `GET /api/users`

Global admin only. All users, oldest first.

## `POST /api/users`

Global admin only. Creates a user. Users have no password; give them a token with the next endpoint, or let them create one once they have any token.

```sh
curl -s -X POST -H "Authorization: Bearer $GORAN_TOKEN" -H 'Content-Type: application/json' \
  $GORAN/api/users -d '{"name":"alice","email":"alice@example.com","admin":false}'
```

| Field | Required | Rules |
| --- | --- | --- |
| `name` | yes | `^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`, not purely numeric, unique |
| `email` | no | free text |
| `admin` | no | `true` makes a global admin |

Answers `201` with the user, `400` for an invalid name, `409 user already exists`.

## `POST /api/users/:name/tokens`

Global admin only. Issues a token for another user. The body is optional; `name` labels the token (default `token`).

```sh
curl -s -X POST -H "Authorization: Bearer $GORAN_TOKEN" -H 'Content-Type: application/json' \
  $GORAN/api/users/alice/tokens -d '{"name":"first login"}'
```

```json
{
  "id": 4, "name": "first login", "prefix": "gu_51e0b3c", "user_id": 2,
  "token": "gu_51e0b3c…",
  "note": "store this token now; it is not shown again"
}
```

`404 user not found` for unknown names. Hand the `token` value to the user over a safe channel.

## `GET /api/tokens`

The caller's own tokens, without secret parts.

## `POST /api/tokens`

Issues a new token for the caller. Body optional, `{"name": "ci"}` labels it. Answers `201` with the same shape as above, including the plaintext `token` once.

## `DELETE /api/tokens/:id`

Revokes a token. Users may delete their own; global admins may delete anyone's. `204` on success, `404` when the token does not exist or belongs to someone else.

```sh
curl -s -X DELETE -H "Authorization: Bearer $GORAN_TOKEN" $GORAN/api/tokens/4
```

Deleting the token you are using works and locks you out immediately.

## Typical onboarding

1. A global admin creates the user: `POST /api/users`.
2. The admin issues a first token: `POST /api/users/alice/tokens`, and sends it to Alice.
3. A workspace admin adds Alice to a workspace: `POST /api/workspaces/client-a/members` with `{"user": "alice", "role": "member"}`.
4. Alice connects the console with the token, and may create further tokens of her own with `POST /api/tokens`.

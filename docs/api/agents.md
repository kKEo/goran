# Agents and registration

An agent joins a workspace by exchanging a single-use registration token for a permanent key. Operators mint registration tokens here; the exchange itself is `POST /agent/register`, documented in the [agent protocol](agent-protocol.md#post-agentregister), and is what `goran-agent register` calls.

## Objects

**Agent**

```json
{
  "id": 7, "created_at": "…", "updated_at": "…",
  "workspace_id": 2, "name": "runner-1",
  "labels": ["client-a", "terraform"],
  "key_prefix": "ga_0c41e8a",
  "last_seen_at": "2026-10-08T17:05:39.512Z"
}
```

`last_seen_at` is updated on every authenticated request the agent makes, which with the default poll interval means every two seconds while it is alive.

## `POST /api/workspaces/:ws/registration-tokens`

Workspace admin only. Mints a registration token. The body is optional.

```sh
curl -s -X POST -H "Authorization: Bearer $GORAN_TOKEN" -H 'Content-Type: application/json' \
  $GORAN/api/workspaces/client-a/registration-tokens -d '{"ttl_minutes": 15}'
```

| Field | Default | Meaning |
| --- | --- | --- |
| `ttl_minutes` | `60` | how long the token stays valid; values `<= 0` mean the default |

```json
{
  "token": "gr_e2b7…",
  "expires_at": "2026-10-08T17:20:00Z",
  "workspace": "client-a",
  "note": "single use; run: goran-agent register --server <url> --token gr_e2b7… --name <agent-name>"
}
```

The token is shown once, can be used once, and is bound to this workspace. It is stored hashed; there is no endpoint to list or revoke registration tokens, so keep lifetimes short.

## `GET /api/workspaces/:ws/agents`

Agents of the workspace, oldest first.

## `DELETE /api/workspaces/:ws/agents/:id`

Workspace admin only. Revokes the agent: its key stops working on its next request. Tasks it holds are requeued when their leases expire (see [Leases](../concepts/task-lifecycle.md#leases-and-heartbeats)). `204` on success, `404 agent not found`.

```sh
curl -s -X DELETE -H "Authorization: Bearer $GORAN_TOKEN" $GORAN/api/workspaces/client-a/agents/7
```

Agent names are not unique; use the `id`.

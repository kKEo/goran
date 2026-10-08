# Workspaces and access

Goran separates **who may talk to the server** (users and their tokens) from **what they may touch** (workspaces and memberships). Agents are a third kind of principal, bound to exactly one workspace.

## Users

A user has a unique `name`, an optional `email` and an `admin` flag. There are no passwords; a user authenticates with one of their API tokens. Users are created by `goran-server bootstrap` (the first admin) or by a global admin through `POST /api/users`.

Names, like workspace, secret and agent names, must match `^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$` and must not be purely numeric. The last rule exists so a workspace can be addressed by name or by id in URLs.

## API tokens

Every user can hold any number of tokens (`POST /api/tokens`). Tokens are random, prefixed with `gu_`, shown once at creation and stored as SHA-256 hashes. Listings show the first ten characters so you can tell tokens apart. `last_used_at` is updated on each use. A token is revoked by deleting it; the owner can delete their own tokens, a global admin can delete anyone's. Tokens do not expire.

Global admins can also mint tokens for other users (`POST /api/users/:name/tokens`), which is how a new colleague gets their first credential.

## Workspaces

A workspace is the tenancy boundary: tasks, agents, secrets, registration tokens and memberships all belong to exactly one workspace. Use one workspace per client, or per client and environment if you want separate agents and secrets for production.

Any authenticated user can create a workspace (`POST /api/workspaces`) and automatically becomes its first admin. Workspace names are unique.

In URLs, `:ws` is the workspace name or its numeric id: `/api/workspaces/client-a/tasks` and `/api/workspaces/3/tasks` are the same thing.

## Memberships and roles

A membership links a user to a workspace with one of two roles:

| Role | Can |
| --- | --- |
| `member` | see the workspace, its members, agents, secret names, tasks and logs; create and cancel tasks; see their own tokens |
| `admin` | everything a member can, plus: approve and reject plans, create, update and delete secrets, issue registration tokens, revoke agents, add members and change roles |

Workspace admins add members with `POST …/members` (`{"user": "alice", "role": "member"}`); the same call changes the role of an existing member. There is no endpoint to remove a membership in this version; revoke the user's tokens instead, or hand the workspace to a new one.

## Global admins

A user with `admin: true` is treated as an admin of every workspace, including ones they were never added to, and additionally may:

- list and create users, and mint tokens for them,
- list every workspace,
- delete any user's token.

Global admins do not appear in a workspace's member list unless they were explicitly added.

## Agents

An agent belongs to the workspace whose registration token it used. It authenticates with its own key (`ga_…`), sees only that workspace's queue, and is listed under **Agents** with its labels, key prefix and `last_seen_at`. Deleting the agent invalidates its key on the next request; tasks it holds are requeued when their leases expire.

## Permission matrix

| Action | member | workspace admin | global admin |
| --- | :-: | :-: | :-: |
| read workspace, members, agents, secret names, tasks, logs | yes | yes | yes |
| create and cancel tasks | yes | yes | yes |
| approve and reject Terraform plans | no | yes | yes |
| create, update, delete secrets | no | yes | yes |
| issue registration tokens, revoke agents | no | yes | yes |
| add members, change roles | no | yes | yes |
| create a workspace (becoming its admin) | yes | yes | yes |
| manage own tokens | yes | yes | yes |
| list users, create users, mint tokens for others | no | no | yes |
| delete other users' tokens | no | no | yes |

Requests that fail a role check answer `403`. Requests for a workspace the caller is not a member of answer `404`, so the existence of other clients' workspaces is not revealed.

# Security model

This page states what Goran protects, how, and what it leaves to you. Read the last section before exposing a server to the internet.

## Credentials

There is no master token and no default password. The only way to obtain the first credential is `goran-server bootstrap`, which needs access to the database file.

| Prefix | Credential | Issued by | Shown | Stored as | Revoked by |
| --- | --- | --- | --- | --- | --- |
| `gu_` | user API token | `bootstrap`, `POST /api/tokens`, `POST /api/users/:name/tokens` | once, at creation | SHA-256 hash | `DELETE /api/tokens/:id` |
| `ga_` | agent key | `POST /agent/register` | once, saved to `agent.json` (mode `0600`) | SHA-256 hash | `DELETE …/agents/:id` |
| `gr_` | registration token | `POST …/registration-tokens` | once | SHA-256 hash | single use; expires after `ttl_minutes` (default 60) |

Tokens are the prefix plus 48 hexadecimal characters from the operating system's random source (24 bytes of entropy). Listings only ever show the first ten characters. Because only hashes are stored, a copy of the database does not yield working credentials. User tokens and agent keys do not expire; rotate them by issuing new ones and deleting the old.

Credentials travel in the `Authorization: Bearer …` header. The server also accepts the bare token as the header value.

## Secrets at rest

Secret values are encrypted with AES-256-GCM under `GORAN_MASTER_KEY`, a 32-byte key given as 64 hexadecimal characters (`goran-server keygen` makes one). Each value is sealed with a fresh random nonce, which is stored in front of the ciphertext. The key is held in memory by the server process only; it is never written to the database.

Consequences:

- The server does not start without the key. Without it, there would be no way to store secrets safely.
- Losing the key loses every secret. Tasks that reference one then fail at claim time with `secret "name" cannot be decrypted (master key changed?)`.
- There is no automatic re-encryption. To rotate the key, start the server with the new key and `PUT` every secret again from your own password manager or vault.
- Back up the key separately from the database. The two together are the complete installation; either alone is useless to an attacker.

## Secret delivery

A secret value leaves the server exactly once per task attempt: inside the `env` field of the `GET /agent/next` response, over the connection of the agent that just claimed the task. The agent injects it into the environment of the task's child processes and keeps it in memory only for the duration of the task.

Nothing user-facing ever returns a value:

- `GET …/secrets` returns names and update times.
- The task object shows the mapping (`"secrets": {"AWS_SECRET_ACCESS_KEY": "aws-secret"}`), never values.
- Task logs contain whatever the task printed. Goran echoes the command lines it runs (`[goran] $ terraform plan …`) but keeps values out of them: Terraform variables travel as `TF_VAR_*` environment variables, Ansible `extra_vars` are written to a file and passed by path. A script that prints a secret puts it in the log in plain text; Goran does not redact logs.

!!! warning "Terraform plan files contain variable values"
    `terraform plan -out` writes variable values, including sensitive ones, into the plan file. The plan file lives in the agent's work directory (`<workdir>/<task id>/tfplan`) until the task finishes. Protect that directory like you would protect the secrets themselves.

## Tenancy

Workspaces isolate clients from each other. Agents see only their workspace's queue, users see only workspaces they belong to, and a workspace a user is not a member of answers `404`. Global admins see everything; keep that flag for the people who run the server. Details are in [Workspaces and access](workspaces-and-access.md).

Labels are not a security boundary. Any agent in a workspace may claim any task in that workspace whose labels it carries.

## The agent is a privileged process

An agent runs whatever its workspace's members queue, as the operating system user it was started as, with that user's environment, files and network reach. Treat every agent host as part of your control plane:

- Run the agent as a dedicated user with only the tools and credentials the tasks need (cloud CLIs, SSH keys for git and Ansible).
- The task environment is the agent's environment plus `GORAN_TASK_*` plus the task's secrets. Do not start the agent with credentials in its environment that tasks should not see.
- Anyone who can create tasks in a workspace can run code on that workspace's agents. Grant membership accordingly, and remember that any user can create new workspaces and register agents to them.
- Registration tokens are short-lived and single-use, but whoever has one for the next hour can add an agent that will receive that workspace's secrets. Create them when you need them and let them expire.

## Transport

The server speaks plain HTTP. Put a TLS-terminating reverse proxy in front of it before anything crosses a network you do not fully control; tokens are bearer credentials and the agent's `GET /agent/next` responses carry decrypted secrets. The agent uses Go's standard HTTP client, so it trusts the host's CA store and honours `HTTPS_PROXY`, `HTTP_PROXY` and `NO_PROXY`. Examples are in [Server deployment](../server/deployment.md#tls-with-a-reverse-proxy).

Agents only need outbound access to the server, plus whatever their tasks talk to (git hosts, cloud APIs, managed hosts). Nothing connects to an agent.

## What Goran does not do yet

Be aware of these gaps when you decide where to run it:

- no TLS of its own,
- no rate limiting or lockout on failed authentication,
- no expiry on user tokens and agent keys,
- no audit log beyond `created_by_id`, `approved_by_id` and the server's access log on stdout,
- no redaction of secrets that tasks print,
- no sandboxing of tasks beyond the agent's OS user,
- no signing or attestation of agents; possession of the key is identity.

The [known limitations](../operations/limitations.md) page tracks these.

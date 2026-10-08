# Quick start

This walkthrough runs a server and an agent on one machine and executes a shell task. It takes about five minutes. Everything below uses the binaries from `make build`; see [Installation](installation.md).

## 1. Create a master key

The server refuses to start without a master key, because it needs one to encrypt secrets at rest.

```sh
bin/goran-server keygen
# export GORAN_MASTER_KEY=4f1c…(64 hex characters)
```

Export the line it prints, and keep the key somewhere safe: secrets stored under one key cannot be read under another.

```sh
eval "$(bin/goran-server keygen)"
```

## 2. Bootstrap the first admin

There is no built-in password or master token. The `bootstrap` command creates an admin user, a workspace, the admin's membership in it, and prints an API token exactly once.

```sh
bin/goran-server bootstrap --user admin --email you@example.com --workspace client-a
```

```text
user:      admin (admin)
workspace: client-a
token:     gu_7d1a…

export GORAN_TOKEN=gu_7d1a…
```

Export `GORAN_TOKEN` as suggested. The command is idempotent: running it again reuses the user and workspace and issues another token.

## 3. Start the server

```sh
bin/goran-server serve
# goran-server listening on :8080 (db local.sqlite, lease 60s)
```

The database file `local.sqlite` is created in the current directory. Open <http://localhost:8080>, paste the token into the **API token** field and click **Connect**. You should see `admin (admin)` and the `client-a` workspace.

## 4. Register an agent

An agent joins a workspace with a single-use registration token. Create one from the console (**Agents** tab, **New registration token**) or with curl:

```sh
curl -s -X POST -H "Authorization: Bearer $GORAN_TOKEN" \
  localhost:8080/api/workspaces/client-a/registration-tokens
```

```json
{
  "token": "gr_3b9e…",
  "expires_at": "2026-10-08T19:02:11Z",
  "workspace": "client-a",
  "note": "single use; run: goran-agent register --server <url> --token gr_3b9e… --name <agent-name>"
}
```

In a second terminal, register and start the agent. Registration exchanges the token for a permanent agent key and writes it to `agent.json` (mode `0600`).

```sh
bin/goran-agent register --server http://localhost:8080 --token gr_3b9e… --name runner-1 --labels local
# registered agent "runner-1" (id 1) in workspace 1
# key saved to agent.json

bin/goran-agent run
# goran-agent "runner-1" starting
# polling http://localhost:8080 every 2s, workdir work
```

The agent now polls `GET /agent/next` every two seconds.

## 5. Store a secret

Secrets are workspace-scoped and referenced by name from tasks. Values never come back out through the API.

```sh
curl -s -X PUT -H "Authorization: Bearer $GORAN_TOKEN" -H 'Content-Type: application/json' \
  localhost:8080/api/workspaces/client-a/secrets/demo-token -d '{"value":"s3cr3t-value"}'
# {"name":"demo-token","updated_at":"…"}
```

## 6. Queue a shell task

The `secrets` map says which secret goes into which environment variable. `labels` restricts the task to agents that carry all listed labels.

```sh
curl -s -X POST -H "Authorization: Bearer $GORAN_TOKEN" -H 'Content-Type: application/json' \
  localhost:8080/api/workspaces/client-a/tasks -d '{
    "name": "hello",
    "kind": "shell",
    "params": {"script": "echo running task $GORAN_TASK_ID on $(hostname); echo token has ${#DEMO_TOKEN} characters"},
    "secrets": {"DEMO_TOKEN": "demo-token"},
    "labels": ["local"]
  }'
```

The response is the task with `"status": "new"` and an `id`. Within two seconds the agent claims it:

```text
task 1 "hello" (shell/run, attempt 1/3): starting
task 1: done
```

Read the log:

```sh
curl -s -H "Authorization: Bearer $GORAN_TOKEN" localhost:8080/api/workspaces/client-a/tasks/1/log
```

```text
[goran] $ echo running task $GORAN_TASK_ID on $(hostname); echo token has ${#DEMO_TOKEN} characters
running task 1 on my-laptop
token has 12 characters
```

The same task is visible in the console with its status, agent, attempts and log, refreshing every three seconds while it runs.

## What just happened

- The server stored your token and the agent key as SHA-256 hashes and compared hashes on every request.
- The secret was encrypted under `GORAN_MASTER_KEY` and decrypted once, into the task payload handed to `runner-1`.
- The task was claimed with a 60 second lease that the agent kept extending through log uploads and heartbeats until it reported the result.
- The agent created `work/1/` for the task, ran the script there, and removed the directory when the task finished.

## Next steps

- [First Terraform task](first-terraform-task.md) walks through plan, approval and apply.
- [Task anatomy](../tasks/index.md) lists every field a task accepts.
- [Server deployment](../server/deployment.md) covers TLS, systemd and Docker for a real installation.

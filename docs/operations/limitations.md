# Known limitations

This page lists what Goran does not do in its current state, so you can decide whether it fits before you depend on it. Items are grouped by area; none of them is hidden behind a flag.

## Platform and scale

- **One server process, one SQLite file.** No clustering, no hot standby. Recovery is restore-from-backup. Sized for tens of agents, not thousands.
- **No log retention.** Task output is kept forever; nothing prunes old tasks or logs. Watch the database size.
- **No pagination.** `GET …/tasks` returns the 200 newest tasks. Older tasks are reachable by id only.
- **Claim window of fifty.** An agent is matched against the fifty oldest `new` tasks in its workspace; tasks behind fifty unroutable ones wait.
- **No metrics endpoint.** Observability is `/healthz`, the access log and the API.
- **Agents compile but are unsupported on Windows**: no process-group termination, and `/bin/sh` as the default shell.

## Security

- **No TLS in the server.** A reverse proxy is required; see [Deployment](../server/deployment.md#tls-with-a-reverse-proxy).
- **Tokens and keys never expire**, and there is no rate limiting or lockout on failed authentication.
- **No audit log.** Who created and approved a task is recorded; who read logs, changed secrets or revoked agents is not, beyond the access log on stdout.
- **Secrets are environment variables** on the agent, visible to every process the task starts. Logs are not redacted.
- **Labels are not a security boundary.** Any agent in a workspace can claim any task in it that matches its labels.
- **Any user can create workspaces** and register agents to them.
- **No member removal and no user deletion.** Access is withdrawn by deleting tokens.
- **No listing or revocation of registration tokens** before they expire (default 60 minutes).
- **Terraform plan files** on the agent contain variable values and are protected only by file permissions.

## Tasks

- **Running tasks cannot be canceled** from the server. They end on completion, timeout, or agent shutdown.
- **Failed tasks are not retried.** `max_attempts` only covers lost leases.
- **No scheduling, no dependencies, no priorities.** Tasks are queued by API call and served oldest first.
- **No concurrency control** between tasks. Two Terraform tasks against the same state can run at once on two agents; the backend's state locking is the only guard.
- **No per-task plain environment variables.** Only secrets become environment variables; use `vars` for Terraform or the agent's environment.
- **No artifacts.** The only output a task has is its log and exit code.
- **Git `ref` must be a branch or tag.** Commit hashes are not supported by `git clone --branch`.
- **The apply is tied to the planning agent.** If that agent loses its work directory, the task is planned again instead of applied. Plans are not uploaded to the server.
- **No task templates or re-run button.** Queue a new task with the same body.

## Agent

- **One task at a time per agent.** Run more agents for parallelism.
- **No drain mode.** Stopping an agent kills its running task.
- **Orphaned work directories** (lease lost, crashes, `keep_workdir`) are not cleaned up automatically.
- **Output lost after long outages.** If more than 64 KiB of output accumulates while the server is unreachable, that output cannot be uploaded; see [Execution model](../agent/execution.md#log-streaming).
- **Labels cannot be edited** after registration.

## Console

- **No user, token or workspace management.** These are API-only.
- **The token is kept in the browser's local storage.**
- **No log search, no download button.** Use `GET …/tasks/:id/log`.

## Directions under consideration

These are ideas from the product review, not commitments: an audit log; webhook and chat notifications on `awaiting_approval` and `error`; scheduled and git-triggered tasks; uploading plan files to the server so any agent can apply; cancellation of running tasks; a `drain` command for agents; log retention settings; metrics. If one of them blocks you, open an issue on the repository so it can be prioritised.

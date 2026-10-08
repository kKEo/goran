# Execution model

This page describes exactly what the agent does from one poll to the next: how it claims work, runs the process, streams output, keeps the lease alive, reports and cleans up. Read it when you need to predict behaviour under failure.

## The loop

```text
loop:
    task = GET /agent/next            # 204: sleep poll interval, loop
    execute(task)
    loop immediately (no sleep after a task)
```

Errors from the poll (server down, `401`, proxy trouble) are logged as `poll: …` and retried at the next interval. The agent never exits on its own; a revoked key produces `poll: unauthorized: credential rejected by the server` every interval until the process is stopped.

## Executing one task

1. **Context.** A deadline of `timeout_seconds` (default one hour) is set. Everything below is bounded by it and by the agent's own shutdown signal.
2. **Work directory.** `<workdir>/<task id>/` is created.
3. **Log sink and heartbeat** start (details below).
4. **Runner.** The runner for the task's `kind` builds the environment ([Task anatomy](../tasks/index.md#environment-of-a-task)), prepares the source if there is one, and starts child processes.
5. **Result.** Once the runner returns, the heartbeat stops, the sink flushes a final time, the result is reported and the directory is removed unless the task parked for approval or asked for `keep_workdir`.

The agent log shows one line at the start and one at the end:

```text
task 42 "client-a network" (terraform/plan, attempt 1/3): starting
task 42: awaiting_approval
task 42 "client-a network" (terraform/apply, attempt 1/3): starting
task 42: done
```

## Child processes

Every command runs through the same wrapper:

- **One process group per command** (`setpgid`). Cancellation sends `SIGKILL` to the whole group, so helpers such as Terraform provider plugins, `ssh` connections and background jobs started by a script die with the command. There is no `SIGTERM` grace period.
- **Interleaved output.** Standard output and standard error are copied into one writer, in arrival order, with individual writes kept whole so lines do not interleave mid-way.
- **Wait delay of five seconds.** After the command exits or is killed, the agent waits at most five seconds for the output pipes to close, then abandons them. This matters when a command exits normally but leaves a background process behind that inherited its output; that process keeps running and its later output is not captured.
- **Exit codes.** The command's exit code becomes the task's `exit_code`. A command that could not be started yields no exit code and the error text: `exec: "x": executable file not found in $PATH` for a missing binary, or `fork/exec /bin/sh: no such file or directory` when the working directory does not exist (the message names the binary, not the directory).

## Log streaming

Output is buffered in memory and shipped to `POST /agent/tasks/:id/logs`:

| Behaviour | Value |
| --- | --- |
| flush on interval | every second |
| flush on size | when 8 KiB or more is pending |
| request timeout | 20 seconds per upload |
| on failure | the data is kept and retried at the next flush |
| buffer cap | 1 MiB; beyond that the oldest half is dropped and `[goran] log truncated: server unreachable for too long` is inserted |
| final flush | after the process ends, before the result is sent |
| server limit | one upload must be at most 64 KiB, otherwise the server answers `413` |

Each successful upload also extends the lease, so a chatty task needs no separate heartbeats.

!!! note "Long server outages and large buffered output"
    If the server is unreachable long enough for more than 64 KiB of output to accumulate, the pending upload exceeds the server's chunk limit and is rejected with `413` on every retry. The task still runs to completion and its result is reported, but the output from the outage onwards is not stored, and the agent logs `could not ship the last log chunk: server answered 413: chunk exceeds 65536 bytes`. Use `keep_workdir` and rerun if you need that output.

## Heartbeats and lease loss

A heartbeat goroutine calls `POST /agent/tasks/:id/heartbeat` every third of the lease (20 seconds by default) with a 20 second timeout. Transient failures are logged (`task 42: heartbeat: …`) and ignored; the next one may succeed.

A `409` from a heartbeat, a log upload or the result means the server no longer considers this agent the owner: the lease expired and the task was requeued, or the task was finished elsewhere. The agent then:

1. logs `task 42: lease lost, stopping`,
2. cancels the task context, which kills the process group,
3. discards the result (`task 42: lease lost; result discarded`) and goes back to polling.

This is what makes a lease expiry safe: by the time another agent runs the task, the first agent has either stopped or will stop at its next contact with the server.

## Result mapping

The first matching rule wins:

| Condition | Reported |
| --- | --- |
| the agent is shutting down | `error`, `agent shut down while the task was running` |
| the task deadline passed | `error`, `timed out after <duration>` |
| the runner returned an error | `error`, the error text (clone failed, missing binary, missing plan, bad params) |
| the process exited non-zero | `error`, `exit status N`, `exit_code = N` |
| the runner asked for approval | `awaiting_approval` |
| otherwise | `done`, `exit_code = 0` |

`POST /agent/tasks/:id/result` is retried up to three times, two seconds apart. A `409` (lease lost) or `401` (revoked) stops the retries at once. If all attempts fail, the agent logs `could not report result` and moves on; the server will requeue or fail the task when the lease expires.

## Work directory lifecycle

| Event | Directory |
| --- | --- |
| task claimed | `<workdir>/<id>/` created |
| git source prepared | cloned into `<workdir>/<id>/src/` |
| Terraform plan saved | `<workdir>/<id>/tfplan` and `goran-plan.json` written |
| Ansible extra vars | `<workdir>/<id>/extra_vars.json` written (mode `0600`) |
| result `done` or `error` | directory removed, unless `params.keep_workdir` is `true` |
| result `awaiting_approval` | directory kept for the apply phase |
| apply finished | directory removed (same rule as above) |
| lease lost or agent shut down | directory is **not** removed |

Directories that outlive their task (lease loss, crashes, `keep_workdir`) are never cleaned up by the agent. Prune `<workdir>` by hand or with a timer, excluding directories of tasks that are `awaiting_approval`.

## Shutdown

On `SIGINT` or `SIGTERM` the agent cancels the task context. The running process group is killed, the result `error` / `agent shut down while the task was running` is reported with the usual retries, and the agent exits. An idle agent exits immediately. There is no drain mode that waits for the current task; see [Stopping, restarting, upgrading](deployment.md#stopping-restarting-upgrading).

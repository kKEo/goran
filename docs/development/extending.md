# Extending Goran

## Conventions

- **Errors** go through `fail(c, status, message)` in the API package, which writes `{"error": message}`. Messages are plain sentences; they are shown to users as they are.
- **JSON is snake case**, including on new model fields (`json:"lease_expires_at"`).
- **State transitions are guarded updates**: `UPDATE … WHERE id = ? AND status = ?` with a check on `RowsAffected`. Never read a task and then write it unconditionally.
- **One database connection.** The pool is limited to one connection so that `:memory:` databases work in tests. Inside a `db.Transaction` closure use only the `tx` handle; a call on the outer `s.db` from inside the closure needs a second connection and deadlocks.
- **gofmt is enforced** by CI. `make fmt vet test` before pushing.
- **Never let secret values leak** into logs, task objects or error messages. Values exist only in `resolveSecrets` and the `env` field of `GET /agent/next`.

## Adding a task kind

Say you want `kind: "kubectl"` that applies manifests from a source.

1. **Protocol.** Add `KindKubectl = "kubectl"` to `wire/wire.go`.
2. **Validation.** Extend `validateParams` in `webapp/api/tasks.go` with the fields an agent cannot do without (the server should reject tasks that could never run). Reuse `validateSource` for sources.
3. **Runner.** Create `agent/runner/kubectl.go` with a type implementing `Run(ctx, rc *Context) (Outcome, error)`. Use `rc.run(ctx, dir, "kubectl", args…)` so the command line is echoed into the log, and `rc.withEnv(...)` for kind-specific environment. Register it in `For()` in `runner.go`.
4. **Two-phase kinds.** If the kind should wait for approval like Terraform, return `Outcome{AwaitApproval: true}` from the first phase and handle `rc.Task.Phase == wire.PhaseApply` in the second. The server currently assigns the `plan` phase only to `terraform` in `nextTask` (`webapp/api/agent_tasks.go`); generalise that condition.
5. **Console.** Add an entry to `TEMPLATES` in `webapp/ui/index.html` and an `<option>` to the kind selector, so the form offers a starting point.
6. **Tests.** A runner test in `agent/runner/runner_test.go` (use a fake binary on `PATH` as the Terraform test does), a validation test in `webapp/api/api_test.go`, and ideally an end-to-end case in `integration/e2e_test.go`.
7. **Docs.** A page under `docs/tasks/` and a row in the table on the [Task anatomy](../tasks/index.md) page.

## Adding an endpoint

1. Write the handler as a method on `*Server` in the matching file under `webapp/api/`. Use `bind` for JSON bodies, `paramID` for numeric path parameters, `middleware.CurrentUser`, `CurrentWorkspace` and `CurrentMembership` for context.
2. Register it in `buildRouter` (`webapp/api/server.go`) on the right group: `api` (any user), `api` with `auth.RequireAdmin()` (global admins), `ws` (workspace members) optionally with `auth.RequireRole(model.RoleAdmin)`, or `ag` (agents).
3. Add a test in `api_test.go`. The `newEnv` helper gives you an in-memory server, a bootstrapped admin, a fake clock and helpers for tokens, agents and tasks.
4. Document it in the [API reference](../api/index.md).

## Changing the schema

Models live in `webapp/model/`. Adding a column or table is a matter of adding the field; `db.Open` runs `AutoMigrate` on start. Custom column types (`JSON`, `Labels`, `StringMap`) implement `driver.Valuer` and `sql.Scanner` in `types.go`. Dropping or renaming columns needs a hand-written migration; there is no framework for that yet.

## Writing another agent

The [agent protocol](../api/agent-protocol.md) is small and stable enough to implement in another language. The reference implementation's behaviours worth copying are listed at the end of that page.

## Embedding the server

`api.New(api.Config{...})` returns an `http.Handler`; `main.go` is a thin wrapper around it. A custom binary can open the database with `db.Open`, construct the server with its own `Box`, lease and defaults, and mount it under another router. Call `StartReaper` if you want expired leases requeued without traffic.

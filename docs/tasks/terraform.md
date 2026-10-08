# Terraform tasks

A Terraform task runs `init` and `plan`, saves the plan file on the agent, waits for a workspace admin to approve it, and then applies exactly that plan on the same agent. OpenTofu works the same way with `"binary": "tofu"`.

## Params

| Key | Type | Required | Default | Meaning |
| --- | --- | --- | --- | --- |
| `source` | object | yes | | where the configuration comes from; see [Sources](index.md#sources) |
| `binary` | string | no | `terraform` | executable to run, e.g. `tofu` or an absolute path |
| `vars` | object of strings | no | `{}` | each entry becomes `TF_VAR_<name>=<value>` in the environment; values must be JSON strings |
| `var_files` | array of strings | no | `[]` | each becomes `-var-file=<path>` on `plan`; paths are relative to the source directory |
| `init_args` | array of strings | no | `[]` | appended to `terraform init`, e.g. `["-backend-config=key=client-a.tfstate", "-upgrade"]` |
| `auto_approve` | boolean | no | `false` | apply immediately after a successful plan instead of waiting for approval |
| `destroy` | boolean | no | `false` | plan a destroy (`plan -destroy`) |
| `keep_workdir` | boolean | no | `false` | keep the task directory after the task ends |

Non-string values in `vars` fail the task when the agent parses the params. To pass a list or map, use HCL syntax inside the string, which Terraform accepts for `TF_VAR_*`: `"zones": "[\"a\", \"b\"]"`.

## What the agent runs

**Plan phase** (`GORAN_TASK_PHASE=plan`), in the source directory:

```sh
terraform init -input=false -no-color [init_args…]
terraform plan -input=false -no-color -out=<workdir>/<id>/tfplan [-destroy] [-var-file=…]…
```

If `plan` exits `0` the agent checks that the plan file exists (a wrong `binary` that exits `0` without writing one fails with `… plan exited 0 but wrote no plan file at …`), records the source directory in `<workdir>/<id>/goran-plan.json`, and then either:

- with `auto_approve`, runs the apply below right away, or
- reports `awaiting_approval` and keeps the task directory.

**Apply phase** (`GORAN_TASK_PHASE=apply`), after an admin approved, in the recorded source directory:

```sh
terraform apply -input=false -no-color <workdir>/<id>/tfplan
```

There is no second `init`: the `.terraform` directory and the checkout from the plan phase are reused. The apply is routed to the agent that planned, because only that agent has the plan file. See [the approval cycle](../concepts/task-lifecycle.md#the-approval-cycle) for what happens when it is gone.

Environment during both phases: the task environment, `TF_IN_AUTOMATION=1`, `TF_INPUT=0` and the `TF_VAR_*` variables. Provider and backend credentials come from `secrets` (`AWS_ACCESS_KEY_ID`, `GOOGLE_CREDENTIALS`, `LINODE_TOKEN`, `ARM_CLIENT_SECRET`, …), exactly as the providers document them.

## State

Terraform state must survive the task. Where it lives depends on the source:

- **Remote backend** (S3, GCS, Azure Blob, Terraform Cloud, Consul, PostgreSQL, …): the recommended setup for every real environment. Credentials for the backend go into `secrets` like provider credentials; backend-specific settings can go into `init_args` as `-backend-config=…`.
- **`dir` source with local state**: the state file sits next to the configuration on the agent host and persists between tasks. Workable for a single agent, but the agent host becomes a single point of failure and state is not locked across agents.
- **`git` source with local state**: the checkout is inside the task directory, which is deleted when the task finishes. **The state would be lost after apply.** Do not do this.

## Examples

Plan and apply with approval, variables, a var file and provider credentials:

```json
{
  "name": "client-a network",
  "kind": "terraform",
  "params": {
    "source": { "git": "git@github.com:org/infra.git", "ref": "main", "path": "envs/client-a" },
    "vars": { "region": "eu-central-1", "environment": "prod" },
    "var_files": ["client-a.tfvars"],
    "init_args": ["-backend-config=bucket=org-tfstate", "-backend-config=key=client-a/network.tfstate"]
  },
  "secrets": { "AWS_ACCESS_KEY_ID": "aws-access-key", "AWS_SECRET_ACCESS_KEY": "aws-secret-key" },
  "labels": ["client-a", "terraform"],
  "timeout_seconds": 3600
}
```

Unattended apply from CI, with OpenTofu:

```json
{
  "name": "nightly drift fix",
  "kind": "terraform",
  "params": {
    "source": { "git": "git@github.com:org/infra.git", "ref": "v2.3.1", "path": "envs/client-a" },
    "binary": "tofu",
    "auto_approve": true
  },
  "secrets": { "AWS_ACCESS_KEY_ID": "aws-access-key", "AWS_SECRET_ACCESS_KEY": "aws-secret-key" },
  "labels": ["client-a"]
}
```

Tear-down with approval:

```json
{
  "name": "decommission staging",
  "kind": "terraform",
  "params": {
    "source": { "dir": "/srv/infra", "path": "envs/client-a-staging" },
    "destroy": true
  },
  "secrets": { "AWS_ACCESS_KEY_ID": "aws-access-key", "AWS_SECRET_ACCESS_KEY": "aws-secret-key" },
  "labels": ["client-a"]
}
```

## Tips

- **Speed up `init`** by setting `TF_PLUGIN_CACHE_DIR=/var/cache/terraform-plugins` in the agent's environment. Tasks inherit it, and providers are downloaded once per agent host.
- **Terraform workspaces**: set `TF_WORKSPACE` in the agent's environment or select the workspace in the backend configuration. Per-task environment variables are not supported yet.
- **Sensitive outputs**: Terraform prints plan and apply output into the task log, which every workspace member can read. Mark outputs and variables `sensitive = true` in the configuration so their values are redacted by Terraform itself.
- **Plan files are sensitive.** `tfplan` contains the values of all variables. It sits in the agent's work directory until the task finishes; see [Security model](../concepts/security.md#secret-delivery).
- **Lock timeouts**: a task that waits on a state lock is still "running" and keeps its lease through heartbeats; it ends when `timeout_seconds` elapses.

## Failure messages

| `error` | Cause | What to do |
| --- | --- | --- |
| `exit status 1` | `init` or `plan` or `apply` failed | read the log |
| `exec: "terraform": executable file not found in $PATH` | the agent host lacks the binary | install it, or use a capability label so the task does not land there |
| `source dir "/srv/x" is not a directory on this agent` | `dir` does not exist on the agent that claimed the task | fix the path or restrict labels |
| `git clone exited with status 128` | authentication or wrong `ref` | check the agent's git credentials and the branch or tag name |
| `terraform plan exited 0 but wrote no plan file at …` | `binary` is not Terraform-compatible | fix `binary` |
| `no saved plan in …; re-run the task` | the agent lost its work directory between plan and apply | queue the task again |

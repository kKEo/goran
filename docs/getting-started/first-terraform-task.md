# First Terraform task

This page runs one Terraform task end to end: the agent plans, the task waits for approval, an admin approves, and the same agent applies the saved plan. It assumes the server, token and agent from the [quick start](quickstart.md).

## Prerequisites

- `terraform` (1.4 or newer, for the built-in `terraform_data` resource) on the agent's `PATH`. OpenTofu works too; pass `"binary": "tofu"` in the task.
- A directory with Terraform code on the agent host, or a git repository the agent can clone.

The example below needs no cloud account. Create this file on the agent host:

```hcl title="/srv/tf/hello/main.tf"
variable "greeting" {
  type = string
}

resource "terraform_data" "hello" {
  input = var.greeting
}

output "greeting" {
  value = terraform_data.hello.output
}
```

## Queue the task

```sh
curl -s -X POST -H "Authorization: Bearer $GORAN_TOKEN" -H 'Content-Type: application/json' \
  localhost:8080/api/workspaces/client-a/tasks -d '{
    "name": "hello terraform",
    "kind": "terraform",
    "params": {
      "source": {"dir": "/srv/tf/hello"},
      "vars": {"greeting": "hello from goran"}
    },
    "labels": ["local"]
  }'
```

`vars` are passed to Terraform as `TF_VAR_*` environment variables, so their values never show up in the task log. The agent claims the task in the `plan` phase and runs:

```text
[goran] using local source /srv/tf/hello
[goran] $ terraform init -input=false -no-color
…
[goran] $ terraform plan -input=false -no-color -out=/home/goran/work/2/tfplan
Terraform will perform the following actions:

  # terraform_data.hello will be created
  + resource "terraform_data" "hello" {
      + id     = (known after apply)
      + input  = "hello from goran"
      + output = (known after apply)
    }

Plan: 1 to add, 0 to change, 0 to destroy.
…
[goran] plan saved to /home/goran/work/2/tfplan; waiting for approval
```

The task is now `awaiting_approval`. The plan file and a small metadata file stay in the task's work directory on that agent.

## Approve

Open the task in the console and click **Approve & apply**, or:

```sh
curl -s -X POST -H "Authorization: Bearer $GORAN_TOKEN" \
  localhost:8080/api/workspaces/client-a/tasks/2/approve
```

Approval needs the workspace `admin` role. The task becomes `approved` and starts a fresh lease. On its next poll, `runner-1` (and only `runner-1`, because it holds the plan file) receives the task again in the `apply` phase:

```text
[goran] applying approved plan
[goran] $ terraform apply -input=false -no-color /home/goran/work/2/tfplan
terraform_data.hello: Creating...
terraform_data.hello: Creation complete after 0s [id=…]

Apply complete! Resources: 1 added, 0 changed, 0 destroyed.

Outputs:

greeting = "hello from goran"
```

The task ends as `done` with exit code `0`, and the agent removes `work/2/`.

!!! warning "Where did the state go?"
    Terraform wrote `terraform.tfstate` next to `main.tf` in `/srv/tf/hello`, because the source is a directory on the agent host. With a **git source** the checkout lives inside the task's work directory, which is deleted when the task finishes, so local state would be lost. Use a remote backend (S3, GCS, Terraform Cloud, Consul, …) for anything that matters. See [Terraform tasks](../tasks/terraform.md#state).

## Reject, cancel, re-run

- **Reject** (`POST …/tasks/2/reject`, admin) ends the task as `rejected`.
- **Cancel** (`POST …/tasks/2/cancel`, any member) ends a `new` or `awaiting_approval` task as `canceled`. A running task cannot be canceled.
- If the agent that planned the task is gone when you approve, its lease expires, the task returns to `new` and is planned again from scratch by whichever agent claims it. Nothing is applied without a plan that an admin saw.

## Using a repository and secrets

A real task fetches code from git and authenticates to a provider with a secret:

```sh
curl -s -X PUT -H "Authorization: Bearer $GORAN_TOKEN" -H 'Content-Type: application/json' \
  localhost:8080/api/workspaces/client-a/secrets/aws-secret-key -d '{"value":"…"}'

curl -s -X POST -H "Authorization: Bearer $GORAN_TOKEN" -H 'Content-Type: application/json' \
  localhost:8080/api/workspaces/client-a/tasks -d '{
    "name": "client-a network",
    "kind": "terraform",
    "params": {
      "source": {"git": "git@github.com:org/infra.git", "ref": "main", "path": "envs/client-a"},
      "vars": {"region": "eu-central-1"},
      "var_files": ["client-a.tfvars"]
    },
    "secrets": {"AWS_ACCESS_KEY_ID": "aws-access-key", "AWS_SECRET_ACCESS_KEY": "aws-secret-key"},
    "labels": ["client-a"],
    "timeout_seconds": 1800
  }'
```

The agent clones the repository with `git clone --depth 1 --branch main` into the task directory, runs `init` and `plan` in `envs/client-a`, and reuses the same checkout for `apply`. Git authentication is the agent host's business: an SSH deploy key or a credential helper. Do not put tokens into the URL, because the clone command line is echoed into the log.

Set `"auto_approve": true` in `params` to apply right after a successful plan without stopping, and `"destroy": true` to plan a destroy.

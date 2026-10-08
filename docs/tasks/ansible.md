# Ansible tasks

An Ansible task runs `ansible-playbook` against a playbook from a git repository or a directory on the agent host. The agent host needs Ansible, SSH access to the managed hosts, and `git` for git sources.

## Params

| Key | Type | Required | Default | Meaning |
| --- | --- | --- | --- | --- |
| `source` | object | yes | | where the playbook comes from; see [Sources](index.md#sources) |
| `playbook` | string | yes | | playbook path, relative to the source directory |
| `inventory` | string | no | | passed as `-i`; a path relative to the source directory, or an inline host list such as `web1.example.net,` |
| `extra_vars` | object | no | | any JSON; written to `<workdir>/<id>/extra_vars.json` (mode `0600`) and passed as `-e @<file>` |
| `args` | array of strings | no | `[]` | extra command line arguments inserted before the playbook, e.g. `["--limit", "web", "--check", "--diff", "-v"]` |
| `binary` | string | no | `ansible-playbook` | executable to run |
| `keep_workdir` | boolean | no | `false` | keep the task directory after the task ends |

## What the agent runs

In the source directory, with the task environment plus `ANSIBLE_NOCOLOR=1` and `ANSIBLE_FORCE_COLOR=0`:

```sh
ansible-playbook [-i <inventory>] [-e @<workdir>/<id>/extra_vars.json] [args…] <playbook>
```

Because the command runs inside the source directory, an `ansible.cfg` at the root of the repository (or the `path` subdirectory) is picked up as usual. `extra_vars` go through a file rather than the command line so their values do not appear in the echoed command or in the process list.

## Examples

Run a site playbook against a production inventory, with vault access from a secret:

```json
{
  "name": "client-a site.yml",
  "kind": "ansible",
  "params": {
    "source": { "git": "git@github.com:org/playbooks.git", "ref": "main" },
    "playbook": "site.yml",
    "inventory": "inventories/client-a/hosts.ini",
    "extra_vars": { "release": "2026.10.1", "restart_services": true },
    "args": ["--vault-password-file", "scripts/vault-pass.sh", "--diff"]
  },
  "secrets": { "ANSIBLE_VAULT_PASS": "client-a-vault-password" },
  "labels": ["client-a", "ansible"],
  "timeout_seconds": 2700
}
```

`scripts/vault-pass.sh` is an executable file in the repository that prints the password from the environment, which is how Ansible's "vault password script" convention works:

```sh
#!/bin/sh
printf '%s' "$ANSIBLE_VAULT_PASS"
```

Dry run on a subset of hosts:

```json
{
  "name": "check web tier",
  "kind": "ansible",
  "params": {
    "source": { "dir": "/srv/playbooks" },
    "playbook": "web.yml",
    "inventory": "inventories/client-a/hosts.ini",
    "args": ["--check", "--diff", "--limit", "web"]
  },
  "labels": ["client-a"]
}
```

## SSH and host access

The agent does not manage SSH for you:

- Put the private key for the managed hosts in the agent user's `~/.ssh/`, or point `ANSIBLE_PRIVATE_KEY_FILE` at it in the agent's environment.
- Host key checking follows Ansible's defaults and your `ansible.cfg`. `ANSIBLE_HOST_KEY_CHECKING=False` in the agent environment disables it if you accept the trade-off.
- Credentials needed by modules (cloud inventories, API tokens) belong in `secrets` and are read by Ansible from the environment or from `extra_vars` that reference `lookup('env', …)`.

## Exit codes

Ansible's exit codes end the task as `error` with `exit status N`: `2` for failed tasks, `4` for unreachable hosts, `1` for errors, `99` for user interruption, and so on. Idempotent runs with no failures exit `0` and end as `done`, regardless of how many hosts changed. Read the play recap in the log for the per-host picture.

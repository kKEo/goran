# Agent configuration

The agent reads its settings from three places, in increasing order of precedence:

1. the **config file** written by `goran-agent register` (default `agent.json`, or `$GORAN_AGENT_CONFIG`),
2. **environment variables** (`GORAN_*`),
3. **command line flags**.

A missing config file is not an error as long as the server URL and the key come from the environment or flags.

## Config file

`register` writes this file with mode `0600`, because it contains the agent key:

```json title="agent.json"
{
  "server": "https://goran.example.net",
  "key": "ga_…",
  "name": "runner-1",
  "labels": ["client-a", "terraform"]
}
```

| Key | Used by `run` | Meaning |
| --- | --- | --- |
| `server` | yes | server URL |
| `key` | yes | the agent key; the only credential the agent has |
| `name` | logged only | the name given at registration, for the start-up log line |
| `labels` | no | informational copy of the labels given at registration; the server's copy is authoritative |
| `workdir` | yes | task directory root; not written by `register`, add it by hand or use `--workdir` |
| `poll_seconds` | yes | poll interval in whole seconds; not written by `register` |

Keep the file readable by the agent's user only. Anyone with the key can claim that workspace's tasks and receive its secrets until the agent is revoked.

## Environment variables

| Variable | Used by | Meaning |
| --- | --- | --- |
| `GORAN_AGENT_CONFIG` | both | path of the config file (default `agent.json`) |
| `GORAN_SERVER` | both | server URL |
| `GORAN_REGISTRATION_TOKEN` | `register` | the single-use `gr_…` token |
| `GORAN_AGENT_NAME` | `register` | agent name (default: host name) |
| `GORAN_AGENT_LABELS` | `register` | comma separated labels |
| `GORAN_AGENT_KEY` | `run` | agent key, for running without a config file |
| `GORAN_WORKDIR` | `run` | task directory root (default `work`) |
| `GORAN_POLL_SECONDS` | `run` | poll interval in seconds (default `2`) |

The Docker image presets `GORAN_AGENT_CONFIG=/home/goran/agent.json` and `GORAN_WORKDIR=/home/goran/work`.

## Running without a config file

Container platforms often prefer injecting credentials as environment variables. Register once anywhere to obtain the key, then run agents with only two variables:

```sh
goran-agent register --server https://goran.example.net --token gr_… --name runner-1 --labels client-a --config /tmp/runner-1.json
jq -r .key /tmp/runner-1.json      # store it in your secret manager, then
shred -u /tmp/runner-1.json

GORAN_SERVER=https://goran.example.net GORAN_AGENT_KEY=ga_… goran-agent run
```

Remember that one registered agent is one identity. If two processes run with the same key they both claim tasks from the same queue and the console shows them as one agent, with `last_seen_at` flapping between them. Register one agent per process.

## Environment seen by tasks

Everything in the agent's environment is inherited by the tasks it runs, which is the intended way to give tasks host-wide settings:

| Variable | Effect |
| --- | --- |
| `PATH` | where `terraform`, `tofu`, `ansible-playbook`, `git` and your scripts' tools are found |
| `HTTPS_PROXY`, `HTTP_PROXY`, `NO_PROXY` | used by the agent's own HTTP client and by most tools |
| `TF_PLUGIN_CACHE_DIR` | shared Terraform provider cache across tasks |
| `TF_WORKSPACE`, `TF_CLI_ARGS_*` | Terraform behaviour for every task on this agent |
| `ANSIBLE_CONFIG`, `ANSIBLE_PRIVATE_KEY_FILE`, `ANSIBLE_HOST_KEY_CHECKING` | Ansible behaviour for every task on this agent |
| `GIT_SSH_COMMAND` | e.g. `ssh -i /etc/goran-agent/deploy_key -o IdentitiesOnly=yes` for git sources |

Do not start the agent with credentials in its environment that tasks must not see. Task secrets are added on top and override variables of the same name. See [Task anatomy](../tasks/index.md#environment-of-a-task).

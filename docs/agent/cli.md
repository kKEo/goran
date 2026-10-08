# Agent CLI reference

```text
goran-agent <command> [flags]

commands:
  register   exchange a one-time registration token for an agent key and save it
  run        poll the server and execute tasks
  help       print usage
```

There is no default command; running `goran-agent` without one prints the usage and exits `2`. Flags override environment variables, which override the config file.

## `register`

Calls `POST /agent/register` with a registration token, receives the agent's permanent key and writes it to the config file with mode `0600`.

| Flag | Environment | Default | Meaning |
| --- | --- | --- | --- |
| `--server` | `GORAN_SERVER` | required | server URL, e.g. `https://goran.example.net` |
| `--token` | `GORAN_REGISTRATION_TOKEN` | required | the `gr_…` token from the console or `POST …/registration-tokens` |
| `--name` | `GORAN_AGENT_NAME` | the host name | agent name shown in the console; must match `^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$` and not be purely numeric |
| `--labels` | `GORAN_AGENT_LABELS` | none | comma separated labels tasks can require, e.g. `client-a,eu,terraform` |
| `--config` | `GORAN_AGENT_CONFIG` | `agent.json` | where to write the config; parent directories are created |

```sh
goran-agent register --server https://goran.example.net --token gr_… --name runner-1 --labels client-a,terraform
# registered agent "runner-1" (id 7) in workspace 2
# key saved to agent.json
```

Notes:

- The request has a 30 second timeout. Failures print `register: …` and exit `1`; the common ones are `unauthorized: credential rejected by the server` (token invalid, already used or expired) and `server answered 400: token and a valid name are required`.
- Labels are stored on the server at this moment and cannot be changed later. To change them, revoke the agent and register again with a new token.
- Agent names are not unique. Registering `runner-1` twice creates two agents; revoke the old one in the console.
- The config file is overwritten if it exists.

## `run`

Loads the configuration, then polls the server and executes tasks until it receives `SIGINT` or `SIGTERM`.

| Flag | Environment | Config key | Default | Meaning |
| --- | --- | --- | --- | --- |
| `--config` | `GORAN_AGENT_CONFIG` | | `agent.json` | config file written by `register`; may be absent if server and key come from elsewhere |
| `--server` | `GORAN_SERVER` | `server` | | server URL |
| `--key` | `GORAN_AGENT_KEY` | `key` | | agent key (`ga_…`) |
| `--workdir` | `GORAN_WORKDIR` | `workdir` | `work` | directory for task checkouts, plan files and output; created if missing and resolved to an absolute path at start |
| `--poll` | `GORAN_POLL_SECONDS` | `poll_seconds` | `2s` | poll interval; the flag is a duration (`5s`, `1m`), the environment variable and the config key are whole seconds |

```sh
goran-agent run --config /etc/goran-agent/agent.json --workdir /var/lib/goran-agent/work --poll 3s
# goran-agent "runner-1" starting
# polling https://goran.example.net every 3s, workdir /var/lib/goran-agent/work
```

Without a server URL the command fails with `server URL is required (--server, GORAN_SERVER or the config file)`; without a key with `agent key is required: run goran-agent register first, or set GORAN_AGENT_KEY`.

The agent's name and labels are not needed by `run`: the key identifies the agent, and the server already knows its labels.

## Signals and exit status

| Signal | Effect |
| --- | --- |
| `SIGINT`, `SIGTERM` | if a task is running, its whole process group is killed and the task is reported as `error` with `agent shut down while the task was running`; then the agent exits `0` with `goran-agent stopped` |

| Status | When |
| --- | --- |
| `0` | stopped by a signal |
| `1` | configuration or registration error; the message is on stderr |
| `2` | no command or invalid flags |

See [Execution model](execution.md) for what the agent logs while it works.

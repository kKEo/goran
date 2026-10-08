# Agent deployment

An agent is a long-running process on a host inside the network where the work has to happen. It needs outbound access to the server and the tools its tasks use, and nothing else.

## Preparing the host

1. Create a dedicated user. Tasks run as this user with its environment and files, so give it exactly what the tasks need: cloud CLIs, `terraform` or `tofu`, `ansible-playbook`, `git`, SSH keys for git hosts and managed hosts.
2. Choose a work directory on a disk with enough room for checkouts and provider plugins, owned by that user.
3. Make sure the host can reach the server URL (through a proxy if needed) and whatever the tasks talk to. Nothing needs to reach the host.
4. Get a registration token from a workspace admin (console, **Agents** tab) and register:

```sh
sudo -u goran goran-agent register \
  --server https://goran.example.net --token gr_… \
  --name runner-1 --labels client-a,terraform,ansible \
  --config /etc/goran-agent/agent.json
```

Register one agent per process you intend to run. Registration tokens are single use; ask for one per agent.

## systemd

```ini title="/etc/systemd/system/goran-agent.service"
[Unit]
Description=Goran agent
After=network-online.target
Wants=network-online.target

[Service]
User=goran
Group=goran
WorkingDirectory=/var/lib/goran-agent
ExecStart=/usr/local/bin/goran-agent run --config /etc/goran-agent/agent.json --workdir /var/lib/goran-agent/work
Environment=TF_PLUGIN_CACHE_DIR=/var/lib/goran-agent/plugin-cache
Environment=GIT_SSH_COMMAND="ssh -i /etc/goran-agent/deploy_key -o IdentitiesOnly=yes"
Restart=always
RestartSec=5
TimeoutStopSec=30

[Install]
WantedBy=multi-user.target
```

```sh
install -d -m 0750 -o goran -g goran /var/lib/goran-agent /var/lib/goran-agent/plugin-cache /etc/goran-agent
chmod 0600 /etc/goran-agent/agent.json && chown goran:goran /etc/goran-agent/agent.json
systemctl enable --now goran-agent
journalctl -u goran-agent -f
```

`TimeoutStopSec` gives the agent time to report a killed task before systemd escalates; a few seconds is enough. Several agents on one host are separate units with their own config files and work directories.

## Docker

The image built from `build/Dockerfile.agent` contains the agent, `git`, `bash`, `openssh-client`, Terraform and Ansible, runs as user `goran`, and expects the config at `/home/goran/agent.json` with the work directory at `/home/goran/work`. Its entrypoint is `goran-agent`; the default command is `run`.

Register into a volume once, then run from it:

```sh
docker build -f build/Dockerfile.agent -t goran-agent .
docker volume create goran-agent-a

docker run --rm -v goran-agent-a:/home/goran goran-agent \
  register --server https://goran.example.net --token gr_… --name runner-a --labels client-a

docker run -d --name goran-agent-a --restart unless-stopped \
  -v goran-agent-a:/home/goran \
  -v /srv/goran/ssh:/home/goran/.ssh:ro \
  -e TF_PLUGIN_CACHE_DIR=/home/goran/plugin-cache \
  goran-agent
```

Or skip the volume and pass the credentials directly (see [Running without a config file](configuration.md#running-without-a-config-file)):

```sh
docker run -d --name goran-agent-a --restart unless-stopped \
  -e GORAN_SERVER=https://goran.example.net -e GORAN_AGENT_KEY=ga_… goran-agent
```

Without a persistent `/home/goran`, a container restart loses the work directory. A Terraform task waiting for approval on that agent then fails at apply with `no saved plan`, and has to be queued again. Mount a volume if approvals may wait across restarts.

Build arguments: `TERRAFORM_VERSION` (default `1.9.8`) and `INSTALL_ANSIBLE` (`1` or `0`). Private CA certificates go to `/etc/ssl/certs/` in the image or as a mount.

```yaml title="docker-compose.yml"
services:
  agent-client-a:
    image: goran-agent
    restart: unless-stopped
    environment:
      GORAN_SERVER: https://goran.example.net
      GORAN_AGENT_KEY: ${CLIENT_A_AGENT_KEY}
      TF_PLUGIN_CACHE_DIR: /home/goran/plugin-cache
    volumes:
      - agent-a-home:/home/goran
      - ./ssh:/home/goran/.ssh:ro
volumes:
  agent-a-home:
```

## Sizing and placement

- **One task at a time per agent.** Concurrency in a workspace equals its number of agents. Two agents on one host are fine; give them different work directories.
- **Place agents where the work is.** Terraform against a cloud API can run from anywhere with internet access; Ansible against private hosts must run inside that network; both should carry the matching labels.
- **Keep clients apart.** An agent belongs to one workspace. Do not share an agent host between clients unless you accept that a task from one client runs next to the other's credentials.
- **Poll interval.** Two seconds is responsive and cheap. Raise `--poll` on metered links; the only cost is start latency.

## Stopping, restarting, upgrading

Stopping the agent (`SIGTERM`) while a task is running **kills the task** and reports it as `error`. For Terraform this can interrupt an apply mid-way and leave the state lock held in the backend. Before restarting an agent, check the Tasks view for `running` tasks assigned to it and wait for them, or accept the consequences.

A task that was `awaiting_approval` survives a restart as long as the work directory is intact: the plan file is on disk and the server routes the apply back to the same agent key.

Upgrading is replacing the binary (or image) and restarting. The config file format has not changed and the agent key stays valid.

## Revoking an agent

Delete the agent in the console (**Revoke**) or with `DELETE /api/workspaces/:ws/agents/:id`. Its next request answers `401`, which the agent logs every poll as `poll: unauthorized: credential rejected by the server` until it is stopped. Any task it held is requeued when the lease runs out. Remove the config file from the host; the key in it is dead but the file still looks like a credential.

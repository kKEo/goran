# Upgrading and backups

## What an installation consists of

| Piece | Where | Needed to restore |
| --- | --- | --- |
| the database file | `GORAN_DB` (`goran.sqlite` and its `-wal`/`-shm` sidecars while running) | yes |
| the master key | your secret manager | yes, for every secret in the database |
| agent config files | `agent.json` on each agent host | no; agents can be re-registered |
| agent work directories | `<workdir>` on each agent host | no; a plan waiting for approval is replanned if lost |

## Backups

Take a consistent copy of the database with SQLite's backup command while the server runs; the file is in WAL mode and a plain `cp` can miss recent writes or copy a torn page:

```sh
sqlite3 /var/lib/goran/goran.sqlite ".backup '/var/backups/goran/goran-$(date +%F-%H%M).sqlite'"
```

For a container, run the same against the volume (`docker run --rm -v goran-data:/data -v /var/backups/goran:/out alpine/sqlite …`), or stop the container and copy the file.

Back up the master key separately and treat the pair as sensitive: the database holds token hashes, encrypted secrets and every task log; the key opens the secrets. Test a restore now and then by starting a server with `GORAN_DB` pointing at a copy and `GORAN_MASTER_KEY` set, and listing a workspace's secrets through a task.

## Restoring

1. Stop the server.
2. Replace the database file with the backup, and delete any leftover `goran.sqlite-wal` and `goran.sqlite-shm`.
3. Start the server with the same master key as before.

Agents keep working: their keys are rows in the restored database. Tasks created after the backup are gone; tasks that were leased at backup time are requeued or failed once their leases expire.

## Upgrading the server

1. Back up as above.
2. Stop the service (`systemctl stop goran-server`, or stop the container). Agents log `poll: …` errors meanwhile and reconnect on their own.
3. Replace the binary or image.
4. Start. The schema migration runs on start; it only adds tables and columns. Check the log for `goran-server listening on …`.

Running tasks survive a short restart as long as it takes less than the lease (default 60 seconds): the agents keep running them and their next heartbeat goes through. A longer outage requeues them.

Downgrading is restoring the backup and the old binary. Schema changes are not reverted automatically.

## Upgrading agents

Replace the binary or image and restart the service. Restarting an agent kills a running task and reports it as `error`; drain it first by checking the **Tasks** view for `running` tasks assigned to it. Tasks `awaiting_approval` on that agent are unaffected as long as the work directory is kept.

Keep agents and server on the same version. The protocol has no version negotiation yet.

## Moving the server to another host

Copy the database file (from a backup) and the master key to the new host, start the server, then point agents at the new URL by editing `server` in their `agent.json` (or `GORAN_SERVER`) and restarting them. Their keys remain valid. If the URL stays the same (DNS change, same reverse proxy), agents need no change at all.

## Rotating credentials

| Credential | How |
| --- | --- |
| user token | create a new one (`POST /api/tokens`), switch, delete the old one (`DELETE /api/tokens/:id`) |
| agent key | mint a registration token, run `goran-agent register` again on the host (it overwrites `agent.json`), restart the agent, revoke the old agent entry |
| registration token | nothing to rotate; they are single use and expire |
| master key | see below |

### Rotating the master key

There is no automatic re-encryption. The procedure is manual and causes a short window in which tasks referencing secrets fail at claim time:

1. Export the list of secret names per workspace (`GET …/secrets`) and have the values ready from your own vault.
2. Generate a new key with `goran-server keygen`.
3. Restart the server with the new key.
4. `PUT` every secret again with its value. Until a secret is rewritten, tasks that reference it fail with `secret "x" cannot be decrypted (master key changed?)`.
5. Retire the old key once every secret has been rewritten.

Tasks already running keep their values; only new claims are affected.

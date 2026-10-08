# Labels and routing

Labels decide which agent may run which task. They are the only routing mechanism in Goran, and they are deliberately simple.

## How matching works

- An **agent** carries a set of labels, given once at registration (`goran-agent register --labels client-a,eu,terraform`). Labels are trimmed, de-duplicated and sorted; order does not matter.
- A **task** lists the labels it requires (`"labels": ["client-a", "terraform"]`).
- An agent may claim a task when it carries **every** label the task requires. A task with no labels can be claimed by any agent in the workspace. Extra labels on the agent are fine.
- Matching happens inside one workspace only. An agent is bound to the workspace of the registration token it used and never sees other workspaces' tasks.
- Among matching agents, whoever polls first wins. Tasks are handed out oldest first.

## Choosing labels

Labels are plain strings. Useful conventions:

| Convention | Example | Why |
| --- | --- | --- |
| network or site | `client-a`, `dc-frankfurt` | the task must run where it can reach the target hosts |
| capability | `terraform`, `tofu`, `ansible` | the server does not check which tools an agent has; a capability label keeps a Terraform task away from an agent without Terraform |
| environment | `prod`, `staging` | keep production applies on dedicated agents |
| hardware or OS | `arm64`, `rhel8` | when a task builds or tests something platform-specific |

A task that lists `["client-a", "prod", "terraform"]` will only ever run on an agent registered with all three.

## Changing an agent's labels

Labels are stored on the server when the agent registers and cannot be edited afterwards. To change them, revoke the agent in the console (or `DELETE …/agents/:id`), create a new registration token and register again with the new labels. Tasks the old agent still held are requeued when their leases expire.

## Things labels do not do

- **They are not a security boundary.** Any agent in a workspace can claim any task in that workspace whose labels it satisfies, and an agent registered with a generous label set can claim a lot. Separate clients by workspace, not by label.
- **They do not express preference or priority.** A task either matches an agent or it does not.
- **They do not guarantee tools exist.** If no agent in the workspace carries the required labels, the task stays `new` forever and the console shows no agent; nothing warns you.

!!! tip "Queue depth and label mismatches"
    When picking the next task, the server looks at the fifty oldest `new` tasks in the workspace. If more than fifty unclaimable tasks (labels no agent carries) sit at the front of the queue, tasks behind them wait until those are canceled. Cancel or fix tasks that cannot be routed rather than leaving them queued.

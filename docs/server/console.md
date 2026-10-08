# Web console

The server embeds a single-page console at `GET /`. It talks to the same JSON API that `curl` and the agent use, needs no build step and no extra service, and works in any current browser.

## Connecting

Paste a user token into the **API token** field and press **Connect** (or Enter). The console calls `GET /api/me`, shows your name (and `(admin)` for global admins), lists the workspaces you belong to in the selector and remembers both in the browser's local storage so the next visit reconnects on its own.

!!! warning "The token stays in the browser"
    Local storage keeps the token until you clear it, so a shared or unattended browser keeps access. Use a dedicated token for console use and delete it from **`GET /api/tokens`** when you are done on an untrusted machine. Clearing site data for the server's origin removes it locally.

Switch workspaces with the selector; the console reloads the current tab for that workspace. **auto-refresh** (on by default) reloads the Tasks tab every three seconds.

## Tasks

The task table lists the 200 newest tasks of the workspace with id, name, kind and phase, status, agent, attempts and the time of the last change. The filter narrows it to **active** (`new`, `pending`, `running`, `awaiting_approval`, `approved`), **awaiting approval**, **done** or **error**.

Click a row to open the task. The detail panel shows the task's metadata, its `params` and the full log, scrolled to the end and refreshed with the table while the task is active. Buttons appear according to the task's state and your role:

| Button | Shown when | Needs |
| --- | --- | --- |
| **Approve & apply** | status `awaiting_approval` | workspace admin |
| **Reject** | status `awaiting_approval` | workspace admin |
| **Cancel** | status `new` or `awaiting_approval` | member |
| **Refresh** | always | |

The console decides whether to show the admin buttons from your memberships; the server enforces the real rule and answers `403 workspace admin only` if you are an admin elsewhere but not here.

### New task

**New task** opens a form with the fields of the [task request](../tasks/index.md#request-fields):

- **Name** and **Kind** (`shell`, `terraform`, `ansible`). Choosing a kind fills **Params** with a template for it that you edit in place.
- **Params (JSON)** is the kind-specific object, validated by the server on submit.
- **Required agent labels**, comma separated.
- **Timeout seconds**; empty uses the server default.
- **Secrets** is the JSON mapping from environment variable name to secret name, for example `{"AWS_SECRET_ACCESS_KEY": "aws-secret"}`.

**Queue task** posts it and selects the new task so you can watch it start.

## Agents

Lists the workspace's agents with their labels, the first characters of their key and when they last polled. **Revoke** deletes the agent after a second click on **Sure?** (the console never uses blocking dialogs); its key stops working on its next request and any task it holds is requeued when the lease expires.

**New registration token** creates a single-use token valid for the number of minutes in the box (default 60) and displays it once, together with the exact command to run on the agent host:

```text
goran-agent register --server https://goran.example.net --token gr_… --name <agent-name> --labels <a,b>
goran-agent run
```

Copy it now; it is not shown again.

## Secrets

Shows secret names and when they were last updated. Values are never displayed. Enter a name and a value and press **Save** to create or overwrite a secret; **Delete** removes it after the **Sure?** confirmation. Tasks that reference a deleted secret fail when claimed. Both actions need the workspace admin role; the server answers `503` if it was started without a master key.

## Members

Lists the workspace's members with email and role. **Add / update** adds an existing user by name with the chosen role, or changes the role of a current member. Users themselves are created through the API by a global admin; see [Users and tokens](../api/users-and-tokens.md).

## What the console does not do

Creating users, issuing tokens, creating workspaces and deleting tokens are API-only in this version. The [API overview](../api/index.md) has `curl` examples for each.

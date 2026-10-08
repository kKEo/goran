# Working on the docs

The documentation is a [MkDocs](https://www.mkdocs.org/) site with the [Material](https://squidfunk.github.io/mkdocs-material/) theme. Sources are Markdown files under `docs/`, the configuration is `mkdocs.yml` at the repository root, and the site is published to GitHub Pages at <https://kkeo.github.io/goran/>.

## Local preview

```sh
python3 -m venv .venv && . .venv/bin/activate
pip install -r docs/requirements.txt
make docs-serve          # http://127.0.0.1:8000, reloads on save
make docs                # strict build into site/; fails on broken links
```

`docs/requirements.txt` pins the exact `mkdocs-material` version CI uses, so local and CI builds agree.

## Publishing

The `Docs` workflow (`.github/workflows/docs.yml`) runs on every push and pull request that touches `docs/`, `mkdocs.yml` or the workflow. Pull requests get a strict build only; pushes to `master` also deploy to GitHub Pages through `actions/deploy-pages`. It can be started by hand from the Actions tab (`workflow_dispatch`).

One-time repository setting: **Settings → Pages → Build and deployment → Source: GitHub Actions**. Without it the deploy job fails with a message saying Pages is not enabled.

## Structure

The navigation is defined in `mkdocs.yml` under `nav`; a page that is not listed there fails the strict build. Sections:

| Section | Audience | Content |
| --- | --- | --- |
| Getting started | first-time users | install, five-minute run, first approval cycle |
| Concepts | everyone | how the system works, independent of any one command |
| Tasks | task authors | one page per kind, parameters and exact commands |
| Server, Agent | operators | CLI, configuration, deployment, internals |
| API | integrators | every endpoint with examples |
| Operations | operators | troubleshooting, upgrades, limitations |
| Development | contributors | building, layout, extending, this page |

## Writing conventions

- One topic per page. Prefer a table for parameters and a short paragraph for behaviour.
- Quote error messages exactly as the code prints them; readers search for them.
- Link between pages with relative paths (`../concepts/security.md#secrets-at-rest`). The strict build checks that files and anchors exist.
- Use admonitions (`!!! note`, `!!! warning`) sparingly, for things that cost people time when missed.
- Diagrams are Mermaid blocks (```` ```mermaid ````); they render client side.
- Keep the [known limitations](../operations/limitations.md) page honest: when a limitation is lifted, remove it there and document the feature where it belongs.

## Keeping docs and code in sync

When you change one of these, update the page next to it:

| Code | Page |
| --- | --- |
| `webapp/main.go` flags | [Server CLI](../server/cli.md), [Server configuration](../server/configuration.md) |
| `agent/main.go`, `agent/config/config.go` | [Agent CLI](../agent/cli.md), [Agent configuration](../agent/configuration.md) |
| `webapp/api/server.go` routes | [API overview](../api/index.md) and the resource page |
| `webapp/api/tasks.go` validation | [Task anatomy](../tasks/index.md), [API: Tasks](../api/tasks.md) |
| `agent/runner/*.go` | the page under Tasks for that kind |
| `agent/worker/worker.go`, `agent/logsink/sink.go` | [Execution model](../agent/execution.md) |
| `webapp/model/task.go` statuses | [Task lifecycle](../concepts/task-lifecycle.md) |
| `build/Dockerfile.*` | [Installation](../getting-started/installation.md), deployment pages |

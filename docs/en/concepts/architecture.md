🌐 [Português](../../pt-br/concepts/architecture.md) | **English** | 🏠 [Index](../index.md)

---

# Architecture Overview

This is the "how it fits together" answer, for users and prospective contributors who want the big picture without diving into per-package internals — see pages like [System management](../architecture/system-management.md) and [Dev environment](../architecture/dev-environment.md) for that level of detail.

## Layers

perci.gnl is a GUI-only desktop app — Go backend, JS frontend, built on Wails — structured in layers:

```
cmd/prci-gui/main.go → cmd/prci-gui (app*.go)   → domain packages   → internal/executor
(launch GUI window)    (Go methods bound to the    (per category,       (single point of
                         JS frontend, five           see below)           sudo escalation)
                         categories, see below)
```

- **`cmd/prci-gui/main.go`** does nothing but configure and launch the Wails window (`wails.Run`) — no logic lives here. The `prci`/`perci` commands take no command-line arguments at all; running either just opens the GUI window.
- **`cmd/prci-gui`** (`app.go` plus one `app_*.go` per screen area, and `catalog.go` for the sidebar menu definitions) holds the Go methods that Wails binds to the JS frontend — one method per action, wired to a `MenuItem.ActionID` inside one of the five sidebar categories.
- **Domain packages** implement the actual work, grouped by category (see below). None of them contain interactive prompts (no forms, no stdin reads) — they take plain parameters and are called directly by the GUI's Go methods.
- **`internal/executor`** is the single point through which every privileged command is escalated via `sudo` — no domain package calls `sudo` directly.

`internal/ui` (script-style terminal helpers: `PrintHeader`/`Info`/`Err`/`Warning`/`Success`/`Step`) is still used, GUI-only now: its output is streamed live into the GUI's own embedded terminal panel (xterm.js) inside the "Execução" tab, using a single fixed color palette instead of the old TUI's user-selectable themes.

## Components

| Package | Responsibility |
|---|---|
| `cmd/prci-gui` | Wails desktop app: `main.go` (window setup), `app.go`/`app_*.go` (Go methods bound to the JS frontend), `catalog.go` (the five sidebar categories and their menu items) |
| `internal/ui` | Terminal primitives (`PrintHeader`, `Info`, `Err`, `Warning`, `Success`, `Step`), streamed into the GUI's embedded terminal panel — no dependency on `config`/`distro` |
| `internal/config` | `~/.perci/config.yaml` load/save |
| `internal/distro` | Debian/Fedora family + desktop environment detection |
| `internal/executor` | Single point of `sudo` escalation |
| `internal/selfupdate` | Self-update via GitHub Releases, self-uninstall |
| `internal/shellrc` | Shared helpers for editing `~/.bashrc`/rc files (PATH exports, idempotent appends) |
| `internal/system/*` | **Linux category**: post-install (Mint/Zorin/Ubuntu/Fedora), fonts, file templates, Flatpak apps, system update, Linux Toys, MegaSync |
| `internal/dev/*` | **Desenvolvimento category**: prerequisites, Go/Flutter SDKs, IDEs, LLM/MCP CLIs, terminals; also backs the **Dev Tools category**'s AI skills (`agentskills`) and MCP server registration (`mcpservers`) |
| `internal/appstack` | **Docker category**: per-project Docker application stack — Nginx/MariaDB/PHP/Node/PHP+Node container creation, lifecycle, Moodle routing, export/import |
| `internal/manager/*` | **Dev Tools category**: AI context (`ai`), database backup/restore (`db`, used by `internal/appstack`), Git repository (`repo`), `.gitignore` (`gitignore`) |

## Data Flow

There is no persistent process or database. Every GUI action follows the same shape: load config (if needed) → do the work, escalating through `internal/executor` when privileged → stream result into the "Execução" terminal panel → return control to the sidebar/menu. The only state that survives between runs is `~/.perci/config.yaml` plus whatever a given feature explicitly writes to disk (Docker containers, an `AGENTS.md` file, an installed font).

## External Integrations

- **GitHub Releases** (`oito2/perci`) — self-update and the `install.sh` installer both fetch from here, verifying `checksums.txt` before installing.
- **APT / DNF / Flatpak / Snap** — system package managers, invoked through `internal/executor`.
- **Docker Engine** (`docker run`/`create`/`start`/`stop`/`rm`, no `docker-compose`) — the per-project Nginx/MariaDB/PHP/Node application stack.
- **mkcert** — local HTTPS for `*.localhost`, one wildcard certificate shared by every app.
- **Various upstream installers** (Go, Flutter, Node-based MCP servers, IDEs, terminal tools) — each with its own download/verification strategy; see [System management](../architecture/system-management.md) and [Dev environment](../architecture/dev-environment.md) for specifics, and the [decisions log](../architecture/decisions.md) for accepted risks where no upstream checksum exists.

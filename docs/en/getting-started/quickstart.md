🌐 [Português](../../pt-br/getting-started/quickstart.md) | **English** | 🏠 [Index](../index.md)

---

# Quickstart

## Open Perci

Install perci.gnl first — see [Installation](./installation.md) if you haven't yet. Then, from a terminal (or an app launcher, once you've pinned it):

```bash
prci
```

This opens the Perci window. There's nothing to type afterward — every action lives in the GUI.

## What You'll See

A sidebar on the left with five categories, and a main panel on the right showing whatever item is selected:

| Category | Covers |
|---|---|
| **Home** | Dashboard ("Visão Geral"), app settings ("Configurações"). |
| **Linux** | System updates, distro/DE-aware post-installation, fonts, file templates, Flatpak/Linux Toys/MegaSync apps. |
| **Desenvolvimento** | Build prerequisites, language SDKs, AI/terminal apps, IDEs (Zed, VS Code, VSCodium, Android Studio, Antigravity). |
| **Docker** | Create and manage the per-project Docker application stack. |
| **Dev Tools** | Git repositories, AI context generation, AI agent skills, MCP server registration. |

The window opens on **Home :: Visão Geral** — a dashboard summarizing your system's status (distro/DE, installed dev tools, Docker containers, and so on), plus shortcuts to update or uninstall Perci itself. See [How perci.gnl works](../concepts/how-perci-works.md) for what's inside each category.

## First-Run Configuration

Perci creates `~/.perci/config.yaml` the first time you change a setting in the GUI — it never fills in defaults on its own. Until you pick a workspace, the Configurações screen says it isn't chosen yet and everything that needs one uses `~/workspace`. You don't need to touch the file by hand: open **Home :: Configurações** to pick the workspace folder or change the Flatpak scope, sidebar logo, app icon, or theme through the GUI. See the [Configuration reference](../reference/configuration.md) for every key, if you'd rather edit the file directly.

## A Typical First Session

1. **Home :: Visão Geral** — check your system's current status before changing anything.
2. **Linux :: Atualizar Sistema** — run a first `apt`/`dnf` + Flatpak + Snap update pass.
3. **Linux :: Pós-instalação** — a checklist scoped to your detected distro/DE.
4. **Desenvolvimento :: Pré-requisitos** — install the compiler toolchain, Git, curl, Docker Engine, mkcert, and NPM.
5. **Docker :: Criar Container** — create the Nginx and MariaDB containers, then your first application container (see the [Docker application stack guide](../guides/environments/docker.md) for the full lifecycle).
6. **Dev Tools :: IA: Contextos** — once inside an actual project directory, generate `AGENTS.md`/`CLAUDE.md`/`.instructions/*.md`.

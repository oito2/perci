🌐 [Português](../../pt-br/concepts/how-perci-works.md) | **English** | 🏠 [Index](../index.md)

---

# How perci.gnl Works

## One Front End, One Domain Layer

`prci` is a single GUI front end (built with [Wails](https://wails.io)) — there's no separate CLI or terminal UI anymore; both were completely removed on 2026-09-14.

This means every domain function in `internal/system`, `internal/dev`, `internal/appstack`, and `internal/manager` takes plain parameters and is called directly by a Go method bound to the GUI's frontend — there are no interactive prompts anywhere in that code, and no separate scriptable path: the GUI is the only way in.

## The Five Categories

The GUI groups every action into five sidebar categories:

| # | Category | Covers |
|---|---|---|
| 1 | **Home** | Dashboard/overview (with self-update and self-uninstall), settings. |
| 2 | **Linux** | System-wide OS actions: updates, distro/DE-specific post-installation, fonts, file templates, Flatpak apps, Linux Toys, MegaSync, WebApps. |
| 3 | **Desenvolvimento** | Building a dev environment: prerequisites, Go/Flutter SDKs, IDEs, LLM/MCP CLIs, terminals. |
| 4 | **Docker** | The per-project Docker application stack: creating and managing Nginx/MariaDB/PHP/Node containers, export/import. |
| 5 | **Dev Tools** | Working inside an already-set-up project: Git repository control, AI Context generation, AI skills, MCP server registration. |

### Screens

![Home → Visão Geral](../../img/screenshots/main-window.png)

![Linux → Atualizar Sistema](../../img/screenshots/linux-window.png)

![Desenvolvimento → Pré-requisitos](../../img/screenshots/dev-window.png)

![Docker → Criar Contêiner](../../img/screenshots/docker-window.png)

![Docker → Gerenciar Contêineres](../../img/screenshots/perci-docker.png)

![Dev Tools → Repositórios](../../img/screenshots/devtools-window.png)

#### Themes

**Home → Configurações** switches the GUI theme live — the same screen in each of the eight themes:

<table>
  <tr>
    <td align="center"><img src="../../img/screenshots/light.png" alt="Home → Configurações, theme light" width="420"><br><code>light</code></td>
    <td align="center"><img src="../../img/screenshots/dark.png" alt="Home → Configurações, theme dark" width="420"><br><code>dark</code></td>
  </tr>
  <tr>
    <td align="center"><img src="../../img/screenshots/cupcake.png" alt="Home → Configurações, theme cupcake" width="420"><br><code>cupcake</code></td>
    <td align="center"><img src="../../img/screenshots/synthwave.png" alt="Home → Configurações, theme synthwave" width="420"><br><code>synthwave</code></td>
  </tr>
  <tr>
    <td align="center"><img src="../../img/screenshots/retro.png" alt="Home → Configurações, theme retro" width="420"><br><code>retro</code></td>
    <td align="center"><img src="../../img/screenshots/valentine.png" alt="Home → Configurações, theme valentine" width="420"><br><code>valentine</code></td>
  </tr>
  <tr>
    <td align="center"><img src="../../img/screenshots/halloween.png" alt="Home → Configurações, theme halloween" width="420"><br><code>halloween</code></td>
    <td align="center"><img src="../../img/screenshots/garden.png" alt="Home → Configurações, theme garden" width="420"><br><code>garden</code></td>
  </tr>
</table>

![System tray menu — Contêineres, Repositórios and Atualizar Sistema open the compact window](../../img/screenshots/tray-window.png)

## Distro-Aware, Not Distro-Agnostic

Menu items tied to a specific distro/DE combination (the post-installation entries) are hidden from the "Linux" category entirely when they don't match the machine perci detects itself running on (`internal/distro`) — only the one entry for your actual distro/DE shows up. Everything else (SDKs, the Docker application stack, AI context, database ops) works the same regardless of distro, as long as the underlying distro family (Debian or Fedora) is supported.

## Config Is the Only Persistent State

Outside of whatever a given feature explicitly generates (Docker containers, an `AGENTS.md` file, a font install), perci itself only persists one thing: `~/.perci/config.yaml`. There's no database, no daemon, no background process — every action is a single, synchronous run that reads config, does the work (streaming its output into the GUI's own terminal panel), and returns control to the sidebar.

## Privilege Escalation Is Centralized

Every command that needs `sudo` goes through `internal/executor.Executor` — no domain package shells out to `sudo` directly. This is what makes it possible to reason about (and audit) every privileged operation perci can perform from a single file. See [Core infrastructure](../architecture/core-infra.md).

## Next

- [Architecture overview](./architecture.md) for the package-level component map.

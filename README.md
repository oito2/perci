# perci.gnl

A Linux desktop app that turns a fresh install into a fully configured development workstation in minutes — post-installation, SDKs, IDEs, and a per-project Docker stack behind one window.

[![Version](https://img.shields.io/github/v/release/oito2/perci?label=version&color=brightgreen)](https://github.com/oito2/perci/releases)
[![Go](https://img.shields.io/badge/Go-1.26%2B-00ADD8?logo=go)](https://go.dev)
[![Platform](https://img.shields.io/badge/platform-Linux-FCC624?logo=linux)](https://kernel.org)
[![License](https://img.shields.io/github/license/oito2/perci?label=license)](LICENSE)
[![CI](https://img.shields.io/github/actions/workflow/status/oito2/perci/ci.yml?branch=main&label=CI)](https://github.com/oito2/perci/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/actions/workflow/status/oito2/perci/release.yml?label=release)](https://github.com/oito2/perci/actions/workflows/release.yml)

🌐 **Language:** English · [Português](docs/pt-br/leiame.md)

---

## Table of Contents

- [Overview](#overview)
- [Prerequisites & Quick Installation](#prerequisites--quick-installation)
- [Automated Installation](#automated-installation)
- [Update & Maintenance](#update--maintenance)
- [Documentation](#documentation)
- [License](#license)

---

## Overview

> This is a pretty personal project. It's the result of a bunch of `.sh` scripts I already had, gathered into a single Go app.

> The goal is simple: land on a fresh machine and have everything I need configured in minutes. That's why it only covers the distros I actually use day to day: Linux Mint 22.3 (Cinnamon and XFCE), Ubuntu 26.04, Fedora 44, and ZorinOS 18.1 (Core and Lite).

> Feel free to use and adapt it however you like — just know it solves pretty specific problems of mine, it's not meant to be a generic setup tool.

![Perci — Home → Visão Geral](docs/img/screenshots/main-window.png)
<!-- TODO: recapture after the first release (the update check shows a 404 until a release exists) -->

A desktop GUI (Go + [Wails](https://wails.io)), five sidebar categories, no daemon, no database — just `~/.perci/config.yaml`:

- **Home** — an overview of your system, update/uninstall Perci itself, settings (theme, sidebar mascot, workspace, Flatpak scope, system tray).
- **Linux** — distro/DE-aware post-installation, system updates, fonts, file templates, Flatpak apps, Linux Toys, MegaSync.
- **Desenvolvimento** — build prerequisites, Go/Flutter SDKs, AI apps, terminals, IDEs (Zed, VS Code, VSCodium, Android Studio, Antigravity).
- **Docker** — a per-project Docker application stack (Nginx/MariaDB/PHP/Node/PHP+Node containers, with Moodle routing), create and manage containers.
- **Dev Tools** — Git repository control, AI context generation (`AGENTS.md`/`CLAUDE.md`/`.instructions/*.md`), AI agent skills, MCP server registration.

An optional system tray icon gives quick access to containers, repositories, and system updates from a compact window.

---

## Prerequisites & Quick Installation

| OS | Status | Distributions | Runtime dependencies |
|---|---|---|---|
| Linux (`amd64`) | Supported | Debian/Ubuntu-based and Fedora — full experience on Linux Mint 22.3, Ubuntu 26.04, Fedora 44, Zorin OS 18.1 | GTK4, WebKitGTK 6.0 |
| Windows | Not supported | — | — |
| macOS | Not supported | — | — |

Privilege escalation (`pkexec`/PolicyKit), package management (`apt`/`dnf`), distro detection, and the GTK4 GUI stack are all Linux-specific — see [Installation → Supported Platforms](docs/en/getting-started/installation.md#supported-platforms).

**1. Runtime libraries** (usually already present on a desktop install):

```bash
# Debian/Ubuntu-based
sudo apt install -y libgtk-4-1 libwebkitgtk-6.0-4

# Fedora
sudo dnf install -y gtk4 webkitgtk6.0
```

**2. Build and install from source** (requires Go 1.26+):

```bash
# Build dependencies — Debian/Ubuntu-based
sudo apt install -y libgtk-4-dev libwebkitgtk-6.0-dev
# Build dependencies — Fedora
sudo dnf install -y gtk4-devel webkitgtk6.0-devel

git clone https://github.com/oito2/perci.git
cd perci
make install    # builds ./prci, installs it to /usr/local/bin with the `perci` alias and a menu entry
```

**3. Run it** — from the application menu, or:

```bash
prci
```

Manual binary install, release artifacts, and supported architectures: **[Installation](docs/en/getting-started/installation.md)**.

---

## Automated Installation

The one-line installer downloads the latest release, verifies its SHA-256 checksum, installs it to `/usr/local/bin/prci` with the `perci` alias, and adds the application menu entry:

```bash
curl --proto '=https' --tlsv1.2 -fsSL https://raw.githubusercontent.com/oito2/perci/main/install.sh | bash
```

Requires `curl`, `jq`, `sha256sum` and `tar`.

---

## Update & Maintenance

- **Installed from a release:** open Perci and use **Home → Visão Geral → Atualizar Perci** — it checks the latest GitHub release, verifies its checksum, and replaces the binary.
- **Installed from source:** pull and reinstall:
  ```bash
  git pull && make install
  ```

To remove Perci, see **[Uninstallation](docs/en/getting-started/uninstallation.md)**.

---

## Documentation

Full bilingual documentation site: **[docs/en/index.md](docs/en/index.md)** · **[docs/pt-br/index.md](docs/pt-br/index.md)**

- [Why perci.gnl](docs/en/concepts/why-perci.md) · [How it works](docs/en/concepts/how-perci-works.md) · [Architecture](docs/en/concepts/architecture.md)
- [Configuration reference](docs/en/reference/configuration.md)
- [Troubleshooting](docs/en/troubleshooting/common-issues.md)
- Contributing: [CONTRIBUTING.md](CONTRIBUTING.md) · [Code of Conduct](CODE_OF_CONDUCT.md)

---

## License

GPL-3.0 — see [LICENSE](LICENSE).

# Documentation — perci.gnl

🌐 [Português](../pt-br/index.md) | **English** | 🏠 [Back to README](../../README.md)

---

perci.gnl (`prci`) is a personal desktop GUI (Go + [Wails](https://wails.io)) that turns a fresh Linux install into a fully configured development workstation in minutes. It bundles OS post-installation, system maintenance, dev SDK/IDE/tooling setup, and a per-project Docker application stack manager (PHP, Node, or both together, with first-class Moodle support) behind one binary, one config file, and five sidebar categories: Home, Linux, Desenvolvimento, Docker, and Dev Tools.

---

## 🚀 Getting Started

- [Installation](./getting-started/installation.md) — automatic installer, manual binary install, or build from source.
- [Uninstallation](./getting-started/uninstallation.md) — the built-in uninstaller, manual removal, and what stays on the system.
- [Quickstart](./getting-started/quickstart.md) — first run: the GUI, the config file, and the fastest path to a working setup.

---

## 🧠 Concepts

- [Why perci.gnl](./concepts/why-perci.md) — the problem it solves and who it's actually for.
- [How perci.gnl works](./concepts/how-perci-works.md) — the five sidebar categories and how the GUI drives the shared domain code.
- [Architecture overview](./concepts/architecture.md) — high-level component map and data flow, for users who want the big picture.
- [Glossary](./concepts/glossary.md) — project-specific terms (stack, workspace, distro family, skills…).

---

## 🏗️ Architecture (for contributors)

- [System management](./architecture/system-management.md) — `internal/system/*` (post-install, fonts, templates, update, apps, linuxtoys, megasync).
- [Dev environment](./architecture/dev-environment.md) — `internal/dev/*` (SDKs, IDEs, LLM/MCP CLIs, terminals).
- [Docker application stack](./architecture/appstack.md) — `internal/appstack` (per-project Nginx/PHP/Node/MariaDB container lifecycle, Moodle routing).
- [Dev Tools backend](./architecture/managers.md) — `internal/manager/*` (AI context, AI skills, database, repository, gitignore).
- [Core infrastructure](./architecture/core-infra.md) — `internal/config`, `internal/distro`, `internal/executor`, `internal/selfupdate`, `internal/shellrc`.
- [Internal decisions log](./architecture/decisions.md) — accepted risks and technical trade-offs that aren't obvious from the code alone.

---

## 📘 Guides

- [End-to-end workflow](./guides/workflows/examples.md) — from a fresh machine to a running Moodle dev environment.
- [Docker application stack environment](./guides/environments/docker.md) — day-to-day operation of the per-project Nginx/PHP/Node/MariaDB stack.

---

## 📖 Reference

- [Configuration reference](./reference/configuration.md) — every `~/.perci/config.yaml` key.

---

## 🛠️ Troubleshooting

- [Common issues](./troubleshooting/common-issues.md) — known limitations, gotchas, and how to work around them.

---

## Elsewhere

- [Contributing guide](../../CONTRIBUTING.md) — development environment, coding conventions, how to submit changes.
- [Code of Conduct](../../CODE_OF_CONDUCT.md)
- [Changelog](../../CHANGELOG.md)

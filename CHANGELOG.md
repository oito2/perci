# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

## [1.0.0] - 2026-10-01

First public release: a Linux desktop app (Go + Wails, GTK4/WebKitGTK 6.0) with five sidebar categories.

### Added
- **Home** — system overview; update and uninstall Perci itself; settings for theme, sidebar mascot, app icon, workspace folder, Flatpak scope and an optional system tray icon with a compact window.
- **Linux** — system update; distro/desktop-aware post-installation for Linux Mint (Cinnamon, XFCE), Ubuntu 24.04/26.04, Fedora, ZorinOS (Core, Lite) and Pop!_OS 24.04 COSMIC; fonts; file templates; Flatpak apps; Linux Toys; MegaSync from MEGA's signed repository.
- **Desenvolvimento** — build prerequisites; Go and Flutter SDKs; AI CLIs (Claude Code, Antigravity CLI, Codex, OpenCode); terminals (Kitty, Alacritty, Black Box, GNOME Console) with "Abrir no <terminal>" file-manager entries and the Starship prompt; Zed, VS Code, VSCodium, Android Studio and Antigravity IDE.
- **Docker** — per-project containers (Nginx, MariaDB, Moodle, PHP, generic, Node, PHP + Node) with local HTTPS, start/stop/recreate/logs, configuration export/import and MariaDB backup/restore.
- **Dev Tools** — repositories (clone, init, global and per-repo git identity, `.gitignore` by project type, Code of Conduct); AI contexts (`AGENTS.md`, `CLAUDE.md`, `.instructions/`); AI skills from skills.sh; MCP servers for Claude Code, Codex and Antigravity.
- Privileged actions ask for the administrator password once per action (PolicyKit).
- Installation through `install.sh` or `make install`; release artifacts (`prci-linux-amd64`, `perci-menu.tar.gz`, `checksums.txt`) with build provenance attestation.

[Unreleased]: https://github.com/oito2/perci/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/oito2/perci/releases/tag/v1.0.0

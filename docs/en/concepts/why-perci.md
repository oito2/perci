🌐 [Português](../../pt-br/concepts/why-perci.md) | **English** | 🏠 [Index](../index.md)

---

# Why perci.gnl

This is a pretty personal project. It's the result of a bunch of `.sh` scripts the author already had lying around, gathered into a single Go binary.

The goal is simple: land on a fresh machine and have everything needed configured in minutes. That's why it only covers the distros actually used day to day — Linux Mint 22.3 (Cinnamon and XFCE), Ubuntu 26.04, Fedora 44, and Zorin OS 18.1 (Core and Lite) — instead of trying to be a generic, universal setup tool.

## The Problem

Setting up a Linux development workstation from scratch means repeating the same sequence over and over:

- Post-install cleanup and codec/essentials installation, which differs per distro and desktop environment.
- Installing the same set of fonts, Flatpak apps, and terminal tools.
- Installing and keeping up to date a handful of SDKs (Go, Flutter), IDEs (VS Code, VSCodium, Zed), and LLM/MCP CLI tools.
- Spinning up a Docker-based Nginx + PHP + MariaDB stack per project, especially for Moodle development — multiple PHP versions, multiple Moodle installs routed through the same stack.
- Generating consistent AI agent context files (`AGENTS.md`, `CLAUDE.md`, `.instructions/*.md`) and skill files for every new project.

Each of these used to be its own throwaway shell script, run manually, updated inconsistently, with no shared UI or safety net.

## The Approach

perci.gnl consolidates all of that into one binary with:

- **One config file** (`~/.perci/config.yaml`) instead of scattered environment variables or hardcoded paths in a dozen scripts.
- **One executor** for every privileged operation (`internal/executor`), so `sudo` usage is centralized, auditable, and consistent.
- **One GUI** (a Wails-based desktop app, `cmd/prci-gui`, five categories: Home, Linux, Desenvolvimento, Docker, Dev Tools), so every flow — forms, checklists, multi-step wizards — looks and behaves the same way.
- **No interactive prompts inside domain code** — every domain package takes plain parameters and is called directly by a Go method bound to the GUI's frontend, so there's exactly one path from a click to the actual work, not a TUI path and a separate scriptable one.

## Who It's For

Primarily: the author, on the author's machines, for the author's actual day-to-day stack (Moodle development on Debian/Fedora-family Linux). It's shared publicly and is free to use and adapt, but it deliberately doesn't try to cover every distro, every desktop environment, or every possible dev stack — see [How perci.gnl works](./how-perci-works.md) for the shape it does cover, and [System management](../architecture/system-management.md) for exactly which distros/DEs are supported.

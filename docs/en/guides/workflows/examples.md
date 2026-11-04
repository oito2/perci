🌐 [Português](../../../pt-br/guides/workflows/examples.md) | **English** | 🏠 [Index](../../index.md)

---

# End-to-End Workflow: Fresh Machine → Running Moodle Dev Environment

A complete walkthrough combining every category, start to finish.

## 1. Install perci

```bash
curl -fsSL https://raw.githubusercontent.com/oito2/perci/main/install.sh | bash
```

See [Installation](../../getting-started/installation.md) for alternatives.

## 2. Configure

```bash
prci
```

**Home → Configurações** — set your workspace path, theme, sidebar logo/app icon, and Flatpak scope. This writes `~/.perci/config.yaml`.

## 3. Post-Installation

**Linux → Atualizar Sistema**, then the post-installation entry matching your distro/DE (only one is enabled at a time — see [Installation → Supported Distributions](../../getting-started/installation.md#supported-distributions)).

Optionally: **Gerenciar Fontes**, **Gerenciar Templates de Arquivos**, **Aplicativos: Flatpak**.

## 4. Dev Environment

**Desenvolvimento →**

1. **Pré-requisitos** — compiler toolchain, Git, curl, Docker Engine, mkcert, NPM.
2. **Linguagens e SDKs** — Go and/or Flutter, if your Moodle plugins/tooling need them.
3. **IDEs** (Zed Editor, VS Code, VSCodium — one table with Instalar/Atualizar/Desinstalar per IDE), **IDE: Android Studio** or **IDE: Antigravity IDE** — whichever you use day to day.

## 5. Docker Application Stack

**Docker → Criar Container**, picking a "Tipo de contêiner" each time:

1. **Nginx** — one-time, creates the reverse proxy, the shared network, and the `*.localhost` wildcard HTTPS cert.
2. **MariaDB** — one-time, asks for a database user/password.
3. **Moodle** — Versão do Moodle `5.1+`, PHP 8.3 or 8.4, URL (e.g. `mdle.localhost`), Acesso a banco = yes.

The app container comes up automatically as part of creation — no separate "start" step. See the [Docker application stack guide](../environments/docker.md) for the full lifecycle.

## 6. Clone a Moodle Install Into It

Clone the Moodle code straight into the app's own `html` folder (`{{workspace_path}}/localhost/html/<folder>`, matching the `folder` you picked when creating the container), from a terminal:

```bash
git clone https://github.com/your-org/your-moodle-site.git ~/workspace/localhost/html/mdle
```

The container was already routed to that folder — no re-run/restart needed. A second Moodle site is a second, independent Container Aplicativo (its own folder, URL, and container), not a new entry added to a shared one.

For repositories of your own GitHub/GitLab account, set the account up in **Repositórios → Configurar Conta** and clone with **Novo Repositório** on the account's screen instead: the clone gets the account's commit identity and SSH key. See [Git accounts and repositories](./repositories.md). The Repositórios menu only clones repositories the account user owns, which is why an organization's Moodle site is cloned with `git` above.

## 7. Database Housekeeping

**Docker → Gerenciar Containers**, select the MariaDB row, then the **Backup**/**Restore** icons.

Both use `cfg.Docker.MariaDB.*` credentials against the shared MariaDB container — GUI-only, there is no command-line equivalent.

## 8. AI Context for the New Project

**Dev Tools → IA: Contextos**, pick the cloned project's folder, check **Moodle**, then click **Aplicar**.

This generates `AGENTS.md`, `.instructions/MOODLE.md` (with the Moodle version filled from `version.php`) and `.instructions/PROJECT.md`, plus each checked agent's files (e.g. `CLAUDE.md` and `.claude/rules/perci/MOODLE.md` for Claude Code). (The `php` model was removed 2026-09-11 — generic PHP guidance is now covered by third-party skills instead, e.g. "PHP: Specialist"/"PHP: Pro 8.3+" via `Dev Tools :: IA: SKILLs` in the GUI, which also replaced the old skills-apply flow — "Claude Code Mode" and the rest of `internal/manager/skills`'s own catalog were removed the same day.)

## 9. Keep Everything Up to Date

**Linux → Atualizar Sistema** updates the OS + Flatpak + Snap. To update perci itself, use the **Atualizar** action on **Home → Visão Geral**.

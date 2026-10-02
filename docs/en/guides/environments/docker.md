🌐 [Português](../../../pt-br/guides/environments/docker.md) | **English** | 🏠 [Index](../../index.md)

---

# Docker Application Stack Environment

Day-to-day operation of the per-project Docker application stack perci manages. For implementation details, see [Docker application stack architecture](../../architecture/appstack.md).

## Prerequisites

Docker itself and **mkcert** (for the `*.localhost` HTTPS certificate) both come from **Desenvolvimento → Pré-requisitos** — install both before creating the Nginx container.

## Creating Containers

Everything lives under a single **Docker → Criar Container** screen — form-driven; there is no command-line interface at all. Pick a "Tipo de contêiner" first; the fields shown depend on what you pick:

1. **Nginx** — one-time setup. Creates the shared reverse-proxy container, the `docker-php-network` Docker network, and the `*.localhost` wildcard HTTPS certificate (mkcert, generated once, covers every future app). If a `nginx` container already exists, this type is disabled in the select instead — use **Gerenciar Containers → Recriar** on the existing row.
2. **MariaDB** — one-time setup (unless you need to change credentials). Asks for a database user/password (defaults to a random password); the root password is generated once and reused on subsequent recreations. Disabled in the select once a `mariadb` container already exists, same as Nginx.
3. **Moodle / PHP / Servidor Genérico** — a Moodle, plain PHP, or "Servidor Genérico" (identical to plain PHP, just a different catalog label) project. Asks for: display name, folder name (also the container name and hostname), type, URL (`<folder>.localhost`), PHP version, and whether it needs database access. Moodle asks one extra question first — "Versão do Moodle" (`3.x`, `4.1`, `4.2-4.3`, `4.4-4.5`, `5.0`, `5.1+`) — which then narrows the PHP version step to that release's supported range (`3.x` → PHP 7.4, `4.1` → 7.4–8.1, `4.2-4.3` → 8.0–8.2, `4.4-4.5` → 8.1–8.3, `5.0` → 8.2–8.4, `5.1+` → 8.3–8.4) and picks the matching Nginx routing recipe.
4. **Node** — a standalone Node/JS project (Vue, React, Svelte, a plain Node API, or anything else that runs from a dev-server command) — no PHP involved. Asks for the same basics plus Node version, the dev-server start command (default `npm run dev`), and its port (default `5173`).
5. **PHP + Node** — a single container running both a PHP API and a Node frontend together (supervisord-managed), for a project split into `api/` (PHP) and `app/` (Node) subfolders. Routes `<folder>.localhost/api/*` to PHP and everything else to the Node dev server, both behind the same origin — no CORS setup needed. Asks for both a PHP and a Node version, plus the same dev-command/port fields as a standalone Node container.

Submitting an app type (Moodle/PHP/Generic/Node/PHP+Node) with a folder name that already exists just replaces that entry and recreates the container outright — there's no separate "already exists" confirmation dialog; submitting the form is the confirmation, same convention as every other non-interactive GUI action. All create flows end with an automatic Nginx config reload — no manual restart needed to pick up a new app.

Generates (per app): the container itself, `~/.local/bin/` CLI wrappers (`php-<folder>`, `composer-<folder>`, `npm-<folder>`, etc. depending on type), and its Nginx `server{}` block. Nothing installs your project's own dependencies (`composer install`/`npm install`) automatically — that stays manual, same philosophy as Moodle's `config.php` not being auto-generated.

## Day-to-Day Lifecycle

**Docker → Gerenciar Containers** lists every container perci knows about (Nginx, MariaDB, each app) with live status. Pick one, then an action:

- **Iniciar / Parar / Reiniciar** — straightforward lifecycle control.
- **Recriar** — reapplies the parameters already on file, no form.
- **Ver logs** — follows the container's logs (Ctrl+C to stop following). For a PHP+Node combo container, both processes' output is interleaved, prefixed by supervisord.
- **Remover** — removes the container and its `config.yaml` entry. **Never deletes `html`/`data` on disk.**
- **Editar** (MariaDB and every app type, not Nginx) — remove-and-recreate with new parameters, existing volumes preserved. The only real way to change a running container's image or PHP/Node version.
- **Backup / Restore** (MariaDB only) — dumps or restores against the shared MariaDB container using `cfg.Docker.MariaDB.*` credentials.

## Re-generating Over an Existing App

Re-running a creation flow against an already-provisioned MariaDB or app reuses what's already on file (e.g. MariaDB's root password) instead of generating fresh values, keeping credentials in sync with what's actually set on the running container/volume.

## Exporting/Importing Your Setup to Another Machine

Useful if you work across multiple workstations sharing a synced workspace (e.g. via MegaSync): **Exportar configurações** writes the whole Docker setup (network, Nginx, MariaDB credentials — in plaintext, so treat the export file like a secret — and every app) to a file you choose. **Importar configurações** on another machine reads it back and recreates everything in one action (network → Nginx → MariaDB → each app). Import refuses if the target machine already has a *different* workspace path configured; on a fresh machine with none set, it adopts the exported one.

## What's Deliberately Not Here

- No dedicated database/user per project — every app with "Acesso a banco" shares the same MariaDB credentials, network-only.
- No automatic `config.php` generation for Moodle — perci prepares the infrastructure (folders, container, network, URL); configuring Moodle itself is manual.
- No "Otimiza"/"Otimiza para Moodle" MariaDB tuning on this stack (it existed on the legacy stack; deliberately not ported — see the [architecture doc](../../architecture/appstack.md) for why).
- No custom domains outside `*.localhost`.

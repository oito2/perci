🌐 [Português](../../pt-br/reference/configuration.md) | **English** | 🏠 [Index](../index.md)

---

# Configuration Reference

Settings live at `~/.perci/config.yaml`, created the first time a setting is saved; every write goes through `config.Update` (atomic, `0600` permissions). A missing key keeps its empty value — nothing is filled in on load.

```yaml
workspace_path: ~/workspace
flatpak_scope: system
docker:
  nginx_created: true
  mariadb:
    db_user: dev_user
    db_pass: aB3xQ9zK2mP7wR1t
    db_root_pass: zK2mP7wR1taB3xQ9
  apps:
    - name: My Moodle
      folder: mdle
      type: moodle
      url: mdle.localhost
      php_version: "8.4"
      moodle_version: "5.1+"
      db_access: true
    - name: My API + SPA
      folder: myapp
      type: php_node
      url: myapp.localhost
      php_version: "8.3"
      db_access: true
      node_version: "24"
      dev_command: npm run dev
      dev_port: 5173
```

## Top-Level Keys

| Key | Type | Set via | Description |
|---|---|---|---|
| `workspace_path` | string | Home → Configurações | Root folder for projects. Accepts `~`. Set from the GUI only through the folder picker ("Criar Workspace" refuses a path that wasn't picked there). Empty means not chosen yet: the Configurações screen says so, **Importar configurações** adopts the exported file's workspace, and everything that needs a folder uses `~/workspace`. |
| `flatpak_scope` | string | Home → Configurações | `system` or `user` — determines the `--system`/`--user` flag passed to every `flatpak` invocation. |
| `docker` | object | Docker → Criar Container / Gerenciar Containers | The whole Docker application stack definition — see below. Omitted from the file entirely while empty. |

The distribution and desktop environment are always detected (`/etc/os-release`, `XDG_CURRENT_DESKTOP`); the `distro`/`de` keys older versions documented were never read and are ignored.

## `docker.*` (Docker Application Stack)

See [Docker application stack architecture](../architecture/appstack.md) for the full model this backs.

| Key | Type | Description |
|---|---|---|
| `docker.nginx_created` | bool | Whether the shared `nginx` container (reverse proxy for every app) has been created. |
| `docker.mariadb.db_user` | string | The shared MariaDB application user every `db_access: true` app connects with. |
| `docker.mariadb.db_pass` | string | That user's password — randomly generated (`GenPassword()`), distinct from `db_root_pass`. |
| `docker.mariadb.db_root_pass` | string | MariaDB root password — randomly generated, distinct from `db_pass`. Reused (not regenerated) on subsequent recreations against an already-provisioned volume. |
| `docker.mariadb.data_user` / `data_pass` | string | The credentials MariaDB's data directory was first initialized with. Kept (with `db_root_pass`) when the container is removed from the stack, so re-creating it over the same data with different credentials is refused — the image ignores new credentials on existing data. |
| `docker.apps` | list | One entry per Container Aplicativo — see below. |

### `docker.apps[]`

| Key | Type | Description |
|---|---|---|
| `name` | string | Display label ("Nome do aplicativo") — purely cosmetic. |
| `folder` | string | Folder name = container name = hostname on `docker-php-network` (technical identifier). Validated by `appstack.ValidAppFolder`. |
| `type` | string | `moodle` \| `php` \| `generic` (identical behavior to `php`, separate catalog label only) \| `node` \| `php_node`. |
| `url` | string | `<folder>.localhost`. Validated by `appstack.ValidAppURL`. |
| `php_version` | string | `7.4`–`8.4`. Used by every type except standalone `node`; for Moodle, restricted to the range `moodle_version` allows. |
| `moodle_version` | string | `moodle` only, omitted otherwise. `3.x` (PHP 7.4) \| `4.x` (PHP 8.0–8.1) \| `5.0` (PHP 8.2–8.4) \| `5.1+` (PHP 8.2–8.4). Picks the Nginx routing recipe (`appstack.moodleRoutingFor`) — `5.0` and `5.1+` share a PHP range but need different recipes, which is why this is its own field rather than derived from `php_version`. Empty/missing is treated as `5.1+`. |
| `db_access` | bool | Whether `DB_HOST`/`DB_PORT`/`DB_USER`/`DB_PASS`/`DB_NAME` env vars (from `docker.mariadb.*`) are injected into the container. Every app is reachable on `docker-php-network` regardless — this only gates whether credentials are injected. |
| `node_version` | string | `node`/`php_node` only. `22`, `24`, or `26`. Validated by `appstack.ValidNodeVersion`. |
| `dev_command` | string | `node`/`php_node` only. Free text (e.g. `npm run dev`) — the dev server's start command. Not regex-validated (it's a shell command by nature); isolated via an environment variable at the container boundary instead. |
| `dev_port` | int | `node`/`php_node` only. The dev server's port, `proxy_pass`'d by Nginx. Validated by `appstack.ValidDevPort` (non-privileged range, can't collide with 9000/fastcgi or 3306/MariaDB). |
| `php_memory_limit` | string | Every type except standalone `node`. Overrides the PHP `memory_limit` baked into the shared image (`appstack.DefaultPHPMemoryLimit`, `512M`) — e.g. `768M`, `1G`, `-1`. Validated by `appstack.ValidPHPMemoryLimit`. Empty means "use the image's default". Written to a per-app `conf.d` snippet (see below), not the image, so it survives Recriar/Editar. |
| `worker_command` | string | `php_node` only. Free text — an extra `supervisord`-managed background process (e.g. a queue/job consumer) run alongside PHP-FPM and the dev server. Empty means no extra process; the container still runs an idle placeholder in its place, since the image's `[program:worker]` section is shared across every combo app. |

## Related Files (Not in `config.yaml`)

| Path | Purpose |
|---|---|
| `~/.perci/appstack/certs/` | mkcert-generated `*.localhost` wildcard TLS certificate, created once by "Criar Container Nginx". |
| `~/.perci/appstack/nginx/default.conf` | Generated Nginx config, one `server{}` block per app in `docker.apps`; regenerated and reloaded automatically on every create/edit/remove. |
| `~/.perci/appstack/php-conf/<folder>/zz-perci-overrides.ini` | Generated per-app php.ini snippet (currently just `memory_limit`), bind-mounted read-only into the app's own container at `/usr/local/etc/php/conf.d/`; regenerated on every create/edit. |
| `{{workspace_path}}/localhost/html\|data\|logs\|databases/*` | Per-app bind-mounted folders — see [Docker application stack architecture](../architecture/appstack.md#path-conventions). |
| `<target-project>/AGENTS.md`, `CLAUDE.md`, `GEMINI.md`, `.instructions/*.md` | Generated by the GUI's **Dev Tools → IA: Contextos** screen inside a target project — not perci's own config. |
| `<target-project>/.agents/skills/<slug>/`, `~/.agents/skills/<slug>/` | Written by the `npx skills` tool (GUI-only "Dev Tools :: IA: SKILLs") for OpenCode/Antigravity/Codex — Claude Code uses `.claude/skills/`/`~/.claude/skills/` instead, not perci's own config. |
| `<target-project>/.agents/mcp_config.json`, `~/.gemini/config/mcp_config.json` | Antigravity's MCP server registry, edited directly (GUI-only "Dev Tools :: IA: MCPs", `internal/dev/mcpservers` — Claude Code/Codex use their own `mcp add`/`mcp` config instead). |

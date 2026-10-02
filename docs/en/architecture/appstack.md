🌐 [Português](../../pt-br/architecture/appstack.md) | **English** | 🏠 [Index](../index.md)

---

# Docker Application Stack (`internal/appstack`)

Backs **Docker → Criar Container** (creation) and **Docker → Gerenciar Containers** (lifecycle) — GUI-only, form-driven; there is no command-line interface at all. One container per project, instead of a single `docker-compose.yml` shared by all of them — see the [decisions log](./decisions.md) for the design rationale.

## Model

Every project is its own **Container Aplicativo**, with its own URL (`<folder>.localhost`), its own PHP and/or Node version, and isolated data folders — instead of one shared PHP-FPM fleet serving arbitrary subfolders. Containers are managed individually via `docker run`/`create`/`start`/`stop`/`rm` — there is no `docker-compose.yml` for this stack. `cfg.Docker` (`internal/config`) is the sole source of truth for *which* containers exist; `docker inspect` is only consulted for live status display.

```go
type DockerConfig struct {
    NginxCreated bool
    MariaDB      MariaDBConfig
    Apps         []AppContainer
}

type AppContainer struct {
    Name, Folder, Type, URL, PHPVersion string
    MoodleVersion                       string // AppTypeMoodle only — "3.x" | "4.1" | "4.2-4.3" | "4.4-4.5" | "5.0" | "5.1+" (or the legacy "4.x")
    DBAccess                            bool
    NodeVersion, DevCommand             string // AppTypeNode/AppTypePHPNode only
    DevPort                             int    // AppTypeNode/AppTypePHPNode only
    PHPMemoryLimit                      string // every type except AppTypeNode — empty = image default
    WorkerCommand                       string // AppTypePHPNode only — empty = no extra background process
}
```

`Type` is one of `AppTypeMoodle`, `AppTypePHP`, `AppTypeGeneric` (technically identical to `AppTypePHP` — a separate catalog label only), `AppTypeNode`, `AppTypePHPNode`.

## Path Conventions

All under `{{cfg.WorkspacePath}}/localhost/`:

| Path | Used by |
|---|---|
| `html/<folder>` | PHP-family types (Moodle/PHP/Generic) → `/var/www/html`; `AppTypeNode` → `/app` |
| `html/<folder>/api`, `html/<folder>/app` | `AppTypePHPNode` only — PHP api/ → `/var/www/html`, Node app/ → `/app` |
| `data/<folder>` | `AppTypeMoodle` only (moodledata) → `/var/www/data` |
| `logs/<folder>` | PHP-family and combo types (PHP-FPM's own error log); Node-only containers log to stdout/stderr via `docker logs`, no file |
| `databases/mariadb` | The shared MariaDB container's data directory |

## Nginx (`nginx.go`)

One `nginx:1.26-alpine` container (name `nginx`), created once via "Criar Container Nginx", bind-mounting the entire `localhost/html` folder read-only (`/var/www/localhost/html:ro`) so a new app never requires *recreating* Nginx — only reloading its config. Ports `127.0.0.1:80`/`127.0.0.1:443`.

- `EnsureNetwork`/`NetworkExists` — the shared `docker-php-network` Docker network every container joins (same name the legacy stack used; no simultaneous-coexistence concern since only one stack runs at a time).
- `EnsureWildcardCert` — a `*.localhost` wildcard cert via **mkcert** (a [Desenvolvimento → Pré-requisitos](../architecture/dev-environment.md) catalog entry, not assumed pre-installed), written once to `~/.perci/appstack/certs/`, covering every future app with no per-project cert step.
- `BuildNginxConf`/`writeNginxConf` — generates `~/.perci/appstack/nginx/default.conf` from `cfg.Docker.Apps`, one `server{}` block per app (`server_name <url>`). `buildAppServerBlock` branches on `app.Type`:
  - **Moodle/PHP/Generic** — `fastcgi_pass <folder>:9000`. PHP/Generic always get the same `root <folder>/`; Moodle's routing recipe and `root` instead depend on `MoodleVersion` (`moodleRoutingFor`) — three release-series layouts, each verified 2026-08-21 against Moodle's own docs: `3.x`/`4.x` buckets (`appMoodleClassicRouting`, `docs.moodle.org/{311,402}/en/Nginx`) serve straight from `root <folder>/` via `index.php`, no `r.php`, no `location /` rewrite at all (Moodle's own docs warn a `try_files` here would break slash-argument URLs); `5.0` (`appMoodleR50Routing`, `docs.moodle.org/500/en/Nginx`) still has `root <folder>/` but introduces `r.php` as the front controller; `5.1+` (`appMoodleRouting`, `docs.moodle.org/502/en/Nginx`) additionally moves `root` to `<folder>/public` (the restructure `moodledev.io/docs/5.1/guides/restructure` describes) — simpler than the legacy stack's version of this recipe either way, since each app already has its own dedicated vhost, no `SCRIPT_NAME` rewrite needed. An empty/unrecognized `MoodleVersion` (pre-existing `config.yaml` entries from before this field existed) falls back to the `5.1+` recipe, the only one Container Aplicativo ever generated before.
  - **Node** — a single `location / { proxy_pass http://<folder>:<devport>; ... }` with WebSocket upgrade headers (`Upgrade`/`Connection: upgrade`), no `root`/`fastcgi_pass` — needed for Vite/webpack-dev-server HMR to survive behind the proxy.
  - **PHP+Node combo** — `location /api/` (fastcgi, no rewrite — the Slim app itself defines routes under the `/api` prefix) plus `location /` (proxy_pass to the dev server, same upgrade headers as Node), plus `location ^~ /uploads/ { alias <mount>/<folder>/api/uploads/; }` serving user-uploaded files straight off Nginx's own filesystem — otherwise they'd fall through to the `location /` proxy and 404 against the dev server (real failure on the learnerflow app, 2026-08-28). `root` stays at the project folder level (not `.../api`) so `root`+URI resolves `/api/index.php` correctly.
  - Security blocks (`appDenyBlocks` — dotfiles except `.well-known`, `vendor/`, `node_modules/`, `composer.json`, etc.) apply only to types that serve files straight off Nginx's own filesystem (Moodle/PHP/Generic). Node gets no deny block at all — its `location /` is a pure `proxy_pass`, and nginx dispatches regex `location` blocks like these ahead of any `location /` prefix regardless of write order, so leaving them in would 404 the dev server's own legitimate paths (Vite's `/node_modules/.vite/deps/...` module cache, hit during Fase 6 testing, 2026-08-24). The combo gets a separate `appComboDenyBlocks`, anchored to `^/api/`, protecting only its fastcgi half. An entry with an invalid `Folder`/`URL`/`DevPort` is skipped during generation rather than emitted malformed — same defense-in-depth already applied to `ValidDBIdentifier` in the legacy stack.
  - Every `fastcgi_pass`/`proxy_pass` above goes through a `set $backend <folder>[:<port>];` variable, paired with a `resolver 127.0.0.11 valid=10s;` (Docker's own embedded DNS) at the top of the generated file — not a literal hostname. Nginx otherwise resolves a literal upstream hostname once, eagerly, at `nginx -t`/reload time, so any *other* app merely being stopped would fail validation (`host not found in upstream`) and revert the whole file; the variable defers resolution to request time instead — confirmed against a real failure during Fase 3/4 end-to-end testing (2026-08-24).
  - `SCRIPT_FILENAME`/`DOCUMENT_ROOT` on every fastcgi block are hardcoded to the app container's own mount path (`/var/www/html`, or `/var/www/html/public` for the `5.1+` `/public` layout) rather than derived from Nginx's `$document_root`/`$realpath_root` — Nginx and each app's php-fpm are separate containers with different mounts of the same project (Nginx sees the whole `localhost/html` tree at once; the app container always sees just its own project root at the fixed `/var/www/html`), so a value built from Nginx's own filesystem view resolves to nothing on php-fpm's side. The combo's `/api/` block captures the URI's post-`/api` remainder via its location regex instead, since only `api/` — not the whole project — is mounted into that container.
- `ReloadNginxConfig` — regenerates, `nginx -t`, `nginx -s reload`; reverts the file if the test fails. Runs automatically after every create/edit/remove of any app, MariaDB, or Nginx itself.

## MariaDB (`mariadb.go`)

One `mariadb:11.4` container (name `mariadb`), port `127.0.0.1:3306`, data at `localhost/databases/mariadb` — a **new** path, deliberately not shared with the legacy stack's `workspace/databases/mariadb` (no automatic migration). `cfg.Docker.MariaDB.{DBUser,DBPass,DBRootPass}` holds the shared credentials; every `AppContainer` with `DBAccess=true` gets network-only access using them (`GRANT ALL … WITH GRANT OPTION`) — no per-project database or user. The credentials reach `docker run` as bare `-e MYSQL_ROOT_PASSWORD`/`-e MYSQL_USER`/`-e MYSQL_PASSWORD` names, with the values in the docker client's own environment (`executor.Options.Env`) — never in the argv, which any local user can read via `ps`/`/proc/<pid>/cmdline`. `ValidDBIdentifier` (`^[A-Za-z0-9_]{1,32}$`) and `GenPassword()` (16-char random) live natively here — originally reexported from the legacy `internal/stack/config`, migrated in when that package was removed.

## PHP Images (`image.go`)

`SupportedPHPVersions = 7.4, 8.0, 8.1, 8.2, 8.3, 8.4`. One `perci-php<version>` image per version, built on first use (`EnsureImage`) and reused by every app on that version. Each image carries a `perci.hash` label — a hash of its Dockerfile, baked-in config files (`php.ini`, `supervisord.conf`) and build args; when an existing image's label doesn't match what this Perci would build (an image from an older version, or a changed template), it's rebuilt automatically, and the terminal names the existing containers that keep the old image until they're recreated. The same applies to the Node and combo images. `AppTypeMoodle` additionally narrows the offered PHP versions by `MoodleVersion` (`PHPVersionsForMoodleVersion`/`ValidPHPVersionForMoodleVersion`): `3.x` → 7.4 only, `4.1` → 7.4–8.1, `4.2-4.3` → 8.0–8.2, `4.4-4.5` → 8.1–8.3, `5.0` → 8.2–8.4, `5.1+` → 8.3–8.4 (it covers 5.1–5.3, and 5.2/5.3 require PHP 8.3); the legacy `4.x` keeps the 4.1 range for containers saved with it — enforced both in the GUI's "Versão do Moodle"/"Versão PHP" form fields and again in `CreateApp`. `MoodleVersion` has to be its own persisted `AppContainer` field rather than derived from `PHPVersion` because `5.0` and `5.1+` now share a PHP range yet need different Nginx recipes (see above); the GUI's own edit-flow inference for pre-`MoodleVersion`-field entries checks `5.1+` before `5.0` on an ambiguous PHP-range match, to keep its long-standing fallback to `5.1+` intact.

`phpIni` bakes `memory_limit = DefaultPHPMemoryLimit` (`512M`) into every image. An individual app can override it via `AppContainer.PHPMemoryLimit` (`ValidPHPMemoryLimit`, e.g. `768M`/`1G`/`-1`) without rebuilding the shared image: `WritePHPMemoryLimitConf` writes `~/.perci/appstack/php-conf/<folder>/zz-perci-overrides.ini` (just `memory_limit = <value>`), bind-mounted read-only into the app's own container at `/usr/local/etc/php/conf.d/` — the same conf.d mechanism `docker-php-ext-enable` already uses to layer extension ini files at build time, just per-app and at run time instead. The file is always mounted, even when `PHPMemoryLimit` is empty (it then just repeats the default) — regenerated on every create/edit, so it survives Recriar/Editar, unlike an edit made by hand inside a running container (found on the learnerflow app, 2026-08-28: a manual `php.ini` bump was silently lost on the next recreate).

## Node Images (`node.go`)

`SupportedNodeVersions = 22, 24, 26`. One `perci-node<version>` image per version (`EnsureNodeImage`), built from the official `node:<version>-bookworm` image with `usermod`'d UID matching the host, same reasoning as PHP's `www-data` UID handling.

## PHP+Node Combo Images (`combo.go`)

`EnsureComboImage` builds `perci-php<phpVersion>-node<nodeVersion>` — the same PHP base (`phpDockerfile`/`phpIni`, see below) plus Node (official NodeSource apt script) plus `supervisor` (Debian package). **supervisord** runs three managed processes:

```ini
[supervisord]
logfile=/dev/null
pidfile=/var/run/supervisord.pid

[program:php-fpm]
command=php-fpm -F
[program:dev-server]
command=%(ENV_DEV_COMMAND)s
directory=/app
user=www-data
[program:worker]
command=sh -c "if [ -n \"$WORKER_COMMAND\" ]; then exec $WORKER_COMMAND; else exec sleep infinity; fi"
directory=/var/www/html
user=www-data
```

`[supervisord]` is mandatory — `comboDockerfile`'s `CMD` points `supervisord -c` straight at this file rather than Debian's own `/etc/supervisor/supervisord.conf` (which supplies that section via its own `[include]`), so this has to be a complete, self-contained config, not just a program-definitions fragment; every combo container crash-looped on `Error: .ini file does not include supervisord section` until this was added (confirmed 2026-08-24).

`DevCommand` (e.g. `npm run dev`) is passed as an environment variable (`-e DEV_COMMAND=...`) rather than interpolated into the config file — `supervisord`'s own `%(ENV_X)s` syntax reads it at process-start time (confirmed against `supervisord.org`, 2026-08-21), so a free-text value from the user never touches `.conf` file syntax. The config itself never varies between containers, so it's baked into the image rather than bind-mounted per project.

`[program:worker]` is `AppContainer.WorkerCommand` — an optional extra background process (e.g. a Symfony Messenger or Laravel queue consumer), added 2026-08-28 while wiring up an async job queue on the learnerflow app. Unlike `dev-server`, its `command=` is a fixed shell wrapper baked into the image rather than `%(ENV_WORKER_COMMAND)s` directly: supervisord refuses to start a program whose referenced `%(ENV_X)s` doesn't exist in the environment at all, which would break every combo app that leaves `WorkerCommand` empty (the common case). `comboAppRunArgs` always sets `WORKER_COMMAND` (possibly to `""`), and the wrapper reads it from its own inherited shell environment at run time instead — running it through its own `sh -c` when non-empty (so variables, `&&` and pipes work as typed), or idling on `sleep infinity` otherwise, so the program still starts cleanly either way.

`phpDockerfile`/`phpIni` (in `image.go`) are the single source of truth for the PHP base every PHP-family image (standalone and combo) builds from — originally reexported from the legacy `internal/stack/config`, migrated in natively when that package was removed.

## Apps (`app.go`, `node.go`, `combo.go`)

`CreateApp` dispatches on `app.Type`: ensures the network, the right image(s), the path structure, runs the container, writes CLI wrappers, persists the entry into `cfg.Docker.Apps` (replacing any existing entry for the same `Folder` rather than duplicating), and reloads Nginx. `RemoveApp` never deletes `html`/`data` on disk — only the container and the config entry.

- **Moodle/PHP/Generic** — `appRunArgs`: mounts `html`(+`data` for Moodle) and the per-app `php-conf/.../zz-perci-overrides.ini` (`WritePHPMemoryLimitConf`, see above), injects `DB_HOST`/`DB_PORT`/`DB_USER`/`DB_PASS`/`DB_NAME` only when `DBAccess=true` (as `-e NAME` with the values in the client environment, same as MariaDB's credentials) (network reachability on `docker-php-network` is unconditional for every app type — `DBAccess` only gates whether credentials are injected).
- **Node** — `nodeAppRunArgs`: mounts the whole project folder at `/app` (no fixed docroot convention across Node tooling), runs as the image's own `node` user (UID-matched), `DevCommand` becomes the container's `CMD` via `sh -c "<command>"` as a single argv element — never interpolated into a host shell. No PHP, so no `php-conf` mount either.
- **PHP+Node combo** — `comboAppRunArgs`: mounts `api/`, `app/` and the per-app `php-conf/...` override separately, injects `DEV_COMMAND` and `WORKER_COMMAND` (always, even empty — see `[program:worker]` above) plus `DB_*` if `DBAccess`, via `-e`, runs as root (supervisord itself needs that to drop privileges per-process — `php-fpm`'s master has no `user=` in its supervisord program block, same implicit root-then-drop behavior every PHP-FPM container already has; `dev-server`/`worker` explicitly run as `www-data`, reusing the same UID-matched user `phpDockerfile` already sets up, instead of a separate `node` user).

## CLI Wrappers (`wrappers.go`)

One set of `~/.local/bin/` wrappers per project, each a thin `docker exec` into that project's container: `php-<folder>`, `composer-<folder>`, `phpunit-<folder>`, etc. for PHP-family types; `npm-<folder>` for Node/combo. A combo app gets both sets side by side, pointing at the same container. PHP-family and combo wrappers run as `www-data` (UID-matched to the host user, `HOME=/tmp`), so `vendor/` and `node_modules/` aren't root-owned on the host; a Node-only container already runs as its `node` user. Deleting an app removes its wrappers; rewriting one replaces the file rather than following a symlink at that path.

## Export/Import (`export.go`)

Lets the whole `cfg.Docker` definition (including MariaDB credentials, in plaintext — an explicit, informed trade-off given the file only travels over a channel the user already controls, e.g. MegaSync) travel to another machine and be replicated in one action — useful across multiple workstations sharing a synced workspace. `ImportConfig` reapplies the trava (lock): refuses if the importing machine already has a *different* `WorkspacePath` configured; adopts the exported path on a fresh machine with none set yet. Recreates everything (network, PHP/Node images, Nginx, MariaDB, every app) by reusing the same `Recreate*` functions "Containers Docker" itself uses — no separate creation logic.

## Docker → Gerenciar Containers

`GetContainerRows` (`cmd/prci-gui/service_docker.go`, part of `DockerService`) lists every container `cfg.Docker` currently knows about (Nginx, MariaDB, each app) with live `docker inspect` status — display only; the config file remains the source of truth for the *list*. Per-container actions: Start/Stop/Restart/Recreate/Ver logs/Remover for every type; **Editar** for MariaDB and every app type (Nginx has no editable parameter of its own, so Editar is identical to Recriar there and was omitted); **Backup**/**Restore** for MariaDB only, delegating to [`manager/db`](./managers.md#managerdb--database-operations) with `cfg.Docker.MariaDB.*` credentials. Editar is really "remove and recreate with new parameters, volumes preserved" — the only real way to change a running container's image/version — dispatched by the entry's actual `Type` so editing a Node container never opens the PHP-family form by mistake.

## Security

New free-text fields get the same defense-in-depth already applied to the legacy stack's `ValidDBIdentifier`/`ValidMoodleFolderName`:

| Validator | Guards |
|---|---|
| `ValidAppFolder` (`^[A-Za-z0-9_-]{1,64}$`) | Folder/container/hostname |
| `ValidAppURL` | `.localhost` suffix + allowed characters, before it reaches `server_name` |
| `ValidDBIdentifier` | Any identifier still touching SQL |
| `ValidDevPort` | Non-privileged range, rejects collision with 9000 (fastcgi) and 3306 (MariaDB) |

Every app is validated as a whole by `ValidateApp` (each validator above, supported PHP/Node/Moodle versions and their compatibility, plus the reserved folder names `nginx`/`mariadb`, which would collide with the infrastructure containers) **before** anything is removed: Editar (`ValidateAppInStack`, which also refuses a URL already used by another app), Recriar and Importar all check first, so invalid input never leaves a running app down. The operations that change the stack and regenerate Nginx's routing (create/recreate/delete an app, import) are serialized by `stackMu`, so two of them can't write `default.conf` from a stale app list.

`ImportConfig` validates the whole file (`validateExported`: every app, unique folders and URLs, the MariaDB user, an absolute workspace path) before saving anything; apps registered locally but missing from the file leave the stack with a warning naming their containers (never removed), and Nginx is always reloaded at the end.

MariaDB only applies its credentials to an empty data directory. `data_user`/`data_pass` record the ones it was initialized with (kept, with `db_root_pass`, when the container is removed from the stack), and `CreateMariaDB` — checked by Editar before removing the running container — refuses different credentials over initialized data, pointing to `ALTER USER` or removing the data directory.

`DevCommand` is the one genuinely free-text field with no character whitelist — it's a shell command by nature — isolated instead via the environment-variable indirection described above, not regex validation.

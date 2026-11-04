# Docker / Compose Standard

Standard for a project's own `Dockerfile`s, `.dockerignore` and Compose files: secure, small, reproducible images and a predictable local stack.

Last verified: 2026-11-04

---

## Scope

- This standard covers the images and Compose files the **project itself** ships or uses.
- When the project runs in **Perci-managed containers** (PHP/Node/MariaDB/Nginx created by Perci), the notes in the project's language standard (e.g. the Docker sections of `MOODLE.md` / `PHP-SLIM.md`) apply to the local environment: use the `~/.local/bin/<tool>-<folder>` wrappers, and do not add a Dockerfile or Compose file that duplicates or replaces those containers unless the user asks.
- Confirm flags against the installed versions (`docker version`, `docker buildx version`, `docker compose version`).

---

## Dockerfile Basics

- Start every Dockerfile with `# syntax=docker/dockerfile:1`: BuildKit fetches the latest stable 1.x frontend, so features such as `RUN --mount`, `COPY --chmod` and build checks are available regardless of the Engine version.
- Use **multi-stage builds**: compile and fetch dependencies in a named build stage (`FROM ... AS build`), copy only the artifacts into a minimal runtime stage. Stage names are lowercase.
- Use **exec form** (JSON array) for `ENTRYPOINT`, `CMD` and `HEALTHCHECK CMD`, so the application runs without a wrapping shell and receives `SIGTERM` directly.
- Prefer `COPY` over `ADD`; use `ADD` only for its specific features (remote Git context, `--checksum`-verified URL, auto-extracting a local tar).
- Set `SHELL ["/bin/bash", "-o", "pipefail", "-c"]` (or `/bin/sh -o pipefail` where supported) before a `RUN` that pipes, so a failing left-hand command fails the build.

---

## Base Images

| Choice | Use for | Trade-off |
|---|---|---|
| Official / Docker Official Images, `-slim` (Debian) | Default for interpreted runtimes (PHP, Node, Python) | glibc, wide compatibility, larger than Alpine |
| Alpine | Small images where the stack is tested on it | **musl** libc: different DNS resolver, locale and performance behavior; prebuilt glibc binaries/wheels may not work |
| Distroless / `scratch` | Static binaries (Go with `CGO_ENABLED=0`), JVM, Node | No shell or package manager: debug with a separate debug image tag; `scratch` also lacks CA certificates and `/etc/passwd` unless copied in |

- Use official or verified-publisher images. Pin a **specific tag and digest**: `FROM node:22.11-bookworm-slim@sha256:<digest>`. The tag documents intent; the digest makes the build reproducible. Never use `latest` or an untagged image.
- Get the digest with `docker buildx imagetools inspect <image>:<tag>`; update pins deliberately, then rebuild and test.
- Pin OS packages where practical (`pkg=<version>`) and rebuild regularly for security updates.

---

## Layers, Cache & Size

- Order instructions from least to most frequently changing: base image, system packages, dependency manifests (`go.mod`/`go.sum`, `package*.json`, `composer.json`/`composer.lock`), dependency install, then the source code.
- Install system packages in a single `RUN` with cleanup in the same layer: `apt-get update && apt-get install -y --no-install-recommends ... && rm -rf /var/lib/apt/lists/*`; `apk add --no-cache ...`.
- Use **cache mounts** for package-manager caches instead of baking them into layers:

```dockerfile
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go build -trimpath -o /out/app ./cmd/app
```

- `apt` cache mounts need `sharing=locked` and removing `/etc/apt/apt.conf.d/docker-clean`.
- Install only production dependencies in the runtime stage (`npm ci --omit=dev`, `composer install --no-dev --classmap-authoritative`).

---

## `.dockerignore`

- Every build context has a `.dockerignore` at its root (or `<Dockerfile>.dockerignore` next to a specific Dockerfile).
- Exclude at least (or use an allow-list, `*` then `!src/`): `.git`, dependency folders rebuilt in the image (`node_modules`, `vendor`), build output, local env files (`.env*`), secrets and keys, IDE folders, logs.

---

## Secrets

- **Never** pass secrets through `ARG`, `ENV`, `COPY` or a file deleted in a later layer: they persist in the image config or history.
- Build-time secrets use secret mounts; they exist only for that `RUN` (default path `/run/secrets/<id>`):

```dockerfile
RUN --mount=type=secret,id=npm_token,env=NPM_TOKEN npm ci
```

```bash
docker build --secret id=npm_token,env=NPM_TOKEN .
```

- Private Git dependencies use `RUN --mount=type=ssh` with `docker build --ssh default .`.
- Runtime secrets are mounted at run time, never baked into the image.

---

## Users & Permissions

- Run as a **non-root user with a fixed UID/GID** (e.g. `10001`) on a port above 1024; set `USER 10001:10001` (numeric IDs also work on images without `/etc/passwd`).
- Distroless images provide a `nonroot` user (UID 65532); use it.
- Copy files with the right owner and mode in one step: `COPY --chown=10001:10001 --chmod=0755 entrypoint.sh /usr/local/bin/`. Application code is owned by root and read-only to the runtime user; only data directories are writable.

---

## Process, Signals & Health

- PID 1 does not get default signal handlers and must reap zombies. If the main process cannot do that (shell scripts, apps that spawn children), add an init: `tini` as `ENTRYPOINT ["/usr/bin/tini", "--", ...]`, or `docker run --init` / Compose `init: true`.
- Entry-point scripts end with `exec "$@"` so the application replaces the shell and receives signals.
- The app shuts down cleanly on `SIGTERM` within the stop timeout (10 s by default); set `STOPSIGNAL` only when it expects another signal.
- `HEALTHCHECK` checks real readiness with a tool present in the image (distroless: ship a health subcommand in the binary; defaults: `interval` 30s, `timeout` 30s, `start-period` 0s, `retries` 3; `--start-interval` requires Engine 25+):

```dockerfile
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD ["/app/healthcheck"]
```


---

## Metadata & Reproducibility

- Label images with OCI annotation keys: `org.opencontainers.image.source`, `.version`, `.revision`, `.licenses`, `.title`, `.description`. Pass dynamic values as build args (`--build-arg VERSION=...`) or `docker buildx build --label`.
- Reproducible builds: digest-pinned bases, lock files, and `SOURCE_DATE_EPOCH=$(git log -1 --format=%ct)`; the image exporter option `rewrite-timestamp=true` (BuildKit 0.13+) applies it to file timestamps.

---

## Supply Chain & Scanning

- Attach attestations when publishing: `docker buildx build --sbom=true --provenance=mode=max --push ...` (minimal provenance is added by default). Attestations need the containerd image store or a `docker-container` builder.
- Scan images before release and in CI with the tool the project already uses — **Docker Scout** (`docker scout cves`) or **Trivy** (`trivy image`) — and fail on high/critical findings. Fix by updating the base image or package, not by suppressing.
- Lint Dockerfiles with **hadolint** and BuildKit **build checks** (`docker build --check`, Buildx 0.15+). `# check=error=true` below the syntax line turns check warnings into build failures.
- Suppress a rule only with a reason: `# hadolint ignore=DL3008` above the instruction.

---

## Multi-Platform Builds

- Build with `docker buildx build --platform linux/amd64,linux/arm64 ...`; only list platforms the project actually supports and tests.
- Prefer cross-compilation over QEMU emulation: `FROM --platform=$BUILDPLATFORM golang:... AS build`, then use the predefined `TARGETOS`/`TARGETARCH` args (`ARG TARGETOS TARGETARCH`, `GOOS=$TARGETOS GOARCH=$TARGETARCH go build ...`).
- Multi-platform images need the containerd image store (default on Engine 29+) or a `docker-container` builder (`--push`).

---

## Compose

- File name: `compose.yaml` (canonical; `docker-compose.yml` is legacy). Local-only changes go in `compose.override.yaml`, which `docker compose` merges automatically; other variants are selected with `-f`.
- **No top-level `version:`** — it is obsolete, ignored and triggers a warning. Use the `docker compose` (v2) CLI, not `docker-compose`.
- Declare `services`, named `networks` and named `volumes` explicitly; set a top-level `name:` only when the default (folder name) is not stable.
- Pin every `image:` to a version tag (digest for production-like stacks); never `privileged: true`.
- Start order follows readiness, not just creation:

```yaml
services:
  app:
    build: .
    depends_on:
      db:
        condition: service_healthy
    ports:
      - "127.0.0.1:8080:8080"
    env_file:
      - path: .env
        required: false
    secrets:
      - db_password
  db:
    image: mariadb:11.4
    healthcheck:
      test: ["CMD", "healthcheck.sh", "--connect", "--innodb_initialized"]
      interval: 10s
    volumes:
      - db-data:/var/lib/mysql
    secrets:
      - db_password

volumes:
  db-data:

secrets:
  db_password:
    file: ./secrets/db_password.txt
```

- **Ports for local development bind to `127.0.0.1`**; a bare `"8080:8080"` publishes on every interface and bypasses host firewalls such as UFW.
- **Volumes vs bind mounts:** named volumes for database and persistent data; bind mounts only for source code in development and for read-only config (`:ro`).
- **Environment & secrets:** commit a `.env.example`, never `.env`. Passwords and tokens go through `secrets:` (mounted at `/run/secrets/<name>`) and images that support `*_FILE` variables, not plain `environment:` values.
- **Profiles** (`profiles: [tools]`) for optional services (admin UIs, mail catchers, debug tools); start them with `--profile`.
- **Resource limits** for services that can exhaust the host: `deploy.resources.limits` (`cpus`, `memory`) or `mem_limit`.
- Overrides: `ports` lists are concatenated across files and `volumes` merge by container path; use `!reset` / `!override` to clear or replace a value instead of fighting merge rules.

---

## Runtime Security

- Least privilege by default, relaxed only for a documented reason:

```yaml
    user: "10001:10001"
    read_only: true
    tmpfs:
      - /tmp
    cap_drop: [ALL]
    security_opt:
      - no-new-privileges:true
```

- Add back only the capabilities the service needs (`cap_add: [NET_BIND_SERVICE]`); avoid `network_mode: host` and `pid: host`.
- **Do not mount the Docker socket** (`/var/run/docker.sock`): it grants root on the host. If a tool truly needs it, ask the user, mount it read-only and prefer a filtering socket proxy.

---

## Anti-Patterns

| Anti-pattern | Why | Instead |
|---|---|---|
| `FROM image:latest` or no tag | Builds change silently | Tag + digest |
| `ARG`/`ENV` for passwords or tokens | Persisted in image history | `RUN --mount=type=secret`, Compose `secrets` |
| `COPY . .` before installing dependencies | Every code edit invalidates the dependency layer | Copy manifests, install, then copy source |
| No `.dockerignore` | Leaks `.git`, `.env`, keys; slow context | Ignore list or allow-list |
| Running as root | Container escape = host root | Non-root `USER` with fixed UID/GID |
| Shell-form `CMD app` / script without `exec` | App never receives `SIGTERM`; killed after timeout | Exec form, `exec "$@"`, `tini`/`init: true` |
| `ports: "3306:3306"` in local dev | Database exposed to the network | `"127.0.0.1:3306:3306"` or no published port |
| `depends_on` without a condition | App starts before the dependency is ready | `condition: service_healthy` + `healthcheck` |
| Mounting `/var/run/docker.sock` | Full host control | Avoid; ask first |

---

## Quality

Run before reporting any Dockerfile or Compose change as done:

```bash
hadolint Dockerfile                          # or: docker run --rm -i hadolint/hadolint < Dockerfile
docker build --check .                       # BuildKit build checks, no image produced
docker compose config --quiet                # validates compose.yaml (+ overrides); prints nothing on success
docker build -t <app>:dev .
docker compose up -d --wait                  # waits until services are running/healthy
docker scout cves --only-severity critical,high --exit-code <app>:dev
# or: trivy image --severity HIGH,CRITICAL --exit-code 1 <app>:dev
```

Stop the stack afterwards with `docker compose down` (add `-v` only when the user agrees to delete volume data).

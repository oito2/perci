# Documentation Complement — Web Applications

## Overview

Type-specific documentation requirements for **web applications** (frontend, backend, or full-stack). This complements the base standard in `.instructions/PROJECT-DOCUMENTATION.md` — apply both. Like the base, it's a standard for when documentation is created or updated, not a task to run on its own.

---

## 1. Environment Variables

Document every environment variable from `.env.example` (or the equivalent config template) in `docs/en/reference/environment.md`, and link to it from the README's quick installation:

| Variable | Required | Default | Description |
|---|---|---|---|
| `DATABASE_URL` | Yes | — | Connection string for the primary database |

- Cross-check the table against the code that reads the variables — flag variables used in code but missing from `.env.example` (and vice versa) to the user instead of silently documenting one side.
- **Never** copy real values from `.env` or any secret into the documentation — only names, placeholders and descriptions.
- The quick install shows the `cp .env.example .env` step.

---

## 2. Scripts

Document the project's scripts in a table, from `package.json` (`dev`, `build`, `start`, `test`, `lint`, ...), `composer.json` `scripts`, a `Makefile`, or the equivalent — what each does and when to use it. Put the everyday ones (`dev`, `build`, `start`) in the README; the full table in `docs/en/reference/`.

---

## 3. Deployment

Write deployment instructions in `docs/en/guides/environments/` — one page per target **the project actually supports** (check for a `Dockerfile`/`compose.yaml`, `vercel.json`, `netlify.toml`, Nginx/Apache config, CI deploy workflows):

- **Docker:** building the image, running with the required env vars/volumes/ports, the compose workflow if present.
- **Vercel / Netlify:** project settings, build command, output directory, env vars to configure in the dashboard.
- **Nginx (or another reverse proxy):** the server block/vhost, TLS, and how the app process is run and supervised (systemd, PM2, PHP-FPM, ...).

Also cover, when they apply: database migrations/seeding, the build output directory, health-check endpoints, and background workers/cron jobs. Targets without real support in the project are not documented.

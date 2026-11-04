# Documentation Complement — Web Applications

Type-specific documentation requirements for **web applications and websites** (frontend, backend, or full-stack). Complements `.instructions/PROJECT-DOCUMENTATION.md` — apply both; this file lists only what it adds.

Last verified: 2026-11-04

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
- **GitHub Pages** (static output only): the deploy workflow, the **base path** and the **custom domain** setup the project uses. The facts to document (workflow actions and permissions, `/<repo>/` base path, `CNAME`/DNS rules, *Enforce HTTPS*) are in `.instructions/WEBSITE.md` → Hosting → GitHub Pages when the project has it; otherwise verify them against GitHub's Pages documentation.

Also cover, when they apply: database migrations/seeding, the build output directory, health-check endpoints, and background workers/cron jobs. Targets without real support in the project are not documented.

---

## Quality

In addition to the base standard's checks:

```bash
# Compare variables read in code with .env.example (Node example; adapt the pattern to the language)
diff <(grep -rhoE 'process\.env\.[A-Z0-9_]+' src | sed 's/process\.env\.//' | sort -u) \
     <(grep -oE '^[A-Z0-9_]+' .env.example | sort -u)
```

- [ ] `docs/en/reference/environment.md` matches `.env.example` and the code; no real secret values.
- [ ] Every documented script exists in `package.json`/`composer.json`/`Makefile`.
- [ ] One deployment page per target the project really supports, with commands verified against its config files.

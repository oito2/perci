# Bilingual Documentation Site Standard

Standard root files, README shape, license handling, GitHub community files and a documentation site under `docs/`, in English and Brazilian Portuguese side by side. Follow it whenever documentation is created or updated; generate or restructure a project's documentation only when the user asks for it.

Last verified: 2026-11-04

This is the **base** standard. Complements (`PROJECT-DOCUMENTATION-MCP.md`, `-MOODLE.md`, `-DESKTOP.md`, `-MOBILE.md`, `-WEB.md`, `-GAME.md` under `.instructions/`) add type-specific requirements. Apply both; on a conflict, the complement's more specific rule wins for its topic.

- **Use it** for projects meant for public/open-source distribution, when scaffolding docs or extending docs that already follow this shape.
- **Don't use it** for internal tools or throwaway scripts (a `README.md` is enough), and don't restructure an existing, different documentation layout without asking.
- **Document only what exists.** Every command, flag, path, version and feature is verified against the project's code, build files and configuration. When a section depends on something the project doesn't support, follow that section's rule (omit it, or say it isn't supported) — never fill it with plausible guesses.
- CHANGELOG handling, updating docs in the same change, and the ban on git/`gh` write commands come from `AGENTS.md` and are not repeated here.

---

## 1. Format

- Headings, short lists, **bold** key terms, fenced code blocks with a language tag; tables for anything with 3+ comparable attributes.
- The repository root holds only the root files of Section 2; everything else lives under `docs/`.
- English at the root and in `docs/en/`; Brazilian Portuguese in `docs/pt-br/`. Never mix languages in one file.

---

## 2. Directory Structure

```text
{{PROJECT_NAME}}/
├── README.md                    # English canon — shape in Section 4
├── CONTRIBUTING.md              # English canon
├── CODE_OF_CONDUCT.md           # English canon (Contributor Covenant)
├── SECURITY.md                  # GitHub-hosted projects — Section 7
├── LICENSE                      # Section 5
├── .github/                     # GitHub-hosted projects — Section 7 (English only)
└── docs/
    ├── en/
    │   ├── index.md             # curated landing page
    │   ├── prompts.md           # only when a complement requires it or it is genuinely useful
    │   ├── getting-started/     # installation.md, quickstart.md, prerequisites.md*, uninstallation.md*
    │   ├── concepts/            # why-{{PROJECT_NAME}}.md, how-{{PROJECT_NAME}}-works.md, architecture.md, glossary.md
    │   ├── architecture/        # deep internals for contributors, one file per subsystem
    │   ├── guides/              # clients/*, workflows/examples.md, environments/*
    │   ├── reference/           # exhaustive CLI/API/config reference
    │   └── troubleshooting/     # common-issues.md
    └── pt-br/
        ├── leiame.md            # mirrors README.md
        ├── contribuindo.md      # mirrors CONTRIBUTING.md
        ├── codigo-de-conduta.md # mirrors CODE_OF_CONDUCT.md
        ├── seguranca.md         # mirrors SECURITY.md, if present
        └── ...                  # every docs/en/ file, same subfolders and filenames
```

`*` = conditional.

1. **Root files** are the English canon and live only at the root. Complements may add root files of their own (e.g. `CREDITS.md` in `PROJECT-DOCUMENTATION-GAME.md`); no other files go at the root.
2. **Portuguese mirrors of root files** live in `docs/pt-br/` as `leiame.md`, `contribuindo.md`, `codigo-de-conduta.md`, `seguranca.md` — the **only** translated filenames. `LICENSE`, complement reference files (e.g. `CREDITS.md`) and `.github/` files have no mirror.
3. **`docs/pt-br/` mirrors `docs/en/` file-for-file**, same subfolders and filenames (`docs/pt-br/concepts/architecture.md`, never `arquitetura.md`). Add, remove or change a page in both trees in the same change; a new subfolder is created in both and added to both `index.md` files.
4. `concepts/architecture.md` (one overview for users) and `architecture/` (per-subsystem internals for contributors) are different documents — don't merge them.
5. Conditional files and folders (`prerequisites.md`, `uninstallation.md`, `guides/clients/`, `guides/environments/`, `prompts.md`) exist only when needed; no empty placeholders. Add other pages (migration guide, FAQ, ...) under the fitting section when the project needs them.

---

## 3. Content by Section

| Section | Content |
|---|---|
| `getting-started/` | Shortest path to "it works". `installation.md`: every supported platform and method, including building from source. `prerequisites.md`: environment setup longer than a few commands. `uninstallation.md`: complete removal (binaries, config/data dirs, services, desktop entries, registered integrations) whenever installing leaves anything behind. |
| `concepts/` | The problem, why the project exists, how it works, a glossary. `architecture.md` covers **data flow**, **folder/package layout**, **main dependencies** (and why), **design decisions** (and rationale). |
| `architecture/` | Storage, internal data flow, module boundaries. |
| `guides/` | Task-oriented how-tos: per-client setup, per-environment deployment, end-to-end workflow examples. |
| `reference/` | Facts only: every CLI flag, API parameter and config key. |
| `troubleshooting/` | Errors and fixes, indexed by symptom. |
| `prompts.md` | One page of example prompts in natural language; its shape is defined by the complement that requires it. |

---

## 4. `README.md`

The README is the front door, not the full documentation. Sections in this order (5 and 6 are conditional — omit them and their TOC entries when the project doesn't support them):

1. **Header** — H1 project name; 1–2 sentence pitch; one line of `shields.io` badges for things the project really has (release, language/runtime + minimum version, license, platform, CI); the language switcher (Section 6).
2. **Table of Contents** — anchor links to this file's own sections, in the file's language.
3. **Overview** — short prose summary plus a list of main features.
4. **Prerequisites & Quick Installation** — requirements with minimum versions and step-by-step clone/install/build/run; install hard prerequisites inline; link to `docs/en/getting-started/installation.md` for the rest.
5. **Automated Installation** — only for a real automated path (install script, package manager, `make install`, plugin installer), with the exact command.
6. **Update & Maintenance** — only for a real update path (self-update, `git pull` + rebuild, package manager update).
7. **Documentation** — links to `docs/en/index.md` and its main pages, plus `CONTRIBUTING.md` and `CODE_OF_CONDUCT.md`.
8. **License** — always last, exactly:
   ```markdown
   ## License

   GPL-3.0 — see [LICENSE](LICENSE).
   ```

`docs/pt-br/leiame.md` mirrors every section, translated, in the same order.

---

## 5. License

**Default: GNU GPL v3.0 or later (GPL-3.0).**

1. **Detect first** — `LICENSE` and license fields (`composer.json`, `package.json`, `pubspec.yaml`, `version.php`, ...). None declared → GPL-3.0. GPL-3.0 declared → keep it. **Another license declared → stop, tell the user which one, and ask for explicit confirmation** before changing anything.
2. **Confirm the copyright holder** with the user before writing — look in existing headers, manifest `author(s)` fields and `git config user.name`; never assume a default.
3. **`LICENSE`** is the canonical, unmodified text from <https://www.gnu.org/licenses/gpl-3.0.txt> — downloaded, never retyped from memory.
4. **Source file headers** — this notice in each language's comment syntax at the top of every source file the project owns (after a shebang or `<?php`). Current year for new headers, original year kept in existing ones. Skip generated, vendored and third-party files.
   ```text
   Copyright (C) {{YEAR}}  {{AUTHOR}}

   This program is free software: you can redistribute it and/or modify
   it under the terms of the GNU General Public License as published by
   the Free Software Foundation, either version 3 of the License, or
   (at your option) any later version.

   This program is distributed in the hope that it will be useful,
   but WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
   GNU General Public License for more details.

   You should have received a copy of the GNU General Public License
   along with this program.  If not, see <https://www.gnu.org/licenses/>.
   ```
5. For a confirmed non-GPL license, use that license's canonical text and header convention, and adapt the README line (e.g. `MIT — see [LICENSE](LICENSE).`).

---

## 6. Cross-Linking

Root files (`README.md`, `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`, `SECURITY.md`), right after the title or badges — with the matching mirror as target:

```markdown
🌐 **Language:** English · [Português](docs/pt-br/leiame.md)
```

Their `docs/pt-br/` mirrors use the inverse: `🌐 **Idioma:** [English](../../README.md) · Português`.

Every other page under `docs/en/` and `docs/pt-br/` (except `index.md`) opens with exactly this line, a rule, then the title — no variants:

```markdown
🌐 [Português](<path-to-pt-br-mirror>) | **English** | 🏠 [Index](<path-to-docs/en/index.md>)

---

# Page Title
```

The pt-br page uses `🌐 [English](<path-to-en-mirror>) | **Português** | 🏠 [Índice](<path-to-docs/pt-br/index.md>)`.

`index.md` (both languages) is a curated landing page: title, `🌐 [Português](../pt-br/index.md) | **English** | 🏠 [Back to README](../../README.md)`, a one-paragraph summary, then sections headed by an emoji (`## 🚀 Getting Started`, `## 🧠 Concepts`, ...) with one-line descriptions per link. Both indexes keep the same section order; `prompts.md`, when present, gets its own section.

---

## 7. GitHub Community Files

Only when the project is (or will be) hosted on GitHub. `.github/` files are English only.

### `SECURITY.md`

Root canon, mirrored as `docs/pt-br/seguranca.md`. Contains:

1. **Supported versions** — table of release lines that receive fixes, from the project's real releases.
2. **Reporting** — privately, never in a public issue, through a channel that exists:
   - **GitHub private vulnerability reporting** (*Security* tab → *Report a vulnerability*). An admin enables it in *Settings → Advanced Security → Private vulnerability reporting → Enable*; tell the user to do it, don't assume it is on.
   - Or a security contact the user provides. **Never invent a contact** — ask when none exists.
3. **What to expect** — what a report should include and the process; a response time only if the user commits to one.

### Issue forms (`.github/ISSUE_TEMPLATE/`)

- `bug_report.yml` and `feature_request.yml` as **issue forms**: required top-level keys `name`, `description`, `body`; optional `title`, `labels`, `assignees`, `type`, `projects`. Body element types: `markdown`, `input`, `textarea`, `dropdown`, `checkboxes`, `upload`, each with `attributes` and (except `markdown`) optional `id` and `validations`.
- Ask only for data the project can use (its real versions, platforms, logs); reference only labels that exist (or tell the user which to create).
- `config.yml`: `blank_issues_enabled` and `contact_links` (`name`, `url`, `about`) for channels that exist (e.g. the security policy).
- Issue forms are in public preview; follow GitHub's current syntax docs if they changed.

### Other files

- `pull_request_template.md` — summary, linked issue, how it was tested, docs/`CHANGELOG.md` updated (when the project has one).
- `CODEOWNERS` — only with more than one maintainer/team, using real handles from the user.
- `FUNDING.yml` — only for a sponsor channel the user has; no placeholder accounts.

---

## 8. Repository Metadata and Releases

Prepare the content and hand the user the exact commands (`AGENTS.md` → Git Operations).

- **Description:** the README pitch in one sentence. **Topics:** lowercase, hyphenated, real technologies/domains (`go`, `wails`, `moodle-plugin`) — no marketing terms. **Homepage:** only when a published site exists.
- **Social preview:** 1280×640 px recommended (minimum 640×320), PNG/JPG/GIF under 1 MB, uploaded by the user in *Settings → Social preview*; list it as pending until provided.

```bash
gh repo edit {{ORG}}/{{REPO}} --description "One-sentence pitch" --add-topic go --add-topic desktop-app
```

- **Tags:** annotated `vX.Y.Z` (SemVer) on the release commit.
- **Notes:** copied from that version's `CHANGELOG.md` section when the project has one.
- **Artifacts:** project, version, OS and architecture in the name (follow what the build/CI already produces), plus a `SHA256SUMS`/`checksums.txt` covering all of them.
- Publishing is done by the user or by the project's CI on tag push:

```bash
git tag -a v1.2.0 -m "Release 1.2.0" && git push origin v1.2.0
gh release create v1.2.0 --verify-tag --title "v1.2.0" --notes-file release-notes.md dist/*
```

Document the real release process in `CONTRIBUTING.md` (or a `guides/` page) and download/checksum verification in `installation.md`.

---

## Quality

Run before reporting documentation work as done:

```bash
# Broken links and anchors (lychee: https://lychee.cli.rs)
lychee --offline --include-fragments README.md CONTRIBUTING.md CODE_OF_CONDUCT.md docs/

# en/pt-br mirror: prints nothing when both trees have the same pages
diff <(cd docs/en && find . -name '*.md' | sort) \
     <(cd docs/pt-br && find . -name '*.md' ! -path './leiame.md' ! -path './contribuindo.md' \
         ! -path './codigo-de-conduta.md' ! -path './seguranca.md' | sort)

# Every docs page (except index.md) starts with the language switcher line
grep -L '^🌐 ' $(find docs/en docs/pt-br -name '*.md' ! -name index.md)
```

- [ ] Every command, flag, path and version checked against the code and build files.
- [ ] README sections in order; conditional ones present only when supported; License last.
- [ ] License detected (and confirmed when not GPL-3.0); author confirmed; headers on owned source files.
- [ ] Placeholders and pending items (social preview, security contact, private reporting) listed in the report.

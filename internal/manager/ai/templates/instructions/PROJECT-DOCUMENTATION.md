# Bilingual Documentation Site Standard

## Overview

This guide defines the standard root files, README shape, license handling, and full documentation site under `docs/` for a project — available in English and Brazilian Portuguese side by side. It is a **standard to follow whenever documentation is created or updated**, not a task to run on its own: only generate or restructure a project's documentation when the user asks for it.

This is the **base** standard. Project-type complements (`PROJECT-DOCUMENTATION-MCP.md`, `PROJECT-DOCUMENTATION-MOODLE.md`, `PROJECT-DOCUMENTATION-DESKTOP.md`, `PROJECT-DOCUMENTATION-MOBILE.md`, `PROJECT-DOCUMENTATION-WEB.md` under `.instructions/`) add type-specific requirements on top of it. When a complement is present, apply both; when they seem to conflict, the complement's more specific rule wins for its own topic.

### When to Use
* The project is intended for public/open-source distribution and deserves a real documentation site, not just a README.
* You are scaffolding docs from scratch for a new project, or extending an existing one that already follows this structure.

### When NOT to Use
* Internal-only tools, throwaway scripts, or projects with no intention of ever being published — a plain `README.md` is enough; do not impose this structure on them.
* Don't retrofit an existing project's unrelated documentation layout onto this standard without asking first — this is the shape for new documentation sites, not a mandate to restructure whatever a project already has.

### Ground Rule: Document Only What Exists
Every command, flag, path, version, and feature you document must be verified against the actual code, build files, and configuration of the project. Never invent an install method, update path, platform, or feature the project doesn't have. When a section depends on something the project doesn't support, follow that section's own rule (omit it, or state explicitly that it isn't supported) — never fill it with plausible-looking guesses.

---

## 1. Format and Readability

1. **Scannability:** use headings (`#`, `##`, `###`), organized lists, bold for key terms, and fenced code blocks with a language tag for syntax highlighting (`bash`, `json`, `go`, `php`, `yaml`, ...). Prefer tables for anything with 3+ comparable attributes (options, env vars, supported versions).
2. **Clean root:** the repository root holds only the canonical root files listed in Section 2. All extended content lives under `docs/`.
3. **Internationalization:**
   - **Default language (root and `docs/en/`): English.**
   - **Secondary language (`docs/pt-br/`): Brazilian Portuguese.**
   - Never mix languages inside the same file.

---

## 2. Required Directory Structure

```text
{{PROJECT_NAME}}/
├── README.md                          # canonical, English — see Section 4 for its required shape
├── CONTRIBUTING.md                    # canonical, English
├── CODE_OF_CONDUCT.md                 # canonical, English (Contributor Covenant)
├── LICENSE                            # see Section 5
└── docs/
    ├── en/
    │   ├── index.md                   # curated landing page / table of contents
    │   ├── prompts.md                 # only when a complement requires it (e.g. PROJECT-DOCUMENTATION-MCP.md), or when genuinely useful
    │   ├── getting-started/
    │   │   ├── installation.md        # full install guide: every supported platform/method, building from source
    │   │   ├── prerequisites.md       # conditional — when prerequisites need more than a few inline commands
    │   │   ├── uninstallation.md      # conditional — when installing leaves anything outside the project folder
    │   │   └── quickstart.md
    │   ├── concepts/
    │   │   ├── why-{{PROJECT_NAME}}.md
    │   │   ├── how-{{PROJECT_NAME}}-works.md
    │   │   ├── architecture.md        # high-level overview, for users — see Section 3
    │   │   └── glossary.md
    │   ├── architecture/              # deep internal docs, for contributors
    │   │   └── *.md                   # filenames vary per project
    │   ├── guides/
    │   │   ├── clients/               # only if the project has pluggable integrations/clients
    │   │   │   └── *.md               # one file per supported client
    │   │   ├── workflows/
    │   │   │   └── examples.md
    │   │   └── environments/          # optional — only when relevant (e.g. docker.md)
    │   ├── reference/
    │   │   └── *.md                   # exhaustive reference: CLI flags, API, tools, config
    │   └── troubleshooting/
    │       └── common-issues.md
    └── pt-br/
        ├── index.md                   # mirrors docs/en/index.md
        ├── prompts.md                 # mirrors docs/en/prompts.md, if present
        ├── leiame.md                  # mirrors README.md
        ├── contribuindo.md            # mirrors CONTRIBUTING.md
        ├── codigo-de-conduta.md       # mirrors CODE_OF_CONDUCT.md
        └── ...                        # mirrors every docs/en/ subfolder and filename exactly
```

Key rules:

1. **Root files are the English canon.** `README.md`, `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`, `LICENSE` live at the repo root, and nowhere else.
2. **Portuguese mirrors never live at the root.** They go under `docs/pt-br/` with translated filenames: `leiame.md`, `contribuindo.md`, `codigo-de-conduta.md` — these three are the **only** translated filenames.
3. **`docs/pt-br/` mirrors `docs/en/` file-for-file.** Same subfolder names, same filenames (content translated, not renamed — e.g. `docs/pt-br/concepts/architecture.md`, never `arquitetura.md`). If a page exists in one language, it must exist in the other — see [Maintenance](#7-maintenance-rule) below.
4. **`concepts/architecture.md` and the `architecture/` folder are not duplicates.** `concepts/architecture.md` is a single high-level overview for users. The `architecture/` folder holds deep, per-component internals for contributors (one file per subsystem). Don't merge them.
5. **Conditional files and folders are not mandatory**: `getting-started/prerequisites.md`, `getting-started/uninstallation.md`, `guides/clients/`, `guides/environments/` and `prompts.md` exist only when the project actually needs them (or a complement requires them). Don't create empty placeholder files or folders. Beyond these, check whether the project needs other technical pages (alternative install methods, migration guides, FAQ, ...) and add them under the most fitting section.
6. **No `CHANGELOG.md` unless the project already has one.** See the `Documentation Maintenance` rule in the shared agent rules — the same "don't invent files that never existed" principle applies here.

---

## 3. Content Guide by Section

| Section | Purpose |
|---|---|
| `getting-started/` | Installation and a first successful run — the shortest path to "it works". `installation.md` covers every supported platform and method, including building from source; `prerequisites.md` details installing the environment when it's more than a few commands; `uninstallation.md` is a complete removal guide (binaries, config/data directories, services, desktop entries, registered integrations) whenever installing leaves anything behind. |
| `concepts/` | What problem the project solves, why it exists, how it works at a conceptual level, and a glossary of project-specific terms. `concepts/architecture.md` must explain: the **data flow** (from input/entry point to output/side effects), the **folder/package layout** and what lives where, the **main dependencies** (and why each is used), and the key **design decisions** with their rationale. |
| `architecture/` | Implementation-level detail for contributors: storage, internal data flow, module boundaries. |
| `guides/` | Task-oriented how-tos: per-client setup, per-environment deployment, real end-to-end workflow examples. |
| `prompts.md` | A single document — not a folder — of example prompts written in natural human language. Its required shape is defined by the complement that requires it (e.g. `PROJECT-DOCUMENTATION-MCP.md`). |
| `reference/` | Exhaustive, boring reference material: every CLI flag, every API parameter, every config key. No prose, no opinions — just facts. |
| `troubleshooting/` | Common errors and their fixes, indexed by symptom. |

---

## 4. `README.md` Required Sections

Unlike the rest of `docs/`, the README is GitHub-facing: it's the first (often only) thing a visitor sees on the repo page. Keep every section tight — this is a front door, not the full documentation. The sections below go in this order; items 1–4, 7 and 8 are always present, items 5 and 6 are conditional.

1. **Header:**
   - Project name as the H1 title.
   - A short, high-impact description (1–2 sentences on the problem it solves).
   - **Badges** — whatever is genuinely relevant and available for the project, as a single line of `shields.io`-style badges. Don't add a badge for something the project doesn't actually have (e.g. no CI badge if there's no CI). Typical set: latest release/version, primary language or runtime + minimum version, license, platform (if platform-specific), build/CI status (if CI exists). Example, adapted from a real Go project:
     ```markdown
     [![Version](https://img.shields.io/github/v/release/{{ORG}}/{{REPO}}?label=version&color=brightgreen)](https://github.com/{{ORG}}/{{REPO}}/releases)
     [![Go](https://img.shields.io/badge/Go-1.26%2B-00ADD8?logo=go)](https://go.dev)
     [![License](https://img.shields.io/github/license/{{ORG}}/{{REPO}}?label=license)](LICENSE)
     ```
   - **Language switcher** — see [Section 6.1](#61-root-files-readmemd-contributingmd-code_of_conductmd).
2. **Table of Contents** — a short bullet list of anchor links to this README's own sections (not a link out to `docs/`; that belongs in item 7). Match the TOC's language to the file (English anchors/labels in `README.md`, Portuguese ones in `docs/pt-br/leiame.md`), e.g. in `leiame.md`:
   ```markdown
   ## Índice
   - [Visão geral](#visão-geral)
   - [Instalação](#instalação)
   - [Como atualizar](#como-atualizar)
   - [Documentação](#documentação)
   - [Licença](#licença)
   ```
3. **Overview & What It Does** — a short prose summary of what the project is and why it exists, followed by a concrete list of its main features and value proposition.
4. **Prerequisites & Quick Installation** — environment requirements (languages, runtimes, global tools, with minimum versions) and a step-by-step quick guide: clone, install dependencies, build/run. Install hard prerequisites inline (don't just link to a prerequisites page and stop). Link out to `docs/en/getting-started/installation.md` for anything beyond the common case.
5. **Automated Installation** *(conditional)* — only if the project really supports an automated install path (an install script, a package manager, an app/plugin installer, a `make install`, etc.): document the exact command. **If the code doesn't support it, omit this section entirely** (and its TOC entry).
6. **Update & Maintenance** *(conditional)* — how to update to the latest version, using the project's real mechanism (a self-update command, `git pull` + rebuild, the package manager's update command, etc.). **If there is no supported or clearly defined update path in the code, omit this section entirely** (and its TOC entry).
7. **Advanced Technical Documentation** — links to the documentation site (`docs/en/index.md`) and its main pages (architecture, reference, prompts, guides), plus Contributing (`CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`). A few lines with links out — the README is never where the full detail lives.
8. **License** — always the last section, in this exact form, using the license resolved in Section 5 (default: GPL-3.0):
   ```markdown
   ## License

   GPL-3.0 — see [LICENSE](LICENSE).
   ```

`docs/pt-br/leiame.md` mirrors every section above, translated, in the same order, with the inverse-language switcher (Section 6.1).

---

## 5. License

**Default: GNU GPL v3.0 or later (GPL-3.0).**

1. **Detect first.** Read the project's `LICENSE` file (and any license field in `composer.json`, `package.json`, `pubspec.yaml`, `version.php`, etc.).
   - No license declared anywhere → use GPL-3.0.
   - GPL-3.0 already declared → keep it.
   - **A different license declared → stop, tell the user which license was found, and ask for explicit confirmation** before changing anything. Never switch a project's license silently.
2. **Detect and confirm the author (copyright holder).** Look for it in an existing `LICENSE`/copyright headers, `composer.json`/`package.json` `authors`/`author`, `pubspec.yaml`, the plugin's `version.php`/`README`, and `git config user.name`. **Always confirm the author with the user before writing** — when nothing is found, ask; never assume a default.
3. **`LICENSE` file:** the canonical, unmodified GPLv3 text from <https://www.gnu.org/licenses/gpl-3.0.txt> — never paraphrased, summarized, or retyped from memory.
4. **Source file headers:** add this notice, as a comment in each language's own syntax, at the top of every source file the project owns (after a shebang line or a `<?php` opening tag, which must stay first). Use the current year for new headers; keep the original year in files that already carry one. Skip generated, vendored, and third-party files (e.g. `vendor/`, `node_modules/`, generated bindings), and files that already have the header.
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
5. If the user confirms a license other than GPL-3.0, apply that license's own canonical `LICENSE` text and header convention instead, and adapt the README's License line to its name (e.g. `MIT — see [LICENSE](LICENSE).`).

---

## 6. Cross-Linking Conventions

### 6.1 Root files (`README.md`, `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`)

Add a language switcher right after the title (or after the badges block, if there is one):

```markdown
🌐 **Language:** English · [Português](docs/pt-br/leiame.md)
```

The `docs/pt-br/` mirror uses the inverse, linking back to the root:

```markdown
🌐 **Idioma:** [English](../../README.md) · Português
```

### 6.2 Every page under `docs/en/**/*.md` and `docs/pt-br/**/*.md` (except `index.md`)

Open with a breadcrumb + language switcher line, a horizontal rule, then the page's `# Title`. Use exactly this format — it is the one canonical form; do not invent variants (flag emoji, "← Back to Index" wording, etc.):

```markdown
🌐 [Português](<relative-path-to-pt-br-mirror>) | **English** | 🏠 [Index](<relative-path-to-docs/en/index.md>)

---

# Page Title
```

The `pt-br` mirror of the same page uses the inverse:

```markdown
🌐 [English](<relative-path-to-en-mirror>) | **Português** | 🏠 [Índice](<relative-path-to-docs/pt-br/index.md>)

---

# Título da Página
```

### 6.3 `index.md` (both languages)

Not a bare file list — a curated landing page. Open with the title, the inverse-language link, a one-paragraph project summary, then grouped sections headed by an emoji, each entry with a one-line description of what the linked page covers:

```markdown
# Documentation — {{PROJECT_NAME}}

🌐 [Português](../pt-br/index.md) | **English** | 🏠 [Back to README](../../README.md)

---

One-paragraph summary of what the project does.

---

## 🚀 Getting Started

- [Installation](./getting-started/installation.md) — one-line description.
- [Quickstart](./getting-started/quickstart.md) — one-line description.

---

## 🧠 Concepts
...
```

Mirror the same section order in both languages so the two indexes stay structurally identical. If `prompts.md` exists, give it its own entry (e.g. `## 💬 Example Prompts`).

---

## 7. Maintenance Rule

Whenever a page is added, removed, or meaningfully changed under `docs/en/`, make the equivalent change to its `docs/pt-br/` mirror **in the same change** — never leave the two trees out of sync as follow-up work. This is a direct application of the `Documentation Maintenance` rule in the shared agent rules (update documentation in the same change, not deferred), specialized for a bilingual doc tree: here, "the documentation" means both language copies, not just one.

If a new subfolder category is needed that isn't listed in [Section 3](#3-content-guide-by-section) (e.g. a project-specific need), create it under both `docs/en/` and `docs/pt-br/` at once, and add it to both `index.md` files.

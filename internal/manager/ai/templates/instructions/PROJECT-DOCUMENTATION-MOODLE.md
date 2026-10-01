# Documentation Complement — Moodle Plugins

## Overview

Type-specific documentation requirements for **Moodle plugins**. This complements the base standard in `.instructions/PROJECT-DOCUMENTATION.md` — apply both. Like the base, it's a standard for when documentation is created or updated, not a task to run on its own.

Source every value from the plugin itself — `version.php` (`$plugin->component`, `version`, `requires`, `supported`, `release`, `maturity`, `dependencies`), `db/`, `settings.php`, `lang/`, `classes/privacy/` — never from memory.

---

## 1. Requirements (README)

In the README's **Prerequisites & Quick Installation** section, include a requirements table:

| Item | Source |
|---|---|
| Moodle versions supported | `$plugin->requires` (minimum) and `$plugin->supported` (range), translated into human versions (e.g. 4.5 – 5.1) |
| PHP versions supported | The range supported by those Moodle versions, narrowed by anything the plugin itself requires |
| Plugin dependencies | `$plugin->dependencies` (component + minimum version) |
| Maturity / release | `$plugin->maturity`, `$plugin->release` |

---

## 2. Installation

Document every install method, the quick one in the README and all of them in `docs/en/getting-started/installation.md`:

1. **Manual install:** copy/extract the plugin into the directory that matches its type — derived from the frankenstyle prefix of `$plugin->component` (e.g. `mod_` → `mod/`, `block_` → `blocks/`, `local_` → `local/`, `tool_` → `admin/tool/`, `theme_` → `theme/`, `auth_` → `auth/`, `enrol_` → `enrol/`, `filter_` → `filter/`, `report_` → `report/`, `qtype_` → `question/type/`). The folder name must be the part after the prefix. On **Moodle 5.1+** the code root is `public/` (e.g. `public/mod/<name>`) — document both layouts when the supported range spans them.
2. **ZIP upload** via *Site administration → Plugins → Install plugins*, when the plugin is distributed as a ZIP.
3. **Git clone** into the target directory, when the repository is public.

### Finishing the installation
- **Database upgrade (CLI):**
  ```bash
  sudo -u www-data php admin/cli/upgrade.php --non-interactive
  ```
- **Cache purge:**
  ```bash
  sudo -u www-data php admin/cli/purge_caches.php
  ```
- **Through the web interface:** log in as an administrator → *Site administration → Notifications*, review the plugin being installed, click *Upgrade Moodle database now*, then fill in the plugin's settings page if it has one (document every setting in `docs/en/reference/`).

Adjust the web server user (`www-data`, `apache`, `nginx`) to the target environment, and verify the CLI script paths against the Moodle versions in the supported range — the 5.1+ `public/` restructure changed where files live, so don't assume the pre-5.1 layout.

---

## 3. Update & Uninstall

- **Update:** replace the plugin files with the new version, then run the database upgrade and cache purge above (or visit *Site administration → Notifications*).
- **Uninstall** (`docs/en/getting-started/uninstallation.md`): *Site administration → Plugins → Plugins overview → Uninstall*, or
  ```bash
  sudo -u www-data php admin/cli/uninstall_plugins.php --plugins={{COMPONENT}} --run
  ```
  then delete the plugin folder. State what data is removed (tables, files, settings) — check `db/uninstall.php` and the Privacy API provider.

---

## 4. Reference

Under `docs/en/reference/`, document (when present in the plugin): admin settings (`settings.php`), capabilities (`db/access.php`), events (`classes/event/`), scheduled/ad-hoc tasks (`db/tasks.php`), web services (`db/services.php`), and what personal data it stores (Privacy API).

# Moodle Plugin Development

Standards and conventions for developing Moodle plugins in this project.
Covers file structure, API usage, architecture patterns, and common mistakes.

---

## Language

| Context | Language |
|---|---|
| Responses to the user | Brazilian Portuguese (pt-BR) |
| Code comments | English |

---

## Development Environment

- **Moodle Version:** {{MOODLE_VERSION}}
- **Installation Path:** {{MOODLE_PATH}}
- **MCP Server:** `lumina-mdle-dev` is configured and indices have been generated.

---

## Docker Development Environment

Each Moodle install runs in its own container ("Container PHP" — one project, one PHP version, one URL), created via perci's **Dev Stuff → Gerenciar Docker → Criar Container PHP**. It provides PHP tools as host-side wrapper scripts in `~/.local/bin/`, suffixed with the project's own folder name — no PHP-version suffix, since a project only ever has the one PHP version it was created with. All commands run inside that project's container automatically.

### Available commands

For a project in folder `<folder>` (e.g. `mdle`):

| Command | Description |
|---|---|
| `php-<folder>` | PHP CLI |
| `phpcs-<folder>` | PHP_CodeSniffer (global, PSR-12 and standards installed in the container) |
| `phpcbf-<folder>` | PHP Code Beautifier and Fixer |
| `phpunit-<folder>` | PHPUnit |
| `composer-<folder>` | Composer |

### Common Moodle CLI commands

```bash
# Run a PHP file inside the container
php-mdle admin/cli/purge_caches.php

# Run the upgrade tool after plugin changes
php-mdle admin/cli/upgrade.php --non-interactive

# Install Moodle coding standard in the project
composer-mdle require --dev moodlehq/moodle-cs

# Run PHPCS with the Moodle standard (uses project's vendor phpcs + moodle-cs)
php-mdle vendor/bin/phpcs --standard=moodle local/myplugin/

# Auto-fix coding style violations
php-mdle vendor/bin/phpcbf --standard=moodle local/myplugin/

# Run PHPUnit test suite
phpunit-mdle

# Run a specific test class
phpunit-mdle --filter MyPluginTest
```

> **Note:** Use `php-mdle vendor/bin/phpcs --standard=moodle` (the project's own phpcs binary) when
> the Moodle coding standard is required, because the Moodle standard is registered through
> `moodlehq/moodle-cs` installed in the project's `vendor/`. The global `phpcs-mdle` wrapper uses
> the system-wide PHPCS installation and does not have access to project-local standards.

---

## lumina-mdle-dev — Usage Guide

Always load the plugin context before starting work:

```text
Load context for plugin local_myplugin.
```

### Available Tools

- `get_plugin_info` — loads the complete plugin context into the current session.
- `search_api` — searches for Moodle core API functions.
- `generate_plugin_context` — generates `PLUGIN_*.md` documentation files for a plugin.
- `update_indexes` — regenerates global indices after installing new plugins.
- `doctor` — runs health checks on the `moodle-dev-mcp` environment.

### Recommended Workflow

1. **Initialize:** Load context using `get_plugin_info` before working on a plugin.
2. **Research:** Use `search_api` before suggesting or implementing core functions.
3. **Document:** Run `generate_plugin_context` after significant code changes.
4. **Sync:** Execute `update_indexes` whenever new plugins are added to the environment.

---

## Target Moodle Version

This project targets **Moodle {{MOODLE_VERSION}}+** (`requires = {{MOODLE_FULLVERSION}}`).

- Use only APIs compatible with Moodle {{MOODLE_VERSION}} or later.
- **Strictly avoid** functions deprecated in previous versions.

### PHP Compatibility Matrix

| Moodle Version | PHP Minimum | PHP Used by this stack |
|---|---|---|
| 4.1 | 7.4 | 8.1 |
| 4.2 | 8.0 | 8.2 |
| 4.3 | 8.0 | 8.2 |
| 4.4 | 8.1 | 8.3 |
| 4.5 | 8.1 | 8.3 |
| 5.0 | 8.2 | 8.3 |
| 5.1 | 8.2 | 8.3 |
| 5.2 | 8.3 | 8.3 |
| 5.3 (not yet released) | 8.3 | 8.3 |

Write PHP code compatible with the **Minimum** column for {{MOODLE_VERSION}}, not just the PHP version running in this stack's container — sites still on that release's lowest supported PHP must keep working. If unsure whether legacy-PHP compatibility matters for this project, ask the user before relying on syntax newer than that minimum (e.g. enums, readonly properties, first-class callable syntax).

### Hook API vs lib.php Callbacks

The Hook API is available from **Moodle 4.3+**. Before implementing any event hook or plugin callback, ask the user:

> "Does this plugin need to support Moodle versions earlier than 4.3?"

Based on the answer:

- **Only 4.3+** — use the Hook API exclusively (`classes/hook/` + `db/hooks.php`).
- **Only < 4.3** — use `lib.php` callbacks exclusively.
- **Both versions** — implement both and guard the `lib.php` callback to avoid double execution on 4.3+:

```php
// lib.php — executed only on Moodle < 4.3
function local_example_before_standard_html_head(): string {
    global $CFG;
    if ($CFG->version >= 2023100900) { // 4.3+ uses Hook API
        return '';
    }
    return local_example_render_head_content();
}

// classes/hook/before_standard_html_head.php — for Moodle 4.3+
namespace local_example\hook;

class before_standard_html_head {
    public static function callback(\core\hook\output\before_standard_html_head $hook): void {
        $hook->add_html(local_example_render_head_content());
    }
}
```

```php
// db/hooks.php — registers the Hook API callback (Moodle 4.3+)
$callbacks = [
    [
        'hook'     => \core\hook\output\before_standard_html_head::class,
        'callback' => \local_example\hook\before_standard_html_head::class . '::callback',
    ],
];
```

---

## Moodle 5.1+ — `/public` Directory & Routing Engine

Starting with Moodle 5.1, the codebase ships a `/public` directory, and the web server document root must point to `{{MOODLE_PATH}}/public` instead of `{{MOODLE_PATH}}`. A new (optional) Routing Engine enables cleaner URLs; it is **not compulsory** — a compatibility layer keeps traditional script-based URLs (e.g. `/mod/forum/view.php?id=1`) working.

**Impact on this Docker stack:** none — this project's container already has its own dedicated nginx `root`, pointed directly at `{{MOODLE_PATH}}/public` by perci itself when the Container Aplicativo was created (Moodle apps always route this way, by design; no manual nginx change needed). To run a Moodle 5.1+ project here:

- `$CFG->wwwroot` (and the browser URL) is just the app's own URL, e.g. `http://<folder>.localhost` — **no** `/public` suffix; the document root already points inside `public/`, so it never appears in the URL.
- Plugin code (`local/`, `mod/`, `blocks/`, etc.) keeps the exact same relative structure — it now lives under `public/<area>/<plugin>` instead of `<area>/<plugin>`. Never hardcode the Moodle root path; use `$CFG->dirroot` / `new moodle_url(...)` as already required elsewhere in this guide.
- This project does not cover **upgrading** an existing install to 5.1+ (moving plugins from above `/public` into it) — only fresh 5.1+ checkouts, which already ship the `public/` layout.
- Do not implement custom routes against the new Routing Engine unless explicitly asked — default to standard script-based pages, which keep working through the compatibility layer.

---

## Required Files

Every plugin must include these files at a minimum:

```text
version.php
lang/en/[component].php
db/install.xml
db/upgrade.php
db/access.php
classes/privacy/provider.php
```

**Optional (recommended):** `settings.php`, `lib.php`, `index.php`, `classes/`, `templates/`, `amd/src/`, `tests/`, `tests/behat/`

---

## File Standards

### version.php

```php
defined('MOODLE_INTERNAL') || die();

$plugin->component = 'local_example';
$plugin->version   = 2026010100; // Format: YYYYMMDDNN (NN = daily increment, starting at 00)
$plugin->requires  = 2022112800; // Matches {{MOODLE_FULLVERSION}}
$plugin->maturity  = MATURITY_STABLE;
$plugin->release   = '1.0';
```

### Language File

**Path:** `lang/en/local_example.php`

```php
defined('MOODLE_INTERNAL') || die();

$string['pluginname'] = 'Example Plugin';
```

All user-facing strings must be defined here. Never hardcode strings in PHP or Mustache templates.

### db/install.xml

- Tables must use the Moodle prefix (handled by XMLDB).
- Every table **must** have a primary key.
- Define indexes for columns used in `WHERE` or `JOIN` clauses.
- Supported types: `INT`, `CHAR`, `TEXT`, `NUMBER`, `FLOAT`.

### db/upgrade.php

Required for all schema migrations after the initial install. Each version bump must have a corresponding upgrade block.

```php
defined('MOODLE_INTERNAL') || die();

function xmldb_local_example_upgrade(int $oldversion): bool {
    global $DB;
    $dbman = $DB->get_manager();

    if ($oldversion < 2026010100) {
        $table = new xmldb_table('example_table');
        $field = new xmldb_field('newfield', XMLDB_TYPE_CHAR, '255', null, XMLDB_NOTNULL, null, '');
        if (!$dbman->field_exists($table, $field)) {
            $dbman->add_field($table, $field);
        }
        upgrade_plugin_savepoint(true, 2026010100, 'local', 'example');
    }

    return true;
}
```

### db/access.php

All capabilities must be declared in this file.

```php
defined('MOODLE_INTERNAL') || die();

$capabilities = [
    'local/example:view' => [
        'riskbitmask'  => RISK_PERSONAL,
        'captype'      => 'read',
        'contextlevel' => CONTEXT_SYSTEM,
        'archetypes'   => [
            'user'    => CAP_ALLOW,
            'manager' => CAP_ALLOW,
        ],
    ],
];
```

---

## PHP Namespaces

All classes must use the plugin namespace. File paths must match namespaces under `classes/`.

```php
namespace local_example\service;
namespace local_example\repository;
namespace local_example\output;
namespace local_example\external;
namespace local_example\task;
namespace local_example\event;
namespace local_example\hook;
namespace local_example\privacy;
```

---

## Recommended Plugin Architecture

```text
classes/
  service/        → Business logic
  repository/     → Database access
  output/         → Rendering logic (renderers and renderables)
  external/       → Web service endpoints
  task/           → Scheduled and ad-hoc tasks
  event/          → Event classes
  hook/           → Hook API callbacks (Moodle 4.3+)
  privacy/        → Privacy API — GDPR compliance (required)
db/
  events.php      → Event observers
  hooks.php       → Hook API registrations (Moodle 4.3+)
  services.php    → Web service definitions
templates/        → Mustache templates
amd/src/          → AMD JavaScript modules
tests/            → PHPUnit tests
tests/behat/      → Behat acceptance tests
```

### Entry Point Pattern (index.php)

```php
require_once(__DIR__ . '/../../config.php');

require_login();
require_capability('local/example:view', context_system::instance());

$PAGE->set_url(new moodle_url('/local/example/index.php'));
$PAGE->set_context(context_system::instance());
$PAGE->set_title(get_string('pluginname', 'local_example'));

$service = new \local_example\service\example_service();
$data    = $service->get_data();

$output  = $PAGE->get_renderer('local_example');
echo $output->header();
echo $output->render_from_template('local_example/main', $data);
echo $output->footer();
```

---

## Privacy API (GDPR)

Every plugin that stores personal data must implement `classes/privacy/provider.php`. Plugins that store no personal data must still declare a null provider.

### Null provider (no personal data)

```php
namespace local_example\privacy;

class provider implements \core_privacy\local\metadata\null_provider {
    public static function get_reason(): string {
        return get_string('privacy:metadata', 'local_example');
    }
}
```

### Full provider (stores personal data)

```php
namespace local_example\privacy;

use core_privacy\local\metadata\collection;
use core_privacy\local\request\contextlist;
use core_privacy\local\request\approved_contextlist;

class provider implements
    \core_privacy\local\metadata\provider,
    \core_privacy\local\request\plugin\provider {

    public static function get_metadata(collection $collection): collection {
        $collection->add_database_table('example_table', [
            'userid'      => 'privacy:metadata:example_table:userid',
            'data'        => 'privacy:metadata:example_table:data',
            'timecreated' => 'privacy:metadata:example_table:timecreated',
        ], 'privacy:metadata:example_table');
        return $collection;
    }

    public static function get_contexts_for_userid(int $userid): contextlist {
        $contextlist = new contextlist();
        $sql = "SELECT ctx.id
                  FROM {example_table} et
                  JOIN {context} ctx ON ctx.instanceid = et.courseid
                       AND ctx.contextlevel = :contextlevel
                 WHERE et.userid = :userid";
        $contextlist->add_from_sql($sql, ['contextlevel' => CONTEXT_COURSE, 'userid' => $userid]);
        return $contextlist;
    }

    public static function export_user_data(approved_contextlist $contextlist): void {
        // Export user data for GDPR subject access requests.
    }

    public static function delete_data_for_all_users_in_context(\context $context): void {
        global $DB;
        // Delete all personal data stored in this context.
    }

    public static function delete_data_for_user(approved_contextlist $contextlist): void {
        global $DB;
        // Delete data for a specific user across their approved contexts.
    }
}
```

---

## External API (Web Services)

Declare each endpoint as a class under `classes/external/` and register it in `db/services.php`.

```php
// classes/external/get_example.php
namespace local_example\external;

use external_api;
use external_function_parameters;
use external_value;
use external_single_structure;

class get_example extends external_api {

    public static function execute_parameters(): external_function_parameters {
        return new external_function_parameters([
            'id' => new external_value(PARAM_INT, 'Record ID'),
        ]);
    }

    public static function execute(int $id): array {
        $params = self::validate_parameters(self::execute_parameters(), ['id' => $id]);
        self::validate_context(\context_system::instance());
        require_capability('local/example:view', \context_system::instance());

        $record = \local_example\repository\example_repository::get($params['id']);
        return ['id' => $record->id, 'name' => $record->name];
    }

    public static function execute_returns(): external_single_structure {
        return new external_single_structure([
            'id'   => new external_value(PARAM_INT, 'Record ID'),
            'name' => new external_value(PARAM_TEXT, 'Record name'),
        ]);
    }
}
```

```php
// db/services.php
defined('MOODLE_INTERNAL') || die();

$functions = [
    'local_example_get_example' => [
        'classname'    => \local_example\external\get_example::class,
        'methodname'   => 'execute',
        'description'  => 'Returns a single example record.',
        'type'         => 'read',
        'ajax'         => true,
        'capabilities' => 'local/example:view',
    ],
];
```

---

## Events

### Defining an event

```php
// classes/event/example_created.php
namespace local_example\event;

class example_created extends \core\event\base {

    protected function init(): void {
        $this->data['crud']        = 'c'; // c=create, r=read, u=update, d=delete
        $this->data['edulevel']    = self::LEVEL_OTHER;
        $this->data['objecttable'] = 'example_table';
    }

    public static function get_name(): string {
        return get_string('event:example_created', 'local_example');
    }

    public function get_description(): string {
        return "User {$this->userid} created example record {$this->objectid}.";
    }
}
```

### Dispatching an event

```php
$event = \local_example\event\example_created::create([
    'objectid' => $record->id,
    'context'  => \context_system::instance(),
]);
$event->trigger();
```

### Observing an event

```php
// db/events.php
$observers = [
    [
        'eventname' => \local_example\event\example_created::class,
        'callback'  => \local_example\observer\example_observer::class . '::on_created',
    ],
];
```

---

## Common Mistakes

### Quick Reference

| Action | Wrong | Correct |
| :--- | :--- | :--- |
| **HTML Output** | `echo "<div>Hello</div>";` | `$output->render_from_template(...)` |
| **Request Vars** | `$id = $_POST['id'];` | `$id = required_param('id', PARAM_INT);` |
| **URLs** | `echo '/local/plugin/index.php';` | `new moodle_url('/local/plugin/index.php')` |
| **DB Queries** | `mysqli_query(...);` | `$DB->get_records('table', [...]);` |
| **Security** | Skipping checks | `require_login(); require_capability();` |

### Database Access

```php
// Wrong
mysqli_query(...);

// Correct — always use the $DB API
$records = $DB->get_records('example_table', ['userid' => $userid]);
$DB->insert_record('example_table', $dataobject);
$DB->update_record('example_table', $dataobject);
$DB->delete_records('example_table', ['id' => $id]);
```

### Request Parameters

```php
// Wrong
$id = $_POST['id'];

// Correct
$id = required_param('id', PARAM_INT);
$id = optional_param('id', 0, PARAM_INT);
```

### Language Strings

```php
// Wrong
echo "Save";

// Correct
echo get_string('save', 'local_example');
```

### JavaScript — Use AMD Modules

Never use inline `<script>` tags. Avoid importing jQuery — use Moodle core modules or native DOM APIs instead.

```javascript
// amd/src/example.js
define(['core/log', 'core/ajax'], function (Log, Ajax) {
    return {
        init: function (config) {
            Log.debug('local_example: module initialized', config);

            document.querySelector('[data-action="example-submit"]')
                ?.addEventListener('click', function () {
                    Ajax.call([{
                        methodname: 'local_example_get_example',
                        args: { id: config.recordId },
                    }])[0].done(function (result) {
                        Log.debug('local_example: result', result);
                    });
                });
        },
    };
});
```

```mustache
{{! Load AMD module from template }}
{{#js}}
require(['local_example/example'], function(mod) {
    mod.init();
});
{{/js}}
```

---

## Theme Compatibility

The plugin must work correctly regardless of which theme is active on the site.

- Treat **Boost** (Moodle's default theme) as the reference/baseline theme — every template, renderer and stylesheet must render correctly under Boost first.
- Never assume markup, CSS classes or layout structure specific to a non-default theme (Boost child themes or third-party themes) — use Moodle's standard renderer/Mustache output and Boost's Bootstrap conventions instead of theme-specific selectors.
- Do not hardcode theme-specific CSS overrides inside the plugin to "fix" appearance under a particular theme; if a visual issue only reproduces under a non-default theme, treat it as that theme's responsibility, not the plugin's.

---

## Global Context Guidelines

- Always use the global `$DB` object for all database operations.
- SQL table names in raw queries must use `{bracket_format}`.
- Follow **Moodle Coding Style** (based on PSR-12).
- **Exclude from indexing:** `.git`, `node_modules`, `vendor`, `.grunt`, `moodledata`, `cache`.

### Database Portability (MySQL/MariaDB priority, PostgreSQL support)

This stack's database is **MariaDB** (MySQL-compatible) — the priority target — but Moodle officially also supports **PostgreSQL**, and the plugin must keep working there too.

- Never write vendor-specific raw SQL; the `$DB` DML API already abstracts engine differences — use it exclusively.
- Use `$DB->sql_concat(...)` instead of `CONCAT()`/`||`.
- Use `$DB->sql_compare_text(...)` when comparing or searching `TEXT` columns — direct comparison works on MySQL but fails on PostgreSQL.
- Use `$DB->sql_like(...)` instead of a raw `LIKE`/`ILIKE` for case-(in)sensitive matching.
- Avoid `LIMIT`/`OFFSET` in raw SQL; pass `$limitfrom`/`$limitnum` to `get_records_sql()` and similar `$DB` methods.
- Declare schema exclusively via `db/install.xml` (XMLDB) — never `CREATE TABLE` SQL — so column types map correctly to both engines.
- Avoid backtick-quoted identifiers (MySQL-only); the `{tablename}` placeholder syntax above is translated correctly per engine by `$DB`.

---

## Quality

```bash
# Install Moodle coding standard (once per project)
composer-mdle require --dev moodlehq/moodle-cs

# Check coding style
php-mdle vendor/bin/phpcs --standard=moodle local/example/

# Auto-fix coding style violations
php-mdle vendor/bin/phpcbf --standard=moodle local/example/

# Run tests
phpunit-mdle

# Run tests for a specific plugin
phpunit-mdle --filter local_example
```

- **moodle-cs** — enforces Moodle coding style on top of PSR-2; install via `composer-<folder> require --dev moodlehq/moodle-cs`.
- **phpunit** — run the full test suite before every commit, via the project's own `phpunit-<folder>` wrapper.

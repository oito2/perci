# Moodle Plugin Development

Standards and conventions for developing Moodle plugins in this project.
Covers file structure, API usage, security, architecture patterns, testing and common mistakes.

Last verified: 2026-11-04

---

## Development Environment

| Item | Value |
|---|---|
| Target Moodle release | {{MOODLE_VERSION}} |
| Target Moodle version number (`YYYYMMDDXX`) | {{MOODLE_FULLVERSION}} |
| Moodle root | The project folder (`.`) when it is a Moodle checkout; from Moodle 5.1 the web-accessible code lives in its `public/` subfolder. A plugin-only project needs a Moodle checkout to run tests — ask the user where it is. |

---

## Docker Development Environment

When the project was created by Perci as an application container of type **Moodle** (one project, one PHP version, one URL), Perci writes host-side wrapper scripts to `~/.local/bin/`, suffixed with the project's folder name. Each wrapper runs the tool inside that project's container, from the project root (`/var/www/html` in the container); host paths under the project folder are translated to container paths.

| Command | Runs |
|---|---|
| `php-<folder>` | The container's PHP CLI — use it for Moodle CLI scripts and for the project's own `vendor/bin/*` tools |
| `composer-<folder>` | Composer |
| `phpunit-<folder>` | The container's **global** PHPUnit PHAR — **not** Moodle's PHPUnit from `composer.json`; do not use it for Moodle tests |
| `phpcs-<folder>` / `phpcbf-<folder>` | The container's **global** PHP_CodeSniffer, which does not know the `moodle` standard |

For Moodle tests and the Moodle coding standard, always run the project's own tools through the PHP wrapper (`php-<folder> vendor/bin/phpunit`, `php-<folder> vendor/bin/phpcs --standard=moodle`). Examples for a project in folder `mdle`:

```bash
php-mdle admin/cli/purge_caches.php                 # Moodle 5.1+: php-mdle public/admin/cli/purge_caches.php
php-mdle admin/cli/upgrade.php --non-interactive    # after a version.php bump
```

---

## Target Moodle Version

Use only APIs available in the plugin's minimum supported Moodle version (`$plugin->requires` in the plugin's `version.php`; for a new plugin, this project's target release: {{MOODLE_VERSION}}). **Strictly avoid** deprecated functions.

### Moodle / PHP Support Matrix

| Moodle | PHP | Security support until | Perci container option → PHP |
|---|---|---|---|
| 4.5 (LTS) | 8.1 – 8.3 | 2027-10-04 | `4.4-4.5` → 8.1, 8.2, 8.3 |
| 5.1 | 8.2 – 8.4 | 2027-04-19 | `5.1+` → 8.3, 8.4 |
| 5.2 | 8.3 – 8.4 | 2027-10-04 | `5.1+` → 8.3, 8.4 |
| 5.3 (LTS) | 8.3 – 8.4 | 2029-10-01 | `5.1+` → 8.3, 8.4 |
| 4.1 – 4.4, 5.0 | — | **Out of support** | — |

Write PHP code compatible with the lowest PHP version supported by the plugin's minimum Moodle release, not just the PHP running in this project's container. If unsure whether that minimum matters for this project, ask the user before relying on newer syntax (e.g. enums, readonly properties, first-class callable syntax). Do not drop support for an older Moodle release on your own — ask first.

### Hook API vs lib.php Callbacks

The Hook API exists from **Moodle 4.3**; most output callbacks (e.g. `before_standard_html_head`) only gained a hook in **Moodle 4.4**. Decide from the plugin's own `version.php` `$plugin->requires`:

- **`requires` ≥ the first version that has the hook** — use the Hook API only (`classes/hook/` or a callbacks class + `db/hooks.php`).
- **`requires` below it** — implement both: the hook callback plus the legacy `lib.php` callback. Moodle ignores a legacy callback automatically when a hook callback replaces it, so no version guard is needed.
- Ask the user only when the plugin has no `version.php` yet and the minimum version is unknown.

```php
// lib.php — legacy callback, used only by versions without the hook
function local_example_before_standard_html_head(): string {
    return \local_example\output\head::content();
}
```

```php
// classes/hook_callbacks.php — Hook API callback (Moodle 4.4+)
namespace local_example;

class hook_callbacks {
    public static function before_standard_head_html_generation(
        \core\hook\output\before_standard_head_html_generation $hook,
    ): void {
        $hook->add_html(\local_example\output\head::content());
    }
}
```

```php
// db/hooks.php — registers the Hook API callback
defined('MOODLE_INTERNAL') || die();

$callbacks = [
    [
        'hook'     => \core\hook\output\before_standard_head_html_generation::class,
        'callback' => [\local_example\hook_callbacks::class, 'before_standard_head_html_generation'],
    ],
];
```

---

## Moodle 5.1+ — `public/` Directory & Routing Engine

From Moodle 5.1 the checkout has a `public/` directory: the web server's document root points to it, and everything web-accessible (including `admin/`, `local/`, `mod/`, `blocks/`, ...) lives under it, while `config.php`, `composer.json` and `vendor/` stay at the root. An optional Routing Engine enables cleaner URLs; a compatibility layer keeps script URLs (e.g. `/mod/forum/view.php?id=1`) working.

- **Check where `admin/` is before running any CLI script** (`ls public/admin/cli` vs `ls admin/cli`) — never hard-code one layout. The same applies to `admin/tool/phpunit/cli/` and `admin/tool/behat/cli/`.
- Plugin code keeps the same relative structure under `public/` (`public/local/<plugin>`). Never hard-code the Moodle root path in code; use `$CFG->dirroot` and `new moodle_url(...)`.
- In a Perci container created with the `5.1+` option, nginx already serves `public/`; `$CFG->wwwroot` is the app URL without a `/public` suffix.
- Moving an existing install to the 5.1 layout is out of scope unless the user asks for it.
- Do not implement routes for the Routing Engine unless explicitly asked — default to standard script-based pages.

---

## Required Files

Every plugin must include:

```text
version.php
lang/en/[component].php
classes/privacy/provider.php
```

Add these when the plugin needs them: `db/install.xml` (it has database tables), `db/upgrade.php` (a released schema changes), `db/access.php` (it defines capabilities).

**Optional (recommended):** `settings.php`, `lib.php`, `index.php`, `classes/`, `templates/`, `amd/src/`, `tests/`, `tests/behat/`

---

## File Standards

### version.php

```php
defined('MOODLE_INTERNAL') || die();

$plugin->component = 'local_example';
$plugin->version   = 2026010100; // Format: YYYYMMDDNN (NN = daily increment, starting at 00)
$plugin->requires  = 2024100700; // Minimum Moodle core version (this example: 4.5)
$plugin->maturity  = MATURITY_STABLE;
$plugin->release   = '1.0';
```

`$plugin->requires` is the version number of the oldest Moodle release the plugin supports. For a new plugin, use this project's target version number — {{MOODLE_FULLVERSION}} — unless the user asks to support older releases.

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
- Supported types: `INT`, `CHAR`, `TEXT`, `NUMBER`, `FLOAT`, `BINARY`. Prefer `NUMBER` (fixed precision and scale) over `FLOAT` (approximate) for decimal values.
- Edit `install.xml` with the XMLDB editor (*Site administration → Development → XMLDB editor*) so it stays valid and `upgrade.php` code can be generated from it.

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

## Security

| Concern | Rule |
|---|---|
| Access | `require_login()` (with the course/module when there is one) at the top of every page, then `require_capability()` / `has_capability()` against the right context. |
| State-changing requests | `require_sesskey()` (or `confirm_sesskey()` when handling the failure yourself) before any create/update/delete; links that change state carry `sesskey`. |
| Forms | Build them with `moodleform` (formslib): it adds and checks `sesskey`, and cleans each element by its declared `PARAM_*` type. |
| Input | `required_param()` / `optional_param()` with the narrowest `PARAM_*` type; never read `$_GET`/`$_POST` directly. |
| Output | Escape with `s()` (attributes and plain text), `format_string()` (short user-entered strings, e.g. names) and `format_text()` (rich text, with its format and context). In Mustache, `{{var}}` escapes; `{{{var}}}` only for already-formatted HTML. |
| SQL | `$DB` methods with `?` or `:named` placeholders and a params array; never concatenate values into SQL. |
| Files | Store through the File API (`get_file_storage()`), serve through `pluginfile.php` with a `<component>_pluginfile()` callback that checks access. |

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
namespace local_example\observer;
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
  observer/       → Event observers
  hook/           → Hook definitions and callbacks
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

Implement the metadata provider, the plugin request provider and `core_userlist_provider` (used to delete several users' data in one context):

```php
namespace local_example\privacy;

use core_privacy\local\metadata\collection;
use core_privacy\local\request\approved_contextlist;
use core_privacy\local\request\approved_userlist;
use core_privacy\local\request\contextlist;
use core_privacy\local\request\userlist;

class provider implements
    \core_privacy\local\metadata\provider,
    \core_privacy\local\request\plugin\provider,
    \core_privacy\local\request\core_userlist_provider {

    public static function get_metadata(collection $collection): collection {
        $collection->add_database_table('local_example', [
            'userid' => 'privacy:metadata:local_example:userid',
            'data'   => 'privacy:metadata:local_example:data',
        ], 'privacy:metadata:local_example');
        return $collection;
    }

    public static function get_contexts_for_userid(int $userid): contextlist {
        $contextlist = new contextlist();
        // $contextlist->add_from_sql(...): the contexts where $userid has data.
        return $contextlist;
    }

    public static function get_users_in_context(userlist $userlist): void {
        // $userlist->add_from_sql(...): the users with data in $userlist->get_context().
    }

    public static function export_user_data(approved_contextlist $contextlist): void {
        // writer::with_context($context)->export_data([...], $data) for each approved context.
    }

    public static function delete_data_for_all_users_in_context(\context $context): void {
        // Delete every user's data in $context.
    }

    public static function delete_data_for_user(approved_contextlist $contextlist): void {
        // Delete $contextlist->get_user()'s data in each approved context.
    }

    public static function delete_data_for_users(approved_userlist $userlist): void {
        // Delete the data of $userlist->get_userids() in $userlist->get_context().
    }
}
```

Every metadata string key (`privacy:metadata:...`) must exist in the language file. Cover the provider with a PHPUnit test extending `\core_privacy\tests\provider_testcase`.

---

## External API (Web Services)

Declare each endpoint as a class under `classes/external/` and register it in `db/services.php`. From Moodle 4.2 the external API classes live in the `core_external` namespace; the global names (`external_api`, `external_value`, ...) are deprecated aliases — use them only when the plugin must support Moodle older than 4.2.

```php
// classes/external/get_example.php
namespace local_example\external;

use core_external\external_api;
use core_external\external_function_parameters;
use core_external\external_single_structure;
use core_external\external_value;

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
| **Security** | Skipping checks | See [Security](#security) |

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

### JavaScript — ES Modules in `amd/src/`

Never use inline `<script>` tags. Avoid importing jQuery — use Moodle core modules or native DOM APIs instead. Write ES modules in `amd/src/` and build them into `amd/build/` (`npx grunt amd`); commit both, since Moodle loads the built files.

```javascript
// amd/src/example.js
import Ajax from 'core/ajax';
import Log from 'core/log';

export const init = (config) => {
    document.querySelector('[data-action="example-submit"]')
        ?.addEventListener('click', async() => {
            const result = await Ajax.call([{
                methodname: 'local_example_get_example',
                args: {id: config.recordId},
            }])[0];
            Log.debug('local_example: result', result);
        });
};
```

```mustache
{{! Load the module from a template }}
{{#js}}
require(['local_example/example'], function(mod) {
    mod.init({recordId: {{id}}});
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
- Follow the **Moodle coding style** (checked by `moodlehq/moodle-cs`).
- **Ignore when searching the codebase:** `.git`, `node_modules`, `vendor`, `.grunt`, `moodledata`, `cache`.

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

Run from the Moodle root, through the Docker wrappers when they exist (`php-<folder> ...`, `composer-<folder> ...`). Paths under `admin/` are under `public/admin/` from Moodle 5.1 — check the checkout. Replace `local/example` and `local_example` with the plugin's path and component.

```bash
# Coding style (install the standard once per checkout)
composer require --dev moodlehq/moodle-cs
php vendor/bin/phpcs --standard=moodle local/example/
php vendor/bin/phpcbf --standard=moodle local/example/     # auto-fix

# PHPUnit: Moodle's own PHPUnit from composer; needs $CFG->phpunit_prefix and $CFG->phpunit_dataroot in config.php
php admin/tool/phpunit/cli/init.php        # again after upgrades, plugin installs or new test files
php vendor/bin/phpunit --testsuite local_example_testsuite

# Behat: needs $CFG->behat_wwwroot, $CFG->behat_prefix and $CFG->behat_dataroot in config.php
php admin/tool/behat/cli/init.php          # prints the exact run command
php vendor/bin/behat --config <behat dataroot>/behatrun/behat/behat.yml --tags=@local_example

# JavaScript, when amd/src/ changed (needs npm install with the Node version in .nvmrc)
npx grunt amd
```

- Zero coding-standard errors in changed files; PHPUnit and Behat green for the plugin before reporting a task as done.
- After a `version.php` bump, run `admin/cli/upgrade.php --non-interactive` and purge caches before testing.

# Slim + Eloquent (illuminate/database)

Complement to `PHP-SLIM.md` — use it **together with** that standard, never on its own.
Covers Slim 4 projects that use Laravel's Eloquent ORM standalone through `illuminate/database` (the Capsule manager), without the Laravel framework: setup, models, mass assignment, relationships and N+1, and migrations.
Rules shared by every data layer (transaction ownership, migrations, test database) live in `PHP-SLIM.md` → **Database Access**; MySQL/MariaDB schema, indexing, locking and SQL dialect rules live in `MYSQL-MARIADB.md`.

Last verified: 2026-11-04

---

## Versions

`illuminate/database` follows Laravel's major versions; its minimum PHP rises with each major:

| `illuminate/database` | PHP requirement | Status |
|---|---|---|
| 13.x (current) | 8.3+ | Supported |
| 12.x | 8.2+ | Security fixes only, until 2027-02-24 |
| 11.x | 8.2+ | **Legacy** — out of security support since 2026-03-12 |
| 10.x | 8.1+ | **Legacy** — out of security support since 2025-02-04 |

- New projects use 13.x. On a legacy major, do not upgrade as a side effect of another task, but tell the user the major no longer receives security fixes.

- Keep every `illuminate/*` package (`database`, `events`, ...) on the **same major**.
- Laravel framework features (facades, `artisan`, service providers, `config()`/`env()` helpers) are not available — do not copy them from Laravel docs.

---

## Setup (Capsule)

Boot Eloquent once, from the container, before any model is used:

```php
// config/container.php
use Illuminate\Container\Container;
use Illuminate\Database\Capsule\Manager as Capsule;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Events\Dispatcher;

Capsule::class => static function (ContainerInterface $c): Capsule {
    $db = $c->get('settings')['db'];

    $capsule = new Capsule();
    $capsule->addConnection([
        'driver' => 'mysql',
        'host' => $db['host'],
        'port' => $db['port'],
        'database' => $db['name'],
        'username' => $db['user'],
        'password' => $db['pass'],
        'charset' => 'utf8mb4',
        'collation' => 'utf8mb4_unicode_ci',
        'prefix' => '',
    ]);
    $capsule->setEventDispatcher(new Dispatcher(new Container())); // needs illuminate/events; enables model events/observers
    $capsule->setAsGlobal();
    $capsule->bootEloquent();

    Model::shouldBeStrict($c->get('settings')['env'] !== 'prod');

    return $capsule;
},
```

- Resolve `Capsule::class` in the bootstrap (`$container->get(Capsule::class)`) so Eloquent is booted before routes run.
- `Model::shouldBeStrict()` turns lazy loading, silently discarded attributes and access to missing attributes into exceptions — enable it outside production.

---

## Models

- One model per table in `src/Infrastructure/Persistence` (or the project's existing location); `final` unless extended.
- Declare `$table` when the name is not the plural snake case of the class; set `public $timestamps = false` for tables without `created_at`/`updated_at`.
- Declare `$casts` for booleans, dates, JSON and enums so attributes come back typed.
- Keep models out of the HTTP layer where practical: repositories or services return models or DTOs; actions never build queries.

### Mass assignment

- Every model declares an explicit `$fillable` allowlist. Never `$guarded = []`.
- Never pass the raw request body to `create()`/`fill()`/`update()` — pass only validated fields:

```php
$user = User::create([
    'name' => $input->name,
    'email' => $input->email,
    'password' => password_hash($input->password, PASSWORD_DEFAULT),
]);
```

---

## Queries and Relationships

- Use the query builder and bindings (`where('email', $email)`); in `whereRaw()`, `selectRaw()`, `orderByRaw()` always pass values as the bindings array, never interpolated.
- Column names in `orderBy()`/`where()` coming from the request must be checked against an allowlist.
- Define relationships with return types (`hasMany()`, `belongsTo()`, `belongsToMany()`).
- **Prevent N+1:** eager load with `with()` (or `load()` on an existing collection) whenever a loop touches a relationship; strict mode catches misses in development.

```php
// One query for posts plus one for all their authors, instead of one per post.
$posts = Post::query()
    ->with('author:id,name')
    ->where('published', true)
    ->latest()
    ->limit(20)
    ->get();
```

- Use `chunkById()` or `lazyById()` for large batch jobs instead of `all()`.
- Wrap multi-model writes in `Capsule::connection()->transaction(fn () => ...)` — it commits on return and rolls back on exception.

---

## Migrations

`artisan migrate` does not exist outside Laravel. Use Phinx or plain SQL files as described in `PHP-SLIM.md` → **Database Access**, or this Eloquent-specific option:

| Option | When |
|---|---|
| **Schema builder scripts** | `Capsule::schema()->create('users', fn (Blueprint $t) => ...)` in numbered scripts, plus a table recording which scripts ran |

---

## Testing

- Do not mock Eloquent models; unit tests use fakes of the repository interfaces.
- Isolate integration tests with `Capsule::connection()->beginTransaction()` in `setUp()` and `rollBack()` in `tearDown()`.
- Keep `Model::shouldBeStrict()` on in tests so N+1 and mass-assignment mistakes fail the suite.

---

## Anti-Patterns

| Anti-pattern | Why |
|---|---|
| `$guarded = []` or `create($request->getParsedBody())` | Mass assignment of fields like `is_admin` |
| Relationship access inside a loop without `with()` | N+1 queries |
| Interpolated values in `*Raw()` methods | SQL injection |
| Mixing `illuminate/*` majors | Incompatible internals, runtime errors |
| Laravel-only helpers (`env()`, facades, `artisan`) | Not available standalone |
| `Model::all()` on large tables | Loads every row into memory |
| Strict mode disabled in development and tests | N+1 and silent attribute loss reach production |

---

## Quality

Adds to the checks in `PHP-SLIM.md` → Quality (run the test suite with strict mode on):

```bash
grep -rn 'guarded = \[\]' src/                 # must print nothing: every model declares $fillable
grep -rnE 'Raw\(' src/                        # review every hit: values only through the bindings array
```

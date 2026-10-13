# Slim + PDO Database Access

Complement to `PHP-SLIM.md` — use it **together with** that standard, never on its own.
Covers Slim 4 projects that talk to the database through PHP's native PDO, without an abstraction layer or ORM: connection setup, repositories, transactions, migrations and database tests.
MySQL/MariaDB schema, indexing, locking and SQL dialect rules live in `MYSQL-MARIADB.md`; apply it as well when the database is MySQL or MariaDB.

---

## Connection

Create one `PDO` instance in the container; inject it only into repository classes.

```php
// config/container.php
PDO::class => static function (ContainerInterface $c): PDO {
    $db = $c->get('settings')['db'];
    $dsn = sprintf('mysql:host=%s;port=%d;dbname=%s;charset=utf8mb4', $db['host'], $db['port'], $db['name']);

    return new PDO($dsn, $db['user'], $db['pass'], [
        PDO::ATTR_ERRMODE => PDO::ERRMODE_EXCEPTION,       // default since PHP 8.0; kept explicit
        PDO::ATTR_EMULATE_PREPARES => false,               // native prepared statements, typed results
        PDO::ATTR_DEFAULT_FETCH_MODE => PDO::FETCH_ASSOC,
    ]);
},
```

| Setting | Rule |
|---|---|
| Charset | `charset=utf8mb4` in the DSN — never `utf8` and never `SET NAMES` after connecting |
| Error mode | `ERRMODE_EXCEPTION`; never check `false` returns |
| Emulated prepares | `false`; a named placeholder can then appear only **once** per statement |
| Credentials | From settings/environment; never in the DSN literal |
| Persistent connections | Not by default; only with a measured reason |

---

## Repositories

- One repository class per aggregate (`PdoUserRepository implements UserRepository`); actions and services depend on the interface.
- Every query uses `prepare()` + bound parameters. `query()`/`exec()` only for constant SQL with no external input.
- Select explicit columns; map rows to typed objects (or arrays with documented shapes) at the repository boundary.
- Identifiers (table/column names, `ORDER BY` direction) cannot be bound — pick them from a hardcoded allowlist.

```php
// Returns the user with the given id, or null when none exists.
public function find(int $id): ?User
{
    $stmt = $this->pdo->prepare('SELECT id, name, email FROM users WHERE id = :id');
    $stmt->execute(['id' => $id]);
    $row = $stmt->fetch();

    return $row === false ? null : User::fromRow($row);
}

// Returns users whose ids are in $ids; builds one placeholder per id.
public function findMany(array $ids): array
{
    if ($ids === []) {
        return [];
    }
    $in = implode(',', array_fill(0, count($ids), '?'));
    $stmt = $this->pdo->prepare("SELECT id, name, email FROM users WHERE id IN ($in)");
    $stmt->execute(array_values($ids));

    return array_map(User::fromRow(...), $stmt->fetchAll());
}
```

Bind `LIMIT`/`OFFSET` with `bindValue($name, $value, PDO::PARAM_INT)`.

---

## Transactions

Wrap multi-statement writes in a transaction owned by the service (not the action), and always roll back on failure:

```php
// Runs $work inside a transaction; commits on success, rolls back and rethrows on any error.
public function transactional(callable $work): mixed
{
    $this->pdo->beginTransaction();
    try {
        $result = $work($this->pdo);
        $this->pdo->commit();
        return $result;
    } catch (\Throwable $e) {
        if ($this->pdo->inTransaction()) {
            $this->pdo->rollBack();
        }
        throw $e;
    }
}
```

- No HTTP calls, mail or file I/O inside the transaction.
- DDL statements commit implicitly on MySQL/MariaDB — never mix them into a data transaction.

---

## Migrations

Use the project's existing mechanism. When there is none, ask the user which to adopt:

| Option | When |
|---|---|
| **Phinx** (`robmorgan/phinx`, 0.16.x: 64-bit PHP 8.1+) | Versioned PHP migrations with up/down, status and seeds |
| **Plain SQL files** | Small projects: `migrations/NNNN_description.sql`, applied in order by a small script that records each file in a `schema_migrations` table |

```bash
vendor/bin/phinx init                     # creates phinx.php
vendor/bin/phinx create CreateUsersTable  # new migration class in db/migrations
vendor/bin/phinx migrate -e development
vendor/bin/phinx rollback -e development
vendor/bin/phinx status
```

- In the generated `phinx.php`, read credentials from the environment and change `'charset' => 'utf8'` to `'utf8mb4'` in every environment.
- Never edit a migration that has already run in a shared environment — add a new one.
- Review large-table changes against the online DDL rules in `MYSQL-MARIADB.md`.

---

## Testing

- Unit-test services against the repository **interface** with in-memory fakes.
- Test repositories against a **dedicated test database** (same engine and major version as production), configured through a separate env file or `phpunit.xml.dist` `<env>` entries — never the development or production database.
- Reset state per test: begin a transaction in `setUp()` and roll back in `tearDown()` (data-only tests), or truncate and re-seed the touched tables. Run migrations once before the suite.
- Do not substitute SQLite for MySQL/MariaDB when the SQL uses engine-specific syntax.

---

## Anti-Patterns

| Anti-pattern | Why |
|---|---|
| String-interpolated values in SQL | SQL injection |
| `charset=utf8` / missing charset | Breaks 4-byte characters (emoji) |
| Emulated prepares left on | Values come back as strings; weaker separation of SQL and data |
| `PDO` injected into actions | Mixes SQL into the HTTP layer; use repositories |
| `beginTransaction()` without rollback on error | Leaves the connection in an open transaction |
| `SELECT *` | Over-fetching; breaks on schema changes |
| Tests against the development database | Destroys data; non-repeatable results |

---

## Quality

In addition to the checks in `PHP-SLIM.md`:

```bash
vendor/bin/phinx status -e testing     # when Phinx is used: no pending migrations
vendor/bin/phpunit                     # including the repository tests against the test database
```

- `grep -rn "query(\|exec(" src/` must show only constant SQL.
- Every new query is covered by a repository test against the test database.

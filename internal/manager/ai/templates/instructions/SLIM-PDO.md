# Slim + PDO Database Access

Complement to `PHP-SLIM.md` — use it **together with** that standard, never on its own.
Covers Slim 4 projects that talk to the database through PHP's native PDO, without an abstraction layer or ORM: connection setup, repositories and transactions.
Rules shared by every data layer (transaction ownership, migrations, test database) live in `PHP-SLIM.md` → **Database Access**; MySQL/MariaDB schema, indexing, locking and SQL dialect rules live in `MYSQL-MARIADB.md`.

Last verified: 2026-11-04

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

A small helper on the repository or a unit-of-work class gives services one way to run a transaction:

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

---

## Testing

- Isolate repository tests with `$pdo->beginTransaction()` in `setUp()` and `$pdo->rollBack()` in `tearDown()`, using the same `PDO` instance the repository receives.
- A test that needs committed data (or runs DDL) truncates and re-seeds its tables instead.

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

---

## Quality

Adds to the checks in `PHP-SLIM.md` → Quality:

```bash
grep -rnE -- '->(query|exec)\(' src/          # every hit must be constant SQL with no external input
vendor/bin/phinx status -e testing             # when Phinx is used: no pending migrations on the test database
```

# Slim + Doctrine DBAL

Complement to `PHP-SLIM.md` — use it **together with** that standard, never on its own.
Covers Slim 4 projects that access the database through Doctrine DBAL (without the ORM): connection wiring, queries and the QueryBuilder, transactions, migrations with `doctrine/migrations`, and database tests.
MySQL/MariaDB schema, indexing, locking and SQL dialect rules live in `MYSQL-MARIADB.md`; apply it as well when the database is MySQL or MariaDB.

---

## Versions

| Package | Current major | PHP requirement | Notes |
|---|---|---|---|
| `doctrine/dbal` | 4.x | 8.2+ | 3.x (PHP 7.4+) is the previous major; APIs differ — check `composer.lock` first |
| `doctrine/migrations` | 3.x | 8.1+ | Works with DBAL 3.6+ and 4 |

Write code for the DBAL major the project has installed; do not upgrade it as part of another task.

---

## Connection

Create one `Doctrine\DBAL\Connection` in the container; inject it only into repository classes.

```php
// config/container.php
use Doctrine\DBAL\Connection;
use Doctrine\DBAL\DriverManager;

Connection::class => static function (ContainerInterface $c): Connection {
    $db = $c->get('settings')['db'];

    return DriverManager::getConnection([
        'driver' => 'pdo_mysql',
        'host' => $db['host'],
        'port' => $db['port'],
        'dbname' => $db['name'],
        'user' => $db['user'],
        'password' => $db['pass'],
        'charset' => 'utf8mb4',   // DBAL adds no charset by default
    ]);
},
```

The connection is lazy: it opens on the first query, so defining it in the container costs nothing on routes that do not use the database.

---

## Queries

Always pass values as parameters; DBAL binds them as prepared-statement parameters.

| Need | API |
|---|---|
| Read rows | `fetchAssociative()`, `fetchAllAssociative()`, `fetchOne()`, `fetchFirstColumn()`, `iterateAssociative()` (large sets) |
| Read via result | `executeQuery($sql, $params, $types)` → `Result` |
| Write (returns affected rows) | `executeStatement($sql, $params, $types)` |
| Simple CRUD | `insert($table, $data)`, `update($table, $data, $criteria)`, `delete($table, $criteria)` |
| New id | `lastInsertId()` |
| Dynamic queries | `createQueryBuilder()` |

```php
use Doctrine\DBAL\ArrayParameterType;
use Doctrine\DBAL\ParameterType;

// Returns the user with the given id, or null when none exists.
public function find(int $id): ?User
{
    $row = $this->db->fetchAssociative(
        'SELECT id, name, email FROM users WHERE id = ?',
        [$id],
        [ParameterType::INTEGER],
    );

    return $row === false ? null : User::fromRow($row);
}

// Returns users whose ids are in $ids; DBAL expands the list parameter.
public function findMany(array $ids): array
{
    $rows = $this->db->fetchAllAssociative(
        'SELECT id, name, email FROM users WHERE id IN (?)',
        [$ids],
        [ArrayParameterType::INTEGER],
    );

    return array_map(User::fromRow(...), $rows);
}
```

QueryBuilder — for filters assembled at runtime; values still go through `setParameter()`:

```php
$qb = $this->db->createQueryBuilder();
$qb->select('id', 'name', 'email')
    ->from('users')
    ->where($qb->expr()->eq('status', ':status'))
    ->setParameter('status', $status)
    ->orderBy('created_at', 'DESC')
    ->setMaxResults(50);

$rows = $qb->executeQuery()->fetchAllAssociative();
```

- Only `setParameter()` values, `setFirstResult()` and `setMaxResults()` are safe for user input. Every other QueryBuilder argument, and the table name and array **keys** of `insert()`/`update()`/`delete()`, is raw SQL — take identifiers only from a hardcoded allowlist, never from the request.
- Select explicit columns; map rows to typed objects at the repository boundary.

---

## Transactions

Use `transactional()`: it commits when the closure returns and rolls back and rethrows when it throws.

```php
$orderId = $this->db->transactional(function (Connection $db) use ($order): int {
    $db->insert('orders', $order->toRow());
    $id = (int) $db->lastInsertId();
    $db->executeStatement('UPDATE stock SET qty = qty - ? WHERE product_id = ?', [$order->qty, $order->productId]);
    return $id;
});
```

- The service layer owns the transaction, not the action.
- No HTTP calls, mail or file I/O inside the closure.

---

## Migrations (`doctrine/migrations`)

Standalone setup — two files at the project root, read by `vendor/bin/doctrine-migrations`:

```php
<?php
// migrations.php — migration storage and paths.
return [
    'table_storage' => ['table_name' => 'doctrine_migration_versions'],
    'migrations_paths' => ['App\Migrations' => __DIR__ . '/migrations'],
];
```

```php
<?php
// migrations-db.php — DriverManager params (returning a Connection instance also works).
require __DIR__ . '/config/env.php';   // the same .env loading the app uses

return [
    'driver' => 'pdo_mysql',
    'host' => $_ENV['DB_HOST'],
    'dbname' => $_ENV['DB_NAME'],
    'user' => $_ENV['DB_USER'],
    'password' => $_ENV['DB_PASS'],
    'charset' => 'utf8mb4',
];
```

```bash
vendor/bin/doctrine-migrations generate          # new class extending AbstractMigration
vendor/bin/doctrine-migrations status
vendor/bin/doctrine-migrations migrate --no-interaction
vendor/bin/doctrine-migrations migrate prev      # roll back one version
```

- Write `up()` and `down()` with `$this->addSql(...)`; keep one logical change per migration.
- Never edit a migration that has already run in a shared environment — add a new one.
- Review large-table changes against the online DDL rules in `MYSQL-MARIADB.md`.

---

## Testing

- Unit-test services against repository **interfaces** with in-memory fakes.
- Test repositories against a **dedicated test database** (same engine and major version as production), configured through test-only environment values — never the development or production database.
- Run `doctrine-migrations migrate` against the test database before the suite; isolate tests by wrapping each in `beginTransaction()`/`rollBack()` or truncating touched tables.
- Do not swap in SQLite (`pdo_sqlite`, `memory: true`) when the SQL uses MySQL/MariaDB-specific syntax.

---

## Anti-Patterns

| Anti-pattern | Why |
|---|---|
| Values concatenated into SQL or QueryBuilder expressions | SQL injection |
| Request data in QueryBuilder methods or CRUD array keys | Only parameter values are escaped |
| Missing `charset` in connection params | Server default may not be `utf8mb4` |
| `Connection` injected into actions | Mixes SQL into the HTTP layer; use repositories |
| Manual `beginTransaction()` without rollback | Leaves an open transaction on error; prefer `transactional()` |
| Code copied from DBAL 2/3 docs into a DBAL 4 project | Removed methods and changed signatures |
| Editing an executed migration | Environments diverge silently |

---

## Quality

In addition to the checks in `PHP-SLIM.md`:

```bash
vendor/bin/doctrine-migrations status            # no unexpected pending migrations
vendor/bin/doctrine-migrations up-to-date
vendor/bin/phpunit                               # including repository tests against the test database
```

- Every new query is covered by a repository test against the test database.

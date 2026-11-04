# MySQL / MariaDB Standard

Apply this standard when working on MySQL or MariaDB schema design, migrations, slow-query investigation, queue-style transactions, connection pools, or production database configuration. Prefer exact version checks before applying a feature-specific pattern because MySQL and MariaDB have diverged in several SQL details.

Last verified: 2026-11-04

## Activation
- Designing MySQL or MariaDB tables, indexes, and constraints
- Reviewing migrations before they run on large production tables
- Debugging slow queries, lock waits, deadlocks, or connection exhaustion
- Adding keyset pagination, upserts, full-text search, JSON columns, or queues
- Configuring application connections, read replicas, TLS, or slow logs

---

## Version Check
Start by identifying the engine and version:

```sql
SELECT VERSION();
SHOW VARIABLES LIKE 'version_comment';
```

### Supported lines

| Engine | Line | Status |
|---|---|---|
| MySQL | 9.7 LTS | Supported (premier support until 2031-04-30) |
| MySQL | 8.4 LTS | Supported (premier support until 2029-04-30) |
| MySQL | 26.x Innovation | Short-lived; only when the project already uses it |
| MySQL | 8.0, 5.7 | **End of life** (8.0 on 2026-04-30) — plan an upgrade to 8.4 or 9.7 |
| MariaDB | 10.11, 11.4, 11.8, 12.3 (LTS) | Supported (community maintenance ends 2028-02-16, 2029-05-29, 2028-06-04, 2029-06-12) |
| MariaDB | 10.6 and older | **End of life** (10.6 community support ended 2026-07-06) |

New projects target a supported LTS line. On an end-of-life version, keep the code compatible with it but tell the user it no longer receives security fixes.

Keep MySQL and MariaDB guidance separate when syntax differs:
- **MySQL 8.0+** documents row aliases as the replacement for `VALUES(col)` in `ON DUPLICATE KEY UPDATE`; `VALUES(col)` is deprecated there.
- **MariaDB** documents `VALUES(col)` as the supported way to reference inserted values in `ON DUPLICATE KEY UPDATE`; use it for cross-engine compatibility or MariaDB targets.
- `SKIP LOCKED` is appropriate for queue-like work only. It skips locked rows and can return an inconsistent view, so do not use it for general accounting or integrity-sensitive reads.

---

## Schema Defaults

```sql
CREATE TABLE orders (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    account_id BIGINT UNSIGNED NOT NULL,
    status VARCHAR(32) NOT NULL,
    total DECIMAL(15, 2) NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at DATETIME NULL,
    PRIMARY KEY (id),
    KEY idx_orders_account_status_created (account_id, status, created_at),
    KEY idx_orders_active (account_id, deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

*`utf8mb4_unicode_ci` works on MySQL and MariaDB; on a MySQL 8.0+-only target prefer `utf8mb4_0900_ai_ci` (see the table below).*

### Default Choices Summary

| Use Case | Prefer | Avoid |
| :--- | :--- | :--- |
| **Surrogate Primary Keys** | `BIGINT UNSIGNED AUTO_INCREMENT` | `INT` for tables that can grow beyond 2B rows |
| **UUID Lookup Keys** | `BINARY(16)` with conversion helpers | `VARCHAR(36)` primary keys on hot tables |
| **Money / Exact Quantities** | `DECIMAL(p, s)` | `FLOAT` or `DOUBLE` |
| **User-facing Text (MySQL 8.0+)** | `utf8mb4_0900_ai_ci` | `utf8` / `utf8mb3` legacy collations |
| **User-facing Text (MariaDB, or both engines)** | `utf8mb4_unicode_ci` | `utf8mb3` |
| **Application Timestamps** | `DATETIME` with UTC managed by app | Assuming `DATETIME` stores time zone metadata |
| **Soft Deletes** | `deleted_at DATETIME NULL` + scoped indexes | Filtering soft-deleted rows without an index |
| **Extensible Status Values** | Lookup table or constrained `VARCHAR` | `ENUM` when values change often |

---

## Migrations & Online DDL

> **CRITICAL RULE:** MySQL/MariaDB DDL statements execute an **implicit `COMMIT`**. DDL cannot be run within a multi-statement transaction or rolled back.

### Non-Blocking ALTER Statements
For online schema changes on InnoDB tables, specify `ALGORITHM` and `LOCK` clauses to avoid table locks. Adding a column can be instant (metadata-only) on MySQL 8.0.12+ and MariaDB 10.3+; fall back to `INPLACE` when `INSTANT` is refused:

```sql
ALTER TABLE orders
    ADD COLUMN notes VARCHAR(255) NULL,
    ALGORITHM=INSTANT;

-- When INSTANT is not supported for the change:
ALTER TABLE orders
    ADD INDEX idx_orders_status (status),
    ALGORITHM=INPLACE, LOCK=NONE;
```

### Large Production Tables (Zero-Downtime Migration)
For massive tables (>10M rows) or high-write systems where native DDL causes buffer pool pressure or replication lag:
- Use external shadow-copy migration tools: **`gh-ost`** or **`pt-online-schema-change`**.
- Always verify read/write traffic during dry runs before executing the final cutover.

---

## Indexing

Composite index order usually follows **equality predicates first**, then **range or sort columns**:

```sql
CREATE INDEX idx_orders_account_status_created
    ON orders (account_id, status, created_at);

SELECT id, total
FROM orders
WHERE account_id = ?
  AND status = 'pending'
  AND created_at >= ?
ORDER BY created_at DESC
LIMIT 50;
```

### FK Indexes
Always add explicit indexes on Foreign Key columns. Lack of FK indexes leads to severe table/gap lock contention during `DELETE` or `UPDATE` operations on parent tables.

### Investigating Queries with EXPLAIN

```sql
EXPLAIN
SELECT id, total
FROM orders
WHERE account_id = 123 AND status = 'pending'
ORDER BY created_at DESC
LIMIT 50;
```

#### Signals to Investigate
- `type`: `ALL` on a large table (Full table scan)
- `key`: `NULL` when a selective predicate exists
- `rows`: Very high row estimate for an interactive path
- `Extra`: `Using temporary`, `Using filesort`, or broad `Using where`

To measure real execution (it **runs** the query — use a replica or a copy for writes and heavy reads):
- **MySQL 8.0.18+:** `EXPLAIN ANALYZE SELECT ...`
- **MariaDB:** `ANALYZE SELECT ...` (or `ANALYZE FORMAT=JSON SELECT ...`); MariaDB has no `EXPLAIN ANALYZE`.

*Avoid adding indexes blindly. Each index increases write cost, migration time, backup size, and buffer-pool pressure.*

---

## Query Patterns

### Upsert

#### Cross-Engine / MariaDB Compatible Form:
```sql
INSERT INTO user_settings (user_id, setting_key, setting_value)
VALUES (?, ?, ?)
ON DUPLICATE KEY UPDATE
    setting_value = VALUES(setting_value),
    updated_at = CURRENT_TIMESTAMP;
```

#### MySQL 8.0+ Row-Alias Form:
```sql
INSERT INTO user_settings (user_id, setting_key, setting_value)
VALUES (?, ?, ?) AS new
ON DUPLICATE KEY UPDATE
    setting_value = new.setting_value,
    updated_at = CURRENT_TIMESTAMP;
```

### Keyset Pagination
Do not use deep `OFFSET` pagination (`LIMIT 50 OFFSET 100000`) on large tables; it forces the engine to scan and discard thousands of unused rows.

```sql
SELECT id, name, created_at
FROM products
WHERE (created_at, id) < (?, ?)
ORDER BY created_at DESC, id DESC
LIMIT 50;
```

Back it with a matching composite index:
```sql
CREATE INDEX idx_products_created_id ON products (created_at, id);
```

### JSON Fields
Use JSON columns for extension data, not for fields needing heavy relational filtering, foreign keys, or integrity constraints.

```sql
CREATE TABLE events (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    payload JSON NOT NULL,
    event_type VARCHAR(64)
        GENERATED ALWAYS AS (JSON_UNQUOTE(JSON_EXTRACT(payload, '$.type'))) STORED,
    KEY idx_events_type (event_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```
*For frequently queried JSON paths, expose a generated column and index that column.*

### Full-Text Search
```sql
ALTER TABLE articles ADD FULLTEXT KEY ft_articles_title_body (title, body);

SELECT id, title, MATCH(title, body) AGAINST (? IN NATURAL LANGUAGE MODE) AS score
FROM articles
WHERE MATCH(title, body) AGAINST (? IN NATURAL LANGUAGE MODE)
ORDER BY score DESC
LIMIT 20;
```
*Use an external search engine (Elasticsearch, Meilisearch) when typo tolerance, complex fuzzy ranking, or cross-table facets are required.*

---

## Transactions & Locking

Keep transactions short and lock rows in a deterministic order across code paths:

```sql
START TRANSACTION;

SELECT id, balance
FROM accounts
WHERE id IN (?, ?)
ORDER BY id
FOR UPDATE;

UPDATE accounts SET balance = balance - ? WHERE id = ?;
UPDATE accounts SET balance = balance + ? WHERE id = ?;

COMMIT;
```

### Deadlock and Lock-Wait Checklist
1. Lock rows in a deterministic order across all code paths.
2. Execute external API calls **before** opening the transaction, never inside it.
3. Add indexes for predicates used in `UPDATE`, `DELETE`, and locking reads.
4. On deadlock, roll back and retry the entire transaction with a bounded retry budget.
5. Capture `SHOW ENGINE INNODB STATUS\G` immediately after a deadlock event.

### Queue-Style Worker Claim
```sql
START TRANSACTION;

SELECT id
FROM jobs
WHERE status = 'pending'
ORDER BY created_at
LIMIT 1
FOR UPDATE SKIP LOCKED;

UPDATE jobs
SET status = 'processing', started_at = CURRENT_TIMESTAMP
WHERE id = ?;

COMMIT;
```
*Use `SKIP LOCKED` only for queue-like workloads where skipping a locked row is acceptable.*

---

## Application Connections (PHP)

PHP-FPM has no shared connection pool: each worker opens its own connection per request (PDO, Doctrine DBAL and Eloquent all build on PDO).

- Size `max_connections` above the sum of all PHP-FPM `pm.max_children` that can connect, plus workers, cron and admin sessions.
- Set the charset in the DSN or connection params (`charset=utf8mb4`), `PDO::ATTR_ERRMODE => PDO::ERRMODE_EXCEPTION` and `PDO::ATTR_EMULATE_PREPARES => false`.
- Avoid `PDO::ATTR_PERSISTENT` unless measured: a persistent connection keeps session state (open transactions, variables, locks) between requests.
- Long-running CLI workers reconnect after `wait_timeout` errors ("MySQL server has gone away") instead of holding one connection for hours.
- Connection details for each data layer: `SLIM-PDO.md`, `SLIM-DBAL.md`, `SLIM-ELOQUENT.md`.

---

## Diagnostics & Performance

First-pass diagnostic commands:
```sql
SHOW FULL PROCESSLIST;
SHOW ENGINE INNODB STATUS\G
SHOW VARIABLES LIKE 'slow_query_log';
SHOW VARIABLES LIKE 'long_query_time';
```

Enabling slow log dynamically:
```sql
SET GLOBAL slow_query_log = 'ON';
SET GLOBAL long_query_time = 1;
-- Temporarily, while hunting missing indexes only (very noisy):
-- SET GLOBAL log_queries_not_using_indexes = 'ON';
```

---

## Replication

Read replicas can lag. Do not route read-your-own-write paths, checkout flows, permission checks, or idempotency-key reads to a replica immediately after a write.

```sql
-- Legacy syntax (older MySQL / MariaDB)
SHOW SLAVE STATUS\G

-- MySQL 8.0.22+ and MariaDB 10.5.1+ (the only form on MySQL 8.4+)
SHOW REPLICA STATUS\G
```

---

## Security

```sql
CREATE USER 'app'@'%' IDENTIFIED BY 'use-a-secret-manager';
GRANT SELECT, INSERT, UPDATE, DELETE ON appdb.* TO 'app'@'%';

ALTER USER 'app'@'%' REQUIRE SSL;

-- Audit anonymous users
SELECT user, host FROM mysql.user WHERE user = '';

DROP USER IF EXISTS ''@'localhost';
DROP USER IF EXISTS ''@'%';
```

### Security Review Checklist
- Do not grant `ALL PRIVILEGES` or `*.*` to application users.
- Require TLS for application users crossing networks.
- Store credentials in platform secret managers, never in code or config files.
- Separate migration/admin users from runtime application users.

---

## Configuration Baseline

Example starting point for a dedicated database host; adjust every value to the measured workload:

```ini
[mysqld]
# Size from available RAM: roughly 50-75% of memory on a dedicated host,
# much less when the web server or PHP-FPM share the machine.
innodb_buffer_pool_size = <size>
innodb_flush_log_at_trx_commit = 1
sync_binlog = 1

max_connections = 300
thread_cache_size = 50

wait_timeout = 300
interactive_timeout = 300
innodb_lock_wait_timeout = 10

slow_query_log = ON
long_query_time = 1
log_queries_not_using_indexes = OFF   # turn on only temporarily; it logs every unindexed query

log_bin = mysql-bin
binlog_format = ROW
binlog_expire_logs_seconds = 604800
```

---

## Anti-Patterns

| Anti-Pattern | Risk Signal | Better Pattern |
| :--- | :--- | :--- |
| `SELECT *` in hot paths | Over-fetching, broken queries on schema change | Select explicit columns |
| Deep `OFFSET` pagination | Linear scans and slow response times | Keyset pagination (`WHERE (created, id) < (?, ?)`) |
| No index on FK joins | Slow joins, gap-lock heavy deletes | Index FK columns intentionally |
| Long transactions | Lock waits, undo log bloat | Commit small units of work quickly |
| DDL in multi-step SQL scripts | Unexpected implicit commits breaking rollback logic | Run DDL separately and handle migration steps carefully |
| Direct DML on `mysql.user` | Grant table corruption risk | Use `CREATE USER`, `ALTER USER`, `DROP USER` |
| Application user with admin grants | High blast radius on vulnerability | Least-privilege runtime user (`SELECT, INSERT, UPDATE, DELETE`) |
| Long-lived worker connection without reconnect | "Server has gone away" after `wait_timeout` | Reconnect on that error, or reconnect per job |
| Replica reads immediately after write | Stale user-facing state | Pin read-after-write flows to Primary |

---

## Output Expectations for Code / Schema Review
When this standard is used for a review, return:
1. **Engine/version assumptions** (MySQL 8.4 / 9.7 vs MariaDB, and whether the version is end of life).
2. **Highest-risk issues** (correctness, locks, implicit commits, security, migration blocking).
3. **Exact SQL or code changes** for the safe path.
4. **Validation plan:** `EXPLAIN` analysis, migration dry run, lock/deadlock checks, and rollback strategy.
5. **Syntax divergences** affecting the recommendation.

---

## Quality

Before reporting a schema or query change as done:

```bash
mysql -e "SELECT VERSION();"                       # or mariadb -e ...; confirm engine and version
mysql --database=<test_db> -e "EXPLAIN <query>;"   # every new or changed query on a large table
mysqlcheck --check --databases <test_db>           # after migrations on the test database
```

- Run the migration forward (and backward, when the tool supports it) on a test database with the **same engine and major version** as production.
- For large tables, confirm the `ALGORITHM`/`LOCK` clause is accepted (the statement fails instead of falling back to a locking copy) or plan a `gh-ost`/`pt-online-schema-change` run.
- Run the project's test suite; every new query has a test against the test database.

# PHP + Slim 4 Development

Standards for web applications and APIs built on PHP 8.x and Slim Framework 4.
Covers project layout, bootstrap, dependency injection, routing, middleware, error handling, security, templates, configuration, tooling and deployment.
Apply it to any task in a Slim 4 project. When the project accesses a database, also apply the matching complement: `SLIM-PDO.md`, `SLIM-DBAL.md` or `SLIM-ELOQUENT.md` (and `MYSQL-MARIADB.md` for MySQL/MariaDB specifics).

---

## Versions and Compatibility

Check the project's `composer.json` (`require.php`, `config.platform.php`) and `composer.lock` first; write code for the **lowest PHP version the project declares**, not for the PHP running locally.

| Package | Role | Current major | PHP requirement |
|---|---|---|---|
| `slim/slim` | Framework | 4.x | 7.4 – 8.5 |
| `slim/psr7` | PSR-7 implementation | 1.x | 8.0+ |
| `php-di/php-di` | PSR-11 container | 7.x | 8.0+ |
| `slim/twig-view` | Twig integration | 3.x (Twig 3) | 7.4+ (current Twig 3 releases need 8.1+) |
| `slim/csrf` | CSRF middleware | 1.x | 7.4+ |
| `vlucas/phpdotenv` | `.env` loading | 5.x | 7.2.5+ |
| `monolog/monolog` | PSR-3 logger | 3.x | 8.1+ |
| `phpstan/phpstan` | Static analysis | 2.x | 7.4+ |
| `phpunit/phpunit` | Tests | 13.x / 12.x / 11.x | 8.4.1+ / 8.3+ / 8.2+ |

- New projects target a PHP branch that still receives security fixes (8.2 or later); prefer 8.3+.
- Pick the newest PHPUnit major the project's minimum PHP allows.
- Do not upgrade a major version of any dependency as a side effect of another task — ask first.

---

## Docker Wrappers

When the project runs in a perci container, use the host wrappers suffixed with the project's folder name instead of a host PHP install: `php-<folder>`, `composer-<folder>`, `phpunit-<folder>`, `phpcs-<folder>`, `phpcbf-<folder>`. Run project-local tools through the PHP wrapper (`php-<folder> vendor/bin/phpstan analyse`). When no wrappers exist, use the project's own documented commands.

---

## Project Layout

```text
project/
├── public/              # web root: index.php (single entry point) and static assets only
├── src/                 # application code, PSR-4 (App\)
│   ├── Action/          # one invokable class per route
│   ├── Domain/          # entities, services, domain exceptions
│   ├── Infrastructure/  # repositories, external clients
│   └── Middleware/
├── config/              # settings.php, container.php, middleware.php, routes.php
├── templates/           # Twig templates
├── var/                 # cache/ and log/ (writable, git-ignored)
├── tests/               # PHPUnit (Tests\ namespace)
├── .env.example         # documented variables, no real secrets
└── composer.json
```

- `public/index.php` is the **only** PHP file reachable over HTTP. Everything else lives outside the web root.
- Respect an existing layout (e.g. the official skeleton's `app/` + `src/Application`, `src/Domain`, `src/Infrastructure`); do not reorganize it.
- `var/`, `vendor/` and `.env` are never committed.

---

## Bootstrap

Slim needs a PSR-7 implementation; `slim/psr7` is the default choice (Nyholm, Guzzle and Laminas Diactoros are also auto-detected). The official skeleton builds a PHP-DI container and passes it to `AppFactory`.

```php
<?php
// public/index.php — builds the container and the app, then handles the request.

declare(strict_types=1);

use DI\ContainerBuilder;
use Slim\Factory\AppFactory;

require __DIR__ . '/../vendor/autoload.php';

require __DIR__ . '/../config/env.php';   // loads .env, see Configuration

$builder = new ContainerBuilder();
$builder->addDefinitions(__DIR__ . '/../config/container.php');
if (($_SERVER['APP_ENV'] ?? 'prod') === 'prod') {
    $builder->enableCompilation(__DIR__ . '/../var/cache');
}
$container = $builder->build();

$app = AppFactory::createFromContainer($container);

(require __DIR__ . '/../config/middleware.php')($app);
(require __DIR__ . '/../config/routes.php')($app);

$app->run();
```

- `php-di/slim-bridge` (`DI\Bridge\Slim\Bridge::create($container)`) is an alternative that injects route placeholders and services into action parameters by name. Use it only if the project already does.

---

## Dependency Injection

- Register definitions in `config/container.php` returning an array; rely on PHP-DI autowiring for concrete classes and declare only interfaces, factories and scalar settings.
- Inject dependencies through constructors. Never pull services from the container inside domain code (no service locator).
- Bind interfaces to implementations (`UserRepository::class => DI\autowire(PdoUserRepository::class)`) so tests can swap them.

```php
<?php
// config/container.php — container definitions.

declare(strict_types=1);

use Monolog\Handler\StreamHandler;
use Monolog\Level;
use Monolog\Logger;
use Psr\Container\ContainerInterface;
use Psr\Http\Message\ResponseFactoryInterface;
use Psr\Log\LoggerInterface;
use Slim\Csrf\Guard;
use Slim\Psr7\Factory\ResponseFactory;
use Slim\Views\Twig;

return [
    'settings' => require __DIR__ . '/settings.php',

    LoggerInterface::class => static function (ContainerInterface $c): LoggerInterface {
        $logger = new Logger('app');
        $logger->pushHandler(new StreamHandler($c->get('settings')['log_file'], Level::Info));
        return $logger;
    },

    Twig::class => static fn (ContainerInterface $c): Twig => Twig::create(
        __DIR__ . '/../templates',
        ['cache' => $c->get('settings')['twig_cache']], // false in development
    ),

    ResponseFactoryInterface::class => DI\autowire(ResponseFactory::class),

    // Resolved lazily by the middleware stack, after the session is started.
    Guard::class => static fn (ResponseFactoryInterface $f): Guard => new Guard($f),
];
```

---

## Routing and Actions

- Use one invokable **action class** per route, resolved from the container by class name. Closures are acceptable only in throwaway prototypes.
- Group routes with a shared prefix or middleware via `$app->group()`.
- Name routes (`->setName()`) and build URLs with the route parser (`url_for()` in Twig) instead of hardcoding paths.
- Actions stay thin: read input, call a service, write the response. Business logic goes in `src/Domain`.

```php
<?php
// config/routes.php — route definitions.

declare(strict_types=1);

use App\Action\User\ListUsersAction;
use App\Action\User\ViewUserAction;
use App\Middleware\AuthMiddleware;
use Slim\App;
use Slim\Routing\RouteCollectorProxy;

return static function (App $app): void {
    $app->group('/users', function (RouteCollectorProxy $group): void {
        $group->get('', ListUsersAction::class)->setName('users.list');
        $group->get('/{id:[0-9]+}', ViewUserAction::class)->setName('users.view');
    })->add(AuthMiddleware::class);
};
```

```php
<?php

declare(strict_types=1);

namespace App\Action\User;

use App\Domain\User\UserService;
use Psr\Http\Message\ResponseInterface;
use Psr\Http\Message\ServerRequestInterface;
use Slim\Exception\HttpNotFoundException;

// Returns one user as JSON, or 404 when it does not exist.
final class ViewUserAction
{
    public function __construct(private readonly UserService $users)
    {
    }

    /** @param array{id: string} $args */
    public function __invoke(ServerRequestInterface $request, ResponseInterface $response, array $args): ResponseInterface
    {
        $user = $this->users->find((int) $args['id'])
            ?? throw new HttpNotFoundException($request);

        $response->getBody()->write(json_encode($user, JSON_THROW_ON_ERROR));
        return $response->withHeader('Content-Type', 'application/json');
    }
}
```

Every PHP file starts with `declare(strict_types=1);`. Custom middleware implements PSR-15 `Psr\Http\Server\MiddlewareInterface`.

---

## Middleware Order

Slim runs middleware **last in, first out**: the last one added is the outermost. Add the error middleware **last** so it wraps everything, including routing.

```php
<?php
// config/middleware.php — registration order is innermost first.

declare(strict_types=1);

use App\Middleware\SessionMiddleware;
use Psr\Log\LoggerInterface;
use Slim\App;
use Slim\Csrf\Guard;
use Slim\Views\Twig;
use Slim\Views\TwigMiddleware;

return static function (App $app): void {
    $container = $app->getContainer();
    $settings = $container->get('settings');

    $app->add(TwigMiddleware::create($app, $container->get(Twig::class)));
    $app->add(Guard::class);                            // resolved lazily; needs an active session
    $app->add(SessionMiddleware::class);                // starts the session before Guard runs
    $app->addBodyParsingMiddleware();                   // JSON/XML/form bodies -> getParsedBody()
    $app->addRoutingMiddleware();
    $app->addErrorMiddleware(
        $settings['display_error_details'],             // false in production
        true,
        true,
        $container->get(LoggerInterface::class),
    );
};
```

---

## Input Validation

- Treat `getParsedBody()`, `getQueryParams()`, route `$args` and uploaded files as untrusted. Validate type, presence, length and format at the action boundary before calling the domain.
- Constrain route placeholders with patterns (`{id:[0-9]+}`) and cast explicitly.
- Reject invalid input with `HttpBadRequestException` (or a 422 response carrying field errors for APIs) — never let it reach SQL or templates unchecked.
- Use the project's existing validation library, if any; otherwise small, tested validator classes.

---

## Error Handling and Logging

- `displayErrorDetails` is `false` in production — no stack traces, SQL or paths in responses.
- Throw Slim's HTTP exceptions (`HttpNotFoundException`, `HttpForbiddenException`, `HttpUnauthorizedException`, ...) for client errors; let the error middleware render them.
- Log through PSR-3 `LoggerInterface` (Monolog), injected — never `echo`, `var_dump` or `error_log` in application code.
- Log context as an array (`$logger->warning('Login failed', ['user_id' => $id])`); never log passwords, tokens or full request bodies.
- For API-only apps, register a custom error handler or renderer that returns a consistent JSON shape.

---

## Security

| Concern | Rule |
|---|---|
| CSRF | `slim/csrf` `Guard` (instantiated only after `session_start()`, or given explicit storage) on every route group that accepts browser form posts; render the token name/value (`getTokenNameKey()`, `getTokenValueKey()` and the matching request attributes) as hidden fields. Token-authenticated APIs without cookies do not need it. |
| Output | Twig autoescaping stays on (the default). `\|raw` only for content the app itself sanitized. |
| SQL | Prepared statements with bound parameters, always. Never interpolate request data into SQL — see the database complement. |
| Passwords | `password_hash($plain, PASSWORD_DEFAULT)` and `password_verify()`; rehash when `password_needs_rehash()` returns true. |
| Sessions | `session_start(['cookie_httponly' => true, 'cookie_secure' => true, 'cookie_samesite' => 'Lax', 'use_strict_mode' => true])`; `session_regenerate_id(true)` after login and privilege changes. |
| CORS | Add CORS headers only when a browser client on another origin needs them, with an explicit origin allowlist — never `*` with credentials. |
| Secrets | From environment variables only; never committed, never logged, never rendered. |
| Uploads | Validate size and MIME type, generate the stored filename, store outside `public/`. |

---

## Templates (Twig)

- Render through the injected `Slim\Views\Twig` (`$twig->render($response, 'user/view.html.twig', $data)`).
- Templates under `templates/`, suffix `.html.twig`; one base layout with `{% extends %}` / `{% block %}`.
- No business logic or queries in templates — actions pass ready-to-render data.
- Enable the Twig cache (`var/cache/twig`) in production and `false` in development.

---

## Configuration

- Load `.env` with `vlucas/phpdotenv` once, before the container is built, and fail fast on missing variables:

```php
<?php
// config/env.php — loads .env into $_ENV and validates required variables.

declare(strict_types=1);

$dotenv = Dotenv\Dotenv::createImmutable(dirname(__DIR__));
$dotenv->safeLoad();                       // no error when .env is absent (real env vars in production)
$dotenv->required(['APP_ENV', 'DB_HOST', 'DB_NAME', 'DB_USER', 'DB_PASS'])->notEmpty();
```

- phpdotenv fills `$_ENV` and `$_SERVER` (not `getenv()`). Read variables only in `config/settings.php`; the rest of the code receives typed settings through the container.
- Keep `.env.example` in sync whenever a variable is added or removed.

---

## Coding Style and Composer Scripts

- Follow **PSR-12** (or PER Coding Style if the project already uses it) with the tool the project already has: `squizlabs/php_codesniffer` (`--standard=PSR12`) or `friendsofphp/php-cs-fixer` (`@PSR12` / `@PER-CS` rule sets).
- Short array syntax, typed properties, parameters and return types everywhere; `final` classes by default; `readonly` for injected dependencies when the minimum PHP allows it.
- Expose checks as Composer scripts so every environment runs the same commands:

```json
"scripts": {
    "start": "php -S localhost:8080 -t public",
    "test": "phpunit",
    "analyse": "phpstan analyse",
    "cs": "phpcs --standard=PSR12 src tests",
    "cs-fix": "phpcbf --standard=PSR12 src tests"
}
```

---

## Testing

- PHPUnit with `phpunit.xml.dist` at the root and tests under `tests/` (`Tests\` namespace in `autoload-dev`).
- Unit-test domain services with test doubles for repositories.
- Test actions end to end without a web server: build the app as the bootstrap does (with test definitions), create a request and call `$app->handle()`.

```php
$request = (new Slim\Psr7\Factory\ServerRequestFactory())->createServerRequest('GET', '/users/1');
$response = $this->app->handle($request);

self::assertSame(200, $response->getStatusCode());
```

- Never point tests at production or shared development data.

---

## Web Server and Deployment

Nginx (PHP-FPM) — document root is `public/`, everything not a file goes to `index.php`:

```nginx
root /var/www/app/public;
index index.php;

location / {
    try_files $uri /index.php$is_args$args;
}

location ~ \.php$ {
    try_files $uri =404;
    include fastcgi_params;
    fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;
    fastcgi_pass unix:/run/php/php-fpm.sock;
}
```

- Apache: enable `mod_rewrite` and `AllowOverride All`; `public/.htaccess` holds `RewriteEngine On`, `RewriteCond %{REQUEST_FILENAME} !-f`, `RewriteCond %{REQUEST_FILENAME} !-d`, `RewriteRule ^ index.php [QSA,L]`.
- If the app is served from a sub-directory, call `$app->setBasePath('/subdir')`.
- Production install: `composer install --no-dev --optimize-autoloader`; `APP_ENV=prod`; error details off; PHP-DI compilation and Twig cache enabled; `var/` writable by the PHP user only; OPcache on.
- The PHP built-in server (`composer start`) is for local development only.

---

## Anti-Patterns

| Anti-pattern | Why |
|---|---|
| PHP files other than `index.php` under `public/` | Exposes code paths and bypasses middleware |
| Error middleware added before other middleware | Errors thrown by later-added (outer) middleware are not handled |
| `displayErrorDetails = true` in production | Leaks stack traces, paths and SQL |
| Fat closures in `routes.php` | Untestable, no DI; use invokable action classes |
| `$container->get()` inside domain classes | Hidden dependencies; inject through the constructor |
| SQL built by string concatenation | SQL injection; use bound parameters |
| `\|raw` on user content / disabling autoescape | XSS |
| `Access-Control-Allow-Origin: *` with credentials | Cross-origin data exposure |
| Secrets in `config/*.php` or committed `.env` | Credential leak |
| `md5`/`sha1` for passwords | Trivially crackable; use `password_hash()` |

---

## Quality

Run every check before reporting a task as done (through the project's Docker wrappers when they exist):

```bash
composer validate --strict
vendor/bin/phpcs --standard=PSR12 src tests      # or: vendor/bin/php-cs-fixer fix --dry-run --diff
vendor/bin/phpstan analyse --level=6 src tests   # raise the level gradually; 10 is the strictest in PHPStan 2
vendor/bin/phpunit
```

- Keep the PHPStan level in `phpstan.neon.dist`; never lower an existing level to make a change pass, and never add baseline entries for new code.
- Zero coding-standard violations and zero PHPStan errors in changed files; all tests green.

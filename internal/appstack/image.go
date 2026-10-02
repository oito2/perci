// Copyright (C) 2026  oito2
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package appstack

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/ui"
)

// SupportedPHPVersions is the full list of PHP versions a Container
// Aplicativo can be created with.
var SupportedPHPVersions = []string{"7.4", "8.0", "8.1", "8.2", "8.3", "8.4"}

// ValidPHPVersion reports whether v is one of SupportedPHPVersions.
func ValidPHPVersion(v string) bool {
	for _, s := range SupportedPHPVersions {
		if s == v {
			return true
		}
	}
	return false
}

func phpTag(version string) string { return strings.ReplaceAll(version, ".", "") }

// Moodle version range identifiers: the release-series buckets the
// "Versão do Moodle" question (Container Aplicativo, AppTypeMoodle only)
// offers. Each one fixes both which PHP versions are valid
// (PHPVersionsForMoodleVersion) and which Nginx routing recipe applies
// (buildAppServerBlock):
//   - 3.x and every 4.x bucket serve straight from the project root via
//     index.php, no r.php, no /public split.
//   - 5.0 uses r.php as the front controller but has no /public split
//     (root stays at the project folder).
//   - 5.1+ additionally serves all web-accessible files from /public.
//
// 3.x and the 4.x buckets share one Nginx recipe (appMoodleClassicRouting)
// but keep separate identifiers because their PHP ranges differ.
// MoodleVersion4x is the former single 4.x bucket: still accepted (with the
// 4.1 PHP range) for containers already saved with it, but no longer offered.
// 5.0 and 5.1+ overlap in PHP (8.3/8.4) and differ in Nginx recipe (r.php-only
// vs the /public split), so MoodleVersion is its own persisted
// config.AppContainer field instead of being inferred from PHPVersion.
const (
	MoodleVersion3x     = "3.x"
	MoodleVersion41     = "4.1"
	MoodleVersion42to43 = "4.2-4.3"
	MoodleVersion44to45 = "4.4-4.5"
	MoodleVersion4x     = "4.x"
	MoodleVersion50     = "5.0"
	MoodleVersion51Plus = "5.1+"
)

// moodleVersionPHP maps each Moodle version range to the PHP versions valid
// for every release in it. MoodleVersion51Plus covers 5.1, 5.2 and 5.3, so
// it offers only PHP 8.3/8.4: 5.2 and 5.3 require PHP 8.3 or later.
var moodleVersionPHP = map[string][]string{
	MoodleVersion3x:     {"7.4"},
	MoodleVersion41:     {"7.4", "8.0", "8.1"},
	MoodleVersion42to43: {"8.0", "8.1", "8.2"},
	MoodleVersion44to45: {"8.1", "8.2", "8.3"},
	MoodleVersion4x:     {"7.4", "8.0", "8.1"},
	MoodleVersion50:     {"8.2", "8.3", "8.4"},
	MoodleVersion51Plus: {"8.3", "8.4"},
}

// MoodleVersions returns the Moodle version range identifiers offered for
// new containers, in the order the "Versão do Moodle" select lists them.
// MoodleVersion4x is not included.
func MoodleVersions() []string {
	return []string{MoodleVersion3x, MoodleVersion41, MoodleVersion42to43, MoodleVersion44to45, MoodleVersion50, MoodleVersion51Plus}
}

// ValidMoodleVersion reports whether v is one of MoodleVersions() or
// MoodleVersion4x.
func ValidMoodleVersion(v string) bool {
	_, ok := moodleVersionPHP[v]
	return ok
}

// PHPVersionsForMoodleVersion returns the PHP versions valid for the given
// Moodle version range, or nil if moodleVersion isn't one of MoodleVersions().
func PHPVersionsForMoodleVersion(moodleVersion string) []string {
	return moodleVersionPHP[moodleVersion]
}

// ValidPHPVersionForMoodleVersion reports whether version is one of the PHP
// versions valid for moodleVersion.
func ValidPHPVersionForMoodleVersion(moodleVersion, version string) bool {
	for _, v := range moodleVersionPHP[moodleVersion] {
		if v == version {
			return true
		}
	}
	return false
}

// ImageName returns the perci-managed base image tag for a PHP version,
// e.g. "perci-php81" for "8.1" — built once per version and reused by every
// Container Aplicativo on that version.
func ImageName(version string) string { return "perci-php" + phpTag(version) }

// phpDockerfile is the Dockerfile every perci-php<version> image builds
// from. Building the image bakes in the invoking user's UID (ARG UID /
// usermod www-data), so every Container Aplicativo on this version shares
// one image without a permission mismatch on the bind-mounted html folder.
const phpDockerfile = `ARG PHP_VERSION=8.1
FROM php:${PHP_VERSION}-fpm

ARG UID=1000

RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    libzip-dev libpng-dev libicu-dev libxml2-dev libonig-dev \
    libjpeg-dev libfreetype6-dev libpq-dev libcurl4-openssl-dev libxslt-dev \
 && rm -rf /var/lib/apt/lists/*

RUN docker-php-ext-configure gd --with-freetype --with-jpeg

RUN docker-php-ext-install \
    pdo_mysql mysqli intl zip gd soap mbstring exif xml opcache

RUN if pecl install redis; then docker-php-ext-enable redis; else echo "WARN: redis extension not installed" >&2; fi
RUN if pecl install xdebug; then docker-php-ext-enable xdebug; else echo "WARN: xdebug extension not installed" >&2; fi

COPY --from=composer:latest /usr/bin/composer /usr/bin/composer

RUN set -e; \
    cd /tmp; \
    PHPCS_VERSION=3.13.5; \
    curl -fsSL "https://github.com/PHPCSStandards/PHP_CodeSniffer/releases/download/${PHPCS_VERSION}/phpcs.phar" -o phpcs.phar; \
    echo "9ff3ddf1fcc6c9347a57bed63da10edfc6610e4339c6bee149e92537609c7939  phpcs.phar" | sha256sum -c -; \
    curl -fsSL "https://github.com/PHPCSStandards/PHP_CodeSniffer/releases/download/${PHPCS_VERSION}/phpcbf.phar" -o phpcbf.phar; \
    echo "a5b097357e78615a5509d569fda709c8751f30d4c1bf2a073b6e2df9884f349b  phpcbf.phar" | sha256sum -c -; \
    mv phpcs.phar /usr/local/bin/phpcs; \
    mv phpcbf.phar /usr/local/bin/phpcbf; \
    chmod +x /usr/local/bin/phpcs /usr/local/bin/phpcbf

RUN set -e; \
    cd /tmp; \
    MINOR=$(echo "${PHP_VERSION}" | cut -d. -f2); \
    if [ "$MINOR" -ge 4 ]; then VER=12.5.30; HASH=2f6d24d19d1238d6b5332f85a12a1aa021c79cc9e704fbe25fca93181752cfe1; \
    elif [ "$MINOR" -ge 2 ]; then VER=11.5.39; HASH=63601d96eca81db5ab675657f4a0b4056ff7b4acfe0d5767a1d74f50b626456d; \
    else VER=10.5.55; HASH=3a29d4dd24dad475b3c761758854bd3d7ecdd85645097ff0589d98b408b7f4ba; fi; \
    curl -fsSL "https://phar.phpunit.de/phpunit-${VER}.phar" -o phpunit.phar; \
    echo "${HASH}  phpunit.phar" | sha256sum -c -; \
    mv phpunit.phar /usr/local/bin/phpunit; \
    chmod +x /usr/local/bin/phpunit

RUN usermod -u ${UID} www-data
WORKDIR /var/www/html
`

// DefaultPHPMemoryLimit is the memory_limit baked into phpIni below: what
// every Container Aplicativo gets unless its own config.AppContainer.
// PHPMemoryLimit overrides it via a per-app bind-mounted conf.d snippet
// (see WritePHPMemoryLimitConf).
const DefaultPHPMemoryLimit = "512M"

// phpIni is the php.ini every perci-php<version> image bakes in.
const phpIni = `log_errors = On
error_log = /var/log/php/error.log

memory_limit = ` + DefaultPHPMemoryLimit + `
max_execution_time = 300
upload_max_filesize = 256M
post_max_size = 256M
max_input_vars = 5000

realpath_cache_size = 4096K
realpath_cache_ttl = 600

opcache.enable = 1
opcache.memory_consumption = 256
opcache.interned_strings_buffer = 16
opcache.max_accelerated_files = 20000
opcache.revalidate_freq = 2
opcache.validate_timestamps = 1
opcache.save_comments = 1
opcache.enable_cli = 1

display_errors = On
display_startup_errors = On
error_reporting = E_ALL

date.timezone = America/Sao_Paulo
expose_php = Off
cgi.fix_pathinfo = 1

file_uploads = On
max_file_uploads = 20

session.cookie_httponly = 1
session.use_strict_mode = 1
session.gc_maxlifetime = 1440

xdebug.mode = off
xdebug.client_host = host.docker.internal
xdebug.client_port = 9003
xdebug.start_with_request = yes
xdebug.log = /var/log/php/xdebug.log
`

// phpImageDockerfile is phpDockerfile plus one extra COPY that bakes
// phpIni into the image at build time. phpIni is written into the build
// context as a sibling file; see EnsureImage.
const phpImageDockerfile = phpDockerfile + "\nCOPY php.ini /usr/local/etc/php/php.ini\n"

// EnsureImage builds ImageName(version), using phpDockerfile plus phpIni
// baked in (see phpImageDockerfile), when it's missing or was built from
// different content (see ensureImage). The build bakes in the invoking
// user's UID, so every Container Aplicativo on this version shares one
// image without a permission mismatch on the bind-mounted html folder.
func EnsureImage(ctx context.Context, exe *executor.Executor, stdout io.Writer, version string) error {
	if !ValidPHPVersion(version) {
		return fmt.Errorf("versão PHP não suportada: %s", version)
	}
	return ensureImage(ctx, exe, stdout, imageBuild{
		name:  ImageName(version),
		first: "primeira vez que a versão " + version + " é usada",
		files: map[string]string{"Dockerfile": phpImageDockerfile, "php.ini": phpIni},
		args:  []string{"PHP_VERSION=" + version, fmt.Sprintf("UID=%d", os.Getuid())},
	})
}

// imageLabel holds the hash of what an image was built from.
const imageLabel = "perci.hash"

// imageBuild is one perci-managed base image: its build-context files
// (name → content) and --build-arg values.
type imageBuild struct {
	name  string
	first string // why a first build happens, for the progress message
	files map[string]string
	args  []string
}

// hash identifies b's content: its files, in name order, and build args.
func (b imageBuild) hash() string {
	names := make([]string, 0, len(b.files))
	for n := range b.files {
		names = append(names, n)
	}
	sort.Strings(names)
	h := sha256.New()
	for _, n := range names {
		_, _ = fmt.Fprintf(h, "%s\x00%d\x00%s", n, len(b.files[n]), b.files[n])
	}
	for _, a := range b.args {
		_, _ = fmt.Fprintf(h, "arg\x00%s\x00", a)
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// ensureImage builds b when the image is missing or its perci.hash label
// differs from b.hash(): an image with no label or built from another
// Dockerfile/php.ini/supervisord.conf is rebuilt. Existing containers keep
// the image they were created from: the rebuild names the ones that need
// "Recriar".
func ensureImage(ctx context.Context, exe *executor.Executor, stdout io.Writer, b imageBuild) error {
	want := b.hash()
	got, inspectErr := exe.Output(ctx, executor.Options{}, "docker", "image", "inspect",
		"-f", `{{index .Config.Labels "`+imageLabel+`"}}`, "--", b.name)
	if inspectErr == nil && strings.TrimSpace(got) == want {
		return nil
	}
	rebuild := inspectErr == nil
	var users string
	if rebuild {
		users, _ = exe.Output(ctx, executor.Options{}, "docker", "ps", "-a",
			"--filter", "ancestor="+b.name, "--format", "{{.Names}}")
	}

	buildDir, err := os.MkdirTemp("", "perci-appstack-image-*")
	if err != nil {
		return fmt.Errorf("criar diretório de build temporário: %w", err)
	}
	defer func() { _ = os.RemoveAll(buildDir) }()
	for n, content := range b.files {
		if err := os.WriteFile(filepath.Join(buildDir, n), []byte(content), 0o644); err != nil {
			return fmt.Errorf("escrever %s: %w", n, err)
		}
	}

	if rebuild {
		ui.Warning(stdout, "A imagem "+b.name+" foi construída por outra versão do Perci — reconstruindo com a configuração atual (pode levar alguns minutos)...")
	} else {
		ui.Info(stdout, "Construindo imagem "+b.name+" ("+b.first+" — pode levar alguns minutos)...")
	}
	args := []string{"build", "-t", b.name, "--label", imageLabel + "=" + want}
	for _, a := range b.args {
		args = append(args, "--build-arg", a)
	}
	args = append(args, "--", buildDir)
	if err := exe.Run(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, "docker", args...); err != nil {
		return fmt.Errorf("construir imagem %s: %w", b.name, err)
	}
	if names := strings.Fields(users); len(names) > 0 {
		ui.Warning(stdout, "Estes contêineres continuam na imagem antiga até serem recriados (Docker → Gerenciar Containers → Recriar): "+strings.Join(names, ", "))
	}
	return nil
}

// PHPConfDir returns ~/.perci/appstack/php-conf/<folder>, where perci
// writes an app's per-container php.ini override (see
// WritePHPMemoryLimitConf). It lives under ~/.perci rather than inside
// cfg.WorkspacePath.
func PHPConfDir(folder string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("obter diretório home: %w", err)
	}
	return filepath.Join(home, ".perci", "appstack", "php-conf", folder), nil
}

// phpMemoryLimitOverrideFile is the fixed filename perci writes inside
// PHPConfDir(folder). The "zz-" prefix makes it sort (and load, and
// therefore win) after the extension .ini files that docker-php-ext-enable
// drops into the same conf.d directory at build time.
const phpMemoryLimitOverrideFile = "zz-perci-overrides.ini"

// WritePHPMemoryLimitConf (re)writes folder's php.ini override with
// memoryLimit (DefaultPHPMemoryLimit if empty) and returns its path, meant
// to be bind-mounted read-only into /usr/local/etc/php/conf.d/ inside the
// app's own container (appRunArgs/comboAppRunArgs). It is never part of
// the shared image or the container's writable layer, so it takes effect
// per-app and survives Recriar/Editar.
func WritePHPMemoryLimitConf(folder, memoryLimit string) (string, error) {
	// memoryLimit is validated here too, since it is interpolated verbatim
	// into an .ini file loaded by php-fpm inside the container.
	if memoryLimit != "" && !ValidPHPMemoryLimit.MatchString(memoryLimit) {
		return "", fmt.Errorf("limite de memória PHP inválido: %s", memoryLimit)
	}

	dir, err := PHPConfDir(folder)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("criar %s: %w", dir, err)
	}
	if memoryLimit == "" {
		memoryLimit = DefaultPHPMemoryLimit
	}
	path := filepath.Join(dir, phpMemoryLimitOverrideFile)
	content := "memory_limit = " + memoryLimit + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("escrever %s: %w", path, err)
	}
	return path, nil
}

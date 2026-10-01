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
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/ui"
)

// MariaDBContainerName is the fixed name of the appstack's single MariaDB
// container.
const MariaDBContainerName = "mariadb"

// MariaDBImage is the MariaDB image tag every MariaDB container is created
// from.
const MariaDBImage = "mariadb:11.4"

// ValidDBIdentifier matches a MariaDB-safe user/database identifier.
var ValidDBIdentifier = regexp.MustCompile(`^[A-Za-z0-9_]{1,32}$`)

// GenPassword generates a random 16-character alphanumeric password.
func GenPassword() string {
	// Removing '='/'+'/'/'/'-'/'_' from the base64 encoding can occasionally
	// leave fewer than 16 usable characters — re-sample instead of silently
	// returning a shorter (weaker) password in that rare case.
	for {
		b := make([]byte, 16)
		_, _ = rand.Read(b)
		s := strings.Map(func(r rune) rune {
			if r == '=' || r == '+' || r == '/' || r == '-' || r == '_' {
				return -1
			}
			return r
		}, base64.URLEncoding.EncodeToString(b))
		if len(s) >= 16 {
			return s[:16]
		}
	}
}

// MariaDBExists reports whether a container named MariaDBContainerName
// exists in any state (running or stopped).
func MariaDBExists(ctx context.Context, exe *executor.Executor) bool {
	return ContainerStatus(ctx, exe, MariaDBContainerName) != ""
}

// RemoveMariaDB force-removes the mariadb container, running or stopped.
// Never call this without the user's confirmation first — the GUI owns
// that prompt, same as RemoveNginx.
func RemoveMariaDB(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
	return removeContainer(ctx, exe, stdout, MariaDBContainerName, "contêiner mariadb")
}

// MariaDBDataDir returns the MariaDB data directory:
// {workspace}/localhost/databases/mariadb. There is no automatic migration
// of databases created under any other path into this one.
func MariaDBDataDir(workspace string) string {
	return filepath.Join(workspace, "localhost", "databases", "mariadb")
}

// MariaDBInitDir returns ~/.perci/appstack/mariadb/init, where the
// GRANT-privileges init script is generated and bind-mounted into
// /docker-entrypoint-initdb.d. Kept under ~/.perci — like CertsDir and
// NginxConfDir — because it's generated perci state, not project content.
func MariaDBInitDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("obter diretório home: %w", err)
	}
	return filepath.Join(home, ".perci", "appstack", "mariadb", "init"), nil
}

// writeGrantSQL (re)writes the 01-permissions.sql init script for dbUser
// and returns the directory it was written into (docker-entrypoint-initdb.d
// scripts must be mounted as a directory, not a single file). dbUser must
// already be validated against ValidDBIdentifier — the identifier is
// interpolated unescaped into the backtick-quoted GRANT statement.
func writeGrantSQL(dbUser string) (string, error) {
	dir, err := MariaDBInitDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("criar %s: %w", dir, err)
	}
	sql := fmt.Sprintf("GRANT ALL PRIVILEGES ON *.* TO `%s`@'%%' WITH GRANT OPTION;\nFLUSH PRIVILEGES;\n", dbUser)
	path := filepath.Join(dir, "01-permissions.sql")
	if err := os.WriteFile(path, []byte(sql), 0o644); err != nil {
		return "", fmt.Errorf("escrever %s: %w", path, err)
	}
	return dir, nil
}

// mariadbRunArgs builds the "docker run" argument list for the MariaDB
// container. Split out from CreateMariaDB, same reasoning as
// nginxRunArgs: the argument list is unit-testable without a real Docker
// daemon.
func mariadbRunArgs(dataDir, initDir string, env []string) []string {
	args := []string{
		"run", "-d",
		"--name", MariaDBContainerName,
		"--network", NetworkName,
		"--restart", "unless-stopped",
		"-p", "127.0.0.1:3306:3306",
		"-v", dataDir + ":/var/lib/mysql",
		"-v", initDir + ":/docker-entrypoint-initdb.d",
	}
	args = append(args, envNames(env)...) // values go through the client env, see envNames
	return append(args, "-e", "MYSQL_DATABASE=dev_db", "--", MariaDBImage)
}

// mariadbEnv returns the credential assignments mariadbRunArgs names —
// passed to docker through executor.Options.Env, never the argv.
func mariadbEnv(dbUser, dbPass, dbRootPass string) []string {
	return []string{
		"MYSQL_ROOT_PASSWORD=" + dbRootPass,
		"MYSQL_USER=" + dbUser,
		"MYSQL_PASSWORD=" + dbPass,
	}
}

// CreateMariaDB creates the appstack's MariaDB container: ensures
// NetworkName, (re)writes the GRANT-privileges init script for dbUser, and
// starts the container. Assumes no container named MariaDBContainerName
// exists yet — the GUI is responsible for checking MariaDBExists and
// calling RemoveMariaDB (with user confirmation) first when recreating.
//
// dbUser must already be validated against ValidDBIdentifier by the
// caller, in its own form, before this is reached.
func CreateMariaDB(ctx context.Context, exe *executor.Executor, stdout io.Writer, dbUser, dbPass string) error {
	if !isDockerInstalled(ctx, exe) {
		return fmt.Errorf("docker não instalado")
	}
	if !ValidDBIdentifier.MatchString(dbUser) {
		return fmt.Errorf("usuário do banco inválido: use apenas letras, números e underscore (1-32 caracteres)")
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("carregar config: %w", err)
	}

	workspace, err := resolveWorkspace(cfg)
	if err != nil {
		return err
	}
	if err := checkMariaDBCredentials(stdout, cfg, workspace, dbUser, dbPass); err != nil {
		return err
	}

	if err := EnsureNetwork(ctx, exe, stdout); err != nil {
		return err
	}

	// MariaDB only applies MYSQL_ROOT_PASSWORD when its data volume is
	// first initialized. Re-running this on an already-provisioned data
	// directory must reuse the persisted root password instead of
	// generating a new one, or cfg.Docker.MariaDB.DBRootPass would silently
	// diverge from the password actually set on the running database.
	dbRootPass := cfg.Docker.MariaDB.DBRootPass
	if dbRootPass == "" {
		dbRootPass = GenPassword()
	}

	dataDir := MariaDBDataDir(workspace)
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("criar diretório de dados: %w", err)
	}

	initDir, err := writeGrantSQL(dbUser)
	if err != nil {
		return err
	}

	ui.Info(stdout, "Criando contêiner mariadb...")
	env := mariadbEnv(dbUser, dbPass, dbRootPass)
	args := mariadbRunArgs(dataDir, initDir, env)
	if err := exe.Run(ctx, executor.Options{Stdout: stdout, Stderr: stdout, Env: env}, "docker", args...); err != nil {
		return fmt.Errorf("criar contêiner mariadb: %w", err)
	}

	if err := config.Update(func(cfg *config.Config) error {
		cfg.Docker.MariaDB.DBUser = dbUser
		cfg.Docker.MariaDB.DBPass = dbPass
		cfg.Docker.MariaDB.DBRootPass = dbRootPass
		cfg.Docker.MariaDB.DataUser = dbUser
		cfg.Docker.MariaDB.DataPass = dbPass
		return nil
	}); err != nil {
		return fmt.Errorf("contêiner mariadb criado, mas falha ao salvar configurações: %w", err)
	}

	return nil
}

// RecreateMariaDB is CreateMariaDB's "Recriar" flavor for the GUI's
// "Docker :: Gerenciar Containers" screen: reuses the dbUser/dbPass already
// persisted in cfg.Docker.MariaDB, with no form and no confirmation of its
// own — the caller already confirmed with the user. Errors if no MariaDB
// has ever been configured (nothing to reuse); use CreateMariaDB via the
// "Criar Container" screen's form for that instead.
func RecreateMariaDB(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("carregar config: %w", err)
	}
	if cfg.Docker.MariaDB.DBUser == "" {
		return fmt.Errorf("nenhum Contêiner MariaDB configurado ainda")
	}
	if MariaDBExists(ctx, exe) {
		if err := RemoveMariaDB(ctx, exe, stdout); err != nil {
			return err
		}
	}
	return CreateMariaDB(ctx, exe, stdout, cfg.Docker.MariaDB.DBUser, cfg.Docker.MariaDB.DBPass)
}

// DeleteMariaDB removes the mariadb container (if any) and clears
// cfg.Docker.MariaDB, keeping config.yaml's container list in sync with
// reality. Never call this without the user's confirmation first. Any
// Container Aplicativo created with DBAccess=true keeps pointing at these
// now-gone credentials until it's edited or recreated — deleting MariaDB
// doesn't cascade into repairing those apps.
func DeleteMariaDB(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
	if MariaDBExists(ctx, exe) {
		if err := RemoveMariaDB(ctx, exe, stdout); err != nil {
			return err
		}
	}
	if err := config.Update(func(cfg *config.Config) error {
		// The stack no longer lists MariaDB (DBUser/DBPass cleared), but the
		// data directory stays on disk with the credentials it was created
		// with — kept so a later CreateMariaDB over it can be checked
		// (checkMariaDBCredentials) and reuses the same root password.
		m := cfg.Docker.MariaDB
		cfg.Docker.MariaDB = config.MariaDBConfig{
			DBRootPass: m.DBRootPass,
			DataUser:   firstNonEmpty(m.DataUser, m.DBUser),
			DataPass:   firstNonEmpty(m.DataPass, m.DBPass),
		}
		return nil
	}); err != nil {
		return fmt.Errorf("mariadb removido, mas falha ao salvar configurações: %w", err)
	}
	return nil
}

// CheckMariaDBCredentials is checkMariaDBCredentials for callers about to
// remove the running container first (the GUI's Editar) — checked before
// anything is removed, so a refusal leaves the database running.
func CheckMariaDBCredentials(stdout io.Writer, dbUser, dbPass string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("carregar config: %w", err)
	}
	workspace, err := resolveWorkspace(cfg)
	if err != nil {
		return err
	}
	return checkMariaDBCredentials(stdout, cfg, workspace, dbUser, dbPass)
}

// checkMariaDBCredentials refuses dbUser/dbPass when MariaDB's data
// directory was already initialized with different credentials: the image
// ignores MYSQL_USER/MYSQL_PASSWORD on existing data, so config.yaml would
// end up holding credentials the database doesn't know (apps, Backup and
// Restore failing to authenticate). An initialized directory with no
// recorded credentials (configs from before they were recorded) can't be
// checked — a warning, not a refusal.
func checkMariaDBCredentials(stdout io.Writer, cfg *config.Config, workspace, dbUser, dbPass string) error {
	if !mariadbDataInitialized(MariaDBDataDir(workspace)) {
		return nil
	}
	m := cfg.Docker.MariaDB
	knownUser, knownPass := firstNonEmpty(m.DataUser, m.DBUser), firstNonEmpty(m.DataPass, m.DBPass)
	if knownUser == "" {
		ui.Warning(stdout, "O banco em "+MariaDBDataDir(workspace)+" já existe e o Perci não sabe com quais credenciais ele foi criado — "+
			"se forem diferentes de "+dbUser+", o MariaDB manterá as antigas.")
		return nil
	}
	if knownUser != dbUser || knownPass != dbPass {
		return fmt.Errorf("o banco em %s já foi criado com o usuário %q e outra senha, e o MariaDB ignora novas credenciais em dados existentes. "+
			"Mantenha as credenciais atuais, altere-as dentro do banco (ALTER USER) antes, ou remova essa pasta para recomeçar do zero",
			MariaDBDataDir(workspace), knownUser)
	}
	return nil
}

// mariadbDataInitialized reports whether dataDir already holds a MariaDB
// system database. A permission error counts as initialized (the image
// chowns the directory to its own user): unknown is treated as "don't
// assume it's empty".
func mariadbDataInitialized(dataDir string) bool {
	_, err := os.Stat(filepath.Join(dataDir, "mysql"))
	return !errors.Is(err, fs.ErrNotExist)
}

// resolveWorkspace is cfg's workspace folder, ~/workspace when unset —
// the one place the stack decides where projects and data live.
func resolveWorkspace(cfg *config.Config) (string, error) {
	return cfg.Workspace()
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

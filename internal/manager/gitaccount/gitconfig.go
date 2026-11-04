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

package gitaccount

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/ui"
)

// Dir is ~/.perci/git, which holds one gitconfig file per account and the
// known_hosts file used by account connections.
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, ".perci", "git"), nil
}

// GitconfigPath is the gitconfig file included for the account id.
func GitconfigPath(id string) (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, id+".gitconfig"), nil
}

// includeKey is the global git configuration key whose value includes
// the account's gitconfig file for every repository under folder.
func includeKey(folder string) string {
	return "includeIf.gitdir:" + filepath.Clean(folder) + "/.path"
}

// sshArgs are the ssh options every connection of acc uses: its key only,
// and the known_hosts file managed by perci.
func sshArgs(acc config.GitAccount) ([]string, error) {
	kh, err := KnownHostsPath()
	if err != nil {
		return nil, err
	}
	if strings.ContainsAny(kh, " \t") {
		return nil, fmt.Errorf("o caminho %s contém espaços, que o ssh não aceita em UserKnownHostsFile", kh)
	}
	return []string{"-i", acc.SSHKey, "-o", "IdentitiesOnly=yes", "-o", "UserKnownHostsFile=" + kh}, nil
}

// SSHCommand is the core.sshCommand value of acc: ssh with sshArgs,
// each argument shell-quoted.
func SSHCommand(acc config.GitAccount) (string, error) {
	args, err := sshArgs(acc)
	if err != nil {
		return "", err
	}
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = executor.ShellQuote(a)
	}
	return "ssh " + strings.Join(quoted, " "), nil
}

// WriteGitconfig writes the account's gitconfig file (user.name,
// user.email and core.sshCommand) through `git config --file` into a
// temporary file that then replaces the previous one.
func WriteGitconfig(ctx context.Context, exe *executor.Executor, acc config.GitAccount) error {
	path, err := GitconfigPath(acc.ID)
	if err != nil {
		return err
	}
	sshCmd, err := SSHCommand(acc)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("criar %s: %w", filepath.Dir(path), err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), "."+acc.ID+".gitconfig.tmp-*")
	if err != nil {
		return fmt.Errorf("criar arquivo temporário: %w", err)
	}
	tmpName := tmp.Name()
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}

	for _, kv := range [][2]string{
		{"user.name", acc.GitName},
		{"user.email", acc.GitEmail},
		{"core.sshCommand", sshCmd},
	} {
		if err := exe.Run(ctx, executor.Options{}, "git", "config", "--file", tmpName, "--", kv[0], kv[1]); err != nil {
			_ = os.Remove(tmpName)
			return fmt.Errorf("git config %s: %w", kv[0], err)
		}
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("gravar %s: %w", path, err)
	}
	return nil
}

// Apply writes the account's gitconfig file and makes sure the global
// git configuration includes it for acc.Folder. prev is the account as
// stored before an edit (nil for a new account): when its folder changed,
// the rule for the old folder is removed.
func Apply(ctx context.Context, exe *executor.Executor, stdout io.Writer, prev *config.GitAccount, acc config.GitAccount) error {
	if err := WriteGitconfig(ctx, exe, acc); err != nil {
		return err
	}
	path, err := GitconfigPath(acc.ID)
	if err != nil {
		return err
	}

	if prev != nil && filepath.Clean(prev.Folder) != filepath.Clean(acc.Folder) {
		if err := removeInclude(ctx, exe, prev.Folder, path); err != nil {
			return err
		}
	}

	key := includeKey(acc.Folder)
	current, err := globalValues(ctx, exe, key)
	if err != nil {
		return err
	}
	for _, v := range current {
		if v == path {
			ui.Info(stdout, "Regra do ~/.gitconfig já existe para "+acc.Folder)
			return nil
		}
	}
	if err := exe.Run(ctx, executor.Options{}, "git", "config", "--global", "--add", "--", key, path); err != nil {
		return fmt.Errorf("git config --global %s: %w", key, err)
	}
	ui.Info(stdout, "Regra adicionada ao ~/.gitconfig: repositórios em "+acc.Folder+" usam a conta "+acc.Name)
	return nil
}

// Remove deletes the global include rule of acc and its gitconfig file.
// The SSH key and the repositories are left untouched.
func Remove(ctx context.Context, exe *executor.Executor, stdout io.Writer, acc config.GitAccount) error {
	path, err := GitconfigPath(acc.ID)
	if err != nil {
		return err
	}
	if err := removeInclude(ctx, exe, acc.Folder, path); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remover %s: %w", path, err)
	}
	ui.Info(stdout, "Regra da conta "+acc.Name+" removida do ~/.gitconfig")
	return nil
}

// Forget removes, for every account of accounts, its includeIf rule, its
// gitconfig file and its stored API token. SSH keys and repositories stay.
// It goes on after a failure and returns every failure joined.
func Forget(ctx context.Context, exe *executor.Executor, stdout io.Writer, accounts []config.GitAccount) error {
	var errs []error
	for _, acc := range accounts {
		if err := Remove(ctx, exe, stdout, acc); err != nil {
			errs = append(errs, fmt.Errorf("conta %s: %w", acc.Name, err))
		}
		if acc.HasToken {
			if err := DeleteToken(ctx, exe, acc.ID); err != nil {
				errs = append(errs, fmt.Errorf("conta %s: remover o token do chaveiro: %w", acc.Name, err))
			}
		}
	}
	return errors.Join(errs...)
}

// removeInclude unsets the global include rule of folder whose value is
// path; a rule that no longer exists is not an error.
func removeInclude(ctx context.Context, exe *executor.Executor, folder, path string) error {
	key := includeKey(folder)
	err := exe.Run(ctx, executor.Options{}, "git", "config", "--global", "--fixed-value", "--unset-all", "--", key, path)
	if exitCode(err) == unsetKeyMissing {
		return nil
	}
	if err != nil {
		return fmt.Errorf("git config --global --unset-all %s: %w", key, err)
	}
	return nil
}

// globalValues lists every value of key in the global git configuration;
// a missing key yields none.
func globalValues(ctx context.Context, exe *executor.Executor, key string) ([]string, error) {
	out, err := exe.Output(ctx, executor.Options{}, "git", "config", "--global", "--get-all", "--", key)
	if exitCode(err) == gitConfigKeyMissing {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("git config --global --get-all %s: %w", key, err)
	}
	var values []string
	for _, line := range strings.Split(out, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			values = append(values, line)
		}
	}
	return values, nil
}

// Exit codes of `git config`: gitConfigKeyMissing for a read of a key
// that does not exist, unsetKeyMissing for an unset with no matching
// value.
const (
	gitConfigKeyMissing = 1
	unsetKeyMissing     = 5
)

// exitCode returns the process exit code wrapped in err, -1 for any
// other error and 0 for nil.
func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return -1
	}
	return exitErr.ExitCode()
}

// portSuffix returns the explicit port of acc when it is set and not 22.
func portSuffix(acc config.GitAccount) (int, bool) {
	if acc.SSHPort == 0 || acc.SSHPort == 22 {
		return 22, false
	}
	return acc.SSHPort, true
}

// SSHURL is the SSH clone URL of owner/repo on acc's host.
func SSHURL(acc config.GitAccount, owner, repo string) string {
	if port, ok := portSuffix(acc); ok {
		return "ssh://git@" + acc.Host + ":" + strconv.Itoa(port) + "/" + owner + "/" + repo + ".git"
	}
	return "git@" + acc.Host + ":" + owner + "/" + repo + ".git"
}

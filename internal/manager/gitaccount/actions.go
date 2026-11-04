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
	"regexp"
	"strings"

	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/ui"
)

// FixURL replaces the HTTP(S) origin of st with the SSH URL of the same
// repository on acc's host (RepoState.SSHURL), after checking with
// `git ls-remote` that the account's key reaches it. Any token embedded in
// the old URL goes away with it.
func FixURL(ctx context.Context, exe *executor.Executor, stdout io.Writer, acc config.GitAccount, st RepoState) error {
	if st.RemoteClass != RemoteHTTPS || st.SSHURL == "" {
		return fmt.Errorf("%s: o remote não é uma URL HTTPS da conta", st.Path)
	}
	env, err := netEnv(acc)
	if err != nil {
		return err
	}
	if err := gitNet(ctx, exe, env, st.Dir, "ls-remote", "--heads", "--", st.SSHURL); err != nil {
		return fmt.Errorf("%s: %s não respondeu pela chave da conta (%v); o remote não foi alterado", st.Path, st.SSHURL, err)
	}
	if err := exe.Run(ctx, executor.Options{}, "git", "-C", st.Dir, "remote", "set-url", "origin", st.SSHURL); err != nil {
		return fmt.Errorf("%s: git remote set-url: %w", st.Path, err)
	}
	ui.Success(stdout, "✓ "+st.Path+": origin agora é "+st.SSHURL)
	return nil
}

// FixURLs runs FixURL on every repository of repos with an HTTP(S)
// origin of the account. It fails when any of them failed.
func FixURLs(ctx context.Context, exe *executor.Executor, stdout io.Writer, acc config.GitAccount, repos []RepoState) error {
	failed, done := 0, 0
	for _, st := range repos {
		if st.RemoteClass != RemoteHTTPS {
			continue
		}
		done++
		if err := FixURL(ctx, exe, stdout, acc, st); err != nil {
			failed++
			ui.Err(stdout, "✗ "+err.Error())
		}
	}
	if done == 0 {
		return errors.New("nenhum repositório com URL HTTPS")
	}
	if failed > 0 {
		return fmt.Errorf("%d URL(s) não puderam ser ajustadas", failed)
	}
	return nil
}

// repoNamePattern accepts a repository name, or GitLab group/name paths.
var repoNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]+(/[A-Za-z0-9._-]+)*$`)

// SetRemote adds origin to st, a repository without one: the SSH URL of
// name (without ".git") under acc's user on acc's host. The repository
// on the server may not exist yet.
func SetRemote(ctx context.Context, exe *executor.Executor, acc config.GitAccount, st RepoState, name string) (string, error) {
	if st.RemoteClass != RemoteNone {
		return "", fmt.Errorf("%s já tem o remote origin", st.Path)
	}
	name, err := cleanRepoName(acc, name)
	if err != nil {
		return "", err
	}
	url := SSHURL(acc, acc.Username, name)
	if err := exe.Run(ctx, executor.Options{}, "git", "-C", st.Dir, "remote", "add", "origin", url); err != nil {
		return "", fmt.Errorf("git remote add: %w", err)
	}
	return url, nil
}

// cleanRepoName validates a repository name typed by the user: a name or
// group/name path, without ".git" and without a leading "<account user>/".
func cleanRepoName(acc config.GitAccount, name string) (string, error) {
	name = strings.TrimSuffix(strings.TrimSpace(name), ".git")
	if owner, rest, ok := strings.Cut(name, "/"); ok && strings.EqualFold(owner, acc.Username) {
		name = rest
	}
	if !repoNamePattern.MatchString(name) || strings.Contains(name, "..") {
		return "", fmt.Errorf("nome de repositório inválido: %q", name)
	}
	return name, nil
}

// ResetIdentity removes user.name and user.email from st's own config, so
// the repository uses the account's identity.
func ResetIdentity(ctx context.Context, exe *executor.Executor, st RepoState) error {
	for _, key := range []string{"user.name", "user.email"} {
		err := exe.Run(ctx, executor.Options{}, "git", "-C", st.Dir, "config", "--local", "--unset-all", key)
		if err != nil && exitCode(err) != unsetKeyMissing {
			return fmt.Errorf("git config --unset-all %s: %w", key, err)
		}
	}
	return nil
}

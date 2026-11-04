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
	"path/filepath"
	"strings"
	"time"

	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/ui"
)

// CloneURL turns source — a repository name ("perci", "group/app") or a
// URL of one of acc's repositories — into the SSH URL to clone. A URL of
// another owner or host is refused: its repository would not be listed
// under the account.
func CloneURL(acc config.GitAccount, source string) (string, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return "", errors.New("informe a URL ou o nome do repositório")
	}
	if !strings.ContainsAny(source, ":@") {
		name, err := cleanRepoName(acc, source)
		if err != nil {
			return "", err
		}
		return SSHURL(acc, acc.Username, name), nil
	}
	switch ClassifyRemote(acc, source) {
	case RemoteOK, RemoteHTTPS:
		r, _ := ParseRemote(source)
		return SSHURL(acc, r.Owner, r.Repo), nil
	}
	return "", fmt.Errorf("%s não é um repositório de %s em %s", RedactURL(source), acc.Username, acc.Host)
}

// DestDir resolves rel, a folder relative to acc's folder, into the
// absolute folder for a new repository. It must lie strictly inside the
// account folder, not exist or be empty, and not be inside another
// repository.
func DestDir(acc config.GitAccount, rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return "", errors.New("informe a pasta do repositório")
	}
	if filepath.IsAbs(rel) {
		return "", errors.New("informe a pasta relativa à pasta da conta")
	}
	dest := filepath.Join(acc.Folder, rel)
	if dest == acc.Folder || !within(dest, acc.Folder) {
		return "", fmt.Errorf("a pasta precisa ficar dentro de %s", acc.Folder)
	}

	for dir := filepath.Dir(dest); ; dir = filepath.Dir(dir) {
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
			return "", fmt.Errorf("%s fica dentro do repositório %s", dest, dir)
		}
		if dir == acc.Folder || dir == filepath.Dir(dir) {
			break
		}
	}

	entries, err := os.ReadDir(dest)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return dest, nil
	case err != nil:
		return "", fmt.Errorf("%s já existe e não é uma pasta", dest)
	case len(entries) > 0:
		return "", fmt.Errorf("%s já existe e não está vazia", dest)
	}
	return dest, nil
}

// cloneTimeout bounds one clone.
const cloneTimeout = 30 * time.Minute

// Clone clones url into dest with acc's key, never prompting, with git's
// progress on stdout. The account's includeIf rule gives the clone its
// identity. A failed clone leaves no folder behind that it created (git
// removes dest itself; Clone removes the parent folders it made).
func Clone(ctx context.Context, exe *executor.Executor, stdout io.Writer, acc config.GitAccount, url, dest string) error {
	env, err := netEnv(acc)
	if err != nil {
		return err
	}
	parent := filepath.Dir(dest)
	created := missingAncestors(parent)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("criar %s: %w", parent, err)
	}

	ctx, cancel := context.WithTimeout(ctx, cloneTimeout)
	defer cancel()
	ui.Info(stdout, "Clonando "+url+" em "+dest)
	opts := executor.Options{Env: env, Stdout: stdout, Stderr: stdout}
	if err := exe.Run(ctx, opts, "git", "clone", "--progress", "--", url, dest); err != nil {
		for _, dir := range created { // deepest first; only empty folders go
			_ = os.Remove(dir)
		}
		return fmt.Errorf("git clone: %w", err)
	}
	ui.Success(stdout, "Repositório clonado em "+dest)
	return nil
}

// missingAncestors returns dir and its ancestors that don't exist yet,
// deepest first.
func missingAncestors(dir string) []string {
	var missing []string
	for ; dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
		if _, err := os.Lstat(dir); err == nil {
			break
		}
		missing = append(missing, dir)
	}
	return missing
}

// Init creates dest (when missing) and runs `git init` in it; with a
// non-empty remoteName it also adds origin (SetRemote).
func Init(ctx context.Context, exe *executor.Executor, acc config.GitAccount, dest, remoteName string) error {
	if strings.TrimSpace(remoteName) != "" {
		if _, err := cleanRepoName(acc, remoteName); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return fmt.Errorf("criar %s: %w", dest, err)
	}
	if err := exe.Run(ctx, executor.Options{}, "git", "init", "-q", "--", dest); err != nil {
		return fmt.Errorf("git init: %w", err)
	}
	if strings.TrimSpace(remoteName) == "" {
		return nil
	}
	_, err := SetRemote(ctx, exe, acc, RepoState{Dir: dest, Path: dest, RemoteClass: RemoteNone}, remoteName)
	return err
}

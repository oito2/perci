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
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/executor"
)

// DefaultKeyPath is where the key generated for the account id goes:
// ~/.ssh/perci_<id>.
func DefaultKeyPath(id string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, ".ssh", "perci_"+id), nil
}

// GenerateKey creates an ed25519 key pair at path (and path+".pub") with
// comment, refusing to replace an existing file. A non-empty passphrase
// is handed to ssh-keygen through askpass, never on its command line.
func GenerateKey(ctx context.Context, exe *executor.Executor, askpass Askpass, path, comment, passphrase string) error {
	for _, p := range []string{path, path + ".pub"} {
		if _, err := os.Lstat(p); err == nil {
			return fmt.Errorf("%s já existe — escolha a chave existente ou outro nome", p)
		}
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("criar %s: %w", dir, err)
	}

	args := []string{"-q", "-t", "ed25519", "-f", path, "-C", comment}
	opts := executor.Options{}
	if passphrase == "" {
		args = append(args, "-N", "")
	} else {
		// ssh-keygen asks for the new passphrase twice.
		env, cleanup, err := askpass.env(passphrase, 2)
		if err != nil {
			return err
		}
		defer cleanup()
		opts.Env = env
	}
	var stderr bytes.Buffer
	opts.Stderr = &stderr
	if err := exe.Run(ctx, opts, "ssh-keygen", args...); err != nil {
		return fmt.Errorf("ssh-keygen: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// CheckKey requires path to be a regular file with its public key next
// to it (path+".pub").
func CheckKey(path string) error {
	if path == "" || !filepath.IsAbs(path) {
		return errors.New("escolha a chave SSH da conta")
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("chave SSH: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s não é um arquivo de chave", path)
	}
	if _, err := PublicKey(path); err != nil {
		return err
	}
	return nil
}

// PublicKey returns the public key of the private key at path, read from
// path+".pub".
func PublicKey(path string) (string, error) {
	data, err := os.ReadFile(path + ".pub")
	if err != nil {
		return "", fmt.Errorf("chave pública %s.pub: %w", path, err)
	}
	line := strings.TrimSpace(string(data))
	if _, err := Fingerprint(line); err != nil {
		return "", fmt.Errorf("chave pública %s.pub: %w", path, err)
	}
	return line, nil
}

// KeyEncrypted reports whether the private key at path is protected by
// a passphrase: ssh-keygen can't read it with an empty one.
func KeyEncrypted(ctx context.Context, exe *executor.Executor, path string) bool {
	_, err := exe.Output(ctx, executor.Options{}, "ssh-keygen", "-y", "-P", "", "-f", path)
	return err != nil
}

// ErrNoAgent means no SSH agent is reachable in this session.
var ErrNoAgent = errors.New("nenhum agente SSH ativo nesta sessão")

// AgentHasKey reports whether the session's SSH agent holds the key at
// path, comparing fingerprints with `ssh-add -l`.
func AgentHasKey(ctx context.Context, exe *executor.Executor, path string) (bool, error) {
	pub, err := PublicKey(path)
	if err != nil {
		return false, err
	}
	fp, err := Fingerprint(pub)
	if err != nil {
		return false, err
	}
	out, err := exe.Output(ctx, executor.Options{}, "ssh-add", "-l")
	switch exitCode(err) {
	case 0:
	case 1: // the agent holds no identities
		return false, nil
	case 2:
		return false, ErrNoAgent
	default:
		return false, fmt.Errorf("ssh-add -l: %w", err)
	}
	for _, line := range strings.Split(out, "\n") {
		if f := strings.Fields(line); len(f) >= 2 && f[1] == fp {
			return true, nil
		}
	}
	return false, nil
}

// unlockTimeout bounds ssh-add, which would otherwise wait on a
// passphrase prompt that never answers.
const unlockTimeout = 30 * time.Second

// UnlockKey adds the key at path to the session's SSH agent, handing
// passphrase to ssh-add through askpass once: a wrong passphrase fails
// instead of being asked again.
func UnlockKey(ctx context.Context, exe *executor.Executor, askpass Askpass, path, passphrase string) error {
	env, cleanup, err := askpass.env(passphrase, 1)
	if err != nil {
		return err
	}
	defer cleanup()

	ctx, cancel := context.WithTimeout(ctx, unlockTimeout)
	defer cancel()
	var out bytes.Buffer
	opts := executor.Options{Env: env, Stdout: &out, Stderr: &out}
	err = exe.Run(ctx, opts, "ssh-add", "--", path)
	switch exitCode(err) {
	case 0:
		return nil
	case 2:
		return ErrNoAgent
	}
	return fmt.Errorf("não foi possível desbloquear a chave (senha incorreta?): %s", strings.TrimSpace(out.String()))
}

var (
	githubGreeting = regexp.MustCompile(`Hi ([A-Za-z0-9._-]+)! You've successfully authenticated`)
	gitlabGreeting = regexp.MustCompile(`Welcome to GitLab, @([A-Za-z0-9._-]+)!`)
)

// connectTimeout bounds TestConnection.
const connectTimeout = 30 * time.Second

// TestConnection authenticates to acc's server with acc's key and returns
// the user name the server greets. It fails when the greeting names a
// user other than acc.Username, or when there is no greeting.
func TestConnection(ctx context.Context, exe *executor.Executor, acc config.GitAccount) (string, error) {
	args, err := sshArgs(acc)
	if err != nil {
		return "", err
	}
	port, _ := portSuffix(acc)
	args = append([]string{"-T", "-o", "BatchMode=yes", "-o", "ConnectTimeout=15", "-p", strconv.Itoa(port)}, args...)
	args = append(args, "--", "git@"+acc.Host)

	ctx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()
	var out bytes.Buffer
	// GitHub ends a successful check with exit code 1, so only the
	// greeting decides.
	_ = exe.Run(ctx, executor.Options{Stdout: &out, Stderr: &out}, "ssh", args...)
	text := strings.TrimSpace(out.String())

	user := ""
	for _, re := range []*regexp.Regexp{githubGreeting, gitlabGreeting} {
		if m := re.FindStringSubmatch(text); m != nil {
			user = m[1]
			break
		}
	}
	switch {
	case user == "" && strings.Contains(text, "Permission denied"):
		return "", fmt.Errorf("o servidor recusou a chave — cadastre a chave pública em %s na conta %s", acc.Host, acc.Username)
	case user == "" && strings.Contains(text, "Host key verification failed"):
		return "", fmt.Errorf("a chave do servidor %s ainda não foi confirmada", acc.Host)
	case user == "":
		return "", fmt.Errorf("não foi possível conectar a %s: %s", acc.Host, text)
	case !strings.EqualFold(user, acc.Username):
		return user, fmt.Errorf("a chave pertence ao usuário %s, não a %s", user, acc.Username)
	}
	return user, nil
}

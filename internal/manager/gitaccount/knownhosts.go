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
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/fsutil"
)

// pinnedHostKeys are the published SSH host keys of the public GitHub and
// GitLab services, trusted without asking.
var pinnedHostKeys = map[string][]string{
	"github.com": {
		"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl",
		"ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBEmKSENjQEezOmxkZMy7opKgwFB9nkt5YRrYMjNuG5N87uRgg6CLrbo5wAdT/y6v0mKV0U2w0WZ2YB/++Tpockg=",
		"ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQCj7ndNxQowgcQnjshcLrqPEiiphnt+VTTvDP6mHBL9j1aNUkY4Ue1gvwnGLVlOhGeYrnZaMgRK6+PKCUXaDbC7qtbW8gIkhL7aGCsOr/C56SJMy/BCZfxd1nWzAOxSDPgVsmerOBYfNqltV9/hWCqBywINIR+5dIg6JTJ72pcEpEjcYgXkE2YEFXV1JHnsKgbLWNlhScqb2UmyRkQyytRLtL+38TGxkxCflmO+5Z8CSSNY7GidjMIZ7Q4zMjA2n1nGrlTDkzwDCsw+wqFPGQA179cnfGWOWRVruj16z6XyvxvjJwbz0wQZ75XK5tKSb7FNyeIEs4TT4jk+S4dhPeAUC5y+bDYirYgM4GC7uEnztnZyaVWQ7B381AK4Qdrwt51ZqExKbQpTUNn+EjqoTwvqNj4kqx5QUCI0ThS/YkOxJCXmPUWZbhjpCg56i+2aB6CmK2JGhn57K5mj0MNdBXA4/WnwH6XoPWJzK5Nyu2zB3nAZp+S5hpQs+p1vN1/wsjk=",
	},
	"gitlab.com": {
		"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAfuCHKVTjquxvt6CM6tdG4SLp1Btn/nOeHHE5UOzRdf",
		"ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCsj2bNKTBSpIYDEGk9KxsGh3mySTRgMtXL583qmBpzeQ+jqCMRgBqB98u3z++J1sKlXHWfM9dyhSevkMwSbhoR8XIq/U0tCNyokEi/ueaBMCvbcTHhO7FcwzY92WK4Yt0aGROY5qX2UKSeOvuP4D6TPqKF1onrSzH9bx9XUf2lEdWT/ia1NEKjunUqu1xOB/StKDHMoX4/OKyIzuS0q/T1zOATthvasJFoPrAjkohTyaDUz2LN5JoH839hViyEG82yB+MjcFV5MU3N1l1QL3cVUCh93xSaua1N85qivl+siMkPGbO5xR/En4iEY6K2XPASUEMaieWVNTRCtJ4S8H+9",
		"ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBFSMqzJeV9rUzU4kWitGjeR4PWSa29SPqJ1fVkhtj3Hw9xjLVXVYrU9QlYWrOLXBpQ6KWjbjTDTdDkoohFzgbEY=",
	},
}

// KnownHostsPath is the known_hosts file of account connections.
func KnownHostsPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "known_hosts"), nil
}

// hostPatternFor is how known_hosts names host: the bare host on port
// 22, "[host]:port" on any other port.
func hostPatternFor(acc config.GitAccount) string {
	if port, ok := portSuffix(acc); ok {
		return "[" + acc.Host + "]:" + strconv.Itoa(port)
	}
	return acc.Host
}

// IsPinnedHost reports whether acc connects to a service whose host keys
// are built in (GitHub or GitLab public host, port 22).
func IsPinnedHost(acc config.GitAccount) bool {
	_, explicitPort := portSuffix(acc)
	return !explicitPort && pinnedHostKeys[acc.Host] != nil
}

// HostKey is one SSH host key offered by a server.
type HostKey struct {
	Line        string `json:"line"` // the known_hosts line
	Type        string `json:"type"`
	Fingerprint string `json:"fingerprint"`
}

// Fingerprint returns the SHA256 fingerprint ("SHA256:...") of an SSH
// key given as "type base64 [comment]", optionally preceded by a host
// pattern.
func Fingerprint(line string) (string, error) {
	fields := strings.Fields(line)
	for i := 0; i+1 < len(fields); i++ {
		if !isKeyType(fields[i]) {
			continue
		}
		blob, err := base64.StdEncoding.DecodeString(fields[i+1])
		if err == nil {
			sum := sha256.Sum256(blob)
			return "SHA256:" + base64.RawStdEncoding.EncodeToString(sum[:]), nil
		}
	}
	return "", fmt.Errorf("chave SSH inválida: %q", line)
}

// isKeyType reports whether field names an SSH public key algorithm.
func isKeyType(field string) bool {
	for _, prefix := range []string{"ssh-", "ecdsa-", "sk-"} {
		if strings.HasPrefix(field, prefix) {
			return true
		}
	}
	return false
}

// TrustPinnedHostKeys adds the built-in host keys of acc's host to the
// known_hosts file; a host without built-in keys is left alone.
func TrustPinnedHostKeys(acc config.GitAccount) error {
	if !IsPinnedHost(acc) {
		return nil
	}
	lines := make([]string, 0, len(pinnedHostKeys[acc.Host]))
	for _, k := range pinnedHostKeys[acc.Host] {
		lines = append(lines, acc.Host+" "+k)
	}
	return addKnownHosts(lines)
}

// HostTrusted reports whether the known_hosts file already holds a key
// for acc's host and port.
func HostTrusted(acc config.GitAccount) (bool, error) {
	lines, err := readKnownHosts()
	if err != nil {
		return false, err
	}
	pattern := hostPatternFor(acc)
	for _, l := range lines {
		if f := strings.Fields(l); len(f) >= 3 && f[0] == pattern {
			return true, nil
		}
	}
	return false, nil
}

// scanTimeout bounds ssh-keyscan, which otherwise waits on an unreachable
// host.
const scanTimeout = 15 * time.Second

// ScanHostKeys asks acc's server for its host keys with ssh-keyscan, for
// the user to confirm before TrustHostKeys stores them.
func ScanHostKeys(ctx context.Context, exe *executor.Executor, acc config.GitAccount) ([]HostKey, error) {
	ctx, cancel := context.WithTimeout(ctx, scanTimeout)
	defer cancel()
	port, _ := portSuffix(acc)
	out, err := exe.Output(ctx, executor.Options{}, "ssh-keyscan", "-T", "10", "-p", strconv.Itoa(port), "--", acc.Host)
	if err != nil {
		return nil, fmt.Errorf("ssh-keyscan %s: %w", acc.Host, err)
	}

	pattern := hostPatternFor(acc)
	var keys []HostKey
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) < 3 || strings.HasPrefix(f[0], "#") || f[0] != pattern {
			continue
		}
		fp, err := Fingerprint(line)
		if err != nil {
			continue
		}
		keys = append(keys, HostKey{Line: strings.Join(f[:3], " "), Type: f[1], Fingerprint: fp})
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("o servidor %s não respondeu com chaves SSH", pattern)
	}
	return keys, nil
}

// TrustHostKeys adds keys to the known_hosts file. Every key must name
// acc's host and port.
func TrustHostKeys(acc config.GitAccount, keys []HostKey) error {
	pattern := hostPatternFor(acc)
	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		f := strings.Fields(k.Line)
		if len(f) != 3 || f[0] != pattern {
			return fmt.Errorf("chave de host não pertence a %s: %q", pattern, k.Line)
		}
		if _, err := Fingerprint(k.Line); err != nil {
			return err
		}
		lines = append(lines, k.Line)
	}
	return addKnownHosts(lines)
}

// readKnownHosts returns the lines of the known_hosts file; a missing
// file has none.
func readKnownHosts() ([]string, error) {
	path, err := KnownHostsPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ler %s: %w", path, err)
	}
	var lines []string
	for _, l := range strings.Split(string(data), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
		}
	}
	return lines, nil
}

// addKnownHosts appends the lines missing from the known_hosts file and
// rewrites it atomically.
func addKnownHosts(lines []string) error {
	path, err := KnownHostsPath()
	if err != nil {
		return err
	}
	current, err := readKnownHosts()
	if err != nil {
		return err
	}
	changed := false
	for _, l := range lines {
		if !slices.Contains(current, l) {
			current = append(current, l)
			changed = true
		}
	}
	if !changed {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("criar %s: %w", filepath.Dir(path), err)
	}
	if err := fsutil.WriteFileAtomic(path, []byte(strings.Join(current, "\n")+"\n"), 0o644); err != nil {
		return fmt.Errorf("gravar %s: %w", path, err)
	}
	return nil
}

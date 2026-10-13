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

package megasync

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/executor"
)

// Installed reflects whether megasync resolves on PATH.
func TestInstalled_FollowsPath(t *testing.T) {
	for _, present := range []bool{true, false} {
		exe := &executor.Executor{LookPath: func(name string) (string, error) {
			if present && name == "megasync" {
				return "/usr/bin/megasync", nil
			}
			return "", errors.New("not found")
		}}
		if got := Installed(context.Background(), exe); got != present {
			t.Errorf("present=%v: Installed = %v", present, got)
		}
	}
}

func TestResolveRepo(t *testing.T) {
	tests := []struct {
		id       string
		ver      string
		wantPath string
		wantRPM  bool
		wantErr  bool
	}{
		{
			id:       "linuxmint",
			ver:      "22.3",
			wantPath: "xUbuntu_24.04",
		},
		{
			id:       "linuxmint",
			ver:      "22.0",
			wantPath: "xUbuntu_24.04",
		},
		{
			id:       "ubuntu",
			ver:      "24.04",
			wantPath: "xUbuntu_24.04",
		},
		{
			id:       "ubuntu",
			ver:      "26.04",
			wantPath: "xUbuntu_26.04",
		},
		{
			id:       "zorin",
			ver:      "18.1",
			wantPath: "xUbuntu_24.04",
		},
		{
			id:       "zorin",
			ver:      "18.0",
			wantPath: "xUbuntu_24.04",
		},
		{
			id:       "fedora",
			ver:      "44",
			wantPath: "Fedora_44",
			wantRPM:  true,
		},
		// unsupported combinations
		{id: "ubuntu", ver: "22.04", wantErr: true},
		{id: "fedora", ver: "43", wantErr: true},
		{id: "arch", ver: "", wantErr: true},
		{id: "", ver: "", wantErr: true},
		{id: "linuxmint", ver: "21.3", wantErr: true}, // based on Ubuntu 22.04, not compatible with xUbuntu_24.04
	}

	for _, tc := range tests {
		repo, err := resolveRepo(tc.id, tc.ver)
		if tc.wantErr {
			if err == nil {
				t.Errorf("resolveRepo(%q, %q): expected error, got nil", tc.id, tc.ver)
			}
			continue
		}
		if err != nil {
			t.Errorf("resolveRepo(%q, %q): unexpected error: %v", tc.id, tc.ver, err)
			continue
		}
		if repo.path != tc.wantPath {
			t.Errorf("resolveRepo(%q, %q).path = %q, want %q", tc.id, tc.ver, repo.path, tc.wantPath)
		}
		if repo.rpm != tc.wantRPM {
			t.Errorf("resolveRepo(%q, %q).rpm = %v, want %v", tc.id, tc.ver, repo.rpm, tc.wantRPM)
		}
	}
}

// Both repository scripts are valid bash and keep the properties that make
// them safe to re-run: a non-interactive key dearmor into a temp file, a
// signed-by source, GPG-checked DNF repo.
func TestInstallScripts(t *testing.T) {
	apt := aptInstallScript("xUbuntu_24.04")
	dnf := dnfInstallScript("Fedora_44")
	for name, script := range map[string]string{"apt": apt, "dnf": dnf} {
		if out, err := exec.Command("bash", "-n", "-c", script).CombinedOutput(); err != nil {
			t.Errorf("%s script: %v\n%s", name, err, out)
		}
		if !strings.Contains(script, "set -Eeuo pipefail") {
			t.Errorf("%s script must stop on the first failure", name)
		}
	}
	for _, want := range []string{"gpg --batch --yes --dearmor", "signed-by=" + aptKeyring, "https://mega.nz/linux/repo/xUbuntu_24.04/",
		"verify_mega_key \"$tmp\"", megaKeyFingerprint, "--force-confold", "DEBIAN_FRONTEND=noninteractive", "--max-time 600"} {
		if !strings.Contains(apt, want) {
			t.Errorf("apt script missing %q", want)
		}
	}
	for _, want := range []string{"gpgcheck=1", "https://mega.nz/linux/repo/Fedora_44/", dnfRepoFile,
		"verify_mega_key \"$tmp\"", "rpm --import " + rpmKeyFile, "gpgkey=file://" + rpmKeyFile, "--max-time 600"} {
		if !strings.Contains(dnf, want) {
			t.Errorf("dnf script missing %q", want)
		}
	}
}

// Installing is one privileged call.
func TestInstallFromAPT_OnePrompt(t *testing.T) {
	var buf bytes.Buffer
	exe := &executor.Executor{DryRun: true, UsePolicyKit: true, Stdout: &buf, Stderr: &buf}
	sudo := executor.Options{RequiresSudo: true, Stdout: &buf, Stderr: &buf}
	if err := installFromAPT(context.Background(), exe, sudo, "xUbuntu_24.04"); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(buf.String(), "pkexec"); n != 1 {
		t.Errorf("want one prompt, got %d", n)
	}
}

// installCore picks the repository for the distro and installs in one
// privileged call; an unsupported or unknown distro fails before any
// password prompt.
func TestInstallCore(t *testing.T) {
	for _, c := range []struct {
		id, ver, want string
	}{
		{"zorin", "18", "https://mega.nz/linux/repo/xUbuntu_24.04/"},
		{"ubuntu", "26.04", "https://mega.nz/linux/repo/xUbuntu_26.04/"},
		{"fedora", "44", dnfRepoFile},
	} {
		t.Run(c.id+"-"+c.ver, func(t *testing.T) {
			var buf bytes.Buffer
			exe := &executor.Executor{DryRun: true, UsePolicyKit: true, Stdout: &buf, Stderr: &buf}
			if err := installCore(context.Background(), exe, &buf, c.id, c.ver); err != nil {
				t.Fatal(err)
			}
			out := buf.String()
			if n := strings.Count(out, "pkexec"); n != 1 {
				t.Errorf("want one prompt, got %d", n)
			}
			if !strings.Contains(out, c.want) {
				t.Errorf("missing %q:\n%s", c.want, out)
			}
		})
	}
	for _, c := range [][2]string{{"debian", "13"}, {"", ""}} {
		var buf bytes.Buffer
		exe := &executor.Executor{DryRun: true, UsePolicyKit: true, Stdout: &buf, Stderr: &buf}
		if err := installCore(context.Background(), exe, &buf, c[0], c[1]); err == nil {
			t.Errorf("installCore(%q, %q): expected an error", c[0], c[1])
		}
		if strings.Contains(buf.String(), "pkexec") {
			t.Errorf("installCore(%q, %q) must not ask for a password", c[0], c[1])
		}
	}
}

// The key is imported only after its fingerprint matched, in both scripts.
func TestInstallScripts_VerifyKeyBeforeUse(t *testing.T) {
	apt, dnf := aptInstallScript("xUbuntu_24.04"), dnfInstallScript("Fedora_44")
	for name, c := range map[string][2]string{
		"apt": {apt, "gpg --batch --yes --dearmor"},
		"dnf": {dnf, "rpm --import"},
	} {
		if v, use := strings.Index(c[0], "verify_mega_key \"$tmp\""), strings.Index(c[0], c[1]); v < 0 || use < v {
			t.Errorf("%s: the key must be verified before it is used (verify at %d, use at %d)", name, v, use)
		}
	}
	if strings.Contains(dnf, "gpgkey=https") {
		t.Error("the dnf repo must trust the verified local key, not a URL")
	}
}

// verify_mega_key accepts only the pinned fingerprint.
func TestKeyCheckScript(t *testing.T) {
	run := func(fpr string) error {
		bin := t.TempDir()
		fake := "#!/bin/sh\nprintf 'pub:u:255:22:X:1:2::-:::scESC::::::23::0:\\nfpr:::::::::" + fpr + ":\\nfpr:::::::::48D4F37062092DA6664D42BECC657CA556002348:\\n'\n"
		if err := os.WriteFile(filepath.Join(bin, "gpg"), []byte(fake), 0o755); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("bash", "-c", "set -Eeuo pipefail\n"+keyCheckScript+"verify_mega_key /dev/null")
		cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Logf("%s", out)
		}
		return err
	}
	if err := run(megaKeyFingerprint); err != nil {
		t.Errorf("the pinned fingerprint was refused: %v", err)
	}
	if err := run("0000000000000000000000000000000000000000"); err == nil {
		t.Error("an unexpected fingerprint was accepted")
	}
}

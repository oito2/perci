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

package distro

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/executor"
)

func dryRun(buf *bytes.Buffer) *executor.Executor {
	return &executor.Executor{DryRun: true, UsePolicyKit: true, Stdout: buf, Stderr: buf}
}

func TestParseOSRelease(t *testing.T) {
	content := "NAME=\"Linux Mint\"\nID=linuxmint\nID_LIKE=\"ubuntu debian\"\nVERSION_ID=\"22.2\"\n"
	family, id, version := parseOSRelease(content)
	if family != Debian || id != "linuxmint" || version != "22.2" {
		t.Errorf("got %q %q %q", family, id, version)
	}
	if f, i, v := parseOSRelease(""); f != Unknown || i != "" || v != "" {
		t.Errorf("empty content: got %q %q %q", f, i, v)
	}
}

func TestNormalizedDistroAndDisplayName(t *testing.T) {
	for id, want := range map[string][2]string{
		"linuxmint": {"mint", "Linux Mint"},
		"zorin":     {"zorin", "Zorin OS"},
		"ubuntu":    {"ubuntu", "Ubuntu"},
		"kubuntu":   {"ubuntu", "Ubuntu"},
		"fedora":    {"fedora", "Fedora"},
		"pop":       {"", "pop"},
		"":          {"", ""},
	} {
		if got := normalizedDistro(id); got != want[0] {
			t.Errorf("normalizedDistro(%q) = %q, want %q", id, got, want[0])
		}
		if got := displayName(id); got != want[1] {
			t.Errorf("displayName(%q) = %q, want %q", id, got, want[1])
		}
	}
}

// The real os-release of the machine running the tests goes through the
// same parser — whatever it is, the cached values must agree with it.
func TestDetectMatchesParse(t *testing.T) {
	if Detect() != detectedFamily || RawID() != cachedID || VersionID() != cachedVer {
		t.Error("accessors disagree with the cached parse")
	}
	if DisplayName() != displayName(RawID()) {
		t.Error("exported helpers disagree with their pure versions")
	}
}

func TestInstallPkgs(t *testing.T) {
	t.Run("debian: update and install in one prompt", func(t *testing.T) {
		var buf bytes.Buffer
		if err := InstallPkgs(context.Background(), dryRun(&buf), &buf, Debian, "git", "curl"); err != nil {
			t.Fatal(err)
		}
		out := buf.String()
		if n := strings.Count(out, "pkexec"); n != 1 {
			t.Errorf("pkexec called %d times, want 1:\n%s", n, out)
		}
		for _, want := range []string{"'apt-get' 'update'", "'install' '-y' '--' 'git' 'curl'"} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q:\n%s", want, out)
			}
		}
	})
	t.Run("fedora: one dnf call", func(t *testing.T) {
		var buf bytes.Buffer
		if err := InstallPkgs(context.Background(), dryRun(&buf), &buf, Fedora, "git"); err != nil {
			t.Fatal(err)
		}
		out := buf.String()
		if strings.Count(out, "pkexec") != 1 || !strings.Contains(out, "dnf install -y -- git") {
			t.Errorf("unexpected batch:\n%s", out)
		}
	})
	t.Run("unknown family", func(t *testing.T) {
		err := InstallPkgs(context.Background(), dryRun(&bytes.Buffer{}), io.Discard, Unknown, "git")
		if !errors.Is(err, ErrUnsupportedFamily) || !strings.Contains(err.Error(), Unknown) {
			t.Errorf("err = %v", err)
		}
	})
}

var testRepo = SignedRepo{
	KeyringPath:    "/usr/share/keyrings/x.gpg",
	KeyURL:         "https://example.com/key.asc",
	AptListPath:    "/etc/apt/sources.list.d/x.list",
	AptListContent: "deb [signed-by=/usr/share/keyrings/x.gpg] https://example.com/apt stable main\n",
	DnfRepoPath:    "/etc/yum.repos.d/x.repo",
	DnfRepoBody:    "[x]\nname=X\nbaseurl=https://example.com/rpm\n",
	PkgName:        "x-pkg",
}

// script extracts the bash script InstallFromSignedRepo hands to bash -c
// (DryRun prints "[dry-run] pkexec [bash -c <script>]").
func script(t *testing.T, out string) string {
	t.Helper()
	i := strings.Index(out, "bash -c ")
	if i < 0 {
		t.Fatalf("no bash -c in %q", out)
	}
	return strings.TrimSuffix(strings.TrimSpace(out[i+len("bash -c "):]), "]")
}

func TestInstallFromSignedRepo(t *testing.T) {
	for _, family := range []string{Debian, Fedora} {
		t.Run(family, func(t *testing.T) {
			var buf bytes.Buffer
			if err := InstallFromSignedRepo(context.Background(), dryRun(&buf), &buf, family, testRepo); err != nil {
				t.Fatal(err)
			}
			out := buf.String()
			if n := strings.Count(out, "pkexec"); n != 1 {
				t.Errorf("pkexec called %d times, want 1", n)
			}
			s := script(t, out)
			want := []string{"'https://example.com/key.asc'", "'x-pkg'"}
			if family == Debian {
				want = append(want, "if [ ! -s '/usr/share/keyrings/x.gpg' ]", "gpg --batch --yes --dearmor", "'/etc/apt/sources.list.d/x.list'")
			} else {
				want = append(want, "rpm --import", "[ -s '/etc/yum.repos.d/x.repo' ]")
			}
			for _, w := range want {
				if !strings.Contains(s, w) {
					t.Errorf("script missing %q:\n%s", w, s)
				}
			}
			if _, err := exec.LookPath("bash"); err == nil {
				cmd := exec.Command("bash", "-n")
				cmd.Stdin = strings.NewReader(s)
				if b, err := cmd.CombinedOutput(); err != nil {
					t.Errorf("bash -n: %v\n%s", err, b)
				}
			}
		})
	}
}

func TestInstallFromSignedRepo_Rejects(t *testing.T) {
	plain := testRepo
	plain.KeyURL = "http://example.com/key.asc"
	if err := InstallFromSignedRepo(context.Background(), dryRun(&bytes.Buffer{}), io.Discard, Debian, plain); err == nil {
		t.Error("an http key URL must be rejected")
	}
	broken := testRepo
	broken.KeyURL = "://bad"
	if err := InstallFromSignedRepo(context.Background(), dryRun(&bytes.Buffer{}), io.Discard, Debian, broken); err == nil {
		t.Error("an invalid key URL must be rejected")
	}
	err := InstallFromSignedRepo(context.Background(), dryRun(&bytes.Buffer{}), io.Discard, Unknown, testRepo)
	if !errors.Is(err, ErrUnsupportedFamily) {
		t.Errorf("unknown family: err = %v", err)
	}
}

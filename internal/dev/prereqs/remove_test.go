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

package prereqs

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/distro"
	"github.com/oito2/perci/internal/executor"
)

// apt-get -s purge output captured on ubuntu:24.04: removing openssl takes
// ca-certificates with it.
const aptSimOpenSSL = `NOTE: This is only a simulation!
      apt-get needs root privileges for real execution.
Reading package lists...
Building dependency tree...
The following packages will be REMOVED:
  ca-certificates* openssl*
0 upgraded, 0 newly installed, 2 to remove and 2 not upgraded.
Purg ca-certificates [20260601~24.04.1]
Purg openssl [3.0.13-0ubuntu3.16]
`

// dnf5 remove --assumeno output captured on fedora:44.
const dnfSimCmakeData = `Package           Arch   Version           Repository      Size
Removing:
 cmake-data       noarch 0:4.3.0-1.fc44    fedora       9.4 MiB
Removing dependent packages:
 cmake            x86_64 0:4.3.0-1.fc44    fedora      40.5 MiB

Transaction Summary:
 Removing:           2 packages

After this operation, 50 MiB will be freed (install 0 B, remove 50 MiB).
Operation aborted by the user.
`

const dnfSimUnused = `Package           Arch   Version           Repository      Size
Removing:
 cmake            x86_64 0:4.3.0-1.fc44    fedora      40.5 MiB
Removing unused dependencies:
 make             x86_64 1:4.4.1-12.fc44   fedora       1.8 MiB

Transaction Summary:
 Removing:          2 packages
`

const dnfSimOnly = `Package  Arch   Version        Repository      Size
Removing:
 openssl x86_64 1:3.5.9-1.fc44 updates      1.9 MiB

Transaction Summary:
 Removing:           1 package
`

func TestExtraAptRemovals(t *testing.T) {
	if got := extraAptRemovals(aptSimOpenSSL, []string{"openssl"}); !slices.Equal(got, []string{"ca-certificates"}) {
		t.Errorf("openssl: %v", got)
	}
	if got := extraAptRemovals(aptSimOpenSSL, []string{"openssl", "ca-certificates"}); len(got) != 0 {
		t.Errorf("both listed: %v", got)
	}
	if got := extraAptRemovals("Remv libfoo:amd64 [1.0]\nRemv bar [2]\n", []string{"libfoo"}); !slices.Equal(got, []string{"bar"}) {
		t.Errorf("arch suffix / Remv: %v", got)
	}
}

func TestExtraDnfRemovals(t *testing.T) {
	if got := extraDnfRemovals(dnfSimCmakeData, []string{"cmake-data"}); !slices.Equal(got, []string{"cmake"}) {
		t.Errorf("dependent: %v", got)
	}
	if got := extraDnfRemovals(dnfSimUnused, []string{"cmake"}); !slices.Equal(got, []string{"make"}) {
		t.Errorf("unused: %v", got)
	}
	if got := extraDnfRemovals(dnfSimOnly, []string{"openssl"}); len(got) != 0 {
		t.Errorf("clean: %v", got)
	}
}

// removalBin puts fake apt-get/dnf/sh first on PATH. apt-get and dnf print
// simOut when asked to simulate (-s / --assumeno) and otherwise record
// their arguments in the returned file.
func removalBin(t *testing.T, simOut string) (log string) {
	t.Helper()
	dir := t.TempDir()
	log = filepath.Join(dir, "real-run.log")
	simFile := filepath.Join(dir, "sim.txt")
	if err := os.WriteFile(simFile, []byte(simOut), 0o644); err != nil {
		t.Fatal(err)
	}
	// apt-get -s exits 0, while dnf exits 1 when --assumeno aborts.
	for name, simStatus := range map[string]string{"apt-get": "0", "dnf": "1"} {
		body := "#!/bin/sh\nfor a in \"$@\"; do case \"$a\" in -s|--assumeno) cat " + simFile + "; exit " + simStatus + ";; esac; done\necho \"$0 $*\" >> " + log + "\n"
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, tool := range []string{"sh", "cat"} {
		if err := os.Symlink("/bin/"+tool, filepath.Join(dir, tool)); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
	return log
}

func TestRemovePackages_RefusesCascade(t *testing.T) {
	for _, tc := range []struct {
		family, sim, pkg, extra string
	}{
		{distro.Debian, aptSimOpenSSL, "openssl", "ca-certificates"},
		{distro.Fedora, dnfSimCmakeData, "cmake-data", "cmake"},
	} {
		log := removalBin(t, tc.sim)
		err := RemovePackages(context.Background(), executor.New(nil, nil), executor.Options{}, tc.family, tc.pkg)
		if err == nil || !strings.Contains(err.Error(), tc.extra) || !strings.Contains(err.Error(), "também removeria") {
			t.Errorf("%s: err = %v", tc.family, err)
		}
		if _, statErr := os.Stat(log); statErr == nil {
			t.Errorf("%s: the removal ran despite the cascade", tc.family)
		}
	}
}

func TestRemovePackages_RunsWhenNothingElseGoes(t *testing.T) {
	for _, tc := range []struct {
		family, sim, want string
	}{
		{distro.Debian, "Purg openssl [3.0]\n", "purge -y -- openssl"},
		{distro.Fedora, dnfSimOnly, "remove -y --no-autoremove -- openssl"},
	} {
		log := removalBin(t, tc.sim)
		if err := RemovePackages(context.Background(), executor.New(nil, nil), executor.Options{}, tc.family, "openssl"); err != nil {
			t.Fatalf("%s: %v", tc.family, err)
		}
		data, _ := os.ReadFile(log)
		if !strings.Contains(string(data), tc.want) {
			t.Errorf("%s: real run = %q, want %q", tc.family, data, tc.want)
		}
	}
}

// A dnf simulation that printed no plan (dnf failed) refuses the removal.
func TestRemovePackages_FedoraRefusesWithoutPlan(t *testing.T) {
	log := removalBin(t, "Unknown argument \"--\" for command \"remove\".\n")
	err := RemovePackages(context.Background(), executor.New(nil, nil), executor.Options{}, distro.Fedora, "openssl")
	if err == nil || !strings.Contains(err.Error(), "não foi possível simular") {
		t.Errorf("err = %v", err)
	}
	if _, statErr := os.Stat(log); statErr == nil {
		t.Error("the removal ran without a simulated plan")
	}
}

// Uninstalling OpenSSL through the catalogue goes through the simulation.
func TestUninstallFor_OpenSSLRefusedOnDebian(t *testing.T) {
	removalBin(t, aptSimOpenSSL)
	err := uninstallFor(context.Background(), executor.New(nil, nil), &bytes.Buffer{}, Prereq{ID: "openssl", Name: "OpenSSL"}, distro.Debian)
	if err == nil || !strings.Contains(err.Error(), "ca-certificates") {
		t.Errorf("err = %v", err)
	}
}

func TestUninstallGH_DebianChecksCascade(t *testing.T) {
	removalBin(t, "Purg gh [2.0]\nPurg git-extras [1]\n")
	err := uninstallFor(context.Background(), executor.New(nil, nil), &bytes.Buffer{}, Prereq{ID: "gh", Name: "GitHub CLI (gh)"}, distro.Debian)
	if err == nil || !strings.Contains(err.Error(), "git-extras") {
		t.Errorf("err = %v", err)
	}
}

// The shared runtime library stays when Libsecret Tools is removed on
// Debian; Fedora also handles the Git credential helper.
func TestRemovalPackagesFor_Libsecret(t *testing.T) {
	if got := removalPackagesFor("libsecret", distro.Debian); slices.Contains(got, "libsecret-1-0") || !slices.Contains(got, "libsecret-tools") {
		t.Errorf("debian removal = %v", got)
	}
	if got := packagesFor("libsecret", distro.Debian); !slices.Contains(got, "libsecret-1-0") {
		t.Errorf("debian install = %v", got)
	}
	for _, got := range [][]string{packagesFor("libsecret", distro.Fedora), removalPackagesFor("libsecret", distro.Fedora)} {
		if !slices.Contains(got, "git-credential-libsecret") {
			t.Errorf("fedora = %v", got)
		}
	}
}

func TestCatalogue_DockerWarnsAboutGroup(t *testing.T) {
	for _, p := range Catalogue {
		if p.ID == "docker" && !strings.Contains(p.Description, "grupo docker") {
			t.Errorf("docker description lacks the group warning: %q", p.Description)
		}
	}
}

func TestNodeInstallScript_PinnedAndBounded(t *testing.T) {
	for _, want := range []string{"/nvm-sh/nvm/" + nvmVersion + "/install.sh", "--connect-timeout 10", "--max-time 600"} {
		if !strings.Contains(nodeInstallScript, want) {
			t.Errorf("script lacks %q", want)
		}
	}
	if strings.Contains(nodeInstallScript, "/HEAD/") {
		t.Error("nvm installer is not pinned")
	}
	if out, err := exec.Command("bash", "-n", "-c", nodeInstallScript).CombinedOutput(); err != nil {
		t.Fatalf("bash -n: %v\n%s", err, out)
	}
}

func TestUninstallNode_RemovesNVMLinesFromRC(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/bash")
	block := "\n" + strings.Join(nvmRCLines, "\n") + "\n"
	bashrc := "alias ll='ls -l'\n" + block + "export EDITOR=vim\n"
	zshrc := "# zsh\n" + block
	for name, content := range map[string]string{".bashrc": bashrc, ".zshrc": zshrc} {
		if err := os.WriteFile(filepath.Join(home, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := uninstallNode(&bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	gotB, _ := os.ReadFile(filepath.Join(home, ".bashrc"))
	if string(gotB) != "alias ll='ls -l'\nexport EDITOR=vim\n" {
		t.Errorf(".bashrc = %q", gotB)
	}
	gotZ, _ := os.ReadFile(filepath.Join(home, ".zshrc"))
	if strings.Contains(string(gotZ), "NVM_DIR") {
		t.Errorf(".zshrc = %q", gotZ)
	}
}

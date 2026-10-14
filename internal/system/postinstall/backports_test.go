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

package postinstall

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/executor"
)

// backportsBudgieSources is the file add-apt-repository writes for the PPA
// on Ubuntu 26.04 (the key block shortened).
const backportsBudgieSources = `Types: deb
URIs: https://ppa.launchpadcontent.net/ubuntubudgie/backports-budgie/ubuntu/
Suites: resolute
Components: main
Signed-By: 
 -----BEGIN PGP PUBLIC KEY BLOCK-----
 Comment: Hostname: keyserver.ubuntu.com
 .
 -----END PGP PUBLIC KEY BLOCK-----
`

// Only Ubuntu Budgie 26.04 offers the PPA (it has no 24.04 builds), right
// after the system upgrade and depending on the package index.
func TestBackportsBudgie_OnlyOn2604(t *testing.T) {
	if _, ok := ubuntuBudgie2404Profile.Action("backports-budgie"); ok {
		t.Error("Ubuntu Budgie 24.04 offers backports-budgie")
	}
	for _, p := range []Profile{ubuntu2404Profile, ubuntu2604Profile} {
		if _, ok := p.Action("backports-budgie"); ok {
			t.Errorf("%s offers backports-budgie", p.ID)
		}
	}
	a, ok := ubuntuBudgie2604Profile.Action("backports-budgie")
	if !ok {
		t.Fatal("Ubuntu Budgie 26.04 lacks backports-budgie")
	}
	if a.DependsOn != "update-index" || a.Applied == nil {
		t.Errorf("action = %+v", a)
	}
	for i, act := range ubuntuBudgie2604Profile.Actions {
		if act.ID == "backports-budgie" && (i == 0 || ubuntuBudgie2604Profile.Actions[i-1].ID != "upgrade-system") {
			t.Errorf("backports-budgie at position %d, want right after upgrade-system", i)
		}
	}
}

// The PPA, the index refresh and the unattended full-upgrade run in one
// password prompt.
func TestBackportsBudgie_OneBatch(t *testing.T) {
	var buf bytes.Buffer
	exe := &executor.Executor{DryRun: true, UsePolicyKit: true, Stdout: &buf, Stderr: &buf}
	if err := applyBackportsBudgie(context.Background(), exe, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if n := strings.Count(out, "pkexec"); n != 1 {
		t.Fatalf("%d password prompts, want 1:\n%s", n, out)
	}
	add := strings.Index(out, "'add-apt-repository' '-y' '-n' 'ppa:ubuntubudgie/backports-budgie'")
	update := strings.Index(out, "'update'")
	upgrade := strings.Index(out, "'full-upgrade' '-y'")
	if add < 0 || update < add || upgrade < update {
		t.Errorf("steps out of order or missing:\n%s", out)
	}
	if !strings.Contains(out, "--force-confold") || !strings.Contains(out, "DEBIAN_FRONTEND=noninteractive") {
		t.Errorf("upgrade not unattended:\n%s", out)
	}
}

// The action counts as applied only while an enabled source points at the
// PPA, in either source format.
func TestAptSourceEnabled(t *testing.T) {
	d := t.TempDir()
	old := aptSourcesDir
	aptSourcesDir = d
	t.Cleanup(func() { aptSourcesDir = old })
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(d, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	remove := func(name string) {
		t.Helper()
		if err := os.Remove(filepath.Join(d, name)); err != nil {
			t.Fatal(err)
		}
	}

	write("ubuntu.sources", "Types: deb\nURIs: http://archive.ubuntu.com/ubuntu/\nSuites: resolute\nComponents: main\n")
	if aptSourceEnabled(backportsBudgieURI) {
		t.Fatal("applied without the PPA")
	}
	write("ppa.sources", backportsBudgieSources)
	if !aptSourceEnabled(backportsBudgieURI) {
		t.Error("the PPA file add-apt-repository writes is not detected")
	}
	write("ppa.sources", strings.Replace(backportsBudgieSources, "Components: main\n", "Components: main\nEnabled: no\n", 1))
	if aptSourceEnabled(backportsBudgieURI) {
		t.Error("a disabled PPA counts as applied")
	}
	remove("ppa.sources")

	write("ppa.list", "# deb https://ppa.launchpadcontent.net/ubuntubudgie/backports-budgie/ubuntu resolute main\n")
	if aptSourceEnabled(backportsBudgieURI) {
		t.Error("a commented line counts as applied")
	}
	write("ppa.list", "deb [signed-by=/etc/apt/keyrings/b.gpg arch=amd64] https://ppa.launchpadcontent.net/ubuntubudgie/backports-budgie/ubuntu resolute main\n")
	if !aptSourceEnabled(backportsBudgieURI) {
		t.Error("a one-line source with options is not detected")
	}
}

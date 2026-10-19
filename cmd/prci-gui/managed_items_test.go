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

package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// With nothing on PATH, the items that need Curl or Go report it as
// missing, and the others report nothing.
func TestGetPrereqsInfo_Missing(t *testing.T) {
	isolateActions(t)
	b, _ := newTestBase(t)
	missing := map[string][]string{}
	for _, it := range (&DevSetupService{*b}).GetPrereqsInfo() {
		missing[it.ID] = it.Missing
	}
	if !slices.Equal(missing["GitHub CLI (gh)"], []string{"Curl"}) {
		t.Errorf("gh: %v", missing["GitHub CLI (gh)"])
	}
	if !slices.Equal(missing["Wails3 CLI"], []string{"Go (Linguagens e SDKs)"}) {
		t.Errorf("wails3: %v", missing["Wails3 CLI"])
	}
	if len(missing["Git"]) != 0 {
		t.Errorf("git: %v", missing["Git"])
	}
}

// Each operation is refused when the item's state doesn't allow it.
func TestRunPrereqAction_Refusals(t *testing.T) {
	isolateActions(t)
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "curl"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	cases := []struct{ name, op, want string }{
		{"Wails3 CLI", "install", "requer: Go (Linguagens e SDKs)"},
		{"Curl", "install", "já está instalado"},
		{"Git", "update", "não está instalado"},
		{"Git", "uninstall", "não está instalado"},
		{"Git", "reinstall", "operação desconhecida"},
		{"Nope", "install", "desconhecido"},
	}
	for _, c := range cases {
		b, rec := newTestBase(t)
		if err := (&DevSetupService{*b}).RunPrereqAction(c.name, c.op); err != nil {
			t.Fatalf("%s %s refused synchronously: %v", c.name, c.op, err)
		}
		logs, done := rec.wait(t)
		if done["ok"] == true || !strings.Contains(done["error"].(string), c.want) {
			t.Errorf("%s %s: done = %v, want error containing %q", c.name, c.op, done, c.want)
		}
		if strings.Contains(logs, "dry-run") {
			t.Errorf("%s %s: a refused action ran commands:\n%s", c.name, c.op, logs)
		}
	}
}

// Update and uninstall reach their domain code for an installed item.
func TestRunPrereqAction_UpdateAndUninstall(t *testing.T) {
	isolateActions(t)
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	for op, want := range map[string]string{"update": "upgrade", "uninstall": "git"} {
		b, rec := newTestBase(t)
		if err := (&DevSetupService{*b}).RunPrereqAction("Git", op); err != nil {
			t.Fatal(err)
		}
		logs, done := rec.wait(t)
		if done["ok"] != true || !strings.Contains(logs, want) {
			t.Errorf("%s: done = %v\n%s", op, done, logs)
		}
	}
}

// Linguagens e SDKs: Go needs Curl and Flutter needs Git from
// Pré-requisitos; an installed Flutter updates through flutter upgrade.
func TestSDKs_RequirementsAndUpdate(t *testing.T) {
	isolateActions(t)
	b, _ := newTestBase(t)
	d := &DevSetupService{*b}
	missing := map[string][]string{}
	for _, it := range d.GetSDKsInfo() {
		missing[it.ID] = it.Missing
	}
	if !slices.Equal(missing["Go SDK"], []string{"Curl (Pré-requisitos)"}) || !slices.Equal(missing["Flutter + Dart SDK"], []string{"Git (Pré-requisitos)"}) {
		t.Errorf("missing = %v", missing)
	}

	b, rec := newTestBase(t)
	if err := (&DevSetupService{*b}).RunSDKAction("Flutter + Dart SDK", "install"); err != nil {
		t.Fatal(err)
	}
	if _, done := rec.wait(t); done["ok"] == true || !strings.Contains(done["error"].(string), "requer: Git (Pré-requisitos)") {
		t.Errorf("install without git: %v", done)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	flutterBin := filepath.Join(home, "development", "flutter", "bin", "flutter")
	if err := os.MkdirAll(filepath.Dir(flutterBin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(flutterBin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	b, rec = newTestBase(t)
	if err := (&DevSetupService{*b}).RunSDKAction("Flutter + Dart SDK", "update"); err != nil {
		t.Fatal(err)
	}
	if logs, done := rec.wait(t); done["ok"] != true || !strings.Contains(logs, flutterBin+" [upgrade]") {
		t.Errorf("flutter update: %v\n%s", done, logs)
	}
}

// IDEs: Zed Editor needs Curl from Pré-requisitos, VS Code and VSCodium need
// nothing. DryRun reports every IDE as installed, so install is refused and
// update reaches the IDE's own installer.
func TestIDEs_RequirementsAndActions(t *testing.T) {
	isolateActions(t)
	b, _ := newTestBase(t)
	missing := map[string][]string{}
	for _, it := range (&DevSetupService{*b}).GetIDEsInfo() {
		missing[it.ID] = it.Missing
	}
	if !slices.Equal(missing["Zed Editor"], []string{"Curl (Pré-requisitos)"}) || len(missing["VS Code"]) != 0 || len(missing["VSCodium"]) != 0 {
		t.Errorf("missing = %v", missing)
	}

	b, rec := newTestBase(t)
	if err := (&DevSetupService{*b}).RunIDEAction("VSCodium", "install"); err != nil {
		t.Fatal(err)
	}
	if _, done := rec.wait(t); done["ok"] == true || !strings.Contains(done["error"].(string), "já está instalado") {
		t.Errorf("install of an installed IDE: %v", done)
	}

	for name, want := range map[string]string{"Zed Editor": "zed.dev/install.sh", "VS Code": "packages.microsoft.com"} {
		b, rec = newTestBase(t)
		if err := (&DevSetupService{*b}).RunIDEAction(name, "update"); err != nil {
			t.Fatal(err)
		}
		if logs, done := rec.wait(t); done["ok"] != true || !strings.Contains(logs, want) {
			t.Errorf("%s update: %v\n%s", name, done, logs)
		}
	}
}

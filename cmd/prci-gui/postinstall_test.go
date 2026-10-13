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
	"context"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/system/postinstall"
)

// fakePostinstallProfile makes the services see profile as the detected
// post-install profile, and returns the IDs of the actions that ran.
func fakePostinstallProfile(t *testing.T, profile *postinstall.Profile) *[]string {
	t.Helper()
	ran := &[]string{}
	for i := range profile.Actions {
		id := profile.Actions[i].ID
		if profile.Actions[i].Run == nil {
			profile.Actions[i].Run = func(context.Context, *executor.Executor, io.Writer) error {
				*ran = append(*ran, id)
				return nil
			}
		}
	}
	orig := detectPostinstallProfile
	detectPostinstallProfile = func() *postinstall.Profile { return profile }
	origID := postinstallMachineID
	postinstallMachineID = func() string { return "machine-a" }
	t.Cleanup(func() { detectPostinstallProfile = orig; postinstallMachineID = origID })
	return ran
}

func detected(applied bool) func(*postinstall.Probe) bool {
	return func(*postinstall.Probe) bool { return applied }
}

func runPostinstallAction(t *testing.T, id string) (logs string, done map[string]interface{}) {
	t.Helper()
	b, rec := newTestBase(t)
	if err := (&LinuxService{*b}).RunPostinstallAction(id); err != nil {
		t.Fatalf("refused: %v", err)
	}
	return rec.wait(t)
}

// An action without detection is recorded after a successful run, once,
// and from then on counts as applied.
func TestRunPostinstallAction_RecordsUndetectableAction(t *testing.T) {
	isolateActions(t)
	ran := fakePostinstallProfile(t, &postinstall.Profile{ID: "test", Actions: []postinstall.Action{
		{ID: "update-index", Label: "Atualizar lista"},
		{ID: "sysctl", Label: "Kernel", Applied: detected(false)},
	}})

	if _, done := runPostinstallAction(t, "update-index"); done["ok"] != true {
		t.Fatalf("action failed: %v", done["error"])
	}
	if _, done := runPostinstallAction(t, "sysctl"); done["ok"] != true {
		t.Fatalf("action failed: %v", done["error"])
	}
	if !slices.Equal(*ran, []string{"update-index", "sysctl"}) {
		t.Errorf("ran = %v", *ran)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.PostinstallApplied["test"]; !slices.Equal(got, []string{"update-index"}) {
		t.Errorf("recorded = %v, want only the undetectable action", got)
	}
	status := (&LinuxService{}).GetPostinstallStatus()
	if !status["update-index"] || status["sysctl"] {
		t.Errorf("status = %v", status)
	}
}

// An applied action, an action whose dependency isn't applied and an
// unknown ID are refused without running anything.
func TestRunPostinstallAction_Refusals(t *testing.T) {
	isolateActions(t)
	ran := fakePostinstallProfile(t, &postinstall.Profile{ID: "test", Actions: []postinstall.Action{
		{ID: "repos", Label: "Repositórios", Applied: detected(false)},
		{ID: "drivers", Label: "Drivers", Applied: detected(false), DependsOn: "repos"},
		{ID: "trim", Label: "TRIM", Applied: detected(true)},
	}})
	cases := map[string]string{
		"trim":    "já aplicado",
		"drivers": `aplique antes "Repositórios"`,
		"nope":    "desconhecido",
	}
	for id, want := range cases {
		_, done := runPostinstallAction(t, id)
		if done["ok"] == true || !strings.Contains(done["error"].(string), want) {
			t.Errorf("%s: done = %v, want error containing %q", id, done, want)
		}
	}
	if len(*ran) != 0 {
		t.Errorf("refused actions ran: %v", *ran)
	}
}

// A failed run is reported and not recorded.
func TestRunPostinstallAction_FailureNotRecorded(t *testing.T) {
	isolateActions(t)
	fakePostinstallProfile(t, &postinstall.Profile{ID: "test", Actions: []postinstall.Action{
		{ID: "update-index", Label: "Atualizar lista", Run: func(context.Context, *executor.Executor, io.Writer) error {
			return errors.New("sem rede")
		}},
	}})
	if _, done := runPostinstallAction(t, "update-index"); done["ok"] == true {
		t.Fatal("a failed run must not succeed")
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.PostinstallApplied["test"]) != 0 {
		t.Errorf("a failed run was recorded: %v", cfg.PostinstallApplied)
	}
}

// Records made on another machine ID don't count as applied, and the next
// recorded action replaces them.
func TestPostinstallRecords_TiedToMachineID(t *testing.T) {
	isolateActions(t)
	fakePostinstallProfile(t, &postinstall.Profile{ID: "test", Actions: []postinstall.Action{
		{ID: "update-index", Label: "Atualizar lista"},
		{ID: "upgrade", Label: "Atualizar pacotes"},
	}})
	if _, done := runPostinstallAction(t, "update-index"); done["ok"] != true {
		t.Fatalf("action failed: %v", done["error"])
	}
	if !(&LinuxService{}).GetPostinstallStatus()["update-index"] {
		t.Fatal("a record from this machine must count")
	}

	postinstallMachineID = func() string { return "machine-b" }
	if (&LinuxService{}).GetPostinstallStatus()["update-index"] {
		t.Fatal("a record from another machine ID must not count")
	}
	if _, done := runPostinstallAction(t, "upgrade"); done["ok"] != true {
		t.Fatalf("action failed: %v", done["error"])
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PostinstallMachineID != "machine-b" || !slices.Equal(cfg.PostinstallApplied["test"], []string{"upgrade"}) {
		t.Errorf("old records must be replaced: id=%q applied=%v", cfg.PostinstallMachineID, cfg.PostinstallApplied)
	}
	status := (&LinuxService{}).GetPostinstallStatus()
	if status["update-index"] || !status["upgrade"] {
		t.Errorf("status = %v", status)
	}
}

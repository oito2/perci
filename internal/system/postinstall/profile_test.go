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
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/executor"
)

// allProfiles lists every Profile this package declares — kept in sync by
// hand (no reflection-based discovery), used only by the structural tests
// below. Add new profiles here too.
func allProfiles() []Profile {
	return []Profile{
		mintCinnamonProfile, mintXFCEProfile,
		ubuntu2404Profile, ubuntu2604Profile,
		fedoraProfile,
		zorinCoreProfile, zorinLiteProfile,
		poposProfile,
	}
}

// TestProfiles_ActionIDsAreUnique catches a copy-paste ID typo (two
// actions sharing an ID) within a single Profile — DependsOn resolution
// and the GUI's per-item state both silently misbehave if IDs collide.
func TestProfiles_ActionIDsAreUnique(t *testing.T) {
	for _, p := range allProfiles() {
		t.Run(p.ID, func(t *testing.T) {
			seen := make(map[string]bool, len(p.Actions))
			for _, a := range p.Actions {
				if a.ID == "" {
					t.Errorf("action with empty ID (label %q)", a.Label)
					continue
				}
				if seen[a.ID] {
					t.Errorf("duplicate action ID %q", a.ID)
				}
				seen[a.ID] = true
			}
		})
	}
}

// TestProfiles_DependsOnResolves catches a DependsOn referencing an ID
// that doesn't exist in the same Profile.
func TestProfiles_DependsOnResolves(t *testing.T) {
	for _, p := range allProfiles() {
		t.Run(p.ID, func(t *testing.T) {
			ids := make(map[string]bool, len(p.Actions))
			for _, a := range p.Actions {
				ids[a.ID] = true
			}
			for _, a := range p.Actions {
				if a.DependsOn == "" {
					continue
				}
				if !ids[a.DependsOn] {
					t.Errorf("action %q depends on %q, which does not exist in this profile", a.ID, a.DependsOn)
				}
				if a.DependsOn == a.ID {
					t.Errorf("action %q depends on itself", a.ID)
				}
			}
		})
	}
}

// TestProfiles_ActionsHaveRunFunc catches an Action literal that forgot
// to set Run — it would panic (nil func call) only when that specific
// action got selected and executed, possibly long after this was written.
func TestProfiles_ActionsHaveRunFunc(t *testing.T) {
	for _, p := range allProfiles() {
		t.Run(p.ID, func(t *testing.T) {
			for _, a := range p.Actions {
				if a.Run == nil {
					t.Errorf("action %q has a nil Run func", a.ID)
				}
				if a.Label == "" {
					t.Errorf("action %q has an empty Label", a.ID)
				}
				if a.Description == "" {
					t.Errorf("action %q has an empty Description", a.ID)
				}
			}
		})
	}
}

// TestProfiles_HaveLabelAndID sanity-checks the Profile-level metadata
// the GUI shows directly ("Pós instalação do <Label>").
func TestProfiles_HaveLabelAndID(t *testing.T) {
	seen := make(map[string]bool)
	for _, p := range allProfiles() {
		if p.ID == "" {
			t.Errorf("profile with empty ID (label %q)", p.Label)
			continue
		}
		if p.Label == "" {
			t.Errorf("profile %q has an empty Label", p.ID)
		}
		if len(p.Actions) == 0 {
			t.Errorf("profile %q has no actions", p.ID)
		}
		if seen[p.ID] {
			t.Errorf("duplicate profile ID %q", p.ID)
		}
		seen[p.ID] = true
	}
}

func TestSwapfileScript_Syntax(t *testing.T) {
	if out, err := exec.Command("bash", "-n", "-c", swapfileScript).CombinedOutput(); err != nil {
		t.Fatalf("swapfileScript has a syntax error: %v\n%s", err, out)
	}
}

// Every action of every profile runs under DryRun without error and asks
// for the password at most once.
func TestProfiles_EveryActionAtMostOnePrompt(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // default config: Flatpak system scope
	for _, flatpakPresent := range []bool{true, false} {
		for _, p := range allProfiles() {
			for _, a := range p.Actions {
				var buf bytes.Buffer
				exe := &executor.Executor{DryRun: true, UsePolicyKit: true, Stdout: &buf, Stderr: &buf,
					LookPath: func(name string) (string, error) {
						if name == "flatpak" && !flatpakPresent {
							return "", errors.New("not found")
						}
						return "/usr/bin/" + name, nil
					}}
				if err := a.Run(context.Background(), exe, &buf); err != nil {
					t.Errorf("%s/%s: %v", p.ID, a.ID, err)
				}
				if n := strings.Count(buf.String(), "pkexec"); n > 1 {
					t.Errorf("%s/%s (flatpak present=%v): %d password prompts:\n%s", p.ID, a.ID, flatpakPresent, n, buf.String())
				}
			}
		}
	}
}

// The sysctl action writes 99-perci.conf and removes the old
// 99-lumina.conf in the same privileged script.
func TestConfigureSysctl_MigratesLegacyFile(t *testing.T) {
	var buf bytes.Buffer
	exe := &executor.Executor{DryRun: true, UsePolicyKit: true, Stdout: &buf, Stderr: &buf}
	if err := configureSysctl(context.Background(), exe, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, sysctlConfPath) || !strings.Contains(out, "rm -f -- '"+legacySysctlConfPath+"'") {
		t.Errorf("unexpected script:\n%s", out)
	}
}

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

package ai

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/config"
)

// isolateHome points HOME (and so config.yaml) at a temporary folder.
func isolateHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
}

func TestRecordContext(t *testing.T) {
	isolateHome(t)
	a, b := t.TempDir(), t.TempDir()

	// A complement pulls in its base; unknown standards are dropped; both
	// lists end in catalog order.
	if err := RecordContext("  Perci  ", a, []string{"Go: Wails v3", "Nope"}, []string{AgentCodex, AgentClaude}); err != nil {
		t.Fatal(err)
	}
	got, err := FindContext(a)
	if err != nil {
		t.Fatal(err)
	}
	want := config.AIContext{Name: "Perci", Path: a, Models: []string{"Go", "Go: Wails v3"}, Agents: []string{AgentClaude, AgentCodex}}
	if got.Name != want.Name || !slices.Equal(got.Models, want.Models) || !slices.Equal(got.Agents, want.Agents) {
		t.Errorf("record = %+v, want %+v", got, want)
	}

	for name, tc := range map[string]struct {
		name, path string
		agents     []string
	}{
		"empty name":    {"  ", b, []string{AgentCodex}},
		"name in use":   {"perci", b, []string{AgentCodex}},
		"line break":    {"a\nb", b, []string{AgentCodex}},
		"too long":      {strings.Repeat("x", maxContextNameLen+1), b, []string{AgentCodex}},
		"no agent":      {"B", b, nil},
		"unknown agent": {"B", b, []string{"nope"}},
		"relative path": {"B", "rel/dir", []string{AgentCodex}},
	} {
		if err := RecordContext(tc.name, tc.path, nil, tc.agents); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}

	// Same path: the record is updated in place, its own name may be kept.
	if err := RecordContext("Perci", a+"/", []string{"Linux Bash"}, []string{AgentGemini}); err != nil {
		t.Fatal(err)
	}
	all, err := Contexts()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || !slices.Equal(all[0].Models, []string{"Linux Bash"}) || all[0].Agents[0] != AgentGemini {
		t.Errorf("after update: %+v", all)
	}
	if _, err := ValidateContextName("PERCI", b); err == nil {
		t.Error("ValidateContextName accepted a name in use")
	}
	if n, err := ValidateContextName(" Perci ", a); err != nil || n != "Perci" {
		t.Errorf("ValidateContextName on its own folder = %q, %v", n, err)
	}

	if err := ForgetContext(a); err != nil {
		t.Fatal(err)
	}
	if _, err := FindContext(a); !errors.Is(err, ErrContextNotFound) {
		t.Errorf("FindContext after Forget: %v", err)
	}
	if err := ForgetContext(a); !errors.Is(err, ErrContextNotFound) {
		t.Errorf("second Forget: %v", err)
	}
}

func TestRegisterDetected(t *testing.T) {
	isolateHome(t)
	dir := t.TempDir()
	if HasContext(dir) {
		t.Error("empty folder has a context")
	}
	if err := RegisterDetected("X", dir); err == nil {
		t.Error("registered a folder without context")
	}
	// A hand-written AGENTS.md is not a Perci context.
	write(t, filepath.Join(dir, "AGENTS.md"), "# My rules\n")
	if HasContext(dir) {
		t.Error("hand-written AGENTS.md counted as a Perci context")
	}

	if err := GenerateSharedFiles(dir, []Model{modelByName(t, "Linux Bash")}, []string{AgentClaude, AgentOpenCode}, true, io.Discard); err != nil {
		t.Fatal(err)
	}
	if !HasContext(dir) {
		t.Fatal("generated context not detected")
	}
	if err := RegisterDetected("X", dir); err != nil {
		t.Fatal(err)
	}
	rec, err := FindContext(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(rec.Models, []string{"Linux Bash"}) || !slices.Equal(rec.Agents, []string{AgentClaude, AgentOpenCode}) {
		t.Errorf("registered %+v", rec)
	}
}

func TestContextState(t *testing.T) {
	bash := modelByName(t, "Linux Bash")
	dir := t.TempDir()
	rec := config.AIContext{Name: "X", Path: dir, Models: []string{bash.Name}, Agents: []string{AgentCodex}}
	state := func() ContextStatus {
		t.Helper()
		st, err := ContextState(rec)
		if err != nil {
			t.Fatal(err)
		}
		return st
	}

	if st := state(); st.State != StatusOutdated || !st.Divergent {
		t.Errorf("before Aplicar: %+v", st)
	}
	if err := GenerateSharedFiles(dir, []Model{bash}, rec.Agents, true, io.Discard); err != nil {
		t.Fatal(err)
	}
	if st := state(); st.State != StatusApplied || st.Divergent {
		t.Errorf("after Aplicar: %+v", st)
	}

	// Files generated for other agents than the recorded ones.
	other := rec
	other.Agents = []string{AgentGemini}
	if st, _ := ContextState(other); st.State != StatusOutdated || !st.Divergent {
		t.Errorf("other agents: %+v", st)
	}

	// A missing standard is outdated; an edited one is edited.
	std := filepath.Join(dir, ".instructions", "BASH.md")
	content := readFile(t, std)
	if err := os.Remove(std); err != nil {
		t.Fatal(err)
	}
	if st := state(); st.State != StatusOutdated {
		t.Errorf("missing standard: %+v", st)
	}
	write(t, std, content+"\nmine\n")
	if st := state(); st.State != StatusEdited {
		t.Errorf("edited standard: %+v", st)
	}
	write(t, std, content)

	// A rule written by hand into AGENTS.md is an edit.
	agentsMD := filepath.Join(dir, "AGENTS.md")
	write(t, agentsMD, readFile(t, agentsMD)+"\n- Always use tabs.\n")
	if st := state(); st.State != StatusEdited {
		t.Errorf("hand rule in AGENTS.md: %+v", st)
	}

	rec.Path = filepath.Join(dir, "missing")
	if st := state(); st.State != StatusPending {
		t.Errorf("missing folder: %+v", st)
	}
}

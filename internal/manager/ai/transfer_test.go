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
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/executor"
)

func TestExportable_RelativeInsideTheWorkspace(t *testing.T) {
	ws := "/home/u/workspace"
	f := Exportable([]config.AIContext{
		{Name: "In", Path: "/home/u/workspace/org/app", Models: []string{"Go"}, Agents: []string{AgentClaude}},
		{Name: "Out", Path: "/srv/other", Agents: []string{AgentCodex}},
		{Name: "Root", Path: "/home/u/workspace", Agents: []string{AgentCodex}},
		{Name: "Sibling", Path: "/home/u/workspace2/x", Agents: []string{AgentCodex}},
	}, ws)
	want := []TransferContext{
		{Name: "In", Path: "org/app", Relative: true, Models: []string{"Go"}, Agents: []string{AgentClaude}},
		{Name: "Out", Path: "/srv/other", Agents: []string{AgentCodex}},
		{Name: "Root", Path: "/home/u/workspace", Agents: []string{AgentCodex}},
		{Name: "Sibling", Path: "/home/u/workspace2/x", Agents: []string{AgentCodex}},
	}
	if f.Kind != TransferKind || f.Version != TransferVersion || !reflect.DeepEqual(f.Contexts, want) {
		t.Errorf("Exportable = %+v", f)
	}
}

func TestTransferFile_WriteLoadAndValidate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ctx.yaml")
	if err := WriteTransfer(path, TransferFile{Kind: TransferKind, Version: 1}); !errors.Is(err, ErrNothingToExport) {
		t.Errorf("empty export: %v", err)
	}
	f := TransferFile{Kind: TransferKind, Version: 1, Contexts: []TransferContext{
		{Name: "A", Path: "org/a", Relative: true, Models: []string{"Go"}, Agents: []string{AgentClaude}},
		{Name: "B", Path: "/srv/b", Agents: []string{AgentCodex}},
	}}
	if err := WriteTransfer(path, f); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("file mode: %v, %v", info, err)
	}
	got, err := LoadTransfer(path)
	if err != nil || !reflect.DeepEqual(got, f) {
		t.Fatalf("LoadTransfer = %+v, %v", got, err)
	}

	head := "kind: perci-ai-contexts\nversion: 1\ncontexts:\n"
	for name, content := range map[string]string{
		"other kind":         "kind: x\nversion: 1\ncontexts: []\n",
		"no kind":            "version: 1\ncontexts: []\n",
		"unknown key":        head + "  - {name: A, path: /a, agents: [codex], extra: 1}\n",
		"docker export":      "workspace_path: /x\ndocker: {}\n",
		"no version":         "kind: perci-ai-contexts\ncontexts: []\n",
		"newer version":      "kind: perci-ai-contexts\nversion: 2\ncontexts: []\n",
		"empty name":         head + "  - {name: '', path: /a, agents: [codex]}\n",
		"padded name":        head + "  - {name: ' A', path: /a, agents: [codex]}\n",
		"repeated name":      head + "  - {name: A, path: /a, agents: [codex]}\n  - {name: a, path: /b, agents: [codex]}\n",
		"repeated path":      head + "  - {name: A, path: /a, agents: [codex]}\n  - {name: B, path: /a, agents: [codex]}\n",
		"relative not local": head + "  - {name: A, path: ../a, relative: true, agents: [codex]}\n",
		"relative absolute":  head + "  - {name: A, path: /a, relative: true, agents: [codex]}\n",
		"absolute relative":  head + "  - {name: A, path: a, agents: [codex]}\n",
		"unclean path":       head + "  - {name: A, path: /a/../b, agents: [codex]}\n",
	} {
		p := filepath.Join(dir, strings.ReplaceAll(name, " ", "-")+".yaml")
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadTransfer(p); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	// The same folder once relative and once absolute is not a repetition.
	p := filepath.Join(dir, "mixed.yaml")
	if err := os.WriteFile(p, []byte(head+"  - {name: A, path: a, relative: true, agents: [codex]}\n  - {name: B, path: /a, agents: [codex]}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadTransfer(p); err != nil {
		t.Errorf("relative and absolute: %v", err)
	}
}

// importFixture is a workspace with an existing folder, a file where a
// folder is expected, and a registered folder.
type importFixture struct {
	ws, exists, registered string
	file                   TransferFile
	contexts               []config.AIContext
}

func newImportFixture(t *testing.T) importFixture {
	t.Helper()
	ws := t.TempDir()
	fx := importFixture{ws: ws, exists: filepath.Join(ws, "exists"), registered: filepath.Join(ws, "reg")}
	for _, d := range []string{fx.exists, fx.registered} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write(t, filepath.Join(ws, "afile"), "x")
	fx.contexts = []config.AIContext{{Name: "Taken", Path: fx.registered, Agents: []string{AgentCodex}}}
	fx.file = TransferFile{Kind: TransferKind, Version: 1, Contexts: []TransferContext{
		{Name: "Exists", Path: "exists", Relative: true, Models: []string{"Linux Bash", "Future Std"}, Agents: []string{AgentCodex, "future"}},
		{Name: "Missing", Path: "missing", Relative: true, Agents: []string{AgentClaude}},
		{Name: "Reg", Path: fx.registered, Agents: []string{AgentCodex}},
		{Name: "NoAgent", Path: "x", Relative: true, Agents: []string{"future"}},
		{Name: "File", Path: "afile", Relative: true, Agents: []string{AgentCodex}},
	}}
	return fx
}

func TestPlanImport(t *testing.T) {
	fx := newImportFixture(t)
	got := PlanImport(fx.file, fx.contexts, fx.ws, nil)
	states := make([]string, len(got))
	for i, ic := range got {
		states[i] = ic.State
	}
	if want := []string{ImportApply, ImportPending, ImportRegistered, ImportInvalid, ImportInvalid}; !slices.Equal(states, want) {
		t.Errorf("states = %v, want %v", states, want)
	}
	if got[0].Path != fx.exists || !slices.Equal(got[0].Models, []string{"Linux Bash"}) || !slices.Equal(got[0].Agents, []string{AgentCodex}) ||
		!slices.Equal(got[0].Unknown, []string{"Future Std", "future"}) {
		t.Errorf("Exists = %+v", got[0])
	}
	if got[3].Reason == "" || got[4].Reason == "" {
		t.Errorf("invalid without reason: %+v", got[3:])
	}

	// A replacement folder takes the place of the file's.
	got = PlanImport(fx.file, fx.contexts, fx.ws, map[string]string{"Missing": fx.exists + "/"})
	if got[1].State != ImportApply || got[1].Path != fx.exists {
		t.Errorf("replaced folder = %+v", got[1])
	}
}

func TestCheckImport_Refusals(t *testing.T) {
	fx := newImportFixture(t)
	pick := func(src, name string) ImportPick { return ImportPick{Source: src, Name: name} }
	for name, picks := range map[string][]ImportPick{
		"nothing":             nil,
		"not in the file":     {pick("Nope", "N")},
		"picked twice":        {pick("Exists", "A"), pick("Exists", "B")},
		"picked twice, moved": {pick("Exists", "A"), {Source: "Exists", Name: "B", Path: filepath.Join(fx.ws, "elsewhere")}},
		"name taken here":     {pick("Exists", "taken")},
		"name of other pick":  {pick("Exists", "Same"), pick("Missing", "same")},
		"empty name":          {pick("Exists", " ")},
		"registered folder":   {pick("Reg", "R")},
		"no known agent":      {pick("NoAgent", "N")},
		"not a folder":        {pick("File", "F")},
		"relative override":   {{Source: "Exists", Name: "E", Path: "rel"}},
		"same folder twice":   {pick("Exists", "A"), {Source: "Missing", Name: "B", Path: fx.exists}},
	} {
		if err := CheckImport(fx.file, picks, fx.contexts, fx.ws); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if err := CheckImport(fx.file, []ImportPick{pick("Exists", " Novo "), pick("Missing", "Exists")}, fx.contexts, fx.ws); err != nil {
		t.Errorf("valid picks: %v", err)
	}
}

func TestPreviewImportAndImport(t *testing.T) {
	isolateHome(t)
	fx := newImportFixture(t)
	if err := RecordContext("Taken", fx.registered, nil, []string{AgentCodex}); err != nil {
		t.Fatal(err)
	}
	picks := []ImportPick{{Source: "Exists", Name: "Novo"}, {Source: "Missing", Name: "Pendente"}}

	plans, changes, err := PreviewImport(fx.file, picks, fx.contexts, fx.ws)
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 2 || plans[0].Name != "Novo" || len(changes[0]) == 0 || changes[1] == nil || len(changes[1]) != 0 {
		t.Fatalf("preview = %+v %+v", plans, changes)
	}
	if exists(t, filepath.Join(fx.exists, "AGENTS.md")) {
		t.Error("the preview wrote AGENTS.md")
	}

	var out bytes.Buffer
	if err := Import(&out, fx.file, picks, fx.ws); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Future Std, future") {
		t.Errorf("no warning about unknown items: %q", out.String())
	}
	if !exists(t, filepath.Join(fx.exists, ".instructions", "BASH.md")) {
		t.Error("existing folder did not get its context")
	}
	all, err := Contexts()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 || all[1].Name != "Novo" || all[1].Path != fx.exists || !slices.Equal(all[1].Agents, []string{AgentCodex}) ||
		all[2].Name != "Pendente" || all[2].Path != filepath.Join(fx.ws, "missing") {
		t.Errorf("registered = %+v", all)
	}
	if st, _ := ContextState(all[2]); st.State != StatusPending {
		t.Errorf("missing folder state = %+v", st)
	}

	// Picks already imported are refused before anything is written.
	if err := Import(&out, fx.file, picks, fx.ws); err == nil {
		t.Error("second import accepted")
	}
}

func TestImport_FailingFolderDoesNotStopTheOthers(t *testing.T) {
	isolateHome(t)
	fx := newImportFixture(t)
	// .instructions as a file makes the generation fail.
	write(t, filepath.Join(fx.exists, ".instructions"), "x")
	picks := []ImportPick{{Source: "Exists", Name: "E"}, {Source: "Missing", Name: "M"}}
	err := Import(&bytes.Buffer{}, fx.file, picks, fx.ws)
	if !errors.Is(err, executor.ErrCompletedWithWarnings) {
		t.Fatalf("Import = %v", err)
	}
	all, _ := Contexts()
	if len(all) != 1 || all[0].Name != "M" {
		t.Errorf("registered = %+v", all)
	}
}

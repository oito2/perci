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

package templates_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/oito2/perci/internal/system/templates"
	"github.com/oito2/perci/internal/ui"
)

func TestDirNeverReturnsBareHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir, err := templates.Dir()
	if err != nil {
		t.Fatalf("Dir: %v", err)
	}
	if dir == home {
		t.Errorf("Dir() returned bare $HOME (%s); expected a Modelos/Templates subdirectory", home)
	}
}

func TestDirPrefersModelos(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	modelos := filepath.Join(home, "Modelos")
	if err := os.Mkdir(modelos, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	dir, err := templates.Dir()
	if err != nil {
		t.Fatalf("Dir: %v", err)
	}
	if dir != modelos {
		t.Errorf("Dir() = %q, want %q", dir, modelos)
	}
}

func TestPresentNames(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Texto.txt"), nil, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	present := templates.PresentNames(dir)
	if !present["Texto.txt"] {
		t.Error("expected Texto.txt to be present")
	}
	if present["Documento.docx"] {
		t.Error("expected Documento.docx to be absent")
	}
}

func TestApplyCreatesMissingDir(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "Modelos")

	var buf bytes.Buffer
	if err := templates.Apply(&buf, dir, []string{"Texto.txt"}, nil); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "Texto.txt")); err != nil {
		t.Errorf("expected Texto.txt to be created: %v", err)
	}
}

func TestApply_ReportsOneStepPerTemplate(t *testing.T) {
	dir := t.TempDir()
	// One to create (Texto.txt) and one to remove (already exists).
	if err := os.WriteFile(filepath.Join(dir, "PHP.php"), nil, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var buf bytes.Buffer
	var labels []string
	ui.SetStepHook(func(index, total int, label string) {
		if total != 2 {
			t.Errorf("Step total = %d, want 2", total)
		}
		if index != len(labels)+1 {
			t.Errorf("Step index = %d, want %d (sequential, 1-based)", index, len(labels)+1)
		}
		labels = append(labels, label)
	})
	t.Cleanup(func() { ui.SetStepHook(nil) })

	if err := templates.Apply(&buf, dir, []string{"Texto.txt"}, []string{"PHP.php"}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Apply processes creations in Catalogue's declared order, then
	// removals in the order passed via toRemove.
	want := []string{"Criando Texto.txt...", "Removendo PHP.php..."}
	if len(labels) != len(want) {
		t.Fatalf("got %d Step calls %v, want %d %v", len(labels), labels, len(want), want)
	}
	for i := range want {
		if labels[i] != want[i] {
			t.Errorf("step[%d] = %q, want %q", i, labels[i], want[i])
		}
	}
}

func TestApply_NoSelectionIsNoOp(t *testing.T) {
	dir := t.TempDir()
	var buf bytes.Buffer

	called := false
	ui.SetStepHook(func(int, int, string) { called = true })
	t.Cleanup(func() { ui.SetStepHook(nil) })

	if err := templates.Apply(&buf, dir, nil, nil); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if called {
		t.Error("expected no Step calls when nothing is selected")
	}
}

// A remove entry that isn't a plain file name inside dir is skipped, and
// an existing file is never overwritten by create.
func TestApply_RejectsTraversalAndKeepsExisting(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "Modelos")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(root, "keep.txt")
	existing := filepath.Join(dir, "Texto.txt")
	for _, p := range []string{outside, existing} {
		if err := os.WriteFile(p, []byte("mine"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := templates.Apply(io.Discard, dir, []string{"Texto.txt"}, []string{"../keep.txt", ".."}); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{outside, existing} {
		if b, err := os.ReadFile(p); err != nil || string(b) != "mine" {
			t.Errorf("%s changed: %q, %v", p, b, err)
		}
	}
}

// The same template produces byte-identical archives.
func TestCreateZip_Deterministic(t *testing.T) {
	da, db := t.TempDir(), t.TempDir()
	for _, d := range []string{da, db} {
		if err := templates.Apply(io.Discard, d, []string{"Documento.docx"}, nil); err != nil {
			t.Fatal(err)
		}
	}
	ba, _ := os.ReadFile(filepath.Join(da, "Documento.docx"))
	bb, _ := os.ReadFile(filepath.Join(db, "Documento.docx"))
	if len(ba) == 0 {
		t.Fatal("docx not created")
	}
	if !bytes.Equal(ba, bb) {
		t.Error("two docx builds differ")
	}
}

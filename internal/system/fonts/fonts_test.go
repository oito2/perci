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

package fonts_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/system/fonts"
	"github.com/oito2/perci/internal/ui"
)

func TestApply_ReportsOneStepPerFont(t *testing.T) {
	// One apt-based font (Carlito) and one downloaded by URL (JetBrains
	// Mono) — covers both install()/remove() paths through the same Step
	// mechanism.
	var buf bytes.Buffer
	exe := &executor.Executor{DryRun: true, Stdout: &buf, Stderr: &buf}

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

	if err := fonts.Apply(context.Background(), exe, &buf, []string{"Carlito"}, []string{"JetBrains Mono"}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Apply processes in Catalogue's declared order, not the order passed
	// to toInstall/toRemove — JetBrains Mono comes before Carlito there.
	want := []string{"Removendo JetBrains Mono...", "Instalando Carlito..."}
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
	var buf bytes.Buffer
	exe := &executor.Executor{DryRun: true, Stdout: &buf, Stderr: &buf}

	called := false
	ui.SetStepHook(func(int, int, string) { called = true })
	t.Cleanup(func() { ui.SetStepHook(nil) })

	if err := fonts.Apply(context.Background(), exe, &buf, nil, nil); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if called {
		t.Error("expected no Step calls when nothing is selected")
	}
}

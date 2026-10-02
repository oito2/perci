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

package ui

import (
	"bytes"
	"strings"
	"testing"
)

func TestStep_WritesIndexedLabel(t *testing.T) {
	var buf bytes.Buffer
	Step(&buf, 2, 5, "Atualizando pacotes...")

	out := buf.String()
	if !strings.Contains(out, "2/5") || !strings.Contains(out, "Atualizando pacotes...") {
		t.Errorf("expected index and label in output, got: %q", out)
	}
}

func TestStep_CallsRegisteredHook(t *testing.T) {
	t.Cleanup(func() { SetStepHook(nil) })

	type call struct {
		index, total int
		label        string
	}
	var got call
	SetStepHook(func(index, total int, label string) {
		got = call{index, total, label}
	})

	var buf bytes.Buffer
	Step(&buf, 3, 4, "Atualizando Flatpaks...")

	want := call{3, 4, "Atualizando Flatpaks..."}
	if got != want {
		t.Errorf("hook received %+v, want %+v", got, want)
	}
}

func TestStep_NoHookRegisteredIsSafe(t *testing.T) {
	SetStepHook(nil)
	var buf bytes.Buffer
	Step(&buf, 1, 1, "Passo único") // must not panic with a nil hook
}

// None of the message functions draw a border box — always colored
// text (truecolor ANSI escape) with no box-drawing character at all.
// Corners (╭╮╰╯) are the unambiguous marker of a border; "│" is
// deliberately excluded from that set because PrintHeader uses it on
// purpose as the "  │  " separator between the two sides of the header.

func TestMessages_NeverDrawABorderBox(t *testing.T) {
	var buf bytes.Buffer
	Info(&buf, "info")
	Err(&buf, "err")
	Warning(&buf, "warning")
	Success(&buf, "success")
	Step(&buf, 1, 1, "step")
	PrintHeader(&buf, "Atualizar Sistema", "Ubuntu")

	if out := buf.String(); strings.ContainsAny(out, "╭╮╰╯") {
		t.Errorf("expected no border box characters from any of these, got: %q", out)
	}
}

func TestInfo_ContainsTheMessageText(t *testing.T) {
	var buf bytes.Buffer
	Info(&buf, "mensagem de teste")
	if !strings.Contains(buf.String(), "mensagem de teste") {
		t.Errorf("expected the message text to be present, got: %q", buf.String())
	}
}

// The header carries the caller's distro label and never clears the
// terminal.
func TestPrintHeader(t *testing.T) {
	var buf bytes.Buffer
	PrintHeader(&buf, "Atualizar Sistema", "Fedora")
	out := buf.String()
	if !strings.Contains(out, "Fedora") || !strings.Contains(out, "Atualizar Sistema") {
		t.Errorf("header missing distro or title: %q", out)
	}
	if strings.Contains(out, "\033[2J") {
		t.Errorf("header must not clear the terminal: %q", out)
	}

	buf.Reset()
	PrintHeader(&buf, "t", "")
	if strings.Count(buf.String(), "│") != 2 {
		t.Errorf("an empty label must be omitted: %q", buf.String())
	}
}

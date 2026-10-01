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

package executor

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestOutput_TruncatesPastMaxOutputBytes(t *testing.T) {
	orig := maxOutputBytes
	maxOutputBytes = 10 // small cap so the test doesn't need to generate real megabytes
	defer func() { maxOutputBytes = orig }()

	e := New(nil, nil)
	// printf with no trailing newline gives an exact, predictable byte count.
	got, err := e.Output(context.Background(), Options{}, "printf", strings.Repeat("x", 100))
	if err != nil {
		t.Fatalf("Output: %v", err)
	}
	if len(got) != maxOutputBytes {
		t.Errorf("Output() returned %d bytes, want exactly %d (capped)", len(got), maxOutputBytes)
	}
	if got != strings.Repeat("x", maxOutputBytes) {
		t.Errorf("Output() = %q, want the first %d 'x' characters", got, maxOutputBytes)
	}
}

func TestLimitedBuffer_WriteNeverShortWrites(t *testing.T) {
	w := &limitedBuffer{limit: 5}
	n, err := w.Write([]byte("hello world"))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != len("hello world") {
		t.Errorf("Write() returned n=%d, want %d (must report the full length even when truncating internally)", n, len("hello world"))
	}
	if w.buf.String() != "hello" {
		t.Errorf("buffered content = %q, want %q", w.buf.String(), "hello")
	}
}

// The batch script's exit status under a real shell: a failing SoftReport
// step doesn't stop the batch but ends it with softReportExitCode; plain
// Soft keeps exiting 0; a non-Soft failure keeps its own status.
func TestBuildSudoSequenceScript_SoftReportExitStatus(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "ran-after")
	touch := PrivilegedStep{Name: "touch", Args: []string{marker}}
	cases := []struct {
		name  string
		steps []PrivilegedStep
		want  int
		after bool // the step after the failure still ran
	}{
		{"soft report failure", []PrivilegedStep{{Name: "false", Soft: true, SoftReport: true, WarnMessage: "w"}, touch}, softReportExitCode, true},
		{"plain soft failure", []PrivilegedStep{{Name: "false", Soft: true, WarnMessage: "w"}, touch}, 0, true},
		{"soft report success", []PrivilegedStep{{Name: "true", Soft: true, SoftReport: true}, touch}, 0, true},
		{"hard failure", []PrivilegedStep{{Name: "sh", Args: []string{"-c", "exit 3"}}, touch}, 3, false},
	}
	for _, tc := range cases {
		_ = os.Remove(marker)
		err := exec.Command("sh", "-c", buildSudoSequenceScript(tc.steps)).Run()
		got := 0
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			got = exitErr.ExitCode()
		} else if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if got != tc.want {
			t.Errorf("%s: exit %d, want %d", tc.name, got, tc.want)
		}
		if _, statErr := os.Stat(marker); (statErr == nil) != tc.after {
			t.Errorf("%s: step after the failure ran = %v, want %v", tc.name, statErr == nil, tc.after)
		}
	}
}

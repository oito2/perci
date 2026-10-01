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

package executor_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/executor"
)

const zeroSHA = "0000000000000000000000000000000000000000000000000000000000000000"

func TestPrivilegedInstallSteps_RejectsInvalidChecksum(t *testing.T) {
	for _, sum := range []string{"", "abc", zeroSHA + "0", strings.Repeat("g", 64)} {
		if _, err := executor.PrivilegedInstallSteps("/tmp/src", "/usr/local/bin/x", sum, "0755"); err == nil {
			t.Errorf("checksum %q should be rejected", sum)
		}
	}
}

// Root must own the staged copy from the first step (never the user's src
// renamed in place), keep it 0600 until verified, and rename it last.
func TestPrivilegedInstallSteps_Shape(t *testing.T) {
	steps, err := executor.PrivilegedInstallSteps("/tmp/src", "/usr/local/bin/x", zeroSHA, "0755")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, s := range steps {
		names = append(names, s.Name)
	}
	if got := strings.Join(names, ","); got != "install,sh,chmod,mv" {
		t.Fatalf("steps = %s", got)
	}
	install := strings.Join(steps[0].Args, " ")
	for _, want := range []string{"-m 0600", "-o root", "-g root", "/tmp/src /usr/local/bin/x.perci-new"} {
		if !strings.Contains(install, want) {
			t.Errorf("install step %q missing %q", install, want)
		}
	}
	if last := steps[3].Args; last[len(last)-1] != "/usr/local/bin/x" {
		t.Errorf("last step must rename onto dest, got %v", last)
	}
}

func TestPrivilegedInstall_DryRunEscalatesOnce(t *testing.T) {
	var buf bytes.Buffer
	exe := executor.New(&buf, &buf)
	exe.DryRun = true
	exe.UsePolicyKit = true
	if err := exe.PrivilegedInstall(context.Background(), executor.Options{}, "/tmp/src", "/usr/local/bin/x", zeroSHA, "0755"); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(buf.String(), "/usr/bin/pkexec"); n != 1 {
		t.Errorf("want 1 escalation, got %d: %s", n, buf.String())
	}
}

// The verification step runs for real here (it needs no root): a matching
// file survives, a mismatching one is deleted and the step fails.
func TestPrivilegedInstallSteps_VerifyStepRealShell(t *testing.T) {
	if _, err := exec.LookPath("sha256sum"); err != nil {
		t.Skip("sha256sum not available")
	}
	dir := t.TempDir()
	staged := filepath.Join(dir, "x.perci-new")
	content := []byte("perci binary")
	sum := sha256.Sum256(content)
	good := hex.EncodeToString(sum[:])

	run := func(sha string) error {
		if err := os.WriteFile(staged, content, 0o600); err != nil {
			t.Fatal(err)
		}
		steps, err := executor.PrivilegedInstallSteps("/unused", filepath.Join(dir, "x"), sha, "0755")
		if err != nil {
			t.Fatal(err)
		}
		verify := steps[1]
		return exec.Command(verify.Name, verify.Args...).Run()
	}

	if err := run(good); err != nil {
		t.Fatalf("matching checksum should pass: %v", err)
	}
	if _, err := os.Stat(staged); err != nil {
		t.Fatalf("staged file should survive a match: %v", err)
	}

	if err := run(zeroSHA); err == nil {
		t.Fatal("mismatching checksum should fail")
	}
	if _, err := os.Stat(staged); !os.IsNotExist(err) {
		t.Errorf("staged file should be deleted on mismatch, stat err = %v", err)
	}
}

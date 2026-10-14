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

package appstack

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/executor"
)

// userMegaIgnore is a .megaignore as MEGAsync creates it (no final newline).
const userMegaIgnore = "-:Thumbs.db\n-:.*\n-dG:vendor"

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestMegaIgnoreFor_NearestAbove(t *testing.T) {
	root := t.TempDir()
	dest := filepath.Join(root, "sync", "html", "mdle", "local", "x")
	if got := megaIgnoreFor(dest); got != "" {
		t.Errorf("no .megaignore: got %q", got)
	}
	outer := filepath.Join(root, "sync", megaIgnoreName)
	writeFile(t, outer, userMegaIgnore, 0o644)
	if got := megaIgnoreFor(dest); got != outer {
		t.Errorf("got %q, want %q", got, outer)
	}
	inner := filepath.Join(root, "sync", "html", megaIgnoreName)
	writeFile(t, inner, userMegaIgnore, 0o644)
	if got := megaIgnoreFor(dest); got != inner {
		t.Errorf("got %q, want the nearest %q", got, inner)
	}
}

func TestMegaIgnoreRule(t *testing.T) {
	tests := []struct {
		name, file, dest, want string
		wantErr                bool
	}{
		{"relative folder path", "/w/html/.megaignore", "/w/html/mdle/public/local/x", "-dpG:mdle/public/local/x", false},
		{"outside the sync", "/w/html/.megaignore", "/w/other/x", "", true},
		{"glob characters", "/w/html/.megaignore", "/w/html/mdle[1]/local/x", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := megaIgnoreRule(tt.file, tt.dest)
			if (err != nil) != tt.wantErr || got != tt.want {
				t.Errorf("got %q, %v; want %q, error %v", got, err, tt.want, tt.wantErr)
			}
		})
	}
}

func TestAddAndRemoveMegaIgnoreRule(t *testing.T) {
	file := filepath.Join(t.TempDir(), megaIgnoreName)
	writeFile(t, file, userMegaIgnore, 0o640)
	const rule = "-dpG:mdle/local/x"

	added, err := addMegaIgnoreRule(file, rule)
	if err != nil || !added {
		t.Fatalf("add: added=%v err=%v", added, err)
	}
	if got, want := readFile(t, file), userMegaIgnore+"\n"+rule+"\n"; got != want {
		t.Errorf("after add:\n%q\nwant\n%q", got, want)
	}
	if info, err := os.Stat(file); err != nil || info.Mode().Perm() != 0o640 {
		t.Errorf("mode not kept: %v %v", info.Mode(), err)
	}
	if added, err := addMegaIgnoreRule(file, rule); err != nil || added {
		t.Errorf("second add: added=%v err=%v, want no change", added, err)
	}

	if err := removeMegaIgnoreRule(file, rule); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, file); strings.Contains(got, rule) || !strings.HasPrefix(got, userMegaIgnore) {
		t.Errorf("after remove: %q", got)
	}
	if err := removeMegaIgnoreRule(filepath.Join(t.TempDir(), megaIgnoreName), rule); err != nil {
		t.Errorf("missing file: %v", err)
	}
	other := filepath.Join(t.TempDir(), "notes.txt")
	writeFile(t, other, rule+"\n", 0o644)
	if err := removeMegaIgnoreRule(other, rule); err == nil || readFile(t, other) != rule+"\n" {
		t.Errorf("a file not named .megaignore was edited (err %v)", err)
	}
}

func TestAddAndRemoveMoodlePlugin_ExcludesFromMega(t *testing.T) {
	workspace, dev := pluginStack(t, map[string]string{"local": "local"})
	syncRoot := filepath.Dir(AppHTMLDir(workspace, "mdle"))
	file := filepath.Join(syncRoot, megaIgnoreName)
	writeFile(t, file, userMegaIgnore, 0o644)
	const rule = "-dpG:mdle/local/example"
	var buf bytes.Buffer
	exe := &executor.Executor{DryRun: true, UsePolicyKit: true, Stdout: &buf, Stderr: &buf}

	if err := AddMoodlePlugin(context.Background(), exe, &buf, dev, "mdle", "local", "example"); err != nil {
		t.Fatalf("AddMoodlePlugin: %v", err)
	}
	if !strings.Contains(readFile(t, file), "\n"+rule+"\n") {
		t.Errorf(".megaignore lacks %s:\n%s", rule, readFile(t, file))
	}
	recs := loadCfg(t).MoodlePlugins
	if len(recs) != 1 || recs[0].MegaIgnore != file || recs[0].MegaIgnoreRule != rule {
		t.Fatalf("records = %+v", recs)
	}

	if err := RemoveMoodlePlugin(context.Background(), exe, &buf, "mdle", "local/example"); err != nil {
		t.Fatalf("RemoveMoodlePlugin: %v", err)
	}
	if got := readFile(t, file); strings.Contains(got, rule) || !strings.HasPrefix(got, userMegaIgnore) {
		t.Errorf("rule kept after removal: %q", got)
	}
}

func TestAddMoodlePlugin_KeepsTheUsersOwnRule(t *testing.T) {
	workspace, dev := pluginStack(t, map[string]string{"local": "local"})
	file := filepath.Join(filepath.Dir(AppHTMLDir(workspace, "mdle")), megaIgnoreName)
	const rule = "-dpG:mdle/local/example"
	writeFile(t, file, userMegaIgnore+"\n"+rule, 0o644)
	var buf bytes.Buffer
	exe := &executor.Executor{DryRun: true, UsePolicyKit: true, Stdout: &buf, Stderr: &buf}

	if err := AddMoodlePlugin(context.Background(), exe, &buf, dev, "mdle", "local", "example"); err != nil {
		t.Fatalf("AddMoodlePlugin: %v", err)
	}
	if recs := loadCfg(t).MoodlePlugins; len(recs) != 1 || recs[0].MegaIgnoreRule != "" {
		t.Fatalf("a rule perci did not add was recorded: %+v", recs)
	}
	if err := RemoveMoodlePlugin(context.Background(), exe, &buf, "mdle", "local/example"); err != nil {
		t.Fatalf("RemoveMoodlePlugin: %v", err)
	}
	if got := readFile(t, file); got != userMegaIgnore+"\n"+rule {
		t.Errorf("the user's .megaignore changed: %q", got)
	}
}

func TestAddMoodlePlugin_RefusedWhenMegaIgnoreIsNotWritable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores folder permissions")
	}
	workspace, dev := pluginStack(t, map[string]string{"local": "local"})
	syncRoot := filepath.Dir(AppHTMLDir(workspace, "mdle"))
	writeFile(t, filepath.Join(syncRoot, megaIgnoreName), userMegaIgnore, 0o644)
	if err := os.Chmod(syncRoot, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(syncRoot, 0o755) })
	var buf bytes.Buffer
	exe := &executor.Executor{DryRun: true, UsePolicyKit: true, Stdout: &buf, Stderr: &buf}

	err := AddMoodlePlugin(context.Background(), exe, &buf, dev, "mdle", "local", "example")
	if err == nil || !strings.Contains(err.Error(), "MEGA") {
		t.Fatalf("got %v, want a MEGA refusal", err)
	}
	if strings.Contains(buf.String(), "pkexec") {
		t.Errorf("mounted despite the refusal:\n%s", buf.String())
	}
	if _, err := os.Stat(filepath.Join(AppHTMLDir(workspace, "mdle"), "local", "example")); err == nil {
		t.Error("destination folder created despite the refusal")
	}
	if recs := loadCfg(t).MoodlePlugins; len(recs) != 0 {
		t.Errorf("record kept: %+v", recs)
	}
}

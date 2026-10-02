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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oito2/perci/internal/executor"
)

func TestValidMoodleVersion(t *testing.T) {
	for _, v := range MoodleVersions() {
		if !ValidMoodleVersion(v) {
			t.Errorf("ValidMoodleVersion(%q) = false, want true (MoodleVersions() entry)", v)
		}
	}
	if !ValidMoodleVersion(MoodleVersion4x) {
		t.Error("ValidMoodleVersion(MoodleVersion4x) = false, want true (containers saved with it)")
	}
	for _, v := range MoodleVersions() {
		if v == MoodleVersion4x {
			t.Error("MoodleVersions() offers MoodleVersion4x, want only the split 4.x buckets")
		}
	}
	if ValidMoodleVersion("6.0") {
		t.Error(`ValidMoodleVersion("6.0") = true, want false`)
	}
	if ValidMoodleVersion("") {
		t.Error(`ValidMoodleVersion("") = true, want false`)
	}
}

func TestPHPVersionsForMoodleVersion(t *testing.T) {
	tests := []struct {
		moodleVersion string
		want          []string
	}{
		{MoodleVersion3x, []string{"7.4"}},
		{MoodleVersion41, []string{"7.4", "8.0", "8.1"}},
		{MoodleVersion42to43, []string{"8.0", "8.1", "8.2"}},
		{MoodleVersion44to45, []string{"8.1", "8.2", "8.3"}},
		{MoodleVersion4x, []string{"7.4", "8.0", "8.1"}},
		{MoodleVersion50, []string{"8.2", "8.3", "8.4"}},
		{MoodleVersion51Plus, []string{"8.3", "8.4"}},
	}
	for _, tt := range tests {
		got := PHPVersionsForMoodleVersion(tt.moodleVersion)
		if len(got) != len(tt.want) {
			t.Fatalf("PHPVersionsForMoodleVersion(%q) = %v, want %v", tt.moodleVersion, got, tt.want)
		}
		for i, v := range tt.want {
			if got[i] != v {
				t.Errorf("PHPVersionsForMoodleVersion(%q) = %v, want %v", tt.moodleVersion, got, tt.want)
			}
		}
	}
	if got := PHPVersionsForMoodleVersion("bogus"); got != nil {
		t.Errorf(`PHPVersionsForMoodleVersion("bogus") = %v, want nil`, got)
	}
}

func TestPHPConfDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	got, err := PHPConfDir("meuapp")
	if err != nil {
		t.Fatalf("PHPConfDir: %v", err)
	}
	want := filepath.Join(tmp, ".perci", "appstack", "php-conf", "meuapp")
	if got != want {
		t.Errorf("PHPConfDir(%q) = %q, want %q", "meuapp", got, want)
	}
}

func TestWritePHPMemoryLimitConf(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	t.Run("overridden value", func(t *testing.T) {
		path, err := WritePHPMemoryLimitConf("meuapp", "768M")
		if err != nil {
			t.Fatalf("WritePHPMemoryLimitConf: %v", err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read written conf: %v", err)
		}
		if got, want := string(data), "memory_limit = 768M\n"; got != want {
			t.Errorf("conf content = %q, want %q", got, want)
		}
	})

	t.Run("empty falls back to DefaultPHPMemoryLimit", func(t *testing.T) {
		path, err := WritePHPMemoryLimitConf("outroapp", "")
		if err != nil {
			t.Fatalf("WritePHPMemoryLimitConf: %v", err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read written conf: %v", err)
		}
		if !strings.Contains(string(data), "memory_limit = "+DefaultPHPMemoryLimit) {
			t.Errorf("conf content = %q, want it to fall back to DefaultPHPMemoryLimit %q", data, DefaultPHPMemoryLimit)
		}
	})

	t.Run("overwrites an existing file", func(t *testing.T) {
		if _, err := WritePHPMemoryLimitConf("terceiroapp", "512M"); err != nil {
			t.Fatalf("first write: %v", err)
		}
		path, err := WritePHPMemoryLimitConf("terceiroapp", "1G")
		if err != nil {
			t.Fatalf("second write: %v", err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read written conf: %v", err)
		}
		if got, want := string(data), "memory_limit = 1G\n"; got != want {
			t.Errorf("conf content after overwrite = %q, want %q", got, want)
		}
	})
}

func TestValidPHPVersionForMoodleVersion(t *testing.T) {
	tests := []struct {
		name          string
		moodleVersion string
		version       string
		want          bool
	}{
		{"3.x accepts 7.4", MoodleVersion3x, "7.4", true},
		{"3.x rejects 8.0", MoodleVersion3x, "8.0", false},
		{"4.1 accepts 7.4", MoodleVersion41, "7.4", true},
		{"4.1 rejects 8.2", MoodleVersion41, "8.2", false},
		{"4.2-4.3 accepts 8.2", MoodleVersion42to43, "8.2", true},
		{"4.2-4.3 rejects 7.4", MoodleVersion42to43, "7.4", false},
		{"4.2-4.3 rejects 8.3", MoodleVersion42to43, "8.3", false},
		{"4.4-4.5 accepts 8.3", MoodleVersion44to45, "8.3", true},
		{"4.4-4.5 rejects 8.0", MoodleVersion44to45, "8.0", false},
		{"4.x (saved containers) accepts 8.0", MoodleVersion4x, "8.0", true},
		{"4.x (saved containers) accepts 8.1", MoodleVersion4x, "8.1", true},
		{"4.x (saved containers) rejects 8.2", MoodleVersion4x, "8.2", false},
		{"5.0 accepts 8.2 (raised its PHP floor, moodledev.io/general/releases/5.0)", MoodleVersion50, "8.2", true},
		{"5.0 rejects 8.1", MoodleVersion50, "8.1", false},
		{"5.1+ rejects 8.2 (5.2 and 5.3 require 8.3)", MoodleVersion51Plus, "8.2", false},
		{"5.1+ accepts 8.3", MoodleVersion51Plus, "8.3", true},
		{"5.1+ accepts 8.4", MoodleVersion51Plus, "8.4", true},
		{"5.1+ rejects 8.1", MoodleVersion51Plus, "8.1", false},
		{"unrecognized Moodle version rejects everything", "6.0", "8.4", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidPHPVersionForMoodleVersion(tt.moodleVersion, tt.version); got != tt.want {
				t.Errorf("ValidPHPVersionForMoodleVersion(%q, %q) = %v, want %v", tt.moodleVersion, tt.version, got, tt.want)
			}
		})
	}
}

// The hash changes with any file or build arg, and not with map order.
func TestImageBuildHash(t *testing.T) {
	base := imageBuild{files: map[string]string{"Dockerfile": "FROM x", "php.ini": "a=1"}, args: []string{"UID=1000"}}
	same := imageBuild{files: map[string]string{"php.ini": "a=1", "Dockerfile": "FROM x"}, args: []string{"UID=1000"}}
	if base.hash() != same.hash() {
		t.Error("hash must not depend on map order")
	}
	for name, b := range map[string]imageBuild{
		"file":  {files: map[string]string{"Dockerfile": "FROM y", "php.ini": "a=1"}, args: []string{"UID=1000"}},
		"arg":   {files: base.files, args: []string{"UID=1001"}},
		"split": {files: map[string]string{"Dockerfile": "FROM xphp.ini", "": "a=1"}, args: []string{"UID=1000"}},
	} {
		if b.hash() == base.hash() {
			t.Errorf("%s change must change the hash", name)
		}
	}
}

// An image whose label doesn't match is rebuilt with the new label.
func TestEnsureImage_LabelsTheBuild(t *testing.T) {
	var out strings.Builder
	exe := executor.New(&out, &out)
	exe.DryRun = true
	if err := EnsureImage(context.Background(), exe, &out, "8.3"); err != nil {
		t.Fatal(err)
	}
	want := imageBuild{files: map[string]string{"Dockerfile": phpImageDockerfile, "php.ini": phpIni},
		args: []string{"PHP_VERSION=8.3", fmt.Sprintf("UID=%d", os.Getuid())}}.hash()
	if !strings.Contains(out.String(), "--label perci.hash="+want) {
		t.Errorf("build must carry the content hash %s:\n%s", want, out.String())
	}
}

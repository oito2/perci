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

package distro

import "testing"

func TestDetectDebian(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"ubuntu", "ID=ubuntu\nID_LIKE=debian"},
		{"linuxmint", "ID=linuxmint\nID_LIKE=ubuntu"},
		{"zorin", "ID=zorin\nID_LIKE=ubuntu"},
		{"pop", "ID=pop\nID_LIKE=\"ubuntu debian\""},
		{"kali", "ID=kali\nID_LIKE=debian"},
		{"elementary", "ID=elementary\nID_LIKE=ubuntu"},
		{"neon", "ID=neon\nID_LIKE=\"ubuntu debian\""},
		{"id_like fallback", "ID=something-unknown\nID_LIKE=ubuntu"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := detect(tc.content); got != Debian {
				t.Errorf("detect(%q) = %q, want %q", tc.content, got, Debian)
			}
		})
	}
}

func TestDetectFedora(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"fedora", "ID=fedora"},
		{"rhel", "ID=rhel"},
		{"rocky", "ID=rocky\nID_LIKE=\"rhel fedora\""},
		{"almalinux", "ID=almalinux\nID_LIKE=\"rhel fedora\""},
		{"nobara", "ID=nobara\nID_LIKE=fedora"},
		{"id_like fallback", "ID=something-unknown\nID_LIKE=fedora"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := detect(tc.content); got != Fedora {
				t.Errorf("detect(%q) = %q, want %q", tc.content, got, Fedora)
			}
		})
	}
}

func TestDetectUnknown(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"opensuse", "ID=opensuse-leap"},
		{"nixos", "ID=nixos"},
		{"gentoo", "ID=gentoo"},
		{"empty", ""},
		{"no id field", "NAME=Linux\nVERSION=1.0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := detect(tc.content); got != Unknown {
				t.Errorf("detect(%q) = %q, want %q", tc.content, got, Unknown)
			}
		})
	}
}

func TestDetectQuotedID(t *testing.T) {
	content := `ID="ubuntu"` + "\n" + `ID_LIKE="debian"`
	if got := detect(content); got != Debian {
		t.Errorf("detect with quoted ID = %q, want %q", got, Debian)
	}
}

func TestClean(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`"Ubuntu"`, "ubuntu"},
		{`ubuntu`, "ubuntu"},
		{`  Fedora  `, "fedora"},
		{`""`, ""},
	}
	for _, tc := range tests {
		got := clean(tc.input)
		if got != tc.want {
			t.Errorf("clean(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestDetectDE(t *testing.T) {
	cases := []struct {
		name    string
		desktop string
		want    string
	}{
		{"cinnamon", "X-Cinnamon", "cinnamon"},
		{"gnome", "GNOME", "gnome"},
		{"xfce", "XFCE", "xfce"},
		{"cosmic", "COSMIC", "cosmic"}, // Pop!_OS 24.04, added 2026-09-10
		{"unrecognized", "KDE", "other"},
		{"empty", "", "other"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("XDG_CURRENT_DESKTOP", tc.desktop)
			t.Setenv("DESKTOP_SESSION", "")
			if got := DetectDE(); got != tc.want {
				t.Errorf("DetectDE() with XDG_CURRENT_DESKTOP=%q = %q, want %q", tc.desktop, got, tc.want)
			}
		})
	}
}

func TestClassify(t *testing.T) {
	debianIDs := []string{"ubuntu", "debian", "linuxmint", "pop", "zorin",
		"elementary", "neon", "kali", "raspbian", "mx", "lmde", "peppermint",
		"tuxedo", "parrot"}
	for _, id := range debianIDs {
		if got := classify(id); got != Debian {
			t.Errorf("classify(%q) = %q, want %q", id, got, Debian)
		}
	}

	fedoraIDs := []string{"fedora", "rhel", "centos", "rocky", "almalinux",
		"ol", "scientific", "nobara", "ultramarine"}
	for _, id := range fedoraIDs {
		if got := classify(id); got != Fedora {
			t.Errorf("classify(%q) = %q, want %q", id, got, Fedora)
		}
	}

	unknownIDs := []string{"opensuse", "nixos", "gentoo", "arch", "manjaro", "", "something-random"}
	for _, id := range unknownIDs {
		if got := classify(id); got != Unknown {
			t.Errorf("classify(%q) = %q, want %q", id, got, Unknown)
		}
	}
}

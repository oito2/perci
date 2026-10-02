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

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
)

// Family constants returned by Detect.
const (
	Debian  = "debian"  // Ubuntu, Mint, Zorin, Elementary, KDE Neon, Pop!_OS, …
	Fedora  = "fedora"  // Fedora, RHEL, Rocky, AlmaLinux, CentOS, …
	Unknown = "unknown" // OpenSUSE, Gentoo, NixOS, …
)

// ErrUnsupportedFamily is returned (wrapped, via UnsupportedFamilyError)
// when a distro-dispatching function's family switch falls through to its
// default case — InstallPkgs and InstallFromSignedRepo.
var ErrUnsupportedFamily = errors.New("unsupported distro family")

// UnsupportedFamilyError wraps ErrUnsupportedFamily with the offending
// family value.
func UnsupportedFamilyError(family string) error {
	return fmt.Errorf("%w: %s", ErrUnsupportedFamily, family)
}

var (
	osReleaseOnce  sync.Once
	detectedFamily string
	cachedID       string
	cachedVer      string
)

// loadOSRelease reads and parses /etc/os-release exactly once per process
// lifetime, in a single pass over its lines computing everything every
// caller needs.
func loadOSRelease() {
	osReleaseOnce.Do(func() {
		data, _ := os.ReadFile("/etc/os-release") // err == read as empty content
		detectedFamily, cachedID, cachedVer = parseOSRelease(string(data))
	})
}

// parseOSRelease returns os-release content's family (see detect), ID=
// and VERSION_ID= — split from loadOSRelease so it can be tested without
// the real /etc/os-release.
func parseOSRelease(content string) (family, id, version string) {
	family = detect(content)
	for _, line := range strings.Split(content, "\n") {
		switch {
		case strings.HasPrefix(line, "ID="):
			id = clean(strings.TrimPrefix(line, "ID="))
		case strings.HasPrefix(line, "VERSION_ID="):
			version = clean(strings.TrimPrefix(line, "VERSION_ID="))
		}
	}
	return family, id, version
}

// Detect reads /etc/os-release once per process lifetime, checks ID= first and
// ID_LIKE= as fallback, and returns the normalized family (Debian, Fedora,
// or Unknown). The result is cached after the first call.
func Detect() string {
	loadOSRelease()
	return detectedFamily
}

// RawID returns the raw ID= field from /etc/os-release (e.g. "linuxmint", "ubuntu", "fedora").
// Unlike Detect, it does not normalize to a distribution family.
// Returns an empty string if /etc/os-release cannot be read or lacks the field.
func RawID() string {
	loadOSRelease()
	return cachedID
}

// VersionID returns the VERSION_ID= field from /etc/os-release (e.g. "24.04", "44").
// Returns an empty string if /etc/os-release cannot be read or lacks the field.
func VersionID() string {
	loadOSRelease()
	return cachedVer
}

// detect parses os-release content and returns the normalized family.
func detect(content string) string {
	var id, idLike string
	for _, line := range strings.Split(content, "\n") {
		switch {
		case strings.HasPrefix(line, "ID="):
			id = clean(strings.TrimPrefix(line, "ID="))
		case strings.HasPrefix(line, "ID_LIKE="):
			idLike = clean(strings.TrimPrefix(line, "ID_LIKE="))
		}
	}

	if f := classify(id); f != Unknown {
		return f
	}
	for _, like := range strings.Fields(idLike) {
		if f := classify(like); f != Unknown {
			return f
		}
	}
	return Unknown
}

func classify(id string) string {
	switch id {
	case "ubuntu", "debian", "linuxmint", "pop", "zorin",
		"elementary", "neon", "kali", "raspbian", "mx", "lmde",
		"peppermint", "tuxedo", "parrot":
		return Debian
	case "fedora", "rhel", "centos", "rocky", "almalinux",
		"ol", "scientific", "nobara", "ultramarine":
		return Fedora
	}
	return Unknown
}

// clean removes surrounding quotes and lowercases the value.
func clean(s string) string {
	return strings.ToLower(strings.Trim(strings.TrimSpace(s), `"`))
}

// normalizedDistro maps a raw OS ID to the distro token
// (mint | zorin | ubuntu | fedora). Returns "" for unsupported distros.
func normalizedDistro(id string) string {
	switch id {
	case "linuxmint":
		return "mint"
	case "zorin":
		return "zorin"
	case "ubuntu", "kubuntu":
		return "ubuntu"
	case "fedora":
		return "fedora"
	}
	return ""
}

// DisplayName is the running distribution's name for headers ("Linux
// Mint", "Ubuntu", ...), falling back to os-release's raw ID.
func DisplayName() string {
	return displayName(RawID())
}

func displayName(id string) string {
	switch normalizedDistro(id) {
	case "mint":
		return "Linux Mint"
	case "zorin":
		return "Zorin OS"
	case "ubuntu":
		return "Ubuntu"
	case "fedora":
		return "Fedora"
	}
	return id
}

// DetectDE returns the normalized desktop environment token
// (cinnamon | gnome | xfce | cosmic | other). Reads XDG_CURRENT_DESKTOP
// with DESKTOP_SESSION as fallback. The COSMIC session sets
// XDG_CURRENT_DESKTOP=COSMIC.
func DetectDE() string {
	desktop := strings.ToLower(os.Getenv("XDG_CURRENT_DESKTOP"))
	if desktop == "" {
		desktop = strings.ToLower(os.Getenv("DESKTOP_SESSION"))
	}
	switch {
	case strings.Contains(desktop, "cinnamon"):
		return "cinnamon"
	case strings.Contains(desktop, "gnome"):
		return "gnome"
	case strings.Contains(desktop, "xfce"):
		return "xfce"
	case strings.Contains(desktop, "cosmic"):
		return "cosmic"
	}
	return "other"
}

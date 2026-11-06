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

package repofiles

import (
	"embed"
	"fmt"
	"strconv"
	"strings"
)

// licenseFS holds the canonical license texts, unmodified: the GNU
// licenses as published at gnu.org, the others as published by
// choosealicense.com (with "[year]" and "[fullname]" in the copyright
// line of MIT and BSD-3-Clause).
//
//go:embed templates/licenses/*.txt
var licenseFS embed.FS

// DefaultLicense is the license selected when none is chosen.
const DefaultLicense = "GPL-3.0"

// License is one license offered for LICENSE.
type License struct {
	// ID is the SPDX identifier.
	ID   string `json:"id"`
	Name string `json:"name"`
	// Copyright says the text carries a copyright line that needs the
	// holder and the year.
	Copyright bool `json:"copyright"`
	file      string
}

// Licenses lists the licenses offered, in display order.
var Licenses = []License{
	{ID: "GPL-3.0", Name: "GNU GPL v3.0", file: "gpl-3.0.txt"},
	{ID: "AGPL-3.0", Name: "GNU AGPL v3.0", file: "agpl-3.0.txt"},
	{ID: "LGPL-3.0", Name: "GNU LGPL v3.0", file: "lgpl-3.0.txt"},
	{ID: "MIT", Name: "MIT", Copyright: true, file: "mit.txt"},
	{ID: "Apache-2.0", Name: "Apache 2.0", file: "apache-2.0.txt"},
	{ID: "MPL-2.0", Name: "Mozilla Public License 2.0", file: "mpl-2.0.txt"},
	{ID: "BSD-3-Clause", Name: "BSD 3-Clause", Copyright: true, file: "bsd-3-clause.txt"},
}

// findLicense returns the license with SPDX identifier id.
func findLicense(id string) (License, bool) {
	for _, l := range Licenses {
		if l.ID == id {
			return l, true
		}
	}
	return License{}, false
}

// licenseText returns the embedded text of l.
func licenseText(l License) string {
	data, err := licenseFS.ReadFile("templates/licenses/" + l.file)
	if err != nil {
		panic("repofiles: missing embedded license " + l.file) // the embed pattern covers every file of Licenses
	}
	return string(data)
}

// renderLicense returns the text of license id, with the copyright line
// filled with holder and year when the license has one.
func renderLicense(id, holder string, year int) (string, error) {
	l, ok := findLicense(id)
	if !ok {
		return "", fmt.Errorf("licença desconhecida: %q", id)
	}
	text := licenseText(l)
	if !l.Copyright {
		return text, nil
	}
	holder = strings.TrimSpace(holder)
	if holder == "" || strings.ContainsAny(holder, "\r\n") || len(holder) > 200 {
		return "", fmt.Errorf("informe o titular dos direitos da licença %s (uma linha, até 200 caracteres)", l.ID)
	}
	if year < 1000 || year > 9999 {
		return "", fmt.Errorf("ano inválido para a licença %s: %d", l.ID, year)
	}
	text = strings.Replace(text, "[year]", strconv.Itoa(year), 1)
	return strings.Replace(text, "[fullname]", holder, 1), nil
}

// DetectLicense returns the SPDX identifier of the offered license whose
// text matches text, or "" when none does. Lines starting with
// "Copyright" are ignored, and so are case and whitespace differences.
func DetectLicense(text string) string {
	got := normalizeLicense(text)
	if got == "" {
		return ""
	}
	for _, l := range Licenses {
		if normalizeLicense(licenseText(l)) == got {
			return l.ID
		}
	}
	return ""
}

// normalizeLicense drops the lines starting with "copyright", lowercases
// the rest and collapses every run of whitespace into one space.
func normalizeLicense(text string) string {
	var kept []string
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.ToLower(strings.TrimSpace(line))
		if strings.HasPrefix(trimmed, "copyright") {
			continue
		}
		kept = append(kept, trimmed)
	}
	return strings.Join(strings.Fields(strings.Join(kept, " ")), " ")
}

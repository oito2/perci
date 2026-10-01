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

package repo

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConductEN_RequiredSections(t *testing.T) {
	for _, section := range []string{
		"# Contributor Covenant",
		"## Our Pledge",
		"## Our Standards",
		"## Enforcement Responsibilities",
		"## Scope",
		"## Enforcement",
		"## Enforcement Guidelines",
		"## Attribution",
		contactPlaceholder,
		"[Português](docs/pt-br/codigo-de-conduta.md)",
	} {
		if !strings.Contains(conductEN, section) {
			t.Errorf("conductEN missing: %q", section)
		}
	}
}

func TestConductPT_RequiredSections(t *testing.T) {
	for _, section := range []string{
		"# Contributor Covenant",
		"## Nosso Compromisso",
		"## Nossos Padrões",
		"## Responsabilidades de Aplicação",
		"## Escopo",
		"## Aplicação",
		"## Diretrizes de Aplicação",
		"## Atribuição",
		contactPlaceholder,
		"[English](../../CODE_OF_CONDUCT.md)",
	} {
		if !strings.Contains(conductPT, section) {
			t.Errorf("conductPT missing: %q", section)
		}
	}
}

// The generated files go into other people's projects: no fixed address.
func TestConductTemplates_NoHardcodedEmail(t *testing.T) {
	for name, text := range map[string]string{"en": conductEN, "pt": conductPT} {
		withoutPlaceholder := strings.ReplaceAll(text, contactPlaceholder, "")
		if strings.Contains(withoutPlaceholder, "@") {
			t.Errorf("%s template contains a fixed e-mail address", name)
		}
	}
}

func TestCreateConduct_WritesBothFilesWithContact(t *testing.T) {
	dir := t.TempDir()
	if err := CreateConduct(io.Discard, dir, " conduta@exemplo.org ", true); err != nil {
		t.Fatalf("CreateConduct: %v", err)
	}
	for _, rel := range []string{"CODE_OF_CONDUCT.md", filepath.Join("docs", "pt-br", "codigo-de-conduta.md")} {
		data, err := os.ReadFile(filepath.Join(dir, rel))
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		text := string(data)
		if !strings.Contains(text, "**conduta@exemplo.org**") || strings.Contains(text, contactPlaceholder) {
			t.Errorf("%s: contact e-mail not substituted", rel)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, legacyFilePT)); !os.IsNotExist(err) {
		t.Error("the legacy root CODIGO_DE_CONDUTA.md must not be created")
	}
}

func TestCreateConduct_RejectsInvalidEmail(t *testing.T) {
	for _, email := range []string{"", "   ", "sem-arroba", "a@b", "a b@c.d", "a@b.c\nx"} {
		if err := CreateConduct(io.Discard, t.TempDir(), email, true); err == nil {
			t.Errorf("e-mail %q should be rejected", email)
		}
	}
}

func TestCreateConduct_KeepsLegacyFile(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, legacyFilePT)
	if err := os.WriteFile(legacy, []byte("editado"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CreateConduct(io.Discard, dir, "a@b.co", true); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(legacy); err != nil || string(data) != "editado" {
		t.Errorf("legacy file must be left untouched, got %q, %v", data, err)
	}
}

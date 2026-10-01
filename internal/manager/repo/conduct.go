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
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/oito2/perci/internal/ui"
)

// Contributor Covenant texts, with {{CONTACT_EMAIL}} standing for the
// project's own enforcement contact — never a fixed address: these files
// are generated into other people's projects.
var (
	//go:embed templates/CODE_OF_CONDUCT.en.md
	conductEN string
	//go:embed templates/CODE_OF_CONDUCT.pt.md
	conductPT string
)

const contactPlaceholder = "{{CONTACT_EMAIL}}"

// Generated file locations, relative to the project root — the English
// canon at the root and its Portuguese mirror under docs/pt-br/, the same
// layout the "Documentação de projeto" standard uses (only the root holds
// canonical files; mirrors never live there).
var (
	fileEN = "CODE_OF_CONDUCT.md"
	filePT = filepath.Join("docs", "pt-br", "codigo-de-conduta.md")
)

// legacyFilePT is where older Perci versions wrote the Portuguese mirror.
const legacyFilePT = "CODIGO_DE_CONDUTA.md"

// validContactEmail is a deliberately loose check (something@domain.tld,
// no whitespace): the value only ends up as text in the generated
// Markdown, but it must look like a usable reporting address.
var validContactEmail = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// CreateConduct writes CODE_OF_CONDUCT.md and docs/pt-br/codigo-de-conduta.md
// into dir, with contactEmail as the address for reporting violations. If
// overwrite is false, existing files are skipped. A CODIGO_DE_CONDUTA.md
// left at the root by an older Perci version is reported, never deleted —
// it may have been edited.
func CreateConduct(stdout io.Writer, dir, contactEmail string, overwrite bool) error {
	contactEmail = strings.TrimSpace(contactEmail)
	if !validContactEmail.MatchString(contactEmail) {
		return fmt.Errorf("informe um e-mail de contato válido para denúncias (recebido: %q)", contactEmail)
	}

	existing := make([]string, 0, 2)
	for _, f := range []string{fileEN, filePT} {
		if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
			existing = append(existing, f)
		}
	}
	if len(existing) > 0 && !overwrite {
		ui.Warning(stdout, "Arquivo(s) de Código de Conduta já existente(s): "+strings.Join(existing, ", "))
		return nil
	}

	for _, f := range []struct {
		name    string
		content string
	}{
		{fileEN, conductEN},
		{filePT, conductPT},
	} {
		path := filepath.Join(dir, f.name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("criar %s: %w", filepath.Dir(path), err)
		}
		content := strings.ReplaceAll(f.content, contactPlaceholder, contactEmail)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("criar %s: %w", path, err)
		}
		ui.Info(stdout, "Criado: "+path)
	}

	if _, err := os.Stat(filepath.Join(dir, legacyFilePT)); err == nil {
		ui.Warning(stdout, legacyFilePT+" na raiz foi gerado por uma versão anterior — o espelho em português agora fica em "+
			filePT+". Remova o arquivo antigo se não precisar mais dele.")
	}

	ui.Success(stdout, "Código de conduta criado com sucesso.")
	return nil
}

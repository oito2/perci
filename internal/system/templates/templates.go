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

package templates

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/oito2/perci/internal/sets"
	"github.com/oito2/perci/internal/ui"
)

// Template describes a file template managed by perci.gnl.
type Template struct {
	Label    string
	Filename string
}

// Catalogue lists all templates managed by perci.gnl.
var Catalogue = []Template{
	{Label: "Documento Word (.docx)", Filename: "Documento.docx"},
	{Label: "Planilha Excel (.xlsx)", Filename: "Planilha.xlsx"},
	{Label: "Apresentação PowerPoint (.pptx)", Filename: "Apresentacao.pptx"},
	{Label: "Documento LibreOffice (.odt)", Filename: "Documento.odt"},
	{Label: "Planilha LibreOffice (.ods)", Filename: "Planilha.ods"},
	{Label: "Texto (.txt)", Filename: "Texto.txt"},
	{Label: "Markdown (.md)", Filename: "Documento.md"},
	{Label: "HTML (.html)", Filename: "HTML.html"},
	{Label: "CSS (.css)", Filename: "Estilo.css"},
	{Label: "JavaScript (.js)", Filename: "Script.js"},
	{Label: "Python (.py)", Filename: "Script.py"},
	{Label: "PHP (.php)", Filename: "PHP.php"},
	{Label: "Shell (.sh)", Filename: "Shell.sh"},
}

// Dir returns the user templates directory, creating it when needed.
func Dir() (string, error) {
	home, herr := os.UserHomeDir()
	if herr != nil {
		return "", herr
	}

	// xdg-user-dir falls back to $HOME itself when user-dirs.dirs isn't
	// configured; that fallback is rejected rather than used as the
	// templates dir.
	out, err := exec.Command("xdg-user-dir", "TEMPLATES").Output()
	resolved := strings.TrimSpace(string(out))
	if err == nil && resolved != "" && resolved != home {
		if _, serr := os.Stat(resolved); serr == nil {
			return resolved, nil
		}
	}

	modelos := filepath.Join(home, "Modelos")
	if _, serr := os.Stat(modelos); serr == nil {
		return modelos, nil
	}

	templatesDir := filepath.Join(home, "Templates")
	if _, serr := os.Stat(templatesDir); serr == nil {
		return templatesDir, nil
	}

	return templatesDir, nil
}

// PresentNames returns a set of template filenames that already exist in dir.
func PresentNames(dir string) map[string]bool {
	result := make(map[string]bool, len(Catalogue))
	for _, t := range Catalogue {
		if _, err := os.Stat(filepath.Join(dir, t.Filename)); err == nil {
			result[t.Filename] = true
		}
	}
	return result
}

// Apply creates templates in toCreate and removes those in toRemove.
// toRemove may contain filenames not in the Catalogue (e.g. external
// templates). Reports one ui.Step per template processed.
func Apply(stdout io.Writer, dir string, toCreate, toRemove []string) error {
	if len(toCreate) > 0 {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create templates dir %s: %w", dir, err)
		}
	}

	total := len(toCreate) + len(toRemove)
	step := 0

	createSet := sets.Of(toCreate)
	for _, t := range Catalogue {
		if !createSet[t.Filename] {
			continue
		}
		step++
		ui.Step(stdout, step, total, "Criando "+t.Filename+"...")
		dest := filepath.Join(dir, t.Filename)
		if err := create(dest, t); err != nil {
			ui.Warning(stdout, fmt.Sprintf("Falha ao criar %s: %v", t.Filename, err))
		}
	}
	for _, filename := range toRemove {
		step++
		ui.Step(stdout, step, total, "Removendo "+filename+"...")
		// toRemove comes from the frontend: only a plain file name inside
		// dir is accepted, never a path that leaves it.
		if filename == "" || filename == "." || filename == ".." || filepath.Base(filename) != filename {
			ui.Warning(stdout, "Nome de modelo inválido, ignorado: "+filename)
			continue
		}
		dest := filepath.Join(dir, filename)
		if err := os.Remove(dest); err != nil && !os.IsNotExist(err) {
			ui.Warning(stdout, fmt.Sprintf("Falha ao remover %s: %v", filename, err))
		}
	}
	return nil
}

func create(dest string, t Template) error {
	ext := filepath.Ext(t.Filename)
	switch ext {
	case ".docx":
		return createDocx(dest)
	case ".xlsx":
		return createXlsx(dest)
	case ".pptx":
		return createPptx(dest)
	case ".odt":
		return createOdt(dest)
	case ".ods":
		return createOds(dest)
	default:
		return createText(dest, textContent(ext))
	}
}

// zipEntry is a stored-first (uncompressed) archive member. ODF's
// "mimetype" file must be the archive's first entry and must not be
// deflated; the OOXML formats pass nil instead.
type zipEntry struct {
	name    string
	content string
}

// createZip builds dest as a zip archive containing entries (name→
// content, deflated), optionally preceded by storedFirst written
// uncompressed.
//
// dest is created exclusively (never overwriting a file that appeared in
// the meantime), and entries are written in name order, so the same
// template always produces the same archive.
func createZip(dest string, storedFirst *zipEntry, entries map[string]string) error {
	f, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	w := zip.NewWriter(f)
	cleanup := func() {
		_ = w.Close()
		_ = f.Close()
		_ = os.Remove(dest)
	}

	if storedFirst != nil {
		fw, err := w.CreateHeader(&zip.FileHeader{Name: storedFirst.name, Method: zip.Store})
		if err != nil {
			cleanup()
			return err
		}
		if _, err := io.WriteString(fw, storedFirst.content); err != nil {
			cleanup()
			return err
		}
	}

	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		content := entries[name]
		fw, err := w.Create(name)
		if err != nil {
			cleanup()
			return err
		}
		if _, err := io.WriteString(fw, content); err != nil {
			cleanup()
			return err
		}
	}
	if err := w.Close(); err != nil {
		_ = f.Close()
		_ = os.Remove(dest)
		return err
	}
	return f.Close()
}

func createDocx(dest string) error {
	return createZip(dest, nil, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`,
		"_rels/.rels":         `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`,
		"word/document.xml":   `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p/></w:body></w:document>`,
	})
}

func createXlsx(dest string) error {
	return createZip(dest, nil, map[string]string{
		"[Content_Types].xml":        `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>`,
		"_rels/.rels":                `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`,
		"xl/workbook.xml":            `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Sheet1" sheetId="1" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`,
		"xl/worksheets/sheet1.xml":   `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData/></worksheet>`,
	})
}

func createPptx(dest string) error {
	return createZip(dest, nil, map[string]string{
		"[Content_Types].xml":  `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/></Types>`,
		"_rels/.rels":          `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldMasterIdLst/><p:sldSz cx="9144000" cy="6858000"/><p:notesSz cx="6858000" cy="9144000"/></p:presentation>`,
	})
}

func createOdt(dest string) error {
	// mimetype must be first and uncompressed
	mimetype := &zipEntry{name: "mimetype", content: "application/vnd.oasis.opendocument.text"}
	return createZip(dest, mimetype, map[string]string{
		"META-INF/manifest.xml": `<?xml version="1.0" encoding="UTF-8"?><manifest:manifest xmlns:manifest="urn:oasis:names:tc:opendocument:xmlns:manifest:1.0" manifest:version="1.2"><manifest:file-entry manifest:full-path="/" manifest:media-type="application/vnd.oasis.opendocument.text"/><manifest:file-entry manifest:full-path="content.xml" manifest:media-type="text/xml"/></manifest:manifest>`,
		"content.xml":           `<?xml version="1.0" encoding="UTF-8"?><office:document-content xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0" office:version="1.2"><office:body><office:text><text:p/></office:text></office:body></office:document-content>`,
	})
}

func createOds(dest string) error {
	mimetype := &zipEntry{name: "mimetype", content: "application/vnd.oasis.opendocument.spreadsheet"}
	return createZip(dest, mimetype, map[string]string{
		"META-INF/manifest.xml": `<?xml version="1.0" encoding="UTF-8"?><manifest:manifest xmlns:manifest="urn:oasis:names:tc:opendocument:xmlns:manifest:1.0" manifest:version="1.2"><manifest:file-entry manifest:full-path="/" manifest:media-type="application/vnd.oasis.opendocument.spreadsheet"/><manifest:file-entry manifest:full-path="content.xml" manifest:media-type="text/xml"/></manifest:manifest>`,
		"content.xml":           `<?xml version="1.0" encoding="UTF-8"?><office:document-content xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" xmlns:table="urn:oasis:names:tc:opendocument:xmlns:table:1.0" xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0" office:version="1.2"><office:body><office:spreadsheet><table:table table:name="Planilha1"><table:table-row><table:table-cell/></table:table-row></table:table></office:spreadsheet></office:body></office:document-content>`,
	})
}

func createText(dest, content string) error {
	perm := os.FileMode(0o644)
	ext := filepath.Ext(dest)
	if ext == ".sh" || ext == ".py" {
		perm = 0o755
	}
	// Exclusive, like createZip: an existing file is never overwritten.
	f, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(f, content); err != nil {
		_ = f.Close()
		_ = os.Remove(dest)
		return err
	}
	return f.Close()
}

func textContent(ext string) string {
	switch ext {
	case ".txt":
		return ""
	case ".md":
		return "# Título\n"
	case ".html":
		return "<!DOCTYPE html>\n<html lang=\"pt-BR\">\n<head>\n    <meta charset=\"UTF-8\">\n    <title></title>\n</head>\n<body>\n</body>\n</html>\n"
	case ".css":
		return "/* ============================================================\n   Stylesheet\n   ============================================================ */\n\n*, *::before, *::after {\n    box-sizing: border-box;\n    margin: 0;\n    padding: 0;\n}\n"
	case ".js":
		return "'use strict';\n"
	case ".py":
		return "#!/usr/bin/env python3\n\n\ndef main():\n    pass\n\n\nif __name__ == '__main__':\n    main()\n"
	case ".php":
		return "<?php\n\ndeclare(strict_types=1);\n"
	case ".sh":
		return "#!/usr/bin/env bash\nset -euo pipefail\n"
	}
	return ""
}

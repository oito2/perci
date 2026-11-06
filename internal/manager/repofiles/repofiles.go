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

// Package repofiles generates, inspects and removes the community files
// of a repository: .gitignore, LICENSE, code of conduct, support page,
// issue templates and pull/merge request template.
package repofiles

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/oito2/perci/internal/fsutil"
	"github.com/oito2/perci/internal/manager/gitignore"
)

// Row identifiers, in table order.
const (
	RowGitignore      = "gitignore"
	RowLicense        = "license"
	RowConduct        = "conduct"
	RowSupport        = "support"
	RowIssueTemplates = "issue-templates"
	RowPRTemplate     = "pr-template"
	RowFunding        = "funding"
)

// Providers.
const (
	ProviderGitHub = "github"
	ProviderGitLab = "gitlab"
)

// State of a row's files compared with what Apply would write now.
type State string

const (
	// StateMissing: none of the row's files exists.
	StateMissing State = "missing"
	// StateCurrent: every file exists with the content Apply would write.
	StateCurrent State = "current"
	// StateDiffers: some file is missing or has other content.
	StateDiffers State = "differs"
	// StatePresent: files exist but cannot be compared, because a value
	// the content depends on (the contact e-mail) is not set.
	StatePresent State = "present"
	// StateDisabled: the row is not available yet.
	StateDisabled State = "disabled"
)

// ErrDisabled is returned by Apply and Remove for a row that is not
// available yet.
var ErrDisabled = errors.New("este arquivo ainda não está disponível")

// ErrUnknownRow is returned for a row identifier that does not exist.
var ErrUnknownRow = errors.New("arquivo desconhecido")

// Options holds the values the generated files depend on.
type Options struct {
	// Provider is ProviderGitHub or ProviderGitLab; it picks the issue
	// and pull/merge request template locations.
	Provider string
	// Project is the project name written in SUPPORT.md.
	Project string
	// WebURL is the repository page (https://host/owner/name), used for
	// the issues and support links; empty when there is no remote.
	WebURL string
	// ContactEmail is the address for private contact and code of
	// conduct reports.
	ContactEmail string
	// License is the SPDX identifier Apply writes and the status compares
	// against; empty means DefaultLicense.
	License string
	// Holder and Year fill the copyright line of licenses that have one.
	Holder string
	Year   int
}

// Row is one line of the files table.
type Row struct {
	ID string `json:"id"`
	// Label is the name shown in the table.
	Label string `json:"label"`
	// Files lists the paths Apply writes, relative to the repository.
	Files []string `json:"files"`
	State State    `json:"state"`
	// Missing is how many .gitignore patterns are missing.
	Missing int `json:"missing,omitempty"`
	// License is the SPDX identifier of the license found in LICENSE, or
	// "" when there is none or it is not recognized.
	License string `json:"license,omitempty"`
	// NeedsEmail says the row's content uses the contact e-mail.
	NeedsEmail bool `json:"needsEmail,omitempty"`
	// Notes are remarks about files outside the row (an old file left
	// by a previous version, another license file name).
	Notes []string `json:"notes,omitempty"`
}

// file is one generated file: its path relative to the repository and
// its content.
type file struct {
	rel     string
	content string
}

// Placeholders of the templates.
const (
	phProject    = "{{PROJECT}}"
	phContact    = "{{CONTACT_EMAIL}}"
	phIssues     = "{{ISSUES}}"
	phSupportURL = "{{SUPPORT_URL}}"
)

// validContactEmail accepts a plain address (letters, digits and
// ._%+- before the @, a dotted domain after it), so the value is safe
// inside Markdown.
var validContactEmail = regexp.MustCompile(`^[A-Za-z0-9._%+\-]+@[A-Za-z0-9\-]+(\.[A-Za-z0-9\-]+)+$`)

// validWebURL accepts an https URL without whitespace, quotes, brackets
// or parentheses, so the value is safe inside Markdown and YAML.
var validWebURL = regexp.MustCompile(`^https://[^\s"'<>()\[\]]+$`)

// rowOrder lists the rows in table order.
var rowOrder = []string{RowGitignore, RowLicense, RowConduct, RowSupport, RowIssueTemplates, RowPRTemplate, RowFunding}

// Legacy and alternative file names reported in Notes.
const legacyConductPT = "CODIGO_DE_CONDUTA.md"

var otherLicenseNames = []string{"LICENSE.md", "LICENSE.txt", "COPYING", "COPYING.md", "LICENCE"}

// Generated file locations.
var (
	conductENPath = "CODE_OF_CONDUCT.md"
	conductPTPath = filepath.Join("docs", "pt-br", "codigo-de-conduta.md")
	supportENPath = "SUPPORT.md"
	supportPTPath = filepath.Join("docs", "pt-br", "suporte.md")
)

// Rows returns the table rows for the repository at dir.
func Rows(dir string, opts Options) ([]Row, error) {
	if err := check(dir, opts); err != nil {
		return nil, err
	}
	rows := make([]Row, 0, len(rowOrder))
	for _, id := range rowOrder {
		row, err := rowState(dir, id, opts)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// rowState builds the row id with its state in dir.
func rowState(dir, id string, opts Options) (Row, error) {
	row := Row{ID: id, Label: label(id, opts), Files: paths(id, opts)}
	switch id {
	case RowFunding:
		row.State = StateDisabled
		return row, nil
	case RowGitignore:
		missing, exists, err := gitignore.Missing(dir)
		if err != nil {
			return Row{}, err
		}
		switch {
		case !exists:
			row.State = StateMissing
		case missing == 0:
			row.State = StateCurrent
		default:
			row.State, row.Missing = StateDiffers, missing
		}
		return row, nil
	case RowLicense:
		return licenseRow(dir, row, opts)
	}

	row.NeedsEmail = id == RowConduct || id == RowSupport
	if id == RowConduct {
		if exists(filepath.Join(dir, legacyConductPT)) {
			row.Notes = append(row.Notes, legacyConductPT+" na raiz é de uma versão anterior; o espelho em português agora fica em "+conductPTPath+".")
		}
	}

	present := 0
	for _, rel := range row.Files {
		if exists(filepath.Join(dir, rel)) {
			present++
		}
	}
	if present == 0 {
		row.State = StateMissing
		return row, nil
	}
	if row.NeedsEmail && strings.TrimSpace(opts.ContactEmail) == "" {
		row.State = StatePresent
		return row, nil
	}
	files, err := render(id, opts)
	if err != nil {
		return Row{}, err
	}
	row.State = StateCurrent
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(dir, f.rel))
		if err != nil || !bytes.Equal(data, []byte(f.content)) {
			row.State = StateDiffers
			break
		}
	}
	return row, nil
}

// licenseRow fills row with the state of dir/LICENSE.
func licenseRow(dir string, row Row, opts Options) (Row, error) {
	for _, name := range otherLicenseNames {
		if exists(filepath.Join(dir, name)) {
			row.Notes = append(row.Notes, "Também existe "+name+".")
		}
	}
	data, err := os.ReadFile(filepath.Join(dir, "LICENSE"))
	if errors.Is(err, os.ErrNotExist) {
		row.State = StateMissing
		return row, nil
	}
	if err != nil {
		return Row{}, fmt.Errorf("ler LICENSE: %w", err)
	}
	row.License = DetectLicense(string(data))
	want := opts.License
	if want == "" {
		want = DefaultLicense
	}
	if _, ok := findLicense(want); !ok {
		return Row{}, fmt.Errorf("licença desconhecida: %q", want)
	}
	if row.License == want {
		row.State = StateCurrent
	} else {
		row.State = StateDiffers
	}
	return row, nil
}

// Apply writes the files of row id into dir and returns a short summary.
// The .gitignore only gets its missing patterns appended; every other
// file is replaced, through a symlink when the path is one, and a
// different previous version is kept as <file>.perci-bak.
func Apply(dir, id string, opts Options) (string, error) {
	if err := check(dir, opts); err != nil {
		return "", err
	}
	switch id {
	case RowFunding:
		return "", ErrDisabled
	case RowGitignore:
		missing, exists, err := gitignore.Missing(dir)
		if err != nil {
			return "", err
		}
		if _, err := gitignore.Generate(io.Discard, dir); err != nil {
			return "", err
		}
		switch {
		case !exists:
			return ".gitignore criado.", nil
		case missing == 0:
			return ".gitignore já contém todos os padrões.", nil
		default:
			return fmt.Sprintf(".gitignore atualizado: %d padrão(ões) acrescentado(s).", missing), nil
		}
	}

	files, err := render(id, opts)
	if err != nil {
		return "", err
	}
	for _, f := range files {
		path := filepath.Join(dir, f.rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return "", fmt.Errorf("criar %s: %w", filepath.Dir(f.rel), err)
		}
		if err := fsutil.WriteGenerated(path, []byte(f.content), 0o644); err != nil {
			return "", fmt.Errorf("gravar %s: %w", f.rel, err)
		}
	}
	return label(id, opts) + " gravado.", nil
}

// Remove deletes the files of row id from dir, then the folders under
// .github/ or .gitlab/ left empty. Missing files are ignored.
func Remove(dir, id string, opts Options) error {
	if err := check(dir, opts); err != nil {
		return err
	}
	if id == RowFunding {
		return ErrDisabled
	}
	if !knownRow(id) {
		return fmt.Errorf("%w: %q", ErrUnknownRow, id)
	}
	for _, rel := range paths(id, opts) {
		path := filepath.Join(dir, rel)
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remover %s: %w", rel, err)
		}
		removeEmptyParents(dir, rel)
	}
	return nil
}

// removeEmptyParents removes the empty folders of rel, from the deepest
// up, when rel lives under .github/ or .gitlab/; the top folder itself
// is removed too when empty.
func removeEmptyParents(dir, rel string) {
	top := strings.SplitN(filepath.ToSlash(rel), "/", 2)[0]
	if top != ".github" && top != ".gitlab" {
		return
	}
	for d := filepath.Dir(rel); d != "." && d != ""; d = filepath.Dir(d) {
		if err := os.Remove(filepath.Join(dir, d)); err != nil {
			return // not empty, or already gone
		}
	}
}

// label returns the table name of row id.
func label(id string, opts Options) string {
	switch id {
	case RowGitignore:
		return ".gitignore"
	case RowLicense:
		return "LICENSE"
	case RowConduct:
		return "CODE_OF_CONDUCT.md"
	case RowSupport:
		return "SUPPORT.md"
	case RowIssueTemplates:
		if opts.Provider == ProviderGitLab {
			return ".gitlab/issue_templates/"
		}
		return ".github/ISSUE_TEMPLATE/"
	case RowPRTemplate:
		if opts.Provider == ProviderGitLab {
			return ".gitlab/merge_request_templates/Default.md"
		}
		return ".github/PULL_REQUEST_TEMPLATE.md"
	case RowFunding:
		return ".github/FUNDING.yml"
	}
	return id
}

// paths returns the files of row id, relative to the repository.
func paths(id string, opts Options) []string {
	gitlab := opts.Provider == ProviderGitLab
	switch id {
	case RowGitignore:
		return []string{".gitignore"}
	case RowLicense:
		return []string{"LICENSE"}
	case RowConduct:
		return []string{conductENPath, conductPTPath}
	case RowSupport:
		return []string{supportENPath, supportPTPath}
	case RowIssueTemplates:
		if gitlab {
			return []string{
				filepath.Join(".gitlab", "issue_templates", "Bug.md"),
				filepath.Join(".gitlab", "issue_templates", "Feature.md"),
			}
		}
		return []string{
			filepath.Join(".github", "ISSUE_TEMPLATE", "bug_report.yml"),
			filepath.Join(".github", "ISSUE_TEMPLATE", "feature_request.yml"),
			filepath.Join(".github", "ISSUE_TEMPLATE", "config.yml"),
		}
	case RowPRTemplate:
		if gitlab {
			return []string{filepath.Join(".gitlab", "merge_request_templates", "Default.md")}
		}
		return []string{filepath.Join(".github", "PULL_REQUEST_TEMPLATE.md")}
	case RowFunding:
		return []string{filepath.Join(".github", "FUNDING.yml")}
	}
	return nil
}

func knownRow(id string) bool {
	for _, r := range rowOrder {
		if r == id {
			return true
		}
	}
	return false
}

// check refuses an unknown provider and a dir that is not an existing
// folder.
func check(dir string, opts Options) error {
	if opts.Provider != ProviderGitHub && opts.Provider != ProviderGitLab {
		return fmt.Errorf("provedor desconhecido: %q", opts.Provider)
	}
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("pasta do repositório: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s não é uma pasta", dir)
	}
	return nil
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

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
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func ghOpts() Options {
	return Options{
		Provider: ProviderGitHub, Project: "demo", WebURL: "https://github.com/someone/demo",
		ContactEmail: "contact@example.org",
	}
}

func glOpts() Options {
	o := ghOpts()
	o.Provider = ProviderGitLab
	o.WebURL = "https://gitlab.com/someone/demo"
	return o
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func rowsByID(t *testing.T, dir string, opts Options) map[string]Row {
	t.Helper()
	rows, err := Rows(dir, opts)
	if err != nil {
		t.Fatalf("Rows: %v", err)
	}
	out := map[string]Row{}
	for _, r := range rows {
		out[r.ID] = r
	}
	return out
}

func mustApply(t *testing.T, dir, id string, opts Options) {
	t.Helper()
	if _, err := Apply(dir, id, opts); err != nil {
		t.Fatalf("Apply(%s): %v", id, err)
	}
}

// The license texts are the canonical ones, byte for byte.
func TestLicenseTexts_Pinned(t *testing.T) {
	want := map[string]string{
		"agpl-3.0.txt":     "0d96a4ff68ad6d4b6f1f30f713b18d5184912ba8dd389f86aa7710db079abcb0",
		"apache-2.0.txt":   "c71d239df91726fc519c6eb72d318ec65820627232b2f796219e87dcf35d0ab4",
		"bsd-3-clause.txt": "f24d1328dbfffe7bf66aa877957db75e60629358357d8c366ccab616fef487ab",
		"gpl-3.0.txt":      "3972dc9744f6499f0f9b2dbf76696f2ae7ad8af9b23dde66d6af86c9dfb36986",
		"lgpl-3.0.txt":     "e3a994d82e644b03a792a930f574002658412f62407f5fee083f2555c5f23118",
		"mit.txt":          "002c2696d92b5c8cf956c11072baa58eaf9f6ade995c031ea635c6a1ee342ad1",
		"mpl-2.0.txt":      "3f3d9e0024b1921b067d6f7f88deb4a60cbe7a78e76c64e3f1d7fc3b779b9d04",
	}
	if len(Licenses) != len(want) {
		t.Fatalf("got %d licenses, want %d", len(Licenses), len(want))
	}
	for _, l := range Licenses {
		sum := sha256.Sum256([]byte(licenseText(l)))
		if got := hex.EncodeToString(sum[:]); got != want[l.file] {
			t.Errorf("%s: sha256 %s, want %s", l.file, got, want[l.file])
		}
		hasPlaceholders := strings.Contains(licenseText(l), "[year]") && strings.Contains(licenseText(l), "[fullname]")
		if hasPlaceholders != l.Copyright {
			t.Errorf("%s: copyright placeholders = %v, want %v", l.ID, hasPlaceholders, l.Copyright)
		}
	}
	if _, ok := findLicense(DefaultLicense); !ok {
		t.Errorf("default license %q is not offered", DefaultLicense)
	}
}

func TestRenderLicense(t *testing.T) {
	text, err := renderLicense("MIT", " Jane Doe ", 2026)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "Copyright (c) 2026 Jane Doe\n") || strings.Contains(text, "[") {
		t.Errorf("MIT copyright line not filled: %.120q", text)
	}
	text, err = renderLicense("BSD-3-Clause", "Jane Doe", 2026)
	if err != nil || !strings.Contains(text, "Copyright (c) 2026, Jane Doe") {
		t.Errorf("BSD copyright line not filled: %v %.120q", err, text)
	}
	if text, err := renderLicense("GPL-3.0", "", 0); err != nil || text != licenseText(Licenses[0]) {
		t.Errorf("GPL-3.0 must be the unmodified text without holder (err %v)", err)
	}
	for _, tc := range []struct {
		name, id, holder string
		year             int
	}{
		{"unknown license", "WTFPL", "x", 2026},
		{"empty holder", "MIT", "  ", 2026},
		{"multiline holder", "MIT", "a\nb", 2026},
		{"long holder", "MIT", strings.Repeat("a", 201), 2026},
		{"bad year", "BSD-3-Clause", "x", 26},
	} {
		if _, err := renderLicense(tc.id, tc.holder, tc.year); err == nil {
			t.Errorf("%s: want an error", tc.name)
		}
	}
}

func TestDetectLicense(t *testing.T) {
	for _, l := range Licenses {
		text, err := renderLicense(l.ID, "Someone Else", 1999)
		if err != nil {
			t.Fatal(err)
		}
		if got := DetectLicense(text); got != l.ID {
			t.Errorf("rendered %s detected as %q", l.ID, got)
		}
		lines := strings.Split(text, "\n")
		for i, line := range lines {
			lines[i] = "  " + strings.ToUpper(strings.Join(strings.Fields(line), "   "))
		}
		if got := DetectLicense(strings.Join(lines, "\r\n")); got != l.ID {
			t.Errorf("re-spaced %s detected as %q", l.ID, got)
		}
	}
	gpl := licenseText(Licenses[0])
	for name, text := range map[string]string{
		"empty":     "",
		"other":     "All rights reserved.\n",
		"truncated": gpl[:len(gpl)/2],
		"edited":    strings.Replace(gpl, "free software", "free-ish software", 1),
	} {
		if got := DetectLicense(text); got != "" {
			t.Errorf("%s text detected as %q", name, got)
		}
	}
}

func TestTemplates_NoFixedAddress(t *testing.T) {
	for name, text := range map[string]string{
		"conductEN": conductEN, "conductPT": conductPT, "supportEN": supportEN, "supportPT": supportPT,
		"githubBug": githubBug, "githubFeature": githubFeature, "githubConfig": githubConfig,
		"gitlabBug": gitlabBug, "gitlabFeature": gitlabFeature, "prTemplate": prTemplate,
	} {
		if strings.Contains(strings.ReplaceAll(text, phContact, ""), "@") {
			t.Errorf("%s contains a fixed e-mail address", name)
		}
		if strings.Contains(text, "github.com/") || strings.Contains(text, "gitlab.com/") {
			t.Errorf("%s contains a fixed repository URL", name)
		}
	}
}

func TestConductTemplates_RequiredSections(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		want       []string
	}{
		{"en", conductEN, []string{
			"# Contributor Covenant", "## Our Pledge", "## Our Standards", "## Enforcement Responsibilities",
			"## Scope", "## Enforcement", "## Enforcement Guidelines", "## Attribution", phContact,
			"[Português](docs/pt-br/codigo-de-conduta.md)",
		}},
		{"pt", conductPT, []string{
			"# Contributor Covenant", "## Nosso Compromisso", "## Nossos Padrões", "## Responsabilidades de Aplicação",
			"## Escopo", "## Aplicação", "## Diretrizes de Aplicação", "## Atribuição", phContact,
			"[English](../../CODE_OF_CONDUCT.md)",
		}},
		{"support en", supportEN, []string{phProject, phIssues, phContact, "[Português](docs/pt-br/suporte.md)"}},
		{"support pt", supportPT, []string{phProject, phIssues, phContact, "[English](../../SUPPORT.md)"}},
	} {
		for _, w := range tc.want {
			if !strings.Contains(tc.text, w) {
				t.Errorf("%s template missing %q", tc.name, w)
			}
		}
	}
}

// The issue forms are valid YAML with the keys GitHub requires.
func TestGitHubIssueForms(t *testing.T) {
	for name, text := range map[string]string{"bug_report": githubBug, "feature_request": githubFeature} {
		var form struct {
			Name        string           `yaml:"name"`
			Description string           `yaml:"description"`
			Body        []map[string]any `yaml:"body"`
		}
		if err := yaml.Unmarshal([]byte(text), &form); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if form.Name == "" || form.Description == "" || len(form.Body) == 0 {
			t.Errorf("%s: missing name, description or body", name)
		}
		ids := map[string]bool{}
		for _, el := range form.Body {
			if el["type"] == nil || el["attributes"] == nil {
				t.Errorf("%s: element without type or attributes: %v", name, el)
			}
			if id, ok := el["id"].(string); ok {
				if ids[id] {
					t.Errorf("%s: repeated id %q", name, id)
				}
				ids[id] = true
			}
		}
	}

	dir := t.TempDir()
	mustApply(t, dir, RowIssueTemplates, ghOpts())
	var config struct {
		Blank    *bool `yaml:"blank_issues_enabled"`
		Contacts []struct {
			Name, URL, About string
		} `yaml:"contact_links"`
	}
	if err := yaml.Unmarshal([]byte(readFile(t, filepath.Join(dir, ".github", "ISSUE_TEMPLATE", "config.yml"))), &config); err != nil {
		t.Fatal(err)
	}
	if config.Blank == nil || *config.Blank || len(config.Contacts) != 1 ||
		config.Contacts[0].URL != "https://github.com/someone/demo/blob/HEAD/SUPPORT.md" {
		t.Errorf("config.yml = %+v", config)
	}

	noRemote := ghOpts()
	noRemote.WebURL = ""
	mustApply(t, dir, RowIssueTemplates, noRemote)
	if got := readFile(t, filepath.Join(dir, ".github", "ISSUE_TEMPLATE", "config.yml")); strings.Contains(got, "contact_links") {
		t.Errorf("config.yml without a remote has contact links: %q", got)
	}
}

func TestRows_EmptyRepository(t *testing.T) {
	dir := t.TempDir()
	rows, err := Rows(dir, ghOpts())
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, r := range rows {
		ids = append(ids, r.ID)
		want := StateMissing
		if r.ID == RowFunding {
			want = StateDisabled
		}
		if r.State != want {
			t.Errorf("%s: state %s, want %s", r.ID, r.State, want)
		}
	}
	if !slices.Equal(ids, rowOrder) {
		t.Errorf("row order %v, want %v", ids, rowOrder)
	}
}

func TestApply_EveryRowBecomesCurrent(t *testing.T) {
	for name, opts := range map[string]Options{"github": ghOpts(), "gitlab": glOpts()} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			for _, id := range rowOrder {
				if id == RowFunding {
					continue
				}
				mustApply(t, dir, id, opts)
			}
			for id, r := range rowsByID(t, dir, opts) {
				if id != RowFunding && r.State != StateCurrent {
					t.Errorf("%s: state %s after Apply, want current", id, r.State)
				}
				for _, rel := range r.Files {
					if id != RowFunding && !exists(filepath.Join(dir, rel)) {
						t.Errorf("%s: %s not written", id, rel)
					}
				}
			}
			if r := rowsByID(t, dir, opts)[RowLicense]; r.License != DefaultLicense {
				t.Errorf("LICENSE detected as %q, want %s", r.License, DefaultLicense)
			}
		})
	}
}

func TestProviderFiles(t *testing.T) {
	dir := t.TempDir()
	mustApply(t, dir, RowIssueTemplates, glOpts())
	mustApply(t, dir, RowPRTemplate, glOpts())
	for _, rel := range []string{".gitlab/issue_templates/Bug.md", ".gitlab/issue_templates/Feature.md", ".gitlab/merge_request_templates/Default.md"} {
		if !exists(filepath.Join(dir, rel)) {
			t.Errorf("%s not written", rel)
		}
	}
	if exists(filepath.Join(dir, ".github")) {
		t.Error("a GitLab account must not get .github/")
	}
	if got := readFile(t, filepath.Join(dir, ".gitlab/merge_request_templates/Default.md")); got != prTemplate {
		t.Error("the merge request template differs from the pull request template")
	}
}

func TestSupportLinks(t *testing.T) {
	for _, tc := range []struct {
		name   string
		opts   Options
		en, pt string
	}{
		{"github", ghOpts(), "[issue tracker](https://github.com/someone/demo/issues)", "[rastreador de issues](https://github.com/someone/demo/issues)"},
		{"gitlab", glOpts(), "[issue tracker](https://gitlab.com/someone/demo/-/issues)", "[rastreador de issues](https://gitlab.com/someone/demo/-/issues)"},
		{"no remote", func() Options { o := ghOpts(); o.WebURL = ""; return o }(), "Search the issue tracker,", "Procure no rastreador de issues,"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			mustApply(t, dir, RowSupport, tc.opts)
			en := readFile(t, filepath.Join(dir, "SUPPORT.md"))
			pt := readFile(t, filepath.Join(dir, "docs", "pt-br", "suporte.md"))
			if !strings.Contains(en, tc.en) || !strings.Contains(pt, tc.pt) {
				t.Errorf("links missing:\n%s\n%s", en, pt)
			}
			for _, text := range []string{en, pt} {
				if strings.Contains(text, "{{") || !strings.Contains(text, "contact@example.org") || !strings.Contains(text, " demo.") {
					t.Errorf("placeholders not filled: %.200q", text)
				}
			}
		})
	}
}

func TestApply_RejectsInvalidValues(t *testing.T) {
	for _, tc := range []struct {
		name string
		id   string
		edit func(*Options)
	}{
		{"empty e-mail", RowConduct, func(o *Options) { o.ContactEmail = " " }},
		{"e-mail without domain dot", RowConduct, func(o *Options) { o.ContactEmail = "a@b" }},
		{"e-mail with space", RowSupport, func(o *Options) { o.ContactEmail = "a b@c.d" }},
		{"e-mail with newline", RowSupport, func(o *Options) { o.ContactEmail = "a@b.c\nx" }},
		{"e-mail with markdown", RowSupport, func(o *Options) { o.ContactEmail = "[a](b)@c.d" }},
		{"empty project", RowSupport, func(o *Options) { o.Project = "" }},
		{"multiline project", RowSupport, func(o *Options) { o.Project = "a\nb" }},
		{"http URL", RowSupport, func(o *Options) { o.WebURL = "http://github.com/a/b" }},
		{"URL with parenthesis", RowIssueTemplates, func(o *Options) { o.WebURL = "https://github.com/a/b)x" }},
		{"unknown license", RowLicense, func(o *Options) { o.License = "WTFPL" }},
		{"MIT without holder", RowLicense, func(o *Options) { o.License = "MIT"; o.Year = 2026 }},
		{"unknown provider", RowPRTemplate, func(o *Options) { o.Provider = "bitbucket" }},
		{"unknown row", "readme", func(*Options) {}},
		{"funding", RowFunding, func(*Options) {}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			opts := ghOpts()
			tc.edit(&opts)
			if _, err := Apply(dir, tc.id, opts); err == nil {
				t.Fatal("want an error")
			}
			entries, _ := os.ReadDir(dir)
			if len(entries) != 0 {
				t.Errorf("a refused Apply wrote %d entries", len(entries))
			}
		})
	}
	if _, err := Apply(t.TempDir(), RowFunding, ghOpts()); !errors.Is(err, ErrDisabled) {
		t.Errorf("funding: got %v, want ErrDisabled", err)
	}
	if _, err := Apply(filepath.Join(t.TempDir(), "missing"), RowPRTemplate, ghOpts()); err == nil {
		t.Error("missing folder: want an error")
	}
}

func TestRows_Differs(t *testing.T) {
	opts := ghOpts()
	for _, tc := range []struct {
		name string
		id   string
		edit func(dir string)
	}{
		{"edited", RowConduct, func(dir string) { writeFile(t, filepath.Join(dir, "CODE_OF_CONDUCT.md"), "mine\n") }},
		{"mirror missing", RowSupport, func(dir string) { _ = os.Remove(filepath.Join(dir, "docs", "pt-br", "suporte.md")) }},
		{"one template missing", RowIssueTemplates, func(dir string) {
			_ = os.Remove(filepath.Join(dir, ".github", "ISSUE_TEMPLATE", "config.yml"))
		}},
		{"edited template", RowPRTemplate, func(dir string) {
			writeFile(t, filepath.Join(dir, ".github", "PULL_REQUEST_TEMPLATE.md"), prTemplate+"\nmore\n")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			mustApply(t, dir, tc.id, opts)
			tc.edit(dir)
			if got := rowsByID(t, dir, opts)[tc.id].State; got != StateDiffers {
				t.Errorf("state %s, want differs", got)
			}
		})
	}

	dir := t.TempDir()
	mustApply(t, dir, RowConduct, opts)
	other := opts
	other.ContactEmail = "other@example.org"
	if got := rowsByID(t, dir, other)[RowConduct].State; got != StateDiffers {
		t.Errorf("another e-mail: state %s, want differs", got)
	}
	noEmail := opts
	noEmail.ContactEmail = ""
	r := rowsByID(t, dir, noEmail)[RowConduct]
	if r.State != StatePresent || !r.NeedsEmail {
		t.Errorf("no e-mail: state %s needsEmail %v, want present true", r.State, r.NeedsEmail)
	}
}

func TestGitignoreRow(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module x\n")
	writeFile(t, filepath.Join(dir, ".gitignore"), "*.exe\n")
	r := rowsByID(t, dir, ghOpts())[RowGitignore]
	if r.State != StateDiffers || r.Missing == 0 {
		t.Fatalf("partial .gitignore: state %s missing %d", r.State, r.Missing)
	}
	msg, err := Apply(dir, RowGitignore, ghOpts())
	if err != nil || !strings.Contains(msg, "acrescentado") {
		t.Fatalf("Apply = %q, %v", msg, err)
	}
	if !strings.HasPrefix(readFile(t, filepath.Join(dir, ".gitignore")), "*.exe\n") {
		t.Error("existing patterns must be kept")
	}
	if r := rowsByID(t, dir, ghOpts())[RowGitignore]; r.State != StateCurrent || r.Missing != 0 {
		t.Errorf("after Apply: state %s missing %d", r.State, r.Missing)
	}
}

func TestLicenseRow(t *testing.T) {
	dir := t.TempDir()
	mit, _ := renderLicense("MIT", "Old Holder", 2001)
	writeFile(t, filepath.Join(dir, "LICENSE"), mit)
	writeFile(t, filepath.Join(dir, "COPYING"), "x")

	r := rowsByID(t, dir, ghOpts())[RowLicense]
	if r.State != StateDiffers || r.License != "MIT" {
		t.Errorf("MIT with GPL selected: state %s license %q", r.State, r.License)
	}
	if len(r.Notes) != 1 || !strings.Contains(r.Notes[0], "COPYING") {
		t.Errorf("notes %v, want COPYING", r.Notes)
	}

	opts := ghOpts()
	opts.License, opts.Holder, opts.Year = "MIT", "New Holder", 2026
	if r := rowsByID(t, dir, opts)[RowLicense]; r.State != StateCurrent {
		t.Errorf("MIT with another holder and year: state %s, want current", r.State)
	}

	writeFile(t, filepath.Join(dir, "LICENSE"), "Proprietary.\n")
	if r := rowsByID(t, dir, opts)[RowLicense]; r.State != StateDiffers || r.License != "" {
		t.Errorf("unknown license: state %s license %q", r.State, r.License)
	}

	bad := ghOpts()
	bad.License = "WTFPL"
	if _, err := Rows(dir, bad); err == nil {
		t.Error("unknown selected license: want an error")
	}
}

func TestConductLegacyNote(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, legacyConductPT), "old")
	mustApply(t, dir, RowConduct, ghOpts())
	if r := rowsByID(t, dir, ghOpts())[RowConduct]; len(r.Notes) != 1 {
		t.Errorf("notes %v, want the legacy file note", r.Notes)
	}
	if readFile(t, filepath.Join(dir, legacyConductPT)) != "old" {
		t.Error("the legacy file must be left untouched")
	}
}

// Overwriting keeps the previous version as .perci-bak and writes through
// a symlink instead of replacing it.
func TestApply_BackupAndSymlink(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "CODE_OF_CONDUCT.md"), "my own code\n")
	shared := filepath.Join(t.TempDir(), "shared-pt.md")
	writeFile(t, shared, "shared\n")
	link := filepath.Join(dir, "docs", "pt-br", "codigo-de-conduta.md")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(shared, link); err != nil {
		t.Fatal(err)
	}

	mustApply(t, dir, RowConduct, ghOpts())
	if bak := readFile(t, filepath.Join(dir, "CODE_OF_CONDUCT.md.perci-bak")); bak != "my own code\n" {
		t.Errorf("backup = %q", bak)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("the symlink was replaced (err=%v)", err)
	}
	if !strings.Contains(readFile(t, shared), "**contact@example.org**") {
		t.Error("the link target was not written")
	}
}

func TestRemove(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".github", "workflows", "ci.yml"), "on: push\n")
	writeFile(t, filepath.Join(dir, "docs", "pt-br", "leiame.md"), "x")
	opts := ghOpts()
	for _, id := range []string{RowIssueTemplates, RowPRTemplate, RowSupport, RowConduct} {
		mustApply(t, dir, id, opts)
		if err := Remove(dir, id, opts); err != nil {
			t.Fatalf("Remove(%s): %v", id, err)
		}
		if got := rowsByID(t, dir, opts)[id].State; got != StateMissing {
			t.Errorf("%s: state %s after Remove, want missing", id, got)
		}
	}
	if exists(filepath.Join(dir, ".github", "ISSUE_TEMPLATE")) {
		t.Error("the empty ISSUE_TEMPLATE folder must be removed")
	}
	if !exists(filepath.Join(dir, ".github", "workflows", "ci.yml")) {
		t.Error(".github/ with other files must be kept")
	}
	if !exists(filepath.Join(dir, "docs", "pt-br", "leiame.md")) {
		t.Error("docs/pt-br must be kept")
	}

	gl := t.TempDir()
	mustApply(t, gl, RowIssueTemplates, glOpts())
	if err := Remove(gl, RowIssueTemplates, glOpts()); err != nil {
		t.Fatal(err)
	}
	if exists(filepath.Join(gl, ".gitlab")) {
		t.Error("the empty .gitlab/ folder must be removed")
	}

	empty := t.TempDir()
	if err := Remove(empty, RowLicense, opts); err != nil {
		t.Errorf("removing a missing file: %v", err)
	}
	if err := Remove(empty, RowFunding, opts); !errors.Is(err, ErrDisabled) {
		t.Errorf("funding: got %v, want ErrDisabled", err)
	}
	if err := Remove(empty, "readme", opts); !errors.Is(err, ErrUnknownRow) {
		t.Errorf("unknown row: got %v, want ErrUnknownRow", err)
	}
}

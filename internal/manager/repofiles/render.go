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
	_ "embed"
	"fmt"
	"strings"
)

// Templates of the generated files. The texts carry no fixed address or
// URL: these files are generated into other people's projects.
var (
	//go:embed templates/CODE_OF_CONDUCT.en.md
	conductEN string
	//go:embed templates/CODE_OF_CONDUCT.pt.md
	conductPT string
	//go:embed templates/SUPPORT.en.md
	supportEN string
	//go:embed templates/SUPPORT.pt.md
	supportPT string
	//go:embed templates/github/bug_report.yml
	githubBug string
	//go:embed templates/github/feature_request.yml
	githubFeature string
	//go:embed templates/github/config.yml
	githubConfig string
	//go:embed templates/github/config.noremote.yml
	githubConfigNoRemote string
	//go:embed templates/gitlab/Bug.md
	gitlabBug string
	//go:embed templates/gitlab/Feature.md
	gitlabFeature string
	//go:embed templates/PULL_REQUEST_TEMPLATE.md
	prTemplate string
)

// render returns the files Apply writes for row id (every row except
// .gitignore and FUNDING.yml).
func render(id string, opts Options) ([]file, error) {
	gitlab := opts.Provider == ProviderGitLab
	files := paths(id, opts)
	switch id {
	case RowLicense:
		license := opts.License
		if license == "" {
			license = DefaultLicense
		}
		text, err := renderLicense(license, opts.Holder, opts.Year)
		if err != nil {
			return nil, err
		}
		return []file{{files[0], text}}, nil

	case RowConduct:
		email, err := contactEmail(opts)
		if err != nil {
			return nil, err
		}
		return []file{
			{files[0], strings.ReplaceAll(conductEN, phContact, email)},
			{files[1], strings.ReplaceAll(conductPT, phContact, email)},
		}, nil

	case RowSupport:
		email, err := contactEmail(opts)
		if err != nil {
			return nil, err
		}
		project := strings.TrimSpace(opts.Project)
		if project == "" || strings.ContainsAny(project, "\r\n") {
			return nil, fmt.Errorf("nome do projeto inválido: %q", opts.Project)
		}
		issuesURL, _, err := webLinks(opts)
		if err != nil {
			return nil, err
		}
		issuesEN, issuesPT := "issue tracker", "rastreador de issues"
		if issuesURL != "" {
			issuesEN = "[issue tracker](" + issuesURL + ")"
			issuesPT = "[rastreador de issues](" + issuesURL + ")"
		}
		fill := func(tpl, issues string) string {
			return strings.NewReplacer(phProject, project, phIssues, issues, phContact, email).Replace(tpl)
		}
		return []file{
			{files[0], fill(supportEN, issuesEN)},
			{files[1], fill(supportPT, issuesPT)},
		}, nil

	case RowIssueTemplates:
		if gitlab {
			return []file{{files[0], gitlabBug}, {files[1], gitlabFeature}}, nil
		}
		_, supportURL, err := webLinks(opts)
		if err != nil {
			return nil, err
		}
		config := githubConfigNoRemote
		if supportURL != "" {
			config = strings.ReplaceAll(githubConfig, phSupportURL, supportURL)
		}
		return []file{{files[0], githubBug}, {files[1], githubFeature}, {files[2], config}}, nil

	case RowPRTemplate:
		return []file{{files[0], prTemplate}}, nil
	}
	return nil, fmt.Errorf("%w: %q", ErrUnknownRow, id)
}

// contactEmail returns the trimmed contact e-mail of opts, or an error
// when it is not a plain address.
func contactEmail(opts Options) (string, error) {
	return CheckContactEmail(opts.ContactEmail)
}

// CheckContactEmail returns email trimmed, or an error when it is not a
// plain address (letters, digits and ._%+- before the @, a dotted domain
// after it).
func CheckContactEmail(email string) (string, error) {
	trimmed := strings.TrimSpace(email)
	if !validContactEmail.MatchString(trimmed) {
		return "", fmt.Errorf("informe um e-mail de contato válido (recebido: %q)", email)
	}
	return trimmed, nil
}

// webLinks returns the issues page and the SUPPORT.md page of the
// repository at opts.WebURL, in the provider's URL layout, or two empty
// strings when WebURL is empty.
func webLinks(opts Options) (issues, support string, err error) {
	base := strings.TrimSuffix(strings.TrimSpace(opts.WebURL), "/")
	if base == "" {
		return "", "", nil
	}
	if !validWebURL.MatchString(base) {
		return "", "", fmt.Errorf("endereço do repositório inválido: %q", opts.WebURL)
	}
	if opts.Provider == ProviderGitLab {
		return base + "/-/issues", base + "/-/blob/HEAD/SUPPORT.md", nil
	}
	return base + "/issues", base + "/blob/HEAD/SUPPORT.md", nil
}

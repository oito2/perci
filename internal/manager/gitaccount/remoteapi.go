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

package gitaccount

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/oito2/perci/internal/config"
)

// RemoteRepo is one repository of the account on its provider.
type RemoteRepo struct {
	Name        string `json:"name"` // path under the account user ("perci", "group/app")
	Private     bool   `json:"private"`
	Description string `json:"description"`
	UpdatedAt   string `json:"updatedAt"` // RFC 3339
	SSHURL      string `json:"sshUrl"`
	// LocalPath is where the repository is already cloned under the
	// account folder ("" when it is not); set by MarkCloned.
	LocalPath string `json:"localPath,omitempty"`
}

// MarkCloned sets LocalPath on each of repos that one of local (the
// account's listed repositories) has as origin.
func MarkCloned(acc config.GitAccount, repos []RemoteRepo, local []RepoState) {
	paths := map[string]string{}
	for _, st := range local {
		if r, ok := ParseRemote(st.Remote); ok && st.RemoteClass != RemoteNone {
			paths[strings.ToLower(r.Owner+"/"+r.Repo)] = st.Path
		}
	}
	for i := range repos {
		repos[i].LocalPath = paths[strings.ToLower(acc.Username+"/"+repos[i].Name)]
	}
}

// APIBase is the REST API root of acc's provider: api.github.com or
// <host>/api/v3 for GitHub, <host>/api/v4 for GitLab.
func APIBase(acc config.GitAccount) string {
	switch {
	case acc.Provider == ProviderGitHub && acc.Host == "github.com":
		return "https://api.github.com"
	case acc.Provider == ProviderGitHub:
		return "https://" + acc.Host + "/api/v3"
	}
	return "https://" + acc.Host + "/api/v4"
}

// API paging: perPage items per request, at most maxPages requests.
const (
	perPage    = 100
	maxPages   = 10
	apiTimeout = 20 * time.Second
)

// ListRemoteRepos lists the repositories owned by acc's user at the API
// rooted at base. With a token, the list includes the private ones;
// without, only the public ones. The token goes only in a request header.
func ListRemoteRepos(ctx context.Context, base string, acc config.GitAccount, token string) ([]RemoteRepo, error) {
	client := &http.Client{Timeout: apiTimeout}
	var repos []RemoteRepo
	for page := 1; page <= maxPages; page++ {
		got, err := fetchRepoPage(ctx, client, base, acc, token, page)
		if err != nil {
			return nil, err
		}
		repos = append(repos, got.repos...)
		if got.count < perPage {
			break
		}
	}
	slices.SortFunc(repos, func(a, b RemoteRepo) int { return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)) })
	return repos, nil
}

type repoPage struct {
	repos []RemoteRepo
	count int // items in the response, before filtering
}

func fetchRepoPage(ctx context.Context, client *http.Client, base string, acc config.GitAccount, token string, page int) (repoPage, error) {
	q := url.Values{"per_page": {strconv.Itoa(perPage)}, "page": {strconv.Itoa(page)}}
	var path string
	switch {
	case acc.Provider == ProviderGitHub && token != "":
		path = "/user/repos"
		q.Set("affiliation", "owner")
	case acc.Provider == ProviderGitHub:
		path = "/users/" + url.PathEscape(acc.Username) + "/repos"
		q.Set("type", "owner")
	case token != "":
		path = "/projects"
		q.Set("owned", "true")
	default:
		path = "/users/" + url.PathEscape(acc.Username) + "/projects"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+path+"?"+q.Encode(), nil)
	if err != nil {
		return repoPage{}, err
	}
	req.Header.Set("Accept", "application/json")
	if token != "" {
		if acc.Provider == ProviderGitHub {
			req.Header.Set("Authorization", "Bearer "+token)
		} else {
			req.Header.Set("PRIVATE-TOKEN", token)
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return repoPage{}, fmt.Errorf("consultar %s: %w", acc.Host, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return repoPage{}, fmt.Errorf("ler a resposta de %s: %w", acc.Host, err)
	}
	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return repoPage{}, fmt.Errorf("%s recusou o token da conta — gere um novo e salve em Configurar Conta", acc.Host)
	case resp.StatusCode == http.StatusNotFound:
		return repoPage{}, fmt.Errorf("usuário %s não encontrado em %s", acc.Username, acc.Host)
	case resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests:
		return repoPage{}, fmt.Errorf("%s recusou a consulta (limite de requisições?) — tente mais tarde ou salve um token", acc.Host)
	case resp.StatusCode != http.StatusOK:
		return repoPage{}, fmt.Errorf("%s respondeu %s", acc.Host, resp.Status)
	}

	if acc.Provider == ProviderGitHub {
		return decodeGitHub(body, acc)
	}
	return decodeGitLab(body, acc)
}

func decodeGitHub(body []byte, acc config.GitAccount) (repoPage, error) {
	var items []struct {
		Name        string `json:"name"`
		Private     bool   `json:"private"`
		Description string `json:"description"`
		PushedAt    string `json:"pushed_at"`
		Owner       struct {
			Login string `json:"login"`
		} `json:"owner"`
	}
	if err := json.Unmarshal(body, &items); err != nil {
		return repoPage{}, fmt.Errorf("resposta inesperada de %s: %w", acc.Host, err)
	}
	page := repoPage{count: len(items)}
	for _, it := range items {
		if !strings.EqualFold(it.Owner.Login, acc.Username) {
			continue
		}
		page.repos = append(page.repos, RemoteRepo{
			Name: it.Name, Private: it.Private, Description: it.Description, UpdatedAt: it.PushedAt,
			SSHURL: SSHURL(acc, acc.Username, it.Name),
		})
	}
	return page, nil
}

func decodeGitLab(body []byte, acc config.GitAccount) (repoPage, error) {
	var items []struct {
		PathWithNamespace string `json:"path_with_namespace"`
		Visibility        string `json:"visibility"`
		Description       string `json:"description"`
		LastActivityAt    string `json:"last_activity_at"`
	}
	if err := json.Unmarshal(body, &items); err != nil {
		return repoPage{}, fmt.Errorf("resposta inesperada de %s: %w", acc.Host, err)
	}
	page := repoPage{count: len(items)}
	for _, it := range items {
		owner, name, ok := strings.Cut(it.PathWithNamespace, "/")
		if !ok || !strings.EqualFold(owner, acc.Username) {
			continue
		}
		page.repos = append(page.repos, RemoteRepo{
			Name: name, Private: it.Visibility != "public", Description: it.Description, UpdatedAt: it.LastActivityAt,
			SSHURL: SSHURL(acc, acc.Username, name),
		})
	}
	return page, nil
}

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
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/oito2/perci/internal/config"
	"github.com/oito2/perci/internal/executor"
)

// skippedDirs are folders Discover never descends into.
var skippedDirs = []string{"node_modules", "vendor"}

// Discover returns the absolute paths of the Git repositories under root,
// root itself included, sorted. It never descends into a repository, a
// hidden folder, node_modules or vendor, and does not follow symlinks.
// Folders it can't read are skipped.
func Discover(ctx context.Context, root string) ([]string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("pasta da conta: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s não é uma pasta", root)
	}

	var repos []string
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if err != nil {
			if d != nil && d.IsDir() && path != root {
				return fs.SkipDir
			}
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		if path != root && (strings.HasPrefix(d.Name(), ".") || slices.Contains(skippedDirs, d.Name())) {
			return fs.SkipDir
		}
		if _, err := os.Lstat(filepath.Join(path, ".git")); err == nil {
			repos = append(repos, path)
			return fs.SkipDir
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.Sort(repos)
	return repos, nil
}

// Remote is a parsed Git remote URL. Owner is the first path segment;
// Repo is the rest, without ".git".
type Remote struct {
	SSH   bool // ssh:// or scp-like "user@host:path"; false for http(s)
	Host  string
	Port  int // 0 when the URL names none
	Owner string
	Repo  string
}

// scpLike matches "user@host:path", the short SSH form.
var scpLike = regexp.MustCompile(`^(?:[^@/:]+@)?([^@/:]+):([^/].*)$`)

// ParseRemote parses an SSH (ssh://, scp-like) or HTTP(S) remote URL. It
// reports false for anything else (local paths, file://, git://) and for
// a URL without owner and repository.
func ParseRemote(raw string) (Remote, bool) {
	raw = strings.TrimSpace(raw)
	var r Remote
	var path string
	if u, err := url.Parse(raw); err == nil && u.Scheme != "" && u.Host != "" {
		switch u.Scheme {
		case "ssh", "git+ssh", "ssh+git":
			r.SSH = true
		case "http", "https":
		default:
			return Remote{}, false
		}
		r.Host = strings.ToLower(u.Hostname())
		if p := u.Port(); p != "" {
			r.Port, _ = strconv.Atoi(p)
		}
		path = u.Path
	} else if m := scpLike.FindStringSubmatch(raw); m != nil && !strings.Contains(raw, "://") {
		r.SSH = true
		r.Host = strings.ToLower(m[1])
		path = m[2]
	} else {
		return Remote{}, false
	}

	path = strings.TrimSuffix(strings.Trim(path, "/"), ".git")
	owner, repo, ok := strings.Cut(path, "/")
	if !ok || owner == "" || repo == "" {
		return Remote{}, false
	}
	r.Owner, r.Repo = owner, repo
	return r, true
}

// RedactURL removes the password (a token, usually) from the user info
// of an HTTP(S) URL; any other value is returned unchanged.
func RedactURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.User == nil || (u.Scheme != "http" && u.Scheme != "https") {
		return raw
	}
	if _, has := u.User.Password(); has {
		u.User = url.User(u.User.Username())
	}
	return u.String()
}

// Classes of a repository's origin remote for an account.
const (
	RemoteOK      = "ok"      // SSH, the account's host and user
	RemoteHTTPS   = "https"   // HTTP(S), the account's host and user
	RemoteNone    = "none"    // no origin remote
	RemoteForeign = "foreign" // another owner, another host or an unknown form
)

// ClassifyRemote classifies the origin URL raw ("" when there is none)
// of a repository under acc's folder.
func ClassifyRemote(acc config.GitAccount, raw string) string {
	if strings.TrimSpace(raw) == "" {
		return RemoteNone
	}
	r, ok := ParseRemote(raw)
	if !ok || r.Host != strings.ToLower(acc.Host) || !strings.EqualFold(r.Owner, acc.Username) {
		return RemoteForeign
	}
	if r.SSH {
		return RemoteOK
	}
	return RemoteHTTPS
}

// RepoState is the local state of one repository: no network access is
// involved, so Ahead/Behind count against the last fetched upstream.
type RepoState struct {
	Dir         string `json:"dir"`
	Path        string `json:"path"` // relative to the account folder ("." for the folder itself)
	Branch      string `json:"branch"`
	Detached    bool   `json:"detached"`
	Upstream    string `json:"upstream"`
	Ahead       int    `json:"ahead"`
	Behind      int    `json:"behind"`
	Modified    int    `json:"modified"` // changed or conflicted tracked files
	Untracked   int    `json:"untracked"`
	LastCommit  string `json:"lastCommit"` // RFC 3339; "" without commits
	LastSubject string `json:"lastSubject"`
	Remote      string `json:"remote"` // origin URL, without any embedded password
	RemoteClass string `json:"remoteClass"`
	// SSHURL is the SSH URL that replaces an HTTP(S) origin (RemoteHTTPS
	// only).
	SSHURL     string `json:"sshUrl,omitempty"`
	LocalName  string `json:"localName"`
	LocalEmail string `json:"localEmail"`
	// IdentityDiverges is set when the repository's own config sets a
	// user.name or user.email other than the account's.
	IdentityDiverges bool   `json:"identityDiverges"`
	LastFetch        string `json:"lastFetch"` // RFC 3339 time of the last fetch; "" if never
	Error            string `json:"error,omitempty"`
}

// AccountRepos is the result of ListRepos: the repositories shown for the
// account, those under its folder that belong to someone else, and the
// pending ones not cloned yet.
type AccountRepos struct {
	Repos   []RepoState    `json:"repos"`
	Hidden  []RepoState    `json:"hidden"`
	Pending []PendingState `json:"pending"`
}

// PendingState is a pending repository of the account; Reason says why
// it can't be cloned into its folder (empty when it can).
type PendingState struct {
	Path   string `json:"path"`
	URL    string `json:"url"`
	Reason string `json:"reason"`
}

// readConcurrency bounds the repositories read at the same time.
const readConcurrency = 8

// ListRepos discovers the repositories under acc's folder and reads their
// local state. Repositories whose origin belongs to another owner or host
// go to Hidden.
func ListRepos(ctx context.Context, exe *executor.Executor, acc config.GitAccount) (AccountRepos, error) {
	dirs, err := Discover(ctx, acc.Folder)
	if err != nil {
		return AccountRepos{}, err
	}
	states := make([]RepoState, len(dirs))
	idx := make([]int, len(dirs))
	for i := range idx {
		idx[i] = i
	}
	var mu sync.Mutex
	err = executor.RunConcurrent(ctx, idx, readConcurrency, func(i int) {
		st := ReadRepo(ctx, exe, acc, dirs[i])
		mu.Lock()
		states[i] = st
		mu.Unlock()
	})
	if err != nil {
		return AccountRepos{}, err
	}

	res := AccountRepos{Repos: []RepoState{}, Hidden: []RepoState{}, Pending: []PendingState{}}
	for _, st := range states {
		if st.RemoteClass == RemoteForeign {
			res.Hidden = append(res.Hidden, st)
		} else {
			res.Repos = append(res.Repos, st)
		}
	}
	for _, p := range acc.Pending {
		dest := filepath.Join(acc.Folder, filepath.FromSlash(p.Path))
		if slices.Contains(dirs, dest) {
			continue // cloned since; ClonePending drops it from the list
		}
		ps := PendingState{Path: p.Path, URL: p.URL}
		if _, err := DestDir(acc, p.Path); err != nil {
			ps.Reason = err.Error()
		}
		res.Pending = append(res.Pending, ps)
	}
	return res, nil
}

// ReadRepo reads the local state of the repository at dir, under acc's
// folder. A failure to read it is reported in Error.
func ReadRepo(ctx context.Context, exe *executor.Executor, acc config.GitAccount, dir string) RepoState {
	st := RepoState{Dir: dir, Path: dir}
	if rel, err := filepath.Rel(acc.Folder, dir); err == nil {
		st.Path = rel
	}

	local, err := localConfig(ctx, exe, dir)
	if err != nil {
		st.Error = err.Error()
		st.RemoteClass = RemoteNone
		return st
	}
	st.Remote = RedactURL(local["remote.origin.url"])
	st.RemoteClass = ClassifyRemote(acc, local["remote.origin.url"])
	if st.RemoteClass == RemoteHTTPS {
		r, _ := ParseRemote(local["remote.origin.url"])
		st.SSHURL = SSHURL(acc, r.Owner, r.Repo)
	}
	st.LocalName, st.LocalEmail = local["user.name"], local["user.email"]
	st.IdentityDiverges = (st.LocalName != "" && st.LocalName != acc.GitName) ||
		(st.LocalEmail != "" && !strings.EqualFold(st.LocalEmail, acc.GitEmail))

	out, err := exe.Output(ctx, executor.Options{}, "git", "--no-optional-locks", "-C", dir,
		"status", "--porcelain=v2", "--branch")
	if err != nil {
		st.Error = fmt.Sprintf("git status: %v", err)
		return st
	}
	parseStatus(out, &st)

	if out, err := exe.Output(ctx, executor.Options{}, "git", "-C", dir, "log", "-1", "--format=%cI%x00%s"); err == nil {
		date, subject, _ := strings.Cut(strings.TrimRight(out, "\n"), "\x00")
		st.LastCommit, st.LastSubject = strings.TrimSpace(date), subject
	}
	if info, err := os.Stat(filepath.Join(dir, ".git", "FETCH_HEAD")); err == nil {
		st.LastFetch = info.ModTime().Format(time.RFC3339)
	}
	return st
}

// localConfig returns the origin URL and the user name/e-mail set in the
// repository's own config (.git/config), keyed by their config names.
func localConfig(ctx context.Context, exe *executor.Executor, dir string) (map[string]string, error) {
	out, err := exe.Output(ctx, executor.Options{}, "git", "-C", dir, "config", "--local", "--get-regexp",
		`^(remote\.origin\.url|user\.name|user\.email)$`)
	values := map[string]string{}
	if exitCode(err) == gitConfigKeyMissing {
		return values, nil
	}
	if err != nil {
		return nil, fmt.Errorf("git config: %w", err)
	}
	for _, line := range strings.Split(out, "\n") {
		if key, value, ok := strings.Cut(line, " "); ok {
			values[key] = strings.TrimSpace(value)
		}
	}
	return values, nil
}

// parseStatus fills st from `git status --porcelain=v2 --branch` output.
func parseStatus(out string, st *RepoState) {
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "# branch.head "):
			head := strings.TrimPrefix(line, "# branch.head ")
			if head == "(detached)" {
				st.Detached = true
			} else {
				st.Branch = head
			}
		case strings.HasPrefix(line, "# branch.upstream "):
			st.Upstream = strings.TrimPrefix(line, "# branch.upstream ")
		case strings.HasPrefix(line, "# branch.ab "):
			for _, f := range strings.Fields(strings.TrimPrefix(line, "# branch.ab ")) {
				n, err := strconv.Atoi(f[1:])
				if err != nil {
					continue
				}
				if f[0] == '+' {
					st.Ahead = n
				} else {
					st.Behind = n
				}
			}
		case strings.HasPrefix(line, "1 "), strings.HasPrefix(line, "2 "), strings.HasPrefix(line, "u "):
			st.Modified++
		case strings.HasPrefix(line, "? "):
			st.Untracked++
		}
	}
}

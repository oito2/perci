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

package ai

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/oito2/perci/internal/config"
)

// StatusPending is the state of a registered folder that does not exist
// on this machine (ContextState).
const StatusPending = "pending"

// ErrContextNotFound is returned when no folder is registered at a path.
var ErrContextNotFound = errors.New("pasta não registrada em IA: Contextos")

// maxContextNameLen caps the length of a context name, in characters.
const maxContextNameLen = 80

// ModelsNamed returns the catalog models named by names, in catalog order;
// unknown names are left out.
func ModelsNamed(names []string) []Model {
	var out []Model
	for _, m := range models {
		if slices.Contains(names, m.Name) {
			out = append(out, m)
		}
	}
	return out
}

// modelNames returns the names of ms.
func modelNames(ms []Model) []string {
	names := make([]string, len(ms))
	for i, m := range ms {
		names[i] = m.Name
	}
	return names
}

// agentIDsIn returns the IDs of the catalog agents set holds, in catalog
// order.
func agentIDsIn(set map[string]bool) []string {
	var ids []string
	for _, a := range agents {
		if set[a.ID] {
			ids = append(ids, a.ID)
		}
	}
	return ids
}

// Detected returns the standards and agents of the context in dir, read
// from its files (DetectActiveModels, DetectAgents), in catalog order.
func Detected(dir string) (modelNamesOut, agentIDs []string) {
	present := DetectActiveModels(dir)
	for _, m := range models {
		if present[m.Name] {
			modelNamesOut = append(modelNamesOut, m.Name)
		}
	}
	return modelNamesOut, agentIDsIn(DetectAgents(dir))
}

// HasContext reports whether dir holds a context Perci generated: an
// AGENTS.md naming its agents, or an AGENTS.md next to a standard of the
// catalog under .instructions/.
func HasContext(dir string) bool {
	data, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		return false
	}
	return agentsMarkerRe.Match(data) || len(DetectActiveModels(dir)) > 0
}

// normalizeName trims name and checks it is set, not too long and not used
// by a record of contexts other than the one at path (names are compared
// ignoring case).
func normalizeName(contexts []config.AIContext, name, path string) (string, error) {
	name = strings.TrimSpace(name)
	switch {
	case name == "":
		return "", errors.New("informe o nome do contexto")
	case len([]rune(name)) > maxContextNameLen:
		return "", fmt.Errorf("o nome do contexto tem mais de %d caracteres", maxContextNameLen)
	case strings.ContainsAny(name, "\n\r\t"):
		return "", errors.New("o nome do contexto não pode ter quebras de linha nem tabulações")
	}
	for _, c := range contexts {
		if c.Path != path && strings.EqualFold(c.Name, name) {
			return "", fmt.Errorf("o nome %q já é usado pela pasta %s", name, c.Path)
		}
	}
	return name, nil
}

// cleanContextPath checks path is absolute and returns it cleaned.
func cleanContextPath(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("caminho não absoluto: %q", path)
	}
	return filepath.Clean(path), nil
}

// ValidateContextName checks name could be given to the folder at path:
// set, at most maxContextNameLen characters, on one line, and not used by
// another registered folder. It returns the name trimmed.
func ValidateContextName(name, path string) (string, error) {
	path, err := cleanContextPath(path)
	if err != nil {
		return "", err
	}
	cfg, err := config.Load()
	if err != nil {
		return "", err
	}
	return normalizeName(cfg.AIContexts, name, path)
}

// Contexts returns the registered folders, in registration order.
func Contexts() ([]config.AIContext, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	return cfg.AIContexts, nil
}

// FindContext returns the record of the folder at path.
func FindContext(path string) (config.AIContext, error) {
	path, err := cleanContextPath(path)
	if err != nil {
		return config.AIContext{}, err
	}
	all, err := Contexts()
	if err != nil {
		return config.AIContext{}, err
	}
	for _, c := range all {
		if c.Path == path {
			return c, nil
		}
	}
	return config.AIContext{}, ErrContextNotFound
}

// RecordContext registers the folder at path as name with the given
// standards and agents, or updates its record: the standards a selected
// complement requires are added, unknown standards are left out, both
// lists are kept in catalog order, and at least one known agent is
// required.
func RecordContext(name, path string, modelNamesIn, agentIDs []string) error {
	path, err := cleanContextPath(path)
	if err != nil {
		return err
	}
	set, err := agentSet(agentIDs)
	if err != nil {
		return err
	}
	resolved, _ := withRequired(ModelsNamed(modelNamesIn))
	rec := config.AIContext{Path: path, Models: modelNames(resolved), Agents: agentIDsIn(set)}
	return config.Update(func(cfg *config.Config) error {
		if rec.Name, err = normalizeName(cfg.AIContexts, name, path); err != nil {
			return err
		}
		for i, c := range cfg.AIContexts {
			if c.Path == path {
				cfg.AIContexts[i] = rec
				return nil
			}
		}
		cfg.AIContexts = append(cfg.AIContexts, rec)
		return nil
	})
}

// RegisterDetected registers the folder at path as name with the
// standards and agents its files hold (Detected), without writing any of
// them. It refuses a folder without a context generated by Perci.
func RegisterDetected(name, path string) error {
	if !HasContext(path) {
		return errors.New("a pasta não tem um contexto gerado pelo Perci")
	}
	ms, ids := Detected(path)
	return RecordContext(name, path, ms, ids)
}

// ForgetContext drops the record of the folder at path; the folder's files
// are not touched.
func ForgetContext(path string) error {
	path, err := cleanContextPath(path)
	if err != nil {
		return err
	}
	return config.Update(func(cfg *config.Config) error {
		i := slices.IndexFunc(cfg.AIContexts, func(c config.AIContext) bool { return c.Path == path })
		if i < 0 {
			return ErrContextNotFound
		}
		cfg.AIContexts = slices.Delete(cfg.AIContexts, i, i+1)
		return nil
	})
}

// ContextStatus is the state of a registered folder: State is
// StatusPending (the folder does not exist), StatusEdited (AGENTS.md has
// rules written by hand or a recorded standard was edited), StatusOutdated
// (AGENTS.md or a recorded standard differs from what this version writes
// for the record, or is missing) or StatusApplied. Divergent says the
// standards or agents the files hold are not the recorded ones.
type ContextStatus struct {
	State     string
	Divergent bool
}

// ContextState reads the state of rec's folder from its files; nothing is
// written.
func ContextState(rec config.AIContext) (ContextStatus, error) {
	info, err := os.Stat(rec.Path)
	if err != nil || !info.IsDir() {
		return ContextStatus{State: StatusPending}, nil
	}
	active, _ := withRequired(ModelsNamed(rec.Models))
	set := map[string]bool{}
	for _, id := range rec.Agents {
		set[id] = true
	}

	st := ContextStatus{State: StatusApplied}
	worse := func(s string) {
		if s == StatusEdited || st.State == StatusApplied {
			st.State = s
		}
	}

	agentsMD, err := os.ReadFile(filepath.Join(rec.Path, "AGENTS.md"))
	if err != nil {
		worse(StatusOutdated)
	} else {
		own, err := projectContent(string(agentsMD))
		if err != nil {
			return ContextStatus{}, err
		}
		want, err := agentsFile(active, set)
		if err != nil {
			return ContextStatus{}, err
		}
		switch {
		case own != "":
			worse(StatusEdited)
		case string(agentsMD) != want:
			worse(StatusOutdated)
		}
	}
	for _, m := range active {
		switch GuideStatus(rec.Path, m) {
		case StatusEdited:
			worse(StatusEdited)
		case StatusOutdated, "":
			worse(StatusOutdated)
		}
	}

	ms, ids := Detected(rec.Path)
	st.Divergent = !slices.Equal(ms, modelNames(active)) || !slices.Equal(ids, agentIDsIn(set))
	return st, nil
}

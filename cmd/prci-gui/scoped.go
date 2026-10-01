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

package main

import (
	"path/filepath"
	"slices"

	"github.com/oito2/perci/internal/config"
)

// oneOf returns name when it is one of valid, def otherwise — every
// "pick one of a fixed list" setter falls back to the default this way.
func oneOf(name string, valid []string, def string) string {
	if slices.Contains(valid, name) {
		return name
	}
	return def
}

// scopeKey is the (slug, global, folder) identity of an install record
// (config.AgentSkillInstall, config.MCPServerInstall).
type scopeKey struct {
	slug   string
	global bool
	folder string
}

// matches reports whether k is the record for (slug, global, folder):
// the folder only counts for local records, compared after
// filepath.Clean so "/x" and "/x/" are the same project.
func (k scopeKey) matches(slug string, global bool, folder string) bool {
	if k.slug != slug || k.global != global {
		return false
	}
	return global || cleanFolder(k.folder) == cleanFolder(folder)
}

func cleanFolder(f string) string {
	if f == "" {
		return ""
	}
	return filepath.Clean(f)
}

// indexOfScoped returns the index of recs' record for (slug, global,
// folder), or -1.
func indexOfScoped[T any](recs []T, key func(T) scopeKey, slug string, global bool, folder string) int {
	return slices.IndexFunc(recs, func(r T) bool { return key(r).matches(slug, global, folder) })
}

// withoutScoped returns a copy of recs without the record for (slug,
// global, folder) — recs itself is left untouched.
func withoutScoped[T any](recs []T, key func(T) scopeKey, slug string, global bool, folder string) []T {
	out := make([]T, 0, len(recs)+1)
	for _, r := range recs {
		if !key(r).matches(slug, global, folder) {
			out = append(out, r)
		}
	}
	return out
}

func agentSkillKey(r config.AgentSkillInstall) scopeKey {
	return scopeKey{r.Slug, r.Global, r.Folder}
}

func mcpServerKey(r config.MCPServerInstall) scopeKey {
	return scopeKey{r.Slug, r.Global, r.Folder}
}

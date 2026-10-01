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
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	htmlComment = regexp.MustCompile(`(?s)<!--.*?-->`)
	scriptTag   = regexp.MustCompile(`(?s)<script([^>]*)>(.*?)</script>`)
	cspMeta     = regexp.MustCompile(`<meta http-equiv="Content-Security-Policy" content="([^"]*)"`)
	scriptSrc   = regexp.MustCompile(`<script[^>]+src="\./([^"]+)"`)
	remoteRef   = regexp.MustCompile(`(?i)<(?:script|link)[^>]+(?:src|href)="(?:https?:)?//`)
)

func readIndexHTML(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("frontend/dist/index.html")
	if err != nil {
		t.Fatal(err)
	}
	// Comments mention "<script>" in prose; only real markup counts.
	return htmlComment.ReplaceAllString(string(data), "")
}

// The CSP allows scripts only from the app's own origin — no inline code,
// no 'unsafe-inline'. The app's JavaScript lives in frontend/dist/js/.
func TestCSP_NoInlineScripts(t *testing.T) {
	html := readIndexHTML(t)
	m := cspMeta.FindStringSubmatch(html)
	if m == nil {
		t.Fatal("index.html has no Content-Security-Policy <meta>")
	}
	scriptSrcDirective := ""
	for _, d := range strings.Split(m[1], ";") {
		if d = strings.TrimSpace(d); strings.HasPrefix(d, "script-src ") {
			scriptSrcDirective = d
		}
	}
	if scriptSrcDirective != "script-src 'self'" {
		t.Errorf("script-src must be exactly 'self', got %q", scriptSrcDirective)
	}
	for _, s := range scriptTag.FindAllStringSubmatch(html, -1) {
		if !strings.Contains(s[1], "src=") && strings.TrimSpace(s[2]) != "" {
			t.Errorf("inline <script> found (%d bytes) — move it to frontend/dist/js/", len(s[2]))
		}
	}
}

// Every local <script src> must exist (a typo would silently drop a screen).
func TestIndexHTML_ScriptsExist(t *testing.T) {
	for _, m := range scriptSrc.FindAllStringSubmatch(readIndexHTML(t), -1) {
		if _, err := os.Stat(filepath.Join("frontend/dist", m[1])); err != nil {
			t.Errorf("<script src=%q>: %v", m[1], err)
		}
	}
}

// Nothing is loaded from another origin (CDN) — vendored under ./vendor/.
func TestIndexHTML_NoRemoteScriptsOrStyles(t *testing.T) {
	if loc := remoteRef.FindStringIndex(readIndexHTML(t)); loc != nil {
		t.Errorf("remote <script>/<link> reference found: %q", readIndexHTML(t)[loc[0]:loc[1]])
	}
}

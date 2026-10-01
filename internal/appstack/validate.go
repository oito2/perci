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

package appstack

import (
	"regexp"

	"github.com/oito2/perci/internal/config"
)

// ValidAppFolder matches the characters allowed in a Container Aplicativo's
// folder name. The folder name doubles as the container name and as the
// fastcgi upstream hostname perci writes into fastcgi_pass (Docker's
// internal DNS resolves a container by its name), and is used verbatim as
// a `docker run --name` argument, so anything outside this set is rejected
// before it can reach either.
var ValidAppFolder = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// ValidAppURL matches the URL field of an AppContainer: a single DNS label
// (letters, digits, hyphens, never leading/trailing with a hyphen) followed
// by the fixed ".localhost" suffix. The value is interpolated unescaped
// into the generated nginx server_name directive, so — same defense in
// depth as ValidAppFolder — anything outside this set (space, {, }, ;,
// quotes, ...) is rejected first.
//
// Exactly one label is enforced deliberately: the Nginx container's TLS
// certificate is a single wildcard for *.localhost, minted once via mkcert
// when the container is created. A value like "foo.bar.localhost" would
// still resolve and route, but wouldn't match that certificate — rejecting
// it here is cheaper than letting it through and failing TLS validation in
// the browser later.
var ValidAppURL = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.localhost$`)

// ValidPHPMemoryLimit matches PHP's own ini shorthand for memory_limit: a
// plain integer (bytes), optionally suffixed with K/M/G (case-insensitive),
// or "-1" for "no limit" (php.net/manual/en/ini.core.php#ini.memory-limit).
// The value is written verbatim into a bind-mounted php.ini snippet
// (WritePHPMemoryLimitConf) — same defense-in-depth reasoning as
// ValidAppFolder/ValidAppURL, even though this file is never shell-
// interpreted.
var ValidPHPMemoryLimit = regexp.MustCompile(`^(-1|[0-9]+[KMGkmg]?)$`)

// ValidAppType reports whether t is one of the 5 known config.AppType*
// values. CreateApp's own switch (app.go) and buildAppServerBlock's
// (nginx.go) both silently treat any unrecognized Type as PHP-family in
// their `default:` case — harmless for values that only ever came from the
// GUI's own fixed dropdown, but a hand-edited config.yaml (or a future
// import from a different perci version) with a typo'd Type would
// otherwise create the wrong kind of container without any warning.
func ValidAppType(t string) bool {
	switch t {
	case config.AppTypeMoodle, config.AppTypePHP, config.AppTypeGeneric, config.AppTypeNode, config.AppTypePHPNode:
		return true
	default:
		return false
	}
}

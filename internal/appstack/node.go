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
	"context"
	"fmt"
	"io"
	"os"

	"github.com/oito2/perci/internal/executor"
)

// SupportedNodeVersions is the list AppTypeNode/AppTypePHPNode containers
// can be created with: "22" (Maintenance LTS, EOL Apr/2027), "24" (Active
// LTS, EOL Apr/2028), "26" (Current since May/2026, becomes LTS Oct/2026).
var SupportedNodeVersions = []string{"22", "24", "26"}

// ValidNodeVersion reports whether v is one of SupportedNodeVersions.
func ValidNodeVersion(v string) bool {
	for _, s := range SupportedNodeVersions {
		if s == v {
			return true
		}
	}
	return false
}

// ValidDevPort reports whether port is usable as a Node dev server's port
// (proxy_pass'd by Nginx): unprivileged range, and distinct from the fixed
// ports every other appstack container
// already claims (9000 for php-fpm's fastcgi listener, 3306 for MariaDB),
// so a careless choice can't collide with those inside the same
// container/network.
func ValidDevPort(port int) bool {
	if port < 1024 || port > 65535 {
		return false
	}
	if port == 9000 || port == 3306 {
		return false
	}
	return true
}

// NodeImageName returns the perci-managed base image tag for a Node
// version, e.g. "perci-node24" for "24" — built once per version and
// reused by every AppTypeNode/AppTypePHPNode container on that version,
// mirroring ImageName for PHP.
func NodeImageName(version string) string { return "perci-node" + version }

// nodeDockerfile builds a perci-node<version> image from the official Node
// image, adding only what npm-installing native dependencies commonly
// needs (git, a C/C++ toolchain, python3 for node-gyp) — nothing
// framework-specific. Vue/React/Svelte/etc. are all just npm packages the
// developer installs themselves inside the mounted project folder — perci
// is framework-agnostic here, matching its general philosophy of only
// preparing infrastructure, never opinionating on the app's own stack.
//
// UID is baked in at build time the same way PHPDockerfile does for
// www-data — official Node images already ship a "node" user (uid 1000),
// usermod'd here to the invoking host user so files the dev server writes
// into the bind-mounted project folder (npm's node_modules, build output,
// ...) don't end up root-owned on the host.
const nodeDockerfile = `ARG NODE_VERSION=24
FROM node:${NODE_VERSION}-bookworm

ARG UID=1000

RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    python3 \
    make \
    g++ \
 && rm -rf /var/lib/apt/lists/*

RUN usermod -u ${UID} node

WORKDIR /app
`

// EnsureNodeImage builds NodeImageName(version) when it's missing or out
// of date — same rules as EnsureImage for PHP (see ensureImage).
func EnsureNodeImage(ctx context.Context, exe *executor.Executor, stdout io.Writer, version string) error {
	if !ValidNodeVersion(version) {
		return fmt.Errorf("versão Node não suportada: %s", version)
	}
	return ensureImage(ctx, exe, stdout, imageBuild{
		name:  NodeImageName(version),
		first: "primeira vez que a versão Node " + version + " é usada",
		files: map[string]string{"Dockerfile": nodeDockerfile},
		args:  []string{"NODE_VERSION=" + version, fmt.Sprintf("UID=%d", os.Getuid())},
	})
}

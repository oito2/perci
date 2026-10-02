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
// can be created with: "22", "24" and "26".
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
// (proxy_pass'd by Nginx): unprivileged range (1024-65535), and not 9000
// (php-fpm's fastcgi listener) or 3306 (MariaDB).
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
// version, e.g. "perci-node24" for "24". The image is built once per
// version and reused by every AppTypeNode/AppTypePHPNode container on that
// version.
func NodeImageName(version string) string { return "perci-node" + version }

// nodeDockerfile builds a perci-node<version> image from the official Node
// image, adding only what npm-installing native dependencies commonly
// needs (git, a C/C++ toolchain, python3 for node-gyp). Nothing is
// framework-specific.
//
// The image's "node" user (uid 1000) is usermod'd at build time to the
// invoking host user's UID, so files the dev server writes into the
// bind-mounted project folder are not root-owned on the host.
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

// EnsureNodeImage builds NodeImageName(version) when it is missing or out
// of date, with the same rules as EnsureImage (see ensureImage).
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

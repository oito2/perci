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

// Package nettimeout bounds the unprivileged commands that depend on the
// network, so a stalled connection cannot hold a running action forever.
// Privileged package-manager steps are never bounded this way.
package nettimeout

import (
	"context"
	"time"
)

// Limit is the longest an unprivileged network command may run.
const Limit = 30 * time.Minute

// CurlFlags are the curl options that bound connection setup and the
// whole transfer.
var CurlFlags = []string{"--connect-timeout", "10", "--max-time", "600"}

// CurlShellFlags is CurlFlags as text for a shell script.
const CurlShellFlags = "--connect-timeout 10 --max-time 600"

// Context returns ctx limited to Limit.
func Context(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, Limit)
}

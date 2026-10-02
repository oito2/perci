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

// Package appstack manages perci's per-project Docker application stack —
// each project gets its own container, its own *.localhost URL, and (for
// PHP-family types) its own PHP version. Nginx, MariaDB, and the app
// containers (moodle/php/generic/node/php_node types) are managed
// separately, with shared input validation and per-container CLI tool
// wrappers under ~/.local/bin.
package appstack

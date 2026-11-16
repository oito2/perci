#!/usr/bin/env bash
#
# Copyright (C) 2026  oito2
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the Free Software Foundation, either version 3 of the License, or
# (at your option) any later version.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU General Public License for more details.
#
# You should have received a copy of the GNU General Public License
# along with this program.  If not, see <https://www.gnu.org/licenses/>.

# =============================================================================
# Script Name : install.sh
# Description : Refuses to install perci.gnl, which was discontinued and
#               replaced by Perci82 System and Perci82 Dev, and prints the
#               install commands of the two new apps.
# Version     : 3.0.0
# =============================================================================
set -Eeuo pipefail
shopt -s inherit_errexit

readonly PERCI82_REPO="oito2/perci82"
readonly CURL_FLAGS="--proto '=https' --proto-redir '=https' --tlsv1.2 --connect-timeout 10 --max-time 600 -fsSL"

# Prints the curl | bash install command of Perci82 installer $1.
install_command() {
    printf '  curl %s https://raw.githubusercontent.com/%s/main/%s | bash\n' "$CURL_FLAGS" "$PERCI82_REPO" "$1"
}

# Prints the discontinuation notice to stderr and exits with status 1.
main() {
    {
        printf '%s\n' "perci.gnl was discontinued and replaced by Perci82 System and Perci82 Dev — this script no longer installs it."
        printf '\n%s\n' "Perci82 System — Linux administration (post-install, updates, fonts, templates, Flatpaks):"
        install_command "install-system.sh"
        printf '\n%s\n' "Perci82 Dev — development (environment, Docker, repositories, AI contexts, skills and MCPs):"
        install_command "install-dev.sh"
        printf '\n%s\n' "Both apps use the same ~/.perci folder, so Perci's settings and backups keep working."
        printf '%s\n' "Each installer offers to remove the old Perci, keeping ~/.perci."
        printf '%s\n' "More at https://github.com/${PERCI82_REPO}"
    } >&2
    exit 1
}

main "$@"

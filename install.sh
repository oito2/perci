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
# Description : Installs the latest perci.gnl release — the prci binary in
#               /usr/local/bin (with the perci alias) and the application
#               menu entry (.desktop + hicolor icon set).
# Version     : 2.0.0
# =============================================================================
set -Eeuo pipefail
shopt -s inherit_errexit

readonly REPO="oito2/perci"
readonly BINARY_NAME="prci"
readonly INSTALL_DIR="/usr/local/bin"
readonly MENU_ASSET="perci-menu.tar.gz"
readonly HICOLOR_DIR="/usr/share/icons/hicolor"
readonly LEGACY_ICON="/usr/share/pixmaps/perci.png"
readonly CURL=(curl --proto '=https' --tlsv1.2 -fsSL)

trap 'printf "\n\033[0;31mError at %s:%d\033[0m\n" "${BASH_SOURCE[0]}" "$LINENO" >&2' ERR
trap '[[ -n "${_tmpdir:-}" ]] && rm -rf -- "$_tmpdir"' EXIT

# --- Output helpers (data is printed with %s, never interpreted) ---
info() { printf '\033[1;34m[INFO]\033[0m %s\n' "$*"; }
success() { printf '\033[1;32m[SUCCESS]\033[0m %s\n' "$*"; }
err() { printf '\033[1;31m[ERROR]\033[0m %s\n' "$*" >&2; }
die() {
    err "$*"
    exit 1
}

# Runs "$@" as root: directly when already root, through sudo otherwise.
as_root() {
    if [[ $EUID -eq 0 ]]; then
        "$@"
    else
        sudo "$@"
    fi
}

# Only amd64 is published — the Perci GUI (Wails) uses cgo (GTK4/WebKitGTK
# 6.0), which isn't cross-compiled for arm64.
release_arch() {
    local arch
    arch=$(uname -m)
    case "$arch" in
        x86_64) printf '%s\n' "amd64" ;;
        aarch64 | arm64) die "perci.gnl only publishes linux/amd64 binaries right now — arm64 ('$arch') is not yet supported. Build from source instead (see cmd/prci-gui/README.md for the system dependencies needed)." ;;
        *) die "Architecture '$arch' is not supported by perci.gnl." ;;
    esac
}

require_tools() {
    local cmd
    for cmd in curl jq sha256sum tar; do
        command -v "$cmd" &>/dev/null || die "Required tool '$cmd' is not installed. Please install it first."
    done
}

# Prints the browser_download_url of asset $2 in release JSON $1 (empty when absent).
asset_url() {
    jq -r --arg name "$2" '.assets[] | select(.name == $name) | .browser_download_url' <<<"$1"
}

# Prints the SHA-256 listed for file $2 in checksums text $1 (sha256sum
# format, text or binary "*name" mode). Empty when absent — never fails.
checksum_for() {
    awk -v n="$2" '{ f = $2; sub(/^\*/, "", f) } f == n { print $1; exit }' <<<"$1" || true
}

# Fails unless file $1 has SHA-256 $2.
verify_sha256() {
    local actual
    actual=$(sha256sum -- "$1" | cut -d ' ' -f1)
    [[ "${actual,,}" == "${2,,}" ]] || die "Checksum mismatch for $(basename -- "$1"): expected $2, got $actual. Aborting installation."
}

# Installs the verified file $1 as $2 (mode $4) owned by root, without
# trusting $1 after the sudo prompt: root copies it into a root-owned staged
# file (0600, so a swapped-in secret is never readable), re-verifies the
# SHA-256 $3 on that copy, and only then sets the mode and renames it into
# place. A plain `sudo mv` would keep the user as the binary's owner and
# accept whatever the file became while the password prompt was open.
privileged_install() {
    # shellcheck disable=SC2016 # $1..$4 expand inside the root shell
    as_root sh -c '
        set -e
        staged="$2.perci-new"
        install -T -m 0600 -o root -g root -- "$1" "$staged"
        printf "%s  %s\n" "$3" "$staged" | sha256sum -c --status - || { rm -f -- "$staged"; echo "checksum mismatch: $2" >&2; exit 1; }
        chmod "$4" -- "$staged"
        mv -f -T -- "$staged" "$2"
    ' sh "$1" "$2" "$3" "$4"
}

# Installs the menu entry from the verified release archive $1 (SHA-256
# $2): root copies it into its own temp file, re-verifies it and extracts
# share/applications/perci.desktop + share/icons/hicolor/* into /usr.
install_menu_entry() {
    # shellcheck disable=SC2016 # $1..$4 expand inside the root shell
    as_root sh -c '
        set -e
        staged=$(mktemp)
        cat -- "$1" >"$staged"
        if ! printf "%s  %s\n" "$2" "$staged" | sha256sum -c --status -; then
            rm -f -- "$staged"
            echo "checksum mismatch: menu entry archive" >&2
            exit 1
        fi
        tar -xzf "$staged" -C /usr --no-same-owner --no-same-permissions --no-overwrite-dir
        rm -f -- "$staged" "$3"
        if command -v gtk-update-icon-cache >/dev/null 2>&1; then
            gtk-update-icon-cache -q -t -f "$4" || true
        fi
    ' sh "$1" "$2" "$LEGACY_ICON" "$HICOLOR_DIR"
}

main() {
    local arch release tag asset_name binary_url checksums_url checksums
    local binary_sum dest menu_url menu_sum

    arch=$(release_arch)
    require_tools

    info "Fetching latest release version from GitHub..."
    release=$("${CURL[@]}" "https://api.github.com/repos/${REPO}/releases/latest")
    tag=$(jq -r '.tag_name' <<<"$release")
    [[ "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([-.][0-9A-Za-z.]+)?$ ]] || die "Unexpected release tag: '$tag'."
    info "Latest release version: $tag"

    asset_name="${BINARY_NAME}-linux-${arch}"
    binary_url=$(asset_url "$release" "$asset_name")
    [[ -n "$binary_url" && "$binary_url" != "null" ]] || die "Release asset '$asset_name' not found in the latest release."
    checksums_url=$(asset_url "$release" "checksums.txt")
    [[ -n "$checksums_url" && "$checksums_url" != "null" ]] || die "Release asset 'checksums.txt' not found in the latest release. Aborting for safety."

    _tmpdir=$(mktemp -d -t perci-install-XXXXXX)

    info "Downloading release asset..."
    "${CURL[@]}" -o "$_tmpdir/$asset_name" "$binary_url"
    checksums=$("${CURL[@]}" "$checksums_url")

    info "Verifying checksum..."
    binary_sum=$(checksum_for "$checksums" "$asset_name")
    [[ -n "$binary_sum" ]] || die "Checksum entry for '$asset_name' not found in checksums.txt. Aborting for safety."
    verify_sha256 "$_tmpdir/$asset_name" "$binary_sum"
    success "Checksum verified."

    dest="${INSTALL_DIR}/${BINARY_NAME}"
    info "Installing binary to $dest..."
    if [[ -w "$INSTALL_DIR" ]]; then
        chmod 755 -- "$_tmpdir/$asset_name"
        mv -f -- "$_tmpdir/$asset_name" "$dest"
        ln -sf -- "$dest" "${INSTALL_DIR}/perci"
    else
        info "Elevated permissions needed. Executing with sudo..."
        privileged_install "$_tmpdir/$asset_name" "$dest" "$binary_sum" 0755
        as_root ln -sf -- "$dest" "${INSTALL_DIR}/perci"
    fi
    info "Linked alias 'perci' to $dest."

    # Application menu entry (a different entry from the hidden autostart
    # .desktop the tray creates). It comes from the release's own
    # perci-menu.tar.gz, listed in checksums.txt like the binary — always the
    # blue icon set. A failure here doesn't abort: the binary already works
    # at this point.
    info "Adding application menu entry..."
    menu_url=$(asset_url "$release" "$MENU_ASSET")
    menu_sum=$(checksum_for "$checksums" "$MENU_ASSET")
    if [[ -z "$menu_url" || "$menu_url" == "null" || -z "$menu_sum" ]]; then
        err "This release has no verified '$MENU_ASSET' — skipping the menu entry (Perci still works via '$BINARY_NAME'/'perci' in the terminal)."
    elif ! "${CURL[@]}" -o "$_tmpdir/$MENU_ASSET" "$menu_url"; then
        err "Failed to download '$MENU_ASSET' — skipping the menu entry (Perci still works via '$BINARY_NAME'/'perci' in the terminal)."
    elif ! install_menu_entry "$_tmpdir/$MENU_ASSET" "$menu_sum"; then
        err "Failed to install the menu entry (Perci still works via '$BINARY_NAME'/'perci' in the terminal)."
    else
        success "Application menu entry created."
    fi

    success "perci.gnl ($tag) has been installed successfully!"
    success "Type '$BINARY_NAME' or 'perci' to launch it."
    info "Perci is a graphical app now (no more terminal UI) — it needs GTK4 and WebKitGTK 6.0 installed to actually run (build-time-only packages like *-dev are NOT needed here, just the runtime libraries, normally already present on a desktop Linux install): libgtk-4-1, libwebkitgtk-6.0-4."
}

main "$@"

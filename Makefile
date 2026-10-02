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

BINARY   := prci
CMD      := ./cmd/prci-gui
VERSION  ?= dev
MODULE   := github.com/oito2/perci
LDFLAGS  := -X $(MODULE)/internal/version.Version=$(VERSION)
DIST     := dist

.PHONY: build run test test-frontend lint clean install release generate-bindings

# Builds without a `-tags` flag: GTK4 + WebKitGTK 6.0 is the default
# stack. -trimpath keeps the builder's local paths out of the binary.
build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) $(CMD)

run: build
	./$(BINARY)

test:
	go test -race ./...

# Frontend smoke test: syntax of every script, then the node:test suite
# (plain Node, no npm dependencies).
test-frontend:
	for f in cmd/prci-gui/frontend/dist/js/*.js cmd/prci-gui/frontend/dist/js/screens/*.js; do node --check "$$f" || exit 1; done
	node --test cmd/prci-gui/frontend/test/*.test.mjs

lint:
	gofmt -l . | (! grep .)
	go vet ./...
	golangci-lint run

clean:
	rm -f $(BINARY)
	rm -rf $(DIST)

# Installs the application menu entry (the .desktop file and the blue
# hicolor icon set), so Perci shows up in the desktop's application menu,
# not just via the terminal or the system tray. The legacy
# /usr/share/pixmaps/perci.png is removed, and the icon cache refresh is
# best-effort.
ICON_SIZES := 512 256 128 64 48 32
HICOLOR    := /usr/share/icons/hicolor

install: build
	sudo install -m 755 $(BINARY) /usr/local/bin/$(BINARY)
	sudo ln -sf /usr/local/bin/$(BINARY) /usr/local/bin/perci
	for s in $(ICON_SIZES); do \
		sudo install -D -m 644 packaging/icons/blue/hicolor/$${s}x$${s}/apps/perci.png $(HICOLOR)/$${s}x$${s}/apps/perci.png || exit 1; \
	done
	sudo rm -f /usr/share/pixmaps/perci.png
	-sudo gtk-update-icon-cache -q -t -f $(HICOLOR)
	sudo install -m 644 packaging/perci.desktop /usr/share/applications/perci.desktop

# Only linux/amd64 is built — Wails uses cgo (GTK4/WebKitGTK 6.0 via C
# bindings), so a plain "GOARCH=arm64 go build" doesn't cross-compile; that
# would need an arm64 cross toolchain (CC=aarch64-linux-gnu-gcc) and the
# arm64 gtk4/webkitgtk-6.0 -dev packages, which this target doesn't
# provision.
#
# The asset name ("prci-linux-amd64") matches the one
# internal/selfupdate.LatestRelease builds
# (fmt.Sprintf("prci-linux-%s", runtime.GOARCH)) to look up in the release.
#
# perci-menu.tar.gz is the application menu entry archive
# (share/applications/perci.desktop + share/icons/hicolor/*/apps/perci.png)
# — published as a release asset and listed in checksums.txt. Built
# reproducibly (sorted entries, fixed owner/mtime, gzip -n) so the same tag
# yields the same hash.
MENU_STAGE := $(DIST)/menu

release:
	@echo "Building release $(VERSION)..."
	@mkdir -p $(DIST)
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST)/prci-linux-amd64 $(CMD)
	rm -rf $(MENU_STAGE)
	mkdir -p $(MENU_STAGE)/share/applications $(MENU_STAGE)/share/icons
	cp packaging/perci.desktop $(MENU_STAGE)/share/applications/perci.desktop
	cp -r packaging/icons/blue/hicolor $(MENU_STAGE)/share/icons/hicolor
	chmod -R u=rwX,go=rX $(MENU_STAGE)
	tar --sort=name --owner=0 --group=0 --numeric-owner --mtime='@0' \
		-C $(MENU_STAGE) -cf - share | gzip -n > $(DIST)/perci-menu.tar.gz
	rm -rf $(MENU_STAGE)
	cd $(DIST) && sha256sum prci-linux-amd64 perci-menu.tar.gz > checksums.txt
	@echo "Binaries in $(DIST):"
	@ls -lh $(DIST)/

# generate-bindings regenerates frontend/dist/bindings/ (JS generated from
# the exported methods of cmd/prci-gui's 6 services — HomeService/
# LinuxService/DevSetupService/DockerService/DevToolsService/TrayService)
# via the `wails3` CLI. Not part of build/release: the generated files are
# versioned on disk. Requires the `wails3` CLI installed once:
# `go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.22`.
generate-bindings:
	cd $(CMD) && wails3 generate bindings -names -b -d frontend/dist/bindings ./...

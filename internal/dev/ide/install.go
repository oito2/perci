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

package ide

import (
	"context"
	"fmt"
	"io"

	"github.com/oito2/perci/internal/distro"
	"github.com/oito2/perci/internal/executor"
)

// InstallOne installs a single IDE.
func InstallOne(ctx context.Context, exe *executor.Executor, stdout io.Writer, e IDE, family string) error {
	opts := executor.Options{Stdout: stdout, Stderr: stdout}
	switch e.Cmd {
	case "zed":
		script := `set -Eeuo pipefail; curl -fsSL https://zed.dev/install.sh | sh`
		return exe.Run(ctx, opts, "bash", "-c", script)

	case "code":
		return installVSCode(ctx, exe, stdout, family)

	case "codium":
		return installVSCodium(ctx, exe, stdout, family)
	}
	return fmt.Errorf("instalador desconhecido para %s", e.Name)
}

// installVSCode installs VS Code from Microsoft's signed apt/dnf
// repository through distro.InstallFromSignedRepo.
func installVSCode(ctx context.Context, exe *executor.Executor, stdout io.Writer, family string) error {
	return distro.InstallFromSignedRepo(ctx, exe, stdout, family, distro.SignedRepo{
		KeyringPath:    "/usr/share/keyrings/microsoft-archive-keyring.gpg",
		KeyURL:         "https://packages.microsoft.com/keys/microsoft.asc",
		AptListPath:    "/etc/apt/sources.list.d/vscode.list",
		AptListContent: "deb [arch=amd64,arm64 signed-by=/usr/share/keyrings/microsoft-archive-keyring.gpg] https://packages.microsoft.com/repos/code stable main\n",
		DnfRepoPath:    "/etc/yum.repos.d/vscode.repo",
		DnfRepoBody:    "[code]\nname=Visual Studio Code\nbaseurl=https://packages.microsoft.com/yumrepos/vscode\nenabled=1\ngpgcheck=1\ngpgkey=https://packages.microsoft.com/keys/microsoft.asc\n",
		PkgName:        "code",
	})
}

func installVSCodium(ctx context.Context, exe *executor.Executor, stdout io.Writer, family string) error {
	return distro.InstallFromSignedRepo(ctx, exe, stdout, family, distro.SignedRepo{
		KeyringPath: "/usr/share/keyrings/vscodium-archive-keyring.gpg",
		KeyURL:      "https://gitlab.com/paulcarroty/vscodium-deb-rpm-repo/raw/master/pub.gpg",
		AptListPath: "/etc/apt/sources.list.d/vscodium.sources",
		AptListContent: "Types: deb\nURIs: https://download.vscodium.com/debs\nSuites: vscodium\n" +
			"Components: main\nArchitectures: amd64 arm64\n" +
			"Signed-by: /usr/share/keyrings/vscodium-archive-keyring.gpg\n",
		DnfRepoPath: "/etc/yum.repos.d/vscodium.repo",
		DnfRepoBody: "[vscodium]\nname=download.vscodium.com\nbaseurl=https://download.vscodium.com/rpms/\n" +
			"enabled=1\ngpgcheck=1\ngpgkey=https://gitlab.com/paulcarroty/vscodium-deb-rpm-repo/raw/master/pub.gpg\n",
		PkgName: "codium",
	})
}

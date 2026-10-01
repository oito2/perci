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

package postinstall

import (
	"context"
	"io"

	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/ui"
)

// Shared building blocks of the Debian-family profiles (Mint, Ubuntu,
// Zorin, Pop!_OS): every action they have in common is defined once here,
// and each profile file only composes its own list — the four used to carry
// near-identical copies of these functions, so a fix in one could miss the
// others. IDs, labels and descriptions are what the GUI's checklist shows
// and what DependsOn refers to.

// aptBaseOpts are the options every apt-get call here runs with: no
// progress bars or colors in the GUI's terminal, and no interactive
// prompt — a conffile question or a debconf dialog has no terminal to be
// answered on, and --force-confold keeps the user's own config files.
var aptBaseOpts = []string{"-o", "Dpkg::Use-Pty=0", "-o", "Dpkg::Progress-Fancy=0", "-o", "APT::Color=0", "-o", "Dpkg::Options::=--force-confold"}

// aptStep runs one privileged, non-interactive apt-get command.
func aptStep(ctx context.Context, exe *executor.Executor, stdout io.Writer, msg string, args ...string) error {
	ui.Info(stdout, msg)
	return exe.Run(ctx, executor.Options{
		RequiresSudo: true,
		Stdout:       stdout,
		Stderr:       stdout,
		Env:          []string{"DEBIAN_FRONTEND=noninteractive"},
	}, "apt-get", append(append([]string{}, aptBaseOpts...), args...)...)
}

// enableUbuntuComponents enables universe and multiverse (whichever is
// missing) in one privileged batch — one password prompt, not one per
// component.
func enableUbuntuComponents(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
	var steps []executor.PrivilegedStep
	for _, component := range []string{"universe", "multiverse"} {
		if aptComponentEnabled(ctx, exe, component) {
			continue
		}
		steps = append(steps, executor.PrivilegedStep{
			Announce: "Habilitando " + component + "...",
			Name:     "add-apt-repository", Args: []string{"-y", component},
		})
	}
	if len(steps) == 0 {
		ui.Info(stdout, "universe e multiverse já estão habilitados.")
		return nil
	}
	return exe.RunSudoSequence(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, steps)
}

func actEnableUbuntuComponents() Action {
	return Action{ID: "enable-repos", Label: "Habilitar repositórios universe e multiverse",
		Description: "Adiciona os componentes universe e multiverse do Ubuntu, necessários para a maioria dos pacotes abaixo.",
		Run:         enableUbuntuComponents}
}

func actUpdateIndex(dependsOn string) Action {
	return Action{ID: "update-index", Label: "Atualizar lista de pacotes",
		Description: "Sincroniza o índice de pacotes (apt-get update) com os repositórios habilitados.",
		DependsOn:   dependsOn,
		Run: func(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
			return aptStep(ctx, exe, stdout, "Atualizando lista de pacotes...", "update")
		}}
}

func actAptUpgrade(dependsOn string) Action {
	return Action{ID: "upgrade-system", Label: "Atualizar pacotes do sistema",
		Description: "Atualiza todos os pacotes já instalados para a versão mais recente (apt-get full-upgrade).",
		DependsOn:   dependsOn,
		Run: func(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
			return aptStep(ctx, exe, stdout, "Atualizando pacotes instalados...", "full-upgrade", "-y")
		}}
}

// actAptInstall installs pkgs; acceptMSFontsEULA pre-accepts the Microsoft
// fonts license first, for lists that include (or pull in) it.
func actAptInstall(id, label, description, dependsOn string, acceptMSFontsEULA bool, pkgs ...string) Action {
	return Action{ID: id, Label: label, Description: description, DependsOn: dependsOn,
		Run: func(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
			if acceptMSFontsEULA {
				return acceptEulaAndAptInstall(ctx, exe, stdout, pkgs...)
			}
			return aptInstall(ctx, exe, stdout, pkgs...)
		}}
}

func actSysctl() Action {
	return Action{ID: "apply-sysctl", Label: "Aplicar ajustes de kernel",
		Description: "Reduz a troca de memória (swappiness) e aumenta o limite de observadores de arquivo (inotify).",
		Run:         configureSysctl}
}

func actSwap() Action {
	return Action{ID: "setup-swap", Label: "Configurar swapfile",
		Description: "Cria e ativa um swapfile de 4 GB, se o sistema ainda não tiver nenhum swap ativo.",
		Run: func(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
			setupSwapfile(ctx, exe, stdout)
			return nil
		}}
}

func actTRIM() Action {
	return Action{ID: "enable-trim", Label: "Ativar TRIM para SSDs",
		Description: "Habilita o serviço fstrim.timer, que mantém discos SSD saudáveis a longo prazo.",
		Run: func(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
			return step(ctx, exe, stdout, "Ativando TRIM para SSDs...", "systemctl", "enable", "--now", "fstrim.timer")
		}}
}

func actAptVAAPIIntel(dependsOn string) Action {
	return Action{ID: "vaapi-intel", Label: "Driver de vídeo Intel (VA-API)",
		Description: "Instala a aceleração de vídeo por hardware para GPUs Intel. Marque só se sua GPU for Intel.",
		DependsOn:   dependsOn,
		Run: func(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
			ui.Info(stdout, "Instalando drivers VA-API para Intel...")
			return aptInstall(ctx, exe, stdout, "intel-media-va-driver")
		}}
}

// actAptVAAPIAMD installs mesa-va-drivers — a real package up to Ubuntu
// 24.04 and a virtual one provided by mesa-libgallium on 26.04 (checked
// against both archives on 2026-09-29); apt resolves either.
func actAptVAAPIAMD(dependsOn string) Action {
	return Action{ID: "vaapi-amd", Label: "Driver de vídeo AMD (VA-API)",
		Description: "Instala a aceleração de vídeo por hardware para GPUs AMD. Marque só se sua GPU for AMD.",
		DependsOn:   dependsOn,
		Run: func(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
			ui.Info(stdout, "Instalando drivers VA-API para AMD...")
			return aptInstall(ctx, exe, stdout, "mesa-va-drivers")
		}}
}

// actFlatpakEssentials ensures Flatpak + Flathub and installs VLC and a
// web-app shortcut manager.
func actFlatpakEssentials(label, description string) Action {
	return Action{ID: "setup-flatpak", Label: label, Description: description,
		Run: func(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
			return flatpakSetupAndInstall(ctx, exe, stdout, "org.videolan.VLC", "net.codelogistics.webapps")
		}}
}

func actUbuntuDrivers(dependsOn string) Action {
	return Action{ID: "detect-drivers", Label: "Detectar drivers adicionais",
		Description: "Roda ubuntu-drivers autoinstall para instalar drivers proprietários recomendados (ex. NVIDIA).",
		DependsOn:   dependsOn,
		Run: func(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
			return step(ctx, exe, stdout, "Detectando drivers adicionais...", "ubuntu-drivers", "autoinstall")
		}}
}

const flatpakSetupDescription = "Garante o Flatpak instalado, adiciona o repositório Flathub e instala VLC e um app de atalhos web."

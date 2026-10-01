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
	"fmt"
	"io"

	"github.com/oito2/perci/internal/executor"
	"github.com/oito2/perci/internal/ui"
)

// fedoraPackages is Fedora's own repositories only (checked against
// Fedora 44 on 2026-09-29): p7zip/p7zip-plugins were replaced by 7zip, and
// unrar lives in RPM Fusion nonfree — its own action (fedoraInstallUnrar),
// since dnf refuses a whole transaction when one package is missing.
var fedoraPackages = []string{
	"git",
	"curl",
	"wget",
	"htop",
	"make",
	"gcc",
	"gcc-c++",
	"tree",
	"jq",
	"7zip",
	"unzip",
	"net-tools",
	"plocate",
	"python3-pip",
}

// --- Granular actions for the GUI's "Linux :: Pós-instalação" screen ------
// Same non-interactive style as mint.go, sharing the same helpers and the
// same fedoraPackages list.

func fedoraUpgradeSystem(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
	return step(ctx, exe, stdout, "Atualizando sistema base...", "dnf", "upgrade", "--refresh", "-y")
}

func fedoraEnableRPMFusion(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
	return enableRPMFusion(ctx, exe, stdout)
}

// fedoraInstallCodecs follows RPM Fusion's own codec guide: swap Fedora's
// patent-free ffmpeg-free for the full ffmpeg, then the multimedia group —
// one privileged batch.
func fedoraInstallCodecs(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
	return exe.RunSudoSequence(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, []executor.PrivilegedStep{
		{Announce: "Trocando ffmpeg-free pelo ffmpeg completo (RPM Fusion)...", Name: "dnf", Args: []string{"swap", "-y", "ffmpeg-free", "ffmpeg", "--allowerasing"}},
		{Announce: "Instalando o grupo multimedia...", Name: "dnf", Args: []string{"group", "install", "-y", "multimedia"}},
	})
}

func fedoraInstallUnrar(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
	ui.Info(stdout, "Instalando unrar (RPM Fusion nonfree)...")
	return dnfInstall(ctx, exe, stdout, "unrar")
}

func fedoraInstallEssentials(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
	return dnfInstall(ctx, exe, stdout, fedoraPackages...)
}

func fedoraInstallNTFS(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
	if err := dnfInstall(ctx, exe, stdout, "ntfs-3g"); err != nil {
		ui.Warning(stdout, "ntfs-3g não disponível — o driver ntfs3 do kernel pode já estar ativo.")
	}
	return nil
}

func fedoraApplySysctl(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
	return configureSysctl(ctx, exe, stdout)
}

func fedoraEnableTRIM(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
	return step(ctx, exe, stdout, "Ativando TRIM para SSDs...", "systemctl", "enable", "--now", "fstrim.timer")
}

// fedoraVAAPIIntel installs intel-media-driver (RPM Fusion nonfree, every
// codec) — libva-intel-driver, used before, is the legacy driver for
// pre-Broadwell GPUs only.
func fedoraVAAPIIntel(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
	ui.Info(stdout, "Instalando drivers VA-API para Intel...")
	return dnfInstall(ctx, exe, stdout, "intel-media-driver")
}

// fedoraVAAPIAMD installs mesa-va-drivers-freeworld (RPM Fusion free, with
// H.264/HEVC). On Fedora 44 the plain VA drivers live inside
// mesa-dri-drivers and "mesa-va-drivers" no longer exists as a package.
func fedoraVAAPIAMD(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
	ui.Info(stdout, "Instalando drivers VA-API para AMD...")
	return dnfInstall(ctx, exe, stdout, "mesa-va-drivers-freeworld")
}

// fedoraCleanup runs both dnf commands as ONE sudo/pkexec authentication
// instead of two separate prompts (pkexec has no session cache the way
// sudo does — same reasoning as internal/system/update.Run). autoremove
// staying Soft means its failure is warned but never aborts the batch.
func fedoraCleanup(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
	return exe.RunSudoSequence(ctx, executor.Options{Stdout: stdout, Stderr: stdout}, []executor.PrivilegedStep{
		{Announce: "Removendo pacotes órfãos...", Soft: true, WarnMessage: "Falha ao remover pacotes órfãos.", Name: "dnf", Args: []string{"autoremove", "-y"}},
		{Announce: "Limpando cache do dnf...", Name: "dnf", Args: []string{"clean", "all"}},
	})
}

var fedoraProfile = Profile{
	ID:    "fedora",
	Label: "Fedora 44",
	Actions: []Action{
		{ID: "upgrade-system", Label: "Atualizar sistema base",
			Description: "Atualiza todos os pacotes já instalados (dnf upgrade --refresh).",
			Run:         fedoraUpgradeSystem},
		{ID: "enable-rpmfusion", Label: "Habilitar RPM Fusion",
			Description: "Adiciona os repositórios RPM Fusion free e nonfree, necessários para codecs e drivers de vídeo.",
			Run:         fedoraEnableRPMFusion},
		{ID: "install-codecs", Label: "Instalar codecs multimídia",
			Description: "Troca o ffmpeg-free pelo ffmpeg completo e instala o grupo multimedia (dnf group install), com suporte a formatos de áudio/vídeo comuns.",
			DependsOn:   "enable-rpmfusion", Run: fedoraInstallCodecs},
		{ID: "install-essentials", Label: "Instalar pacotes essenciais",
			Description: "Ferramentas de compilação, compressão de arquivos e utilitários de linha de comando comuns.",
			Run:         fedoraInstallEssentials},
		{ID: "install-unrar", Label: "Instalar unrar",
			Description: "Instala o unrar (do RPM Fusion nonfree), para extrair arquivos .rar.",
			DependsOn:   "enable-rpmfusion", Run: fedoraInstallUnrar},
		{ID: "install-ntfs", Label: "Instalar suporte a NTFS",
			Description: "Instala o ntfs-3g (dos repositórios do Fedora) para ler/gravar discos formatados em NTFS — melhor esforço, pode já estar coberto pelo driver ntfs3 do kernel.",
			Run:         fedoraInstallNTFS},
		{ID: "apply-sysctl", Label: "Aplicar ajustes de kernel",
			Description: "Reduz a troca de memória (swappiness) e aumenta o limite de observadores de arquivo (inotify).",
			Run:         fedoraApplySysctl},
		{ID: "enable-trim", Label: "Ativar TRIM para SSDs",
			Description: "Habilita o serviço fstrim.timer, que mantém discos SSD saudáveis a longo prazo.",
			Run:         fedoraEnableTRIM},
		{ID: "vaapi-intel", Label: "Driver de vídeo Intel (VA-API)",
			Description: "Instala a aceleração de vídeo por hardware para GPUs Intel (do RPM Fusion). Marque só se sua GPU for Intel.",
			DependsOn:   "enable-rpmfusion", Run: fedoraVAAPIIntel},
		{ID: "vaapi-amd", Label: "Driver de vídeo AMD (VA-API)",
			Description: "Instala a aceleração de vídeo por hardware para GPUs AMD (do RPM Fusion). Marque só se sua GPU for AMD.",
			DependsOn:   "enable-rpmfusion", Run: fedoraVAAPIAMD},
		{ID: "cleanup", Label: "Limpar pacotes desnecessários",
			Description: "Remove dependências órfãs e limpa o cache do dnf (autoremove + clean all).",
			Run:         fedoraCleanup},
	},
}

func enableRPMFusion(ctx context.Context, exe *executor.Executor, stdout io.Writer) error {
	ui.Info(stdout, "Habilitando repositórios RPM Fusion...")

	if rpmFusionEnabled(ctx, exe) {
		ui.Info(stdout, "RPM Fusion já habilitado, pulando.")
		return nil
	}

	ver, err := exe.Output(ctx, executor.Options{}, "rpm", "-E", "%fedora")
	if err != nil {
		return fmt.Errorf("detectar versão fedora: %w", err)
	}
	v := stripNewline(ver)

	freeURL := fmt.Sprintf(
		"https://download1.rpmfusion.org/free/fedora/rpmfusion-free-release-%s.noarch.rpm", v,
	)
	nonfreeURL := fmt.Sprintf(
		"https://download1.rpmfusion.org/nonfree/fedora/rpmfusion-nonfree-release-%s.noarch.rpm", v,
	)
	return exe.Run(ctx,
		executor.Options{RequiresSudo: true, Stdout: stdout, Stderr: stdout},
		"dnf", "install", "-y", freeURL, nonfreeURL,
	)
}

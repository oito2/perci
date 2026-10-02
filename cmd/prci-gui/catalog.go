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

package main

// MenuItem is one entry inside a category's accordion panel — rendered
// with the daisyUI `menu` component.
type MenuItem struct {
	Title string `json:"title"`
	Desc  string `json:"desc"` // may hold multiple lines (\n) — rendered with line breaks preserved

	// ActionID identifies which bound method the "Executar" button calls for
	// this item — empty means "no real screen yet": the button stays
	// disabled.
	ActionID string `json:"actionId,omitempty"`

	// Button is the "Executar" button's text when ActionID != "" —
	// screen-specific text (e.g. "Executar atualização"). Empty uses the
	// generic "Executar" label.
	Button string `json:"button,omitempty"`

	// Notes are warnings shown below the screen's buttons. Empty renders no
	// notes section.
	Notes []string `json:"notes,omitempty"`

	// Tab is the "📝 <Tab>" tab's text (#tab-desc-label) when this item is
	// selected — empty uses the generic "Perci" label (the app's own name,
	// used by Home :: Visão Geral).
	Tab string `json:"tab,omitempty"`
}

// MenuCategory is one of the top-level sidebar sections.
type MenuCategory struct {
	Label string     `json:"label"`
	Items []MenuItem `json:"items"`
}

// categories is the sidebar's category/item catalogue. An item without an
// ActionID keeps the placeholder behavior (Executar disabled).
var categories = []MenuCategory{
	{
		// Home has 2 wired items: "Visão Geral" ("dashboard" mode,
		// #dashboard-container — also the initial screen shown before any sidebar
		// selection) and "Configurações".
		Label: "Home",
		Items: []MenuItem{
			{
				Title:    "Visão Geral",
				Desc:     "Atualize ou desinstale o Perci, e veja um resumo do estado do seu sistema.",
				ActionID: "dashboard",
			},
			{
				// "self-config" mode (#selfconfig-container): workspace, Flatpak scope and
				// theme settings, plus the sidebar mascot/app icon pickers.
				Title:    "Configurações",
				Desc:     "Tema, logo da sidebar, ícone do aplicativo, workspace e escopo de instalação do Flatpak.",
				ActionID: "self-config",
				Tab:      "Configurações",
			},
		},
	},
	{
		Label: "Linux",
		Items: []MenuItem{
			{
				// Shows the "Sistema" status bar (#simple-container, fed by
				// GetPostinstallProfile, the same source as the dashboard's "Sistema"
				// section), so it has its own Tab ("Atualizar Sistema") instead of the
				// generic "Definições".
				Title: "Atualizar Sistema",
				Desc: "Executa o processo de atualização do seu SO.\n" +
					"- Atualiza os pacotes da sua distribuição\n" +
					"- Atualiza os pacotes Snap (se disponível)\n" +
					"- Atualiza os pacotes Flatpak, do usuário e do sistema (se disponível)\n" +
					"\nApós a atualização...",
				ActionID: "system-update",
				Button:   "Executar atualização",
				Tab:      "Atualizar Sistema",
			},
			{
				// Desc/Button are deliberately empty: this screen uses the "checklist com
				// dependência" mode (ActionID "postinstall", #postinstall-container), not
				// the simple description+button mode.
				Title:    "Pós-instalação",
				ActionID: "postinstall",
				Tab:      "Definições",
			},
			{
				// "checklist simples" mode (#multiselect-container) — checkbox reflects
				// the current state (installed ⇄ unchecked), with no dependency between
				// items.
				Title:    "Gerenciar Fontes",
				Desc:     "Selecione as fontes que deseja instalar. Para desinstalar, remova a seleção.",
				ActionID: "fonts",
				Tab:      "Definições",
			},
			{
				// "checklist simples" mode, same as Gerenciar Fontes above.
				Title:    "Gerenciar Templates de Arquivos",
				Desc:     "Selecione os templates de arquivos que deseja instalar. Para desinstalar, remova a seleção.",
				ActionID: "templates",
				Tab:      "Definições",
			},
			{
				// "checklist simples" mode, same as Fontes/Templates above.
				Title:    "Aplicativos: Flatpak",
				Desc:     "Selecione aplicativos que deseja instalar. Para desinstalar, remova a seleção.",
				ActionID: "flatpak-apps",
				Tab:      "Definições",
			},
			{
				// "app único install/desinstalar" mode (#singleapp-container): an
				// external application, two buttons instead of a checklist or a single
				// button.
				Title:    "Aplicativos: Linux Toys",
				Desc:     "O LinuxToys reúne software útil, manutenção do sistema e ferramentas para repetir sua configuração em um único aplicativo fácil de usar — em dezenas de distribuições Linux.",
				ActionID: "linuxtoys",
				Tab:      "Definições",
			},
			{
				// "app único install/desinstalar" mode, same as Linux Toys above.
				Title:    "Aplicativos: MegaSync",
				Desc:     "O MEGAsync é o aplicativo oficial do serviço de armazenamento em nuvem MEGA para computadores. Ele permite sincronizar pastas locais com a nuvem de forma automática e gerenciar downloads ou uploads.",
				ActionID: "megasync",
				Tab:      "Definições",
			},
		},
	},
	{
		Label: "Desenvolvimento",
		Items: []MenuItem{
			{
				// "checklist simples" mode, same as Fontes/Templates/Flatpak
				// (#multiselect-container).
				Title:    "Pré-requisitos",
				Desc:     "Selecione os pacotes que deseja instalar. Para desinstalar, remova a seleção.",
				ActionID: "prereqs",
				Tab:      "Definições",
			},
			{
				// "checklist simples" mode, same as above.
				Title:    "Linguagens e SDKs",
				Desc:     "Selecione os pacotes que deseja instalar. Para desinstalar, remova a seleção.",
				ActionID: "sdks",
				Tab:      "Definições",
			},
			{
				// "checklist simples" mode, same as above.
				Title:    "Aplicativos: IA",
				Desc:     "Selecione aplicativos de IA que deseja instalar. Para desinstalar, remova a seleção.",
				ActionID: "ai-apps",
				Tab:      "Definições",
			},
			{
				// "checklist simples" mode plus extra "Aplicar Starship" / "Remover
				// Starship" buttons (#multiselect-container, via the extras on the
				// MULTISELECT_SCREENS registry). Starship isn't a checkable item in this
				// list — the extra buttons handle it, independent of the selection.
				Title:    "Aplicativos: Terminais",
				Desc:     "Selecione os terminais que deseja instalar. Para desinstalar, remova a seleção.\nTerminais instalados ganham a opção \"Abrir no <terminal>\" no menu de contexto do Nautilus (submenu Scripts), Nemo e Dolphin.",
				ActionID: "terminals",
				Tab:      "Definições",
			},
			{
				// "IDE install/atualizar/desinstalar" mode (#ide-container): three
				// buttons, Atualizar/Desinstalar enabled only once already installed. One
				// item per IDE (internal/dev/ide), one ActionID per IDE.
				Title:    "IDE: Zed Editor",
				Desc:     "O Zed é um editor de código minimalista criado para velocidade e colaboração entre humanos e IA.",
				ActionID: "ide-zed",
				Tab:      "Definições",
			},
			{
				Title:    "IDE: VS Code",
				Desc:     "O Visual Studio Code é um editor de código com IA, gratuito e de código aberto. Desenvolva com agentes de IA que planejam, escrevem código e depuram para você.",
				ActionID: "ide-code",
				Tab:      "Definições",
			},
			{
				Title:    "IDE: VSCodium",
				Desc:     "O VSCodium é uma distribuição binária do editor VS Code, da Microsoft, licenciada livremente e mantida pela comunidade.",
				ActionID: "ide-codium",
				Tab:      "Definições",
			},
			{
				// "external app via local file install/uninstall" mode
				// (#fileapp-container): no Atualizar button — Android Studio updates
				// itself (the frontend shows Notes).
				Title: "IDE: Android Studio",
				Desc: "O ambiente de desenvolvimento integrado oficial para desenvolvimento de apps Android agora acelera sua produtividade com o Gemini no Android Studio, seu parceiro de programação com tecnologia de IA.\n\n" +
					"Para instalar o Android Studio, faça o download do arquivo .tar.gz em https://developer.android.com/studio?hl=pt-br",
				Notes: []string{
					`O instalador do Android Studio abrirá em uma janela gráfica. Durante o assistente inicial, na tela de boas-vindas ou no menu principal, clique no ícone de engrenagem (Configure ou menu de três pontos) e selecione "Create Desktop Entry". Isso adicionará o ícone do Android Studio ao menu de aplicativos do seu sistema operacional.`,
					"Utilize o Android Studio para atualizar.",
				},
				ActionID: "androidstudio",
				Tab:      "Definições",
			},
			{
				// "external app via local file install/update/uninstall" mode (same
				// #fileapp-container, with the Atualizar button enabled).
				Title: "IDE: Antigravity IDE",
				Desc: "A visualização do editor do Google Antigravity oferece preenchimento automático com a tecla Tab, comandos de código em linguagem natural e um agente configurável e sensível ao contexto.\n\n" +
					"Para instalar o Antigravity IDE (Standalone), faça o download do arquivo .tar.gz em https://antigravity.google/download#antigravity-ide",
				ActionID: "antigravityide",
				Tab:      "Definições",
			},
		},
	},
	// Its own category, separate from "Desenvolvimento".
	{
		Label: "Docker",
		Items: []MenuItem{
			{
				// "criar container" mode (#dockercreate-container): a "Tipo de contêiner"
				// select with per-type dynamic fields, covering the 7 types the domain
				// layer (internal/appstack) validates — Nginx/MariaDB are disabled in the
				// select once they already exist (both are singletons).
				Title:    "Criar Contêiner",
				Desc:     "Crie um novo contêiner Nginx, MariaDB ou Aplicativo (Moodle, PHP, Genérico, Node ou PHP + Node).",
				ActionID: "docker-create",
				Tab:      "Novo Contêiner",
			},
			{
				// "gerenciar containers" mode (#dockerlist-container): a table with live
				// status and per-row action icons, plus Exportar/Importar configurações
				// below the table.
				Title:    "Gerenciar Contêineres",
				Desc:     "Inicie, pare, reinicie, recrie, veja logs, edite ou remova os contêineres Docker do Perci.",
				ActionID: "docker-manage",
				Tab:      "Contêineres",
			},
		},
	},
	// Its own category, alongside "Docker": repositories and the AI tooling
	// (contexts, skills, MCP servers).
	{
		Label: "Dev Tools",
		Items: []MenuItem{
			{
				// "repositórios" mode (#repos-container + #panel-repos-list): 2 top-level
				// tabs, "Identidade Global" (Tab below, content of #repos-container) and
				// "Repositórios" (the #tab-label-repos-list tab — remembered-folder cards
				// + a Clonar/Iniciar Repositório/Identidade Local/Arquivos horizontal
				// menu).
				Title:    "Repositórios",
				Desc:     "Clone, inicie e configure identidade de repositórios Git, gere .gitignore e Código de Conduta.",
				ActionID: "repos",
				Tab:      "Identidade Global",
			},
			{
				// "IA: Contextos" mode (#aicontext-container): a working-folder picker +
				// a simple checklist of internal/manager/ai.Models().
				Title:    "IA: Contextos",
				Desc:     "Contextos de IA são regras para direcionar o seu agente na realização de tarefas.",
				ActionID: "aicontext",
				Tab:      "Definir Contextos",
			},
			{
				// "IA: SKILLs" mode (#agentskills-container): shortcuts to install
				// third-party skills.sh skills (internal/dev/agentskills, `npx skills
				// add/remove/update`), separate from internal/manager/skills, which
				// copies Perci's own templates.
				Title:    "IA: SKILLs",
				Desc:     "Atalhos para instalar skills de agente de IA disponibilizadas pelo skills.sh",
				ActionID: "agentskills",
				Tab:      "SKILLs",
			},
			{
				// "IA: MCPs" mode (#mcpservers-container): registers MCP servers with
				// Claude Code/Codex/Antigravity (internal/dev/mcpservers) — a per-agent
				// mechanism.
				Title:    "IA: MCPs",
				Desc:     "Registre servidores MCP para Claude Code, Codex e Antigravity.",
				ActionID: "mcpservers",
				Tab:      "MCPs",
			},
		},
	},
}

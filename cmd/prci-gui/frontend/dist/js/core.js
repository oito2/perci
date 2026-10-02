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

// Shared state and plumbing: DOM references, compact (tray) window, theme, sidebar logo/collapse, tabs, the xterm terminal and the Execução run lifecycle (startExecution, log-line/step/action-done events).
//
// Classic script (not a module): shares one global scope with the other
// scripts under js/.
"use strict";

// Any rejected backend call nobody handled (a Go method returning an error,
// a failed dialog...) is shown instead of vanishing into the webview's
// console. Run-starting calls handle their own (failRun).
window.addEventListener("unhandledrejection", function (ev) {
  ev.preventDefault();
  const reason = ev.reason;
  openError("Erro inesperado: " + ((reason && reason.message) || reason));
});


const breadcrumb = document.getElementById("breadcrumb");
const tabDescLabel = document.getElementById("tab-desc-label");
const contentDesc = document.getElementById("content-desc");
const simpleSystemLabel = document.getElementById("simple-system-label");
const accordionEl = document.getElementById("accordion");
const tabRadioDesc = document.getElementById("tab-radio-desc");
const tabRadioTerminal = document.getElementById("tab-radio-terminal");
const sidebar = document.getElementById("sidebar");

// --- compact window (system tray: a single window with tabs) -----------
// Opened with ?compact=<tab>. Sidebar/breadcrumb are hidden and the
// compact header (image + "Perci" + "Abrir Aplicativo") and the
// Contêineres/Repositórios/Atualizar Sistema tab bar are shown instead;
// the rest of the screen (Definições/Execução tabs, containers,
// render*Screen) is shared with the main window — the tabs just pick, via
// selectItem, WHICH item to show. The "compact-switch-tab" event (see
// Events.On below) switches the tab of an already open window.
const compactAction = new URLSearchParams(location.search).get("compact");
const drawerSideEl = document.querySelector(".drawer-side");
const mobileTopbar = document.getElementById("mobile-topbar");
const normalHeaderRow = document.getElementById("normal-header-row");
const compactHeader = document.getElementById("compact-header");
const compactHeaderIcon = document.getElementById("compact-header-icon");
const btnCompactOpenApp = document.getElementById("btn-compact-open-app");
const compactTabsEl = document.getElementById("compact-tabs");
const compactTabRadios = {
  "docker-manage": document.getElementById("compact-tab-docker-manage"),
  "repos": document.getElementById("compact-tab-repos"),
  "system-update": document.getElementById("compact-tab-system-update"),
};
let compactCats = null; // catalog (App.GetCategories()) cached to switch tabs without reloading

if (compactAction) {
  drawerSideEl.classList.add("hidden");
  mobileTopbar.classList.add("hidden");
  normalHeaderRow.classList.add("hidden");
  compactHeader.classList.remove("hidden");
  compactHeader.classList.add("flex");
  compactTabsEl.classList.remove("hidden");
  // #dashboard-container is the initial state, visible by default (no
  // class="hidden" in the HTML) — hidden right here, even before
  // App.GetCategories() resolves, so the full dashboard doesn't flash
  // in a compact window that's about to show something else.
  document.getElementById("dashboard-container").classList.add("hidden");
}

btnCompactOpenApp.addEventListener("click", function () {
  App.FocusMainWindow();
});

// Finds, in the catalog already loaded by App.GetCategories(), the item
// that corresponds to a tab — always by actionId (the compact window's
// 3 tabs use the same actionId key as the item: "docker-manage",
// "repos", "system-update").
function findCompactTarget(cats, tab) {
  for (var catIdx = 0; catIdx < cats.length; catIdx++) {
    var cat = cats[catIdx];
    for (var itemIdx = 0; itemIdx < cat.items.length; itemIdx++) {
      var item = cat.items[itemIdx];
      if (item.actionId === tab) {
        return { cat: cat, item: item, catIdx: catIdx, itemIdx: itemIdx };
      }
    }
  }
  return null;
}

// Switches the compact window's active tab — checks the right radio and
// calls the existing selectItem to build the content, both on initial
// load (?compact=<tab>) and when the user clicks a tab, and when the
// "compact-switch-tab" event arrives (window already open, another tray
// item clicked).
function switchCompactTab(tab) {
  if (!compactCats) return;
  const target = findCompactTarget(compactCats, tab);
  if (!target) return;
  const radio = compactTabRadios[tab];
  if (radio) radio.checked = true;
  selectItem(target.cat, target.item, target.catIdx, target.itemIdx);
}

Object.keys(compactTabRadios).forEach(function (tab) {
  compactTabRadios[tab].addEventListener("change", function () {
    if (compactTabRadios[tab].checked) switchCompactTab(tab);
  });
});

Events.On("compact-switch-tab", function (ev) {
  switchCompactTab(ev.data);
});

// Activates the requested compact window — called after
// App.GetCategories() resolves (selectItem depends on the loaded
// catalog, the same dependency a normal sidebar click already has).
function activateCompactMode(cats) {
  compactCats = cats;
  switchCompactTab(compactAction);
}

// --- theme -----------------------------------------------------------------
// Applies the saved theme (data-theme) on load; the swatch grid is built
// by renderSelfConfigScreen.
function applyTheme(name) {
  document.documentElement.setAttribute("data-theme", name);
  // If the logo "easter egg" (below) already has an active mask, its
  // color depends on the theme — reapply it right away, without
  // waiting for a new click.
  if (sidebarLogoMaskIndex >= 0) logoImgEl.style.backgroundColor = sidebarLogoMaskColor();
}

App.GetTheme().then(applyTheme);

// --- sidebar logo --------------------------------------------------------
// Applies the saved sidebar logo on load; the clickable grid is built by
// renderSelfConfigScreen.
App.GetSidebarLogo().then(function (name) {
  sidebarLogoImg.src = "./assets/perci-" + name + ".png";
  compactHeaderIcon.src = "./assets/perci-" + name + ".png";
});

// --- sidebar logo easter egg: each click on the image applies a
// different daisyUI mask, cycling through a fixed list of shapes. Both
// mascot variants (perci-blue.png/perci-pink.png) have a transparent
// background, so coloring the <img> itself (background-color) shows that
// color clipped exactly to the mask's shape. Color per theme:
// "synthwave"/"retro"/"valentine"/"halloween"/"garden" use the theme's
// primary color (`var(--color-primary)`); every other theme uses the
// fixed pink #FEC5C4. Purely client-side, no persistence — resets (back
// to the original circle) every time Perci reopens. Uses
// `document.getElementById` (not the `sidebarLogoImg` const declared
// further below) because this snippet runs before that const exists;
// `logoImgEl` has its own name so it doesn't collide with that const. ---
const SIDEBAR_LOGO_MASKS = [
  "mask-squircle", "mask-heart", "mask-hexagon", "mask-hexagon-2",
  "mask-decagon", "mask-pentagon", "mask-diamond", "mask-star",
  "mask-star-2", "mask-triangle", "mask-circle",
];
const SIDEBAR_LOGO_PRIMARY_THEMES = ["synthwave", "retro", "valentine", "halloween", "garden"];
let sidebarLogoMaskIndex = -1; // -1 = still the original circle, no mask
const logoImgEl = document.getElementById("sidebar-logo-img");

function sidebarLogoMaskColor() {
  const theme = document.documentElement.getAttribute("data-theme");
  return SIDEBAR_LOGO_PRIMARY_THEMES.indexOf(theme) >= 0 ? "var(--color-primary)" : "#FEC5C4";
}

function applySidebarLogoMask() {
  logoImgEl.classList.remove("rounded-full", "mask", ...SIDEBAR_LOGO_MASKS);
  if (sidebarLogoMaskIndex < 0) {
    logoImgEl.classList.add("rounded-full");
    logoImgEl.style.backgroundColor = "";
    return;
  }
  logoImgEl.classList.add("mask", SIDEBAR_LOGO_MASKS[sidebarLogoMaskIndex]);
  logoImgEl.style.backgroundColor = sidebarLogoMaskColor();
}

// Keyboard: the logo is focusable and Enter/Space act as a click.
logoImgEl.addEventListener("keydown", function (ev) {
  if (ev.key === "Enter" || ev.key === " ") {
    ev.preventDefault();
    logoImgEl.click();
  }
});
logoImgEl.addEventListener("click", function () {
  sidebarLogoMaskIndex = (sidebarLogoMaskIndex + 1) % SIDEBAR_LOGO_MASKS.length;
  applySidebarLogoMask();
});

// --- collapse sidebar -------------------------------------------------------
document.getElementById("btn-collapse-sidebar").addEventListener("click", function () {
  const collapsed = sidebar.getAttribute("data-collapsed") === "true";
  sidebar.setAttribute("data-collapsed", collapsed ? "false" : "true");
});

// --- tabs (Definições / Execução) — each tab-content's visibility is
// handled via CSS by the matching radio's :checked; the code only checks
// the right radio. --
function selectTab(which) {
  (which === "desc" ? tabRadioDesc : tabRadioTerminal).checked = true;
}

// --- terminal + run lifecycle ---------------------------------------------
// getPostinstallProfile memoizes App.GetPostinstallProfile — the detected
// distribution can't change while Perci runs, and three screens read it.
// A failed call isn't cached.
let postinstallProfilePromise = null;
function getPostinstallProfile() {
  if (!postinstallProfilePromise) {
    postinstallProfilePromise = App.GetPostinstallProfile().catch(function (err) {
      postinstallProfilePromise = null;
      throw err;
    });
  }
  return postinstallProfilePromise;
}

const term = new Terminal({
  fontSize: 13,
  convertEol: true,
  disableStdin: true,
  allowTransparency: true, // needed for the transparent background below
  scrollback: 5000,
  theme: { background: "#00000000" },
});
const terminalEl = document.getElementById("terminal");
term.open(terminalEl);
term.writeln("Nenhuma ação em execução.");

// FitAddon sizes the terminal to its panel
// instead of xterm's fixed 80x24. The observer also fires when the
// Execução tab becomes visible (0 → real size); a hidden panel is skipped.
const termFit = new FitAddon.FitAddon();
term.loadAddon(termFit);
new ResizeObserver(function () {
  if (terminalEl.clientWidth > 0 && terminalEl.clientHeight > 0) termFit.fit();
}).observe(terminalEl);

const btnExecutar = document.getElementById("btn-executar");
const executarHint = document.getElementById("executar-hint");
const termTitle = document.getElementById("term-title");
const termProgress = document.getElementById("term-progress");
const simpleContainer = document.getElementById("simple-container");
const postinstallContainer = document.getElementById("postinstall-container");
const postinstallUnsupported = document.getElementById("postinstall-unsupported");
const postinstallSupported = document.getElementById("postinstall-supported");
const postinstallTitle = document.getElementById("postinstall-title");
const postinstallActionsList = document.getElementById("postinstall-actions");
const btnExecutarPostinstall = document.getElementById("btn-executar-postinstall");
const multiselectContainer = document.getElementById("multiselect-container");
const multiselectIntro = document.getElementById("multiselect-intro");
const multiselectItemsList = document.getElementById("multiselect-items");
const btnExecutarMultiselect = document.getElementById("btn-executar-multiselect");
const multiselectExtras = document.getElementById("multiselect-extras");
// Buttons built from MULTISELECT_SCREENS[actionId].extras for the screen
// currently shown (renderMultiselectScreen).
let multiselectExtraButtons = [];
const singleappContainer = document.getElementById("singleapp-container");
const singleappDesc = document.getElementById("singleapp-desc");
const btnSingleappInstall = document.getElementById("btn-singleapp-install");
const btnSingleappUninstall = document.getElementById("btn-singleapp-uninstall");
const ideContainer = document.getElementById("ide-container");
const ideDesc = document.getElementById("ide-desc");
const btnIdeInstall = document.getElementById("btn-ide-install");
const btnIdeUpdate = document.getElementById("btn-ide-update");
const btnIdeUninstall = document.getElementById("btn-ide-uninstall");
const fileappContainer = document.getElementById("fileapp-container");
const fileappDesc = document.getElementById("fileapp-desc");
const btnFileappPick = document.getElementById("btn-fileapp-pick");
const fileappPath = document.getElementById("fileapp-path");
const btnFileappInstall = document.getElementById("btn-fileapp-install");
const btnFileappUpdate = document.getElementById("btn-fileapp-update");
const btnFileappUninstall = document.getElementById("btn-fileapp-uninstall");
const fileappNotes = document.getElementById("fileapp-notes");
const dockercreateContainer = document.getElementById("dockercreate-container");
const dcKind = document.getElementById("dc-kind");
const dcAppTypeWrap = document.getElementById("dc-apptype-wrap");
const dcAppType = document.getElementById("dc-apptype");
const dcFields = document.getElementById("dc-fields");
const btnDcCreate = document.getElementById("btn-dc-create");
const dcHint = document.getElementById("dc-hint");
const dockerlistContainer = document.getElementById("dockerlist-container");
const dockerlistRows = document.getElementById("dockerlist-rows");
const dockerlistEmpty = document.getElementById("dockerlist-empty");
const btnDockerExport = document.getElementById("btn-docker-export");
const btnDockerImport = document.getElementById("btn-docker-import");
const confirmModal = document.getElementById("confirm-modal");
const confirmModalTitle = document.getElementById("confirm-modal-title");
const confirmModalMessage = document.getElementById("confirm-modal-message");
const btnConfirmOk = document.getElementById("btn-confirm-ok");
const errorModal = document.getElementById("error-modal");
const errorModalMessage = document.getElementById("error-modal-message");
const logsModal = document.getElementById("logs-modal");
const logsModalTitle = document.getElementById("logs-modal-title");
const logsModalContent = document.getElementById("logs-modal-content");
const btnLogsRefresh = document.getElementById("btn-logs-refresh");
const btnLogsExport = document.getElementById("btn-logs-export");
const editModal = document.getElementById("edit-modal");
const editModalTitle = document.getElementById("edit-modal-title");
const editFields = document.getElementById("edit-fields");
const btnEditSave = document.getElementById("btn-edit-save");
const reposContainer = document.getElementById("repos-container");
const reposIdentityCheckboxWrap = document.getElementById("repos-identity-checkbox-wrap");
const reposIdentityChange = document.getElementById("repos-identity-change");
const reposIdentityName = document.getElementById("repos-identity-name");
const reposIdentityEmail = document.getElementById("repos-identity-email");
const btnReposIdentityApply = document.getElementById("btn-repos-identity-apply");
const tabLabelReposList = document.getElementById("tab-label-repos-list");
const tabRadioReposList = document.getElementById("tab-radio-repos-list");
const tabLabelTerminal = document.getElementById("tab-label-terminal");
const reposListCards = document.getElementById("repos-list-cards");
const reposFolderAlertText = document.getElementById("repos-folder-alert-text");
const reposSubmenu = document.getElementById("repos-submenu");
const reposSubPanel = document.getElementById("repos-sub-panel");
const agentskillsContainer = document.getElementById("agentskills-container");
const asScopeSelect = document.getElementById("as-scope");
const asScopeAlertTitle = document.getElementById("as-scope-alert-title");
const asScopeAlertDesc = document.getElementById("as-scope-alert-desc");
const btnAsPickFolder = document.getElementById("btn-as-pick-folder");
const asRows = document.getElementById("as-rows");
const asSkillsShLink = document.getElementById("as-skills-sh-link");
const aicontextContainer = document.getElementById("aicontext-container");
const aicAlertTitle = document.getElementById("aic-alert-title");
const aicAlertDesc = document.getElementById("aic-alert-desc");
const btnAicPickFolder = document.getElementById("btn-aic-pick-folder");
const aicItems = document.getElementById("aic-items");
const btnAicApply = document.getElementById("btn-aic-apply");
const mcpserversContainer = document.getElementById("mcpservers-container");
const mcpScopeSelect = document.getElementById("mcp-scope");
const mcpScopeAlertTitle = document.getElementById("mcp-scope-alert-title");
const mcpScopeAlertDesc = document.getElementById("mcp-scope-alert-desc");
const btnMcpPickFolder = document.getElementById("btn-mcp-pick-folder");
const mcpCodexNote = document.getElementById("mcp-codex-note");
const mcpRows = document.getElementById("mcp-rows");
const dashboardContainer = document.getElementById("dashboard-container");
const dashboardSystemLabel = document.getElementById("dashboard-system-label");
const dashboardDockerSummary = document.getElementById("dashboard-docker-summary");
const dashboardVersion = document.getElementById("dashboard-version");
const suStatus = document.getElementById("su-status");
const btnSuCheck = document.getElementById("btn-su-check");
const btnSuUpdate = document.getElementById("btn-su-update");
const su2BackupConfig = document.getElementById("su2-backup-config");
const su2RemoveDocker = document.getElementById("su2-remove-docker");
const su2RemoveConfig = document.getElementById("su2-remove-config");
const btnSu2Uninstall = document.getElementById("btn-su2-uninstall");
const selfconfigContainer = document.getElementById("selfconfig-container");
const cfgThemeGrid = document.getElementById("cfg-theme-grid");
const cfgLogoGrid = document.getElementById("cfg-logo-grid");
const cfgIconGrid = document.getElementById("cfg-icon-grid");
const scWorkspaceAlertTitle = document.getElementById("sc-workspace-alert-title");
const scWorkspaceAlertDesc = document.getElementById("sc-workspace-alert-desc");
const btnScPickWorkspace = document.getElementById("btn-sc-pick-workspace");
const btnScCreateWorkspace = document.getElementById("btn-sc-create-workspace");
const scFlatpak = document.getElementById("sc-flatpak");
const scTrayEnabled = document.getElementById("sc-tray-enabled");
const sidebarLogoImg = document.getElementById("sidebar-logo-img");

// ACTIONS maps each catalog actionId to the corresponding bound method
// — every real screen gets an entry here.
const suOptJournal = document.getElementById("su-opt-journal");
const suOptAutoremove = document.getElementById("su-opt-autoremove");
const ACTIONS = {
  "system-update": function () { return App.RunSystemUpdate(suOptJournal.checked, suOptAutoremove.checked); },
};

let running = false;

// activeRun describes the action THIS window started (startExecution):
// the actionId of the screen it was started from and the
// buttons it disabled. null when this window isn't running anything — the
// log-line/step/action-done events reach every window (main + the tray's
// compact window), and only the one that started the action handles them.
let activeRun = null;

// Events.On delivers the payload inside ev.data.
// Output is written once per frame instead of once per event: apt or
// docker build emit thousands of small events.
let pendingLog = "";
Events.On("log-line", function (ev) {
  if (!activeRun) return;
  if (!pendingLog) requestAnimationFrame(flushLog);
  pendingLog += ev.data;
});
function flushLog() {
  if (pendingLog) term.write(pendingLog);
  pendingLog = "";
}

Events.On("step", function (ev) {
  if (!activeRun) return;
  const step = ev.data;
  termTitle.textContent = "Executando: " + step.label;
  termProgress.value = step.index;
  termProgress.max = step.total;
});

Events.On("action-done", function (ev) {
  if (!activeRun) return;
  finishRun(ev.data);
});

// failRun ends this window's run when the backend refused to start it
// ("outra ação já está em execução") or the call itself failed — no
// "action-done" event comes in that case. Every App call that follows a
// startExecution() chains .catch(failRun).
function failRun(err) {
  if (!activeRun) return;
  finishRun({ ok: false, error: String((err && err.message) || err) });
}

// finishRun closes the run lifecycle: terminal status, then every button
// startExecution disabled is released (whatever screen is showing now —
// the sidebar isn't locked during a run, and those buttons are shared by
// several screens), then the screen showing now refreshes its own state
// (ACTION_DONE_HANDLERS). The screen the run started from re-renders
// anyway the next time it's selected.
function finishRun(result) {
  flushLog(); // output still waiting for its frame comes before the status line
  const run = activeRun;
  activeRun = null;
  running = false;
  term.writeln("");
  term.writeln(result.ok ? "--- concluído com sucesso ---" : "--- falhou: " + result.error + " ---");
  termTitle.textContent = result.ok ? "Concluído." : "Falhou.";
  if (result.ok) termProgress.value = termProgress.max;
  run.buttons.forEach(function (b) { b.disabled = false; });
  const current = window.__selectedItem && window.__selectedItem.actionId;
  doneHandlerFor(current)(run);
  if (run.actionId === "dashboard") dashboardRunningAction = null;
}

// Per-screen refresh after a run, keyed by actionId. Screens that share one
// container (checklists, single apps, IDEs, local-file apps) resolve by
// family in doneHandlerFor; anything else just re-evaluates "Executar".
const ACTION_DONE_HANDLERS = {
  postinstall: function () { btnExecutarPostinstall.disabled = false; },
  // Creating Nginx/MariaDB may have just disabled that option in the select
  // (singleton) — re-fetches the whole catalog.
  "docker-create": function () { renderDockerCreateScreen(); },
  // Recriar/Editar/Backup/Restore/Importar — re-fetches the whole table
  // (status + whatever Import may have created/changed).
  "docker-manage": function () { renderDockerListScreen(); },
  // Identidade Global/Clonar/Iniciar/Identidade Local/Arquivos — re-fetches
  // the global identity and the working folder's state.
  repos: function () {
    renderReposIdentityTab();
    if (reposFolderPath) refreshReposFolderState();
  },
  agentskills: function () { renderAgentSkillsTable(); },
  aicontext: function () { if (aicFolder) renderAicItems(); },
  mcpservers: function () { renderMCPServersTable(); },
  // "Atualizar Perci" and "Desinstalar Perci" both live in Visão Geral —
  // dashboardRunningAction (set before each one) says which one finished.
  // A successful uninstall closes the window on its own.
  dashboard: function () {
    if (dashboardRunningAction === "self-update") loadSelfUpdateStatus(true);
  },
};

function doneHandlerFor(actionId) {
  if (ACTION_DONE_HANDLERS[actionId]) return ACTION_DONE_HANDLERS[actionId];
  if (MULTISELECT_SCREENS[actionId]) {
    return function () {
      btnExecutarMultiselect.disabled = false;
      multiselectExtraButtons.forEach(function (btn) { btn.disabled = false; });
    };
  }
  if (SINGLE_APP_SCREENS[actionId]) {
    // Re-fetches the installed state: after Instalar, Desinstalar should
    // become enabled (and Instalar disabled), and vice versa.
    return function () {
      SINGLE_APP_SCREENS[actionId].fetch().then(function (info) {
        btnSingleappInstall.disabled = info.installed;
        btnSingleappUninstall.disabled = !info.installed;
      });
    };
  }
  if (IDE_SCREENS[actionId]) {
    return function () {
      ideFetch(IDE_SCREENS[actionId].cmd).then(function (info) {
        btnIdeInstall.disabled = false;
        btnIdeUpdate.disabled = !info.installed;
        btnIdeUninstall.disabled = !info.installed;
      });
    };
  }
  if (FILEAPP_SCREENS[actionId]) {
    return function () {
      const cfg = FILEAPP_SCREENS[actionId];
      const hasPath = !!window.__fileappPath;
      cfg.fetch().then(function (info) {
        btnFileappInstall.disabled = !hasPath;
        if (cfg.update) btnFileappUpdate.disabled = !hasPath;
        btnFileappUninstall.disabled = !info.installed;
      });
    };
  }
  return updateExecutarButton;
}

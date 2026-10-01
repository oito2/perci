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

// Home :: Visão Geral (dashboard, Atualizar/Desinstalar Perci) and Home :: Configurações.
//
// Classic script (not a module): every file under js/ shares one global
// scope, loaded in order by index.html — see js/bootstrap.js.
"use strict";

// --- "Home :: Visão Geral" (dashboard — also the screen's initial
// state: #dashboard-container is visible by default in the HTML, without
// going through selectItem). Brings together 3 blocks: the stats (reusing
// bindings that already existed for other screens — GetAppVersion,
// GetPostinstallProfile, GetContainerRows — no new endpoint was created
// just for them), "Atualizar Perci" and "Desinstalar Perci" — the latter
// two used to be their own menu items until they were moved into Visão
// Geral (in place of the per-category shortcuts that lived there before)
// and removed from the main menu (catalog.go). dashboardRunningAction
// marks which of the two embedded actions is in progress — App's
// "action-done" event only has the actionId "dashboard" to go on (both
// now live on the same screen), so this extra flag is needed to know what
// to reload. ---------------------------------------------------------
let dashboardRunningAction = null;

function renderDashboardStats() {
  App.GetAppVersion().then(function (v) {
    dashboardVersion.textContent = v;
  });
  getPostinstallProfile().then(function (profile) {
    dashboardSystemLabel.textContent = (profile && profile.supported) ? profile.label : "Não identificado";
  });
  App.GetContainerRows().then(function (rows) {
    rows = rows || [];
    if (rows.length === 0) {
      dashboardDockerSummary.textContent = "Nenhum";
      return;
    }
    const runningCount = rows.filter(function (r) { return r.status === "running"; }).length;
    dashboardDockerSummary.textContent = runningCount + " / " + rows.length;
  });
}

// --- "Atualizar Perci" (embedded in Visão Geral) ------------------------
// Reopening Visão Geral re-checks GitHub at most every 10 minutes; force
// (the "Verificar novamente" button, after an update) always does.
const SELF_UPDATE_RECHECK_MS = 10 * 60 * 1000;
let selfUpdateCheckedAt = 0;
function loadSelfUpdateStatus(force) {
  if (!force && selfUpdateCheckedAt && Date.now() - selfUpdateCheckedAt < SELF_UPDATE_RECHECK_MS) return;
  selfUpdateCheckedAt = Date.now();
  suStatus.textContent = "Verificando...";
  btnSuUpdate.disabled = true;
  App.GetSelfUpdateInfo().then(function (info) {
    if (info.error) {
      selfUpdateCheckedAt = 0; // retried on the next visit
      suStatus.textContent = "Não foi possível verificar atualizações: " + info.error;
      return;
    }
    if (info.updateAvailable) {
      suStatus.textContent = "Existe uma nova versão do Perci disponível (" + info.latestVersion + ").";
      btnSuUpdate.disabled = false;
    } else {
      suStatus.textContent = "O Perci está atualizado.";
    }
  });
}

btnSuCheck.addEventListener("click", function () {
  if (running) return;
  loadSelfUpdateStatus(true);
});

btnSuUpdate.addEventListener("click", function () {
  if (running) return;
  dashboardRunningAction = "self-update";
  startExecution("Atualizar Perci", [btnSuUpdate, btnSuCheck], 1, "Iniciando atualização do Perci...");
  App.RunSelfUpdate().catch(failRun);
});

// --- "Desinstalar Perci" (embedded in Visão Geral) — three independent
// checkboxes (also remove config / remove Docker containers / back up
// config before uninstalling), a "Desinstalar" button after them. The
// backup reuses the same "Exportar configurações" that "Docker ::
// Gerenciar Containers" already uses (appstack.ExportConfig, via
// App.PickUninstallBackupPath + the path passed to RunSelfUninstall) — if
// the user checks that option and then cancels the "Salvar Como" dialog,
// the whole uninstall is aborted instead of proceeding without the
// requested backup. Success closes the GUI's own window
// (App.RunSelfUninstall handles that). -------------------------------
btnSu2Uninstall.addEventListener("click", function () {
  if (running) return;
  const removeConfig = su2RemoveConfig.checked;
  const removeDocker = su2RemoveDocker.checked;
  const backupConfig = su2BackupConfig.checked;

  let message = "O binário do Perci será removido do sistema.";
  if (backupConfig) message += "\nUm backup das configurações será salvo antes.";
  if (removeDocker) message += "\nOs contêineres Docker registrados serão removidos (dados em disco são mantidos).";
  message += removeConfig
    ? "\nAs configurações em ~/.perci também serão removidas."
    : "\nAs configurações em ~/.perci serão mantidas.";
  message += "\n\nEsta ação não pode ser desfeita.";

  openConfirm("Desinstalar o Perci?", message, function () {
    function startUninstall(backupPath) {
      if (running) return; // re-checked: the dialogs are async
      dashboardRunningAction = "self-uninstall";
      startExecution("Desinstalar", [btnSu2Uninstall], 1, "Desinstalando o Perci...");
      App.RunSelfUninstall(removeConfig, removeDocker, backupPath || "").catch(failRun);
    }

    if (!backupConfig) {
      startUninstall("");
      return;
    }
    App.PickUninstallBackupPath().then(function (path) {
      if (!path) {
        openMessage("Desinstalação cancelada", "Nenhum caminho foi escolhido para o backup — a desinstalação não foi iniciada.", false);
        return;
      }
      startUninstall(path);
    });
  });
});

// The dashboard is the main window's initial screen; the tray's compact
// window never shows it.
if (!compactAction) {
  renderDashboardStats();
  loadSelfUpdateStatus();
}

// --- "Home :: Configurar" (self-config) ---------------------------------
// Generic button grid with an image above the label — reused for Logo
// da Sidebar and Ícone do Aplicativo (same visual pattern as the theme
// grid, just with a preview instead of only the name).
function renderImagePickerGrid(container, values, current, srcFor, onPick) {
  container.innerHTML = "";
  values.forEach(function (value) {
    const btn = document.createElement("button");
    btn.type = "button";
    btn.className = "btn btn-ghost h-auto flex-col items-center gap-1 p-2" + (value === current ? " btn-active" : "");
    const img = document.createElement("img");
    img.src = srcFor(value);
    img.alt = value;
    img.className = "w-12 h-12 object-contain rounded";
    const span = document.createElement("span");
    span.className = "text-xs capitalize";
    span.textContent = value;
    btn.appendChild(img);
    btn.appendChild(span);
    btn.addEventListener("click", function () { onPick(value); });
    container.appendChild(btn);
  });
}

// Tema — the same swatches that used to live in the gear icon's
// "Configurações" modal (now removed); a click applies and persists
// right away, with no "Salvar" button involved.
function renderThemeGrid(current) {
  cfgThemeGrid.innerHTML = "";
  App.GetGUIThemes().then(function (themes) {
    themes.forEach(function (name) {
      const btn = document.createElement("button");
      btn.type = "button";
      btn.className = "btn btn-sm justify-start" + (name === current ? " btn-active" : "");
      btn.textContent = name;
      btn.addEventListener("click", function () {
        applyTheme(name);
        App.SetTheme(name).catch(function (err) {
          openError("Falha ao salvar o tema: " + err);
        });
        renderThemeGrid(name);
      });
      cfgThemeGrid.appendChild(btn);
    });
  });
}

// Logo da Sidebar — a click swaps the sidebar image (#sidebar-logo-img)
// and persists right away, same pattern as the theme — it's just a
// visual preference, no restart needed.
function renderLogoGrid(current) {
  App.GetSidebarLogos().then(function (values) {
    renderImagePickerGrid(
      cfgLogoGrid, values, current,
      function (v) { return "./assets/perci-" + v + ".png"; },
      function (value) {
        sidebarLogoImg.src = "./assets/perci-" + value + ".png";
        App.SetSidebarLogo(value).catch(function (err) {
          openError("Falha ao salvar o logo da sidebar: " + err);
        });
        renderLogoGrid(value);
      }
    );
  });
}

// Ícone do Aplicativo — persists right away like the two above, but the
// window icon can't be applied in this session: it's only read once, at
// Wails startup (cmd/prci-gui/main.go, before Window.NewWithOptions —
// Wails' API has no "swap the icon live" call). The application menu
// icon, when installed, is replaced by SetAppIcon itself (one pkexec
// prompt). Hence the fixed notice below the grid (see HTML). On failure
// (e.g. a cancelled prompt) nothing was saved, so the grid goes back to
// the persisted value.
function renderIconGrid(current) {
  App.GetAppIcons().then(function (values) {
    renderImagePickerGrid(
      cfgIconGrid, values, current,
      function (v) { return "./assets/perci-" + v + ".ico"; },
      function (value) {
        App.SetAppIcon(value).catch(function (err) {
          openError("Falha ao salvar o ícone do aplicativo: " + err);
          App.GetAppIcon().then(renderIconGrid);
        });
        renderIconGrid(value);
      }
    );
  });
}

// Caminho do Workspace — same alert-card pattern as "Dev Tools :: IA:
// Contextos", but unlike that screen (which always starts over with no
// folder), this card reflects the value already persisted in
// config.yaml as soon as the screen opens.
let scWorkspacePath = "";       // persisted value (config.yaml)
let scPickedWorkspacePath = ""; // picked this session, not saved yet

function renderScWorkspaceAlert() {
  if (scWorkspacePath) {
    scWorkspaceAlertTitle.textContent = "O workspace está criado";
    scWorkspaceAlertDesc.textContent = scWorkspacePath;
    btnScCreateWorkspace.classList.add("hidden");
    return;
  }
  scWorkspaceAlertTitle.textContent = "O workspace não foi criado";
  scWorkspaceAlertDesc.textContent = scPickedWorkspacePath || "Selecione uma pasta";
  btnScCreateWorkspace.classList.toggle("hidden", !scPickedWorkspacePath);
}

btnScPickWorkspace.addEventListener("click", function () {
  App.PickWorkspaceFolder().then(function (path) {
    if (!path) return; // user cancelled the dialog
    scPickedWorkspacePath = path;
    renderScWorkspaceAlert();
  });
});

btnScCreateWorkspace.addEventListener("click", function () {
  if (!scPickedWorkspacePath) return;
  btnScCreateWorkspace.disabled = true;
  App.SetWorkspacePath(scPickedWorkspacePath).then(function () {
    scWorkspacePath = scPickedWorkspacePath;
    scPickedWorkspacePath = "";
    btnScCreateWorkspace.disabled = false;
    renderScWorkspaceAlert();
  }).catch(function (err) {
    btnScCreateWorkspace.disabled = false;
    openError("Falha ao criar workspace: " + err);
  });
});

// Escopo de instalação Flatpak — applies right away, like Tema/Logo/
// Ícone, with no "Salvar" button at all.
scFlatpak.addEventListener("change", function () {
  App.SetFlatpakScope(scFlatpak.value).catch(function (err) {
    openError("Falha ao salvar o escopo do Flatpak: " + err);
  });
});

// Bandeja do sistema — applies right away, same "no Salvar button"
// convention as the rest of the screen; unlike SetAppIcon, SetTrayEnabled
// has real immediate effect (SystemTray.New()/Destroy() at runtime, not
// just on the next launch). Reverts the checkbox if the call fails, so
// it doesn't lie about the actual state.
scTrayEnabled.addEventListener("change", function () {
  const enabled = scTrayEnabled.checked;
  App.SetTrayEnabled(enabled).catch(function (err) {
    scTrayEnabled.checked = !enabled;
    openError("Falha ao " + (enabled ? "ligar" : "desligar") + " a bandeja do sistema: " + err);
  });
});

function renderSelfConfigScreen() {
  App.GetTheme().then(renderThemeGrid);
  App.GetSidebarLogo().then(renderLogoGrid);
  App.GetAppIcon().then(renderIconGrid);
  App.GetTrayEnabled().then(function (enabled) {
    scTrayEnabled.checked = enabled;
  });

  scPickedWorkspacePath = "";
  App.GetSelfConfigInfo().then(function (info) {
    scWorkspacePath = info.workspacePath || "";
    renderScWorkspaceAlert();
    scFlatpak.value = info.flatpakScope === "user" ? "user" : "system";
  });
}

// Re-runs the whole Visão Geral screen — called when "Visão Geral" is
// clicked again in the sidebar (the buttons' click listeners were already
// attached once, outside selectItem's dispatch, since this screen isn't
// dynamically recreated like the others).
function renderDashboardScreen() {
  renderDashboardStats();
  loadSelfUpdateStatus();
}

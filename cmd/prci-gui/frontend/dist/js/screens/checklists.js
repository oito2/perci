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

// Checklist screens (fonts, templates, apps...), single-app install/uninstall screens, IDE screens and the local-file app screens (Android Studio, Antigravity IDE).
//
// Classic script (not a module): shares one global scope with the other
// scripts under js/.
"use strict";

// --- "checklist simples" screens (installed ⇄ unchecked, no dependency
// between items) — MULTISELECT_SCREENS maps actionId to the two bound
// methods the screen needs: fetch() brings the items with their current
// state (installed), run(ids) executes whatever got checked. Each new
// screen of this type only needs one entry here. -----------------------
const MULTISELECT_SCREENS = {
  fonts: {
    fetch: function () { return App.GetFontsInfo(); },
    run: function (ids) { return App.RunFonts(ids); },
  },
  templates: {
    fetch: function () { return App.GetTemplatesInfo(); },
    run: function (ids) { return App.RunTemplates(ids); },
  },
  "flatpak-apps": {
    fetch: function () { return App.GetFlatpakAppsInfo(); },
    run: function (ids) { return App.RunFlatpakApps(ids); },
  },
  prereqs: {
    fetch: function () { return App.GetPrereqsInfo(); },
    run: function (ids) { return App.RunPrereqs(ids); },
  },
  sdks: {
    fetch: function () { return App.GetSDKsInfo(); },
    run: function (ids) { return App.RunSDKs(ids); },
  },
  "ai-apps": {
    fetch: function () { return App.GetAIAppsInfo(); },
    run: function (ids) { return App.RunAIApps(ids); },
  },
  terminals: {
    fetch: function () { return App.GetTerminalsInfo(); },
    run: function (ids) { return App.RunTerminals(ids); },
    // Extra buttons — not part of the checklist (Starship isn't a
    // checklist item), run
    // independent of selection. confirm (optional) is asked first.
    extras: [
      {
        label: "Aplicar Starship",
        run: function () { return App.ApplyStarship(); },
      },
      {
        label: "Remover Starship",
        confirm: "Remove o binário do Starship e a inicialização dele no bash, zsh e fish. " +
          "A configuração (~/.config/starship.toml) é preservada como starship.toml.perci-bak.",
        run: function () { return App.RemoveStarship(); },
      },
    ],
  },
};

// buildChecklistItemLI builds the <li> (checkbox + label [+ description])
// shared by the 3 "checklist simples" variants below (multiselect/
// postinstall/aicontext), instead of each one hand-rolling the same DOM
// construction. Returns {li, checkbox} — callers keep the checkbox
// reference from creation time instead of re-querying it later by id
// (avoids repeating getElementById("prefix-" + id) in loops/filters for
// an element created a few lines earlier in the very same pass).
//   idPrefix     — e.g. "ms-check-"/"pi-check-"/"aic-check-"
//   item         — { id, label, description?, installed? }
//   opts.checked        — overrides item.installed when set
//   opts.withDescription — wraps label (+ description, when present) in a <div>
//   opts.onChange        — extra "change" listener on the checkbox
function buildChecklistItemLI(idPrefix, item, opts) {
  opts = opts || {};
  const li = document.createElement("li");
  li.className = "flex items-start gap-3";

  const cb = document.createElement("input");
  cb.type = "checkbox";
  cb.className = "checkbox checkbox-sm mt-1";
  cb.id = idPrefix + item.id;
  cb.checked = opts.checked !== undefined ? opts.checked : !!item.installed;
  if (opts.onChange) cb.addEventListener("change", opts.onChange);

  const label = document.createElement("label");
  label.setAttribute("for", cb.id);
  label.className = "font-medium cursor-pointer";
  label.textContent = item.label;

  li.appendChild(cb);
  if (opts.withDescription) {
    const textWrap = document.createElement("div");
    textWrap.appendChild(label);
    if (item.description) {
      const desc = document.createElement("p");
      desc.className = "text-sm opacity-70";
      desc.textContent = item.description;
      textWrap.appendChild(desc);
    }
    li.appendChild(textWrap);
  } else {
    li.appendChild(label);
  }
  return { li: li, checkbox: cb };
}

// multiselectButtons lists every button of the current checklist screen —
// disabled together while one of its runs is in progress.
function multiselectButtons() {
  return [btnExecutarMultiselect].concat(multiselectExtraButtons);
}

function renderMultiselectScreen(item) {
  multiselectIntro.textContent = item.desc || "";
  multiselectItemsList.innerHTML = "";

  const cfg = MULTISELECT_SCREENS[item.actionId];

  multiselectExtras.innerHTML = "";
  multiselectExtraButtons = (cfg.extras || []).map(function (extra) {
    const btn = document.createElement("button");
    btn.className = "btn btn-outline";
    btn.textContent = extra.label;
    btn.onclick = function () {
      if (running) return;
      const run = function () {
        startExecution(extra.label, multiselectButtons());
        extra.run().catch(failRun);
      };
      if (extra.confirm) {
        openConfirm(extra.label, extra.confirm, run);
        return;
      }
      run();
    };
    multiselectExtras.appendChild(btn);
    return btn;
  });

  cfg.fetch().then(latestOnly("multiselect", function (items) {
    items = items || [];
    const checkboxes = {}; // it.id -> checkbox, captured at creation time
    items.forEach(function (it) {
      const built = buildChecklistItemLI("ms-check-", it, { withDescription: true });
      checkboxes[it.id] = built.checkbox;
      multiselectItemsList.appendChild(built.li);
    });

    btnExecutarMultiselect.onclick = function () {
      if (running) return;
      const selected = items
        .filter(function (it) { return checkboxes[it.id].checked; })
        .map(function (it) { return it.id; });
      const run = function () {
        startExecution(item.title, multiselectButtons());
        cfg.run(selected).catch(failRun);
      };

      // Items whose removal takes more than the item itself (e.g. Node.js
      // deletes ~/.nvm) are confirmed before anything runs.
      const warnings = items
        .filter(function (it) { return it.installed && it.removeWarning && !checkboxes[it.id].checked; })
        .map(function (it) { return it.removeWarning; });
      if (warnings.length) {
        openConfirm("Confirmar remoção", warnings.join("\n\n"), run);
        return;
      }
      run();
    };
  }));
}

// --- "single app install/uninstall" screens: an external app with no
// checklist — two buttons (Instalar always enabled, Desinstalar only if
// already installed). SINGLE_APP_SCREENS maps actionId to the three
// bound methods the screen needs: fetch() brings {installed}, install()
// and uninstall() trigger the action. Each new screen of this type only
// needs one entry here. ----------------------------------------------
const SINGLE_APP_SCREENS = {
  linuxtoys: {
    fetch: function () { return App.GetLinuxToysInfo(); },
    install: function () { return App.InstallLinuxToys(); },
    uninstall: function () { return App.UninstallLinuxToys(); },
  },
  megasync: {
    fetch: function () { return App.GetMegaSyncInfo(); },
    install: function () { return App.InstallMegaSync(); },
    uninstall: function () { return App.UninstallMegaSync(); },
  },
};

function renderSingleAppScreen(item) {
  singleappDesc.textContent = item.desc || "";
  const cfg = SINGLE_APP_SCREENS[item.actionId];
  btnSingleappInstall.disabled = true;
  btnSingleappUninstall.disabled = true;

  cfg.fetch().then(function (info) {
    btnSingleappInstall.disabled = info.installed;
    btnSingleappUninstall.disabled = !info.installed;
  });

  const buttons = [btnSingleappInstall, btnSingleappUninstall];
  btnSingleappInstall.onclick = function () { runScreenAction(item, "instalar", buttons, cfg.install); };
  btnSingleappUninstall.onclick = function () { runScreenAction(item, "desinstalar", buttons, cfg.uninstall); };
}

// runScreenAction starts one of a screen's install/update/uninstall
// actions: nothing while another runs, then the Execução lifecycle with
// the screen's buttons disabled, and fn()'s refusal handled by failRun.
function runScreenAction(item, label, buttons, fn) {
  if (running) return;
  startExecution(item.title, buttons, 1, "Iniciando " + item.title + " (" + label + ")...");
  fn().catch(failRun);
}

// --- "IDE install/atualizar/desinstalar" screens: three buttons —
// Atualizar/Desinstalar enabled only once already installed, same
// backend call for all three, differentiated by the cmd
// parameter. IDE_SCREENS maps actionId ("ide-zed"/
// "ide-code"/"ide-codium") to the cmd the bound methods expect. -------
const IDE_SCREENS = {
  "ide-zed": { cmd: "zed" },
  "ide-code": { cmd: "code" },
  "ide-codium": { cmd: "codium" },
};

function renderIDEScreen(item) {
  ideDesc.textContent = item.desc || "";
  const cfg = IDE_SCREENS[item.actionId];
  btnIdeInstall.disabled = true;
  btnIdeUpdate.disabled = true;
  btnIdeUninstall.disabled = true;

  App.GetIDEInfo(cfg.cmd).then(function (info) {
    btnIdeInstall.disabled = false;
    btnIdeUpdate.disabled = !info.installed;
    btnIdeUninstall.disabled = !info.installed;
  });

  const buttons = [btnIdeInstall, btnIdeUpdate, btnIdeUninstall];
  btnIdeInstall.onclick = function () { runScreenAction(item, "instalar", buttons, function () { return App.InstallIDE(cfg.cmd); }); };
  btnIdeUpdate.onclick = function () { runScreenAction(item, "atualizar", buttons, function () { return App.UpdateIDE(cfg.cmd); }); };
  btnIdeUninstall.onclick = function () { runScreenAction(item, "desinstalar", buttons, function () { return App.UninstallIDE(cfg.cmd); }); };
}

// --- "app externo via arquivo local install/(atualizar/)desinstalar"
// screens: the user points at a manually downloaded .tar.gz (Android
// Studio, Antigravity IDE) before being able to install/update —
// Desinstalar doesn't depend on any file. FILEAPP_SCREENS maps actionId
// to the bound methods; update is optional (Android Studio doesn't have
// one — it updates itself). ---------------------------------------------
const FILEAPP_SCREENS = {
  androidstudio: {
    pick: function () { return App.PickAndroidStudioTarball(); },
    fetch: function () { return App.GetAndroidStudioInfo(); },
    install: function (path) { return App.InstallAndroidStudio(path); },
    uninstall: function () { return App.UninstallAndroidStudio(); },
  },
  antigravityide: {
    pick: function () { return App.PickAntigravityIDETarball(); },
    fetch: function () { return App.GetAntigravityIDEInfo(); },
    install: function (path) { return App.InstallAntigravityIDE(path); },
    update: function (path) { return App.UpdateAntigravityIDE(path); },
    uninstall: function () { return App.UninstallAntigravityIDE(); },
  },
};

// linkifyDesc escapes the text and turns http(s) URLs into clickable
// links — used only by renderFileAppScreen (Android Studio/Antigravity
// IDE carry the download link inside their own desc). The click is
// intercepted (see the fileappDesc listener right below) and opens in
// the system's default browser via Browser.OpenURL — a plain <a href>
// would navigate the Wails window itself away from the app.
// Every piece is escaped exactly once, from the raw text: the text between
// URLs with escHTML, each URL with escAttr (attribute) and escHTML (label).
function linkifyDesc(text) {
  let out = "";
  let last = 0;
  text.replace(/https?:\/\/[^\s<>"]+/g, function (url, idx) {
    out += escHTML(text.slice(last, idx)) +
      '<a href="#" class="link link-primary" data-url="' + escAttr(url) + '">' + escHTML(url) + "</a>";
    last = idx + url.length;
    return url;
  });
  return out + escHTML(text.slice(last));
}

fileappDesc.addEventListener("click", function (ev) {
  const a = ev.target.closest("a[data-url]");
  if (!a) return;
  ev.preventDefault();
  Browser.OpenURL(a.dataset.url);
});

function renderFileAppScreen(item) {
  fileappDesc.innerHTML = linkifyDesc(item.desc || "");
  fileappNotes.innerHTML = "";
  (item.notes || []).forEach(function (n) {
    const li = document.createElement("li");
    li.textContent = n;
    fileappNotes.appendChild(li);
  });

  const cfg = FILEAPP_SCREENS[item.actionId];
  window.__fileappPath = "";
  fileappPath.textContent = "Nenhum arquivo selecionado.";
  btnFileappInstall.disabled = true;
  btnFileappUninstall.disabled = true;

  if (cfg.update) {
    btnFileappUpdate.classList.remove("hidden");
    btnFileappUpdate.disabled = true;
  } else {
    btnFileappUpdate.classList.add("hidden");
  }

  cfg.fetch().then(function (info) {
    btnFileappUninstall.disabled = !info.installed;
  });

  btnFileappPick.onclick = function () {
    cfg.pick().then(function (path) {
      if (!path) return; // user cancelled the dialog
      window.__fileappPath = path;
      fileappPath.textContent = path;
      btnFileappInstall.disabled = false;
      if (cfg.update) btnFileappUpdate.disabled = false;
    });
  };

  const buttons = [btnFileappInstall, btnFileappUpdate, btnFileappUninstall];
  btnFileappInstall.onclick = function () {
    if (!window.__fileappPath) return;
    runScreenAction(item, "instalar", buttons, function () { return cfg.install(window.__fileappPath); });
  };
  btnFileappUpdate.onclick = function () {
    if (!window.__fileappPath || !cfg.update) return;
    runScreenAction(item, "atualizar", buttons, function () { return cfg.update(window.__fileappPath); });
  };
  btnFileappUninstall.onclick = function () {
    runScreenAction(item, "desinstalar", buttons, cfg.uninstall);
  };
}

function updateExecutarButton() {
  const item = window.__selectedItem;
  if (!item) {
    btnExecutar.disabled = true;
    btnExecutar.textContent = "Executar";
    executarHint.textContent = "Selecione um item para continuar.";
    return;
  }
  if (!item.actionId) {
    btnExecutar.disabled = true;
    btnExecutar.textContent = item.button || "Executar";
    executarHint.textContent = "Esta tela ainda não foi implementada.";
    return;
  }
  btnExecutar.disabled = running;
  btnExecutar.textContent = item.button || "Executar";
  executarHint.textContent = running ? "Executando — acompanhe na aba Execução." : "";
}

btnExecutar.addEventListener("click", function () {
  const item = window.__selectedItem;
  if (!item || !item.actionId || running) return;
  const fn = ACTIONS[item.actionId];
  if (!fn) {
    executarHint.textContent = "actionId '" + item.actionId + "' sem função registrada em ACTIONS.";
    return;
  }

  startExecution(item.title);
  updateExecutarButton();
  fn().catch(failRun);
});

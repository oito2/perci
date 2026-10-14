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

// Docker :: Exportar/Importar.
//
// Classic script (not a module): shares one global scope with the other
// scripts under js/.
"use strict";

// ===========================================================================
// "Docker :: Exportar/Importar": the Exportar checklist (main tab) and the
// Importar checklist (top-level tab #panel-transfer-import).
// ===========================================================================

const TRANSFER_NO_FILE = "Nenhum arquivo selecionado";

// transferExportItems / transferImportItems are the lists last shown on each
// tab (TransferItems from the backend); transferImportPath is the file the
// Importar tab shows.
let transferExportItems = null;
let transferImportItems = null;
let transferImportPath = "";

// appKey / pluginKey are the checkbox keys of an app and a plugin mount.
function appKey(folder) { return "app:" + folder; }
function pluginKey(key) { return "plugin:" + key; }

// transferDependencies returns, for one checklist key, the keys it depends
// on that are also in items: every app needs Nginx, an app with database
// access needs MariaDB, and a plugin mount needs its Moodle app.
function transferDependencies(items, key) {
  if (key.indexOf("app:") === 0) {
    const app = (items.apps || []).find(function (a) { return appKey(a.folder) === key; });
    if (!app) return [];
    const deps = [];
    if (items.nginx) deps.push("nginx");
    if (app.dbAccess && items.mariadb) deps.push("mariadb");
    return deps;
  }
  if (key.indexOf("plugin:") === 0) {
    const plugin = (items.plugins || []).find(function (p) { return pluginKey(p.key) === key; });
    if (!plugin) return [];
    return (items.apps || []).some(function (a) { return a.folder === plugin.app; }) ? [appKey(plugin.app)] : [];
  }
  return [];
}

// transferCompanions returns, for one checklist key, the keys checked along
// with it besides its dependencies: a Moodle app brings the plugin mounts
// mounted on it.
function transferCompanions(items, key) {
  if (key.indexOf("app:") !== 0) return [];
  const folder = key.slice(4);
  return (items.plugins || []).filter(function (p) { return p.app === folder; }).map(function (p) { return pluginKey(p.key); });
}

// transferNotes lists the dependency problems of a selection: an app
// without Nginx or the database it uses, a plugin mount without its Moodle.
// mode is "export" (the importing machine must already have it) or
// "import" (what exists here counts). blocking is true when the import
// would be refused (an app whose database exists neither here nor in the
// selection).
function transferNotes(items, sel, mode) {
  const notes = [];
  let blocking = false;
  const importing = mode === "import";
  const hasNginx = sel.nginx || (importing && items.localNginx);
  const hasDB = sel.mariadb || (importing && items.localMariaDb);
  (items.apps || []).forEach(function (app) {
    if (sel.apps.indexOf(app.folder) < 0) return;
    if (!hasNginx) {
      notes.push(importing
        ? '"' + app.folder + '" depende do Nginx, que não existe nesta máquina: o aplicativo fica sem URL até o Nginx ser criado.'
        : '"' + app.folder + '" depende do Nginx: ele precisa já existir na máquina que importar o arquivo.');
    }
    if (app.dbAccess && !hasDB) {
      if (importing) blocking = true;
      notes.push(importing
        ? '"' + app.folder + '" usa o banco e o MariaDB não existe nesta máquina: marque o MariaDB.'
        : '"' + app.folder + '" usa o banco: o MariaDB precisa já existir na máquina que importar o arquivo.');
    }
  });
  (items.plugins || []).forEach(function (p) {
    if (sel.plugins.indexOf(p.key) < 0 || sel.apps.indexOf(p.app) >= 0) return;
    const inFile = (items.apps || []).find(function (a) { return a.folder === p.app; });
    if (!importing) {
      notes.push('"' + p.component + '" depende do Moodle ' + p.app + ": ele precisa já existir na máquina que importar o arquivo.");
    } else if (inFile && !inFile.exists) {
      notes.push('"' + p.component + '" depende do Moodle ' + p.app + ", que não está marcado nem existe nesta máquina: o plugin será pulado.");
    }
  });
  return { notes: notes, blocking: blocking };
}

// transferSelection reads the checked boxes of a checklist into a
// TransferSelection.
function transferSelection(list) {
  const sel = { nginx: false, mariadb: false, apps: [], plugins: [] };
  list.querySelectorAll("input[data-key]").forEach(function (box) {
    if (!box.checked) return;
    const key = box.dataset.key;
    if (key === "nginx") sel.nginx = true;
    else if (key === "mariadb") sel.mariadb = true;
    else if (key.indexOf("app:") === 0) sel.apps.push(key.slice(4));
    else if (key.indexOf("plugin:") === 0) sel.plugins.push(key.slice(7));
  });
  return sel;
}

// selectionCount is the number of items a selection names.
function selectionCount(sel) {
  return (sel.nginx ? 1 : 0) + (sel.mariadb ? 1 : 0) + sel.apps.length + sel.plugins.length;
}

// transferRow builds one checklist row: a checkbox for key with a title, a
// detail line and an optional badge. A disabled row is unchecked and shows
// detail as the reason.
function transferRow(key, title, detail, badge, disabled) {
  const label = document.createElement("label");
  label.className = "flex items-start gap-3 py-1" + (disabled ? " opacity-60" : " cursor-pointer");
  const box = document.createElement("input");
  box.type = "checkbox";
  box.className = "checkbox checkbox-sm mt-1";
  box.dataset.key = key;
  box.checked = !disabled;
  box.disabled = !!disabled;
  label.appendChild(box);
  const text = document.createElement("div");
  text.className = "min-w-0";
  const head = document.createElement("span");
  head.className = "font-medium";
  head.textContent = title;
  text.appendChild(head);
  if (badge) {
    const b = document.createElement("span");
    b.className = "badge badge-sm badge-ghost ml-2";
    b.textContent = badge;
    text.appendChild(b);
  }
  if (detail) {
    const d = document.createElement("div");
    d.className = "text-xs opacity-70 break-all";
    d.textContent = detail;
    text.appendChild(d);
  }
  label.appendChild(text);
  return label;
}

// transferGroup appends a titled group of rows to list, with a "Marcar
// todos" box (which calls onChange) when it has more than one enabled row.
function transferGroup(list, title, rows, onChange) {
  if (!rows.length) return;
  const group = document.createElement("div");
  group.className = "mb-4";
  const head = document.createElement("div");
  head.className = "flex items-center gap-3 mb-1";
  const h = document.createElement("h3");
  h.className = "font-bold";
  h.textContent = title;
  head.appendChild(h);
  group.appendChild(head);
  rows.forEach(function (row) { group.appendChild(row); });
  const boxes = rows.map(function (r) { return r.querySelector("input"); }).filter(function (b) { return !b.disabled; });
  if (boxes.length > 1) {
    const all = document.createElement("label");
    all.className = "flex items-center gap-2 text-xs cursor-pointer";
    const allBox = document.createElement("input");
    allBox.type = "checkbox";
    allBox.className = "checkbox checkbox-xs";
    allBox.checked = true;
    allBox.addEventListener("change", function () {
      boxes.forEach(function (b) { b.checked = allBox.checked; });
      onChange();
    });
    all.appendChild(allBox);
    const allText = document.createElement("span");
    allText.textContent = "Marcar todos";
    all.appendChild(allText);
    head.appendChild(all);
  }
  list.appendChild(group);
}

// renderTransferList fills list with the groups of items. mode "import"
// adds what already exists here and disables plugin mounts that can't be
// made. Checking a row also checks what it depends on and, for a Moodle,
// the plugin mounts mounted on it (disabled rows stay unchecked); every
// change calls onChange.
function renderTransferList(list, items, mode, onChange) {
  list.innerHTML = "";
  const importing = mode === "import";
  const infra = [];
  if (items.nginx) {
    infra.push(transferRow("nginx", "Nginx", "Proxy reverso de todos os aplicativos (*.localhost).",
      importing && items.localNginx ? "já existe aqui — será recriado" : "", false));
  }
  if (items.mariadb) {
    infra.push(transferRow("mariadb", "MariaDB", "Banco compartilhado e suas credenciais (em texto puro no arquivo).",
      importing && items.localMariaDb ? "já existe aqui — será recriado" : "", false));
  }
  const apps = (items.apps || []).map(function (a) {
    const type = CONTAINER_TYPE_LABELS[a.type] || a.type;
    return transferRow(appKey(a.folder), a.name || a.folder, type + " · " + a.url + (a.dbAccess ? " · usa o banco" : ""),
      importing && a.exists ? "substitui o existente" : "", false);
  });
  const plugins = (items.plugins || []).map(function (p) {
    if (importing && p.mounted) return transferRow(pluginKey(p.key), p.component, "Já está montado nesta máquina.", "montado", true);
    if (importing && p.reason) return transferRow(pluginKey(p.key), p.component, "Indisponível: " + p.reason, "", true);
    return transferRow(pluginKey(p.key), p.component, "Moodle " + p.app + " · " + p.devPath, "", false);
  });
  transferGroup(list, "Infraestrutura", infra, onChange);
  transferGroup(list, "Aplicativos", apps, onChange);
  transferGroup(list, "Plugins Moodle montados", plugins, onChange);

  list.querySelectorAll("input[data-key]").forEach(function (box) {
    box.addEventListener("change", function () {
      if (box.checked) {
        const key = box.dataset.key;
        transferDependencies(items, key).concat(transferCompanions(items, key)).forEach(function (dep) {
          const depBox = list.querySelector('input[data-key="' + dep + '"]');
          if (depBox && !depBox.disabled) depBox.checked = true;
        });
      }
      onChange();
    });
  });
  onChange();
}

// showTransferNotes shows notes in the warning alert (alertEl/textEl), or
// hides it when there are none.
function showTransferNotes(alertEl, textEl, notes) {
  alertEl.classList.toggle("hidden", notes.length === 0);
  textEl.textContent = notes.map(function (n) { return "• " + n; }).join("\n");
}

// ---- Exportar ---------------------------------------------------------------

function updateTransferExport() {
  if (!transferExportItems) return;
  const sel = transferSelection(transferExportList);
  showTransferNotes(transferExportNote, transferExportNoteText, transferNotes(transferExportItems, sel, "export").notes);
  btnTransferExport.disabled = selectionCount(sel) === 0;
}

function loadTransferExport() {
  btnTransferExport.disabled = true;
  App.GetExportItems().then(latestOnly("transfer-export", function (items) {
    transferExportItems = items;
    const empty = !items.nginx && !items.mariadb && !(items.apps || []).length && !(items.plugins || []).length;
    transferExportEmpty.classList.toggle("hidden", !empty);
    renderTransferList(transferExportList, items, "export", updateTransferExport);
  })).catch(function (err) { openError("Falha ao listar o que exportar: " + err); });
}

btnTransferExport.addEventListener("click", function () {
  const sel = transferSelection(transferExportList);
  if (selectionCount(sel) === 0) return;
  App.PickExportConfigPath().then(function (path) {
    if (!path) return; // user cancelled the dialog
    return App.ExportDockerConfig(path, sel).then(function () {
      openMessage("Exportado", selectionCount(sel) + " item(ns) exportado(s) para:\n" + path, false);
    });
  }).catch(function (err) { openError("Falha ao exportar: " + err); });
});

// ---- Importar ---------------------------------------------------------------

function updateTransferImport() {
  if (!transferImportItems) {
    btnTransferImport.disabled = true;
    return;
  }
  const sel = transferSelection(transferImportList);
  const result = transferNotes(transferImportItems, sel, "import");
  showTransferNotes(transferImportNote, transferImportNoteText, result.notes);
  btnTransferImport.disabled = running || result.blocking || selectionCount(sel) === 0;
}

// resetTransferImport clears the Importar tab back to "no file".
function resetTransferImport() {
  invalidate("transfer-import");
  transferImportItems = null;
  transferImportPath = "";
  transferImportPathEl.textContent = TRANSFER_NO_FILE;
  transferImportPathEl.classList.add("opacity-60");
  transferImportWorkspace.classList.add("hidden");
  transferImportList.innerHTML = "";
  showTransferNotes(transferImportNote, transferImportNoteText, []);
  btnTransferImport.disabled = true;
}

// loadTransferImport reads the export file at path and shows its items.
function loadTransferImport(path) {
  App.PreviewImportConfig(path).then(latestOnly("transfer-import", function (items) {
    transferImportItems = items;
    transferImportPath = path;
    transferImportPathEl.textContent = path;
    transferImportPathEl.classList.remove("opacity-60");
    transferImportWorkspace.textContent = "Workspace do arquivo: " + (items.workspacePath || "(nenhum)");
    transferImportWorkspace.classList.remove("hidden");
    renderTransferList(transferImportList, items, "import", updateTransferImport);
  })).catch(function (err) {
    resetTransferImport();
    openError("Falha ao ler o arquivo: " + err);
  });
}

btnTransferPick.addEventListener("click", function () {
  App.PickImportConfigPath().then(function (path) {
    if (path) loadTransferImport(path);
  }).catch(function (err) { openError("Falha ao selecionar o arquivo: " + err); });
});

// importConfirmMessage is the text of the import confirmation: what will
// be imported, the commands the selected apps run inside their containers,
// and how the import merges.
function importConfirmMessage(items, sel) {
  const parts = [];
  if (sel.nginx) parts.push("Nginx");
  if (sel.mariadb) parts.push("MariaDB");
  if (sel.apps.length) parts.push(sel.apps.length + " aplicativo(s)");
  if (sel.plugins.length) parts.push(sel.plugins.length + " plugin(s) montado(s)");
  let msg = "Importar: " + parts.join(", ") + ".\n\n";
  const commands = (items.commands || []).filter(function (c) {
    return sel.apps.indexOf(c.split(/[: ]/)[0]) >= 0;
  });
  if (commands.length > 0) {
    msg += "Comandos que serão executados dentro dos contêineres:\n" +
      commands.map(function (c) { return "• " + c; }).join("\n") + "\n\n";
  }
  if (sel.plugins.length) msg += "Montar os plugins pede a senha de administrador.\n\n";
  return msg + "Os itens marcados são criados ou substituem os de mesma pasta e os contêineres deles são recriados; o resto desta máquina não muda. Continuar?";
}

btnTransferImport.addEventListener("click", function () {
  if (running || !transferImportItems) return;
  const sel = transferSelection(transferImportList);
  if (selectionCount(sel) === 0) return;
  const path = transferImportPath;
  openConfirm("Importar configurações", importConfirmMessage(transferImportItems, sel), function () {
    beginRun("Importar configurações", function () {
      return App.ImportDockerContainerConfig(path, sel);
    }, { buttons: [btnTransferImport] });
  });
});

function renderTransferScreen() {
  loadTransferExport();
  resetTransferImport();
}

// refreshTransferScreen re-reads both lists after a run, keeping the file
// shown on the Importar tab.
function refreshTransferScreen() {
  loadTransferExport();
  if (transferImportPath) loadTransferImport(transferImportPath);
}

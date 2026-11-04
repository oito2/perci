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

// Repositórios :: Gerenciar Repositórios.
//
// Classic script (not a module): shares one global scope with the other
// scripts under js/.
"use strict";

// ===========================================================================
// "Repositórios :: Gerenciar Repositórios": the Exportar checklist (main
// tab) and the Importar checklist (top-level tab #panel-git-import).
// ===========================================================================

const gittransferContainer = document.getElementById("gittransfer-container");
const gtExportEmpty = document.getElementById("gt-export-empty");
const gtExportList = document.getElementById("gt-export-list");
const gtExportNote = document.getElementById("gt-export-note");
const gtExportNoteText = document.getElementById("gt-export-note-text");
const btnGtExport = document.getElementById("btn-gt-export");
const tabLabelGitImport = document.getElementById("tab-label-git-import");
const btnGtPick = document.getElementById("btn-gt-pick");
const gtImportPathEl = document.getElementById("gt-import-path");
const gtImportList = document.getElementById("gt-import-list");
const gtImportNote = document.getElementById("gt-import-note");
const gtImportNoteText = document.getElementById("gt-import-note-text");
const btnGtImport = document.getElementById("btn-gt-import");

// gtExportRows / gtImportRows are the checklists last shown on each tab:
// one entry per account, { name, box, repos: [{ path, box }] }.
// gtImportPath is the file the Importar tab shows and gtFolders the
// folders picked for its new accounts, by account name.
let gtExportRows = [];
let gtImportRows = [];
let gtImportPlans = null;
let gtImportPath = "";
let gtFolders = {};

const GT_STATE_LABELS = {
  pending: "já está pendente",
  present: "já está clonado",
};

// gtPicks reads rows into TransferPicks: every checked account with its
// checked repositories.
function gtPicks(rows) {
  return rows.filter(function (r) { return r.box.checked; }).map(function (r) {
    return {
      account: r.name,
      repos: r.repos.filter(function (x) { return x.box.checked; }).map(function (x) { return x.path; }),
    };
  });
}

// gtCount is the number of accounts and repositories picks names.
function gtCount(picks) {
  return picks.reduce(function (n, p) { return n + 1 + p.repos.length; }, 0);
}

// gtBox builds one checklist row: a checkbox with a title, a detail line
// and an optional badge, indented for a repository. A disabled row is
// unchecked. Returns { row, box, text }.
function gtBox(title, detail, badge, disabled, indent) {
  const row = document.createElement("label");
  row.className = "flex items-start gap-3 py-1" + (indent ? " ml-8" : "") + (disabled ? " opacity-60" : " cursor-pointer");
  const box = document.createElement("input");
  box.type = "checkbox";
  box.className = "checkbox checkbox-sm mt-1";
  box.checked = !disabled;
  box.disabled = !!disabled;
  row.appendChild(box);
  const text = document.createElement("div");
  text.className = "min-w-0";
  const head = document.createElement("span");
  head.className = indent ? "" : "font-medium";
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
  row.appendChild(text);
  return { row: row, box: box, text: text };
}

// gtLink ties an account box to its repository boxes: checking the account
// checks its enabled repositories, unchecking it unchecks them all, and
// checking a repository checks its account. Every change calls onChange.
function gtLink(entry, onChange) {
  entry.box.addEventListener("change", function () {
    entry.repos.forEach(function (x) {
      if (!x.box.disabled) x.box.checked = entry.box.checked;
    });
    onChange();
  });
  entry.repos.forEach(function (x) {
    x.box.addEventListener("change", function () {
      if (x.box.checked && !entry.box.disabled) entry.box.checked = true;
      onChange();
    });
  });
}

// gtAccountDetail is the detail line of an account.
function gtAccountDetail(a) {
  const provider = a.provider === "gitlab" ? "GitLab" : "GitHub";
  return provider + " · " + a.host + (a.sshPort && a.sshPort !== 22 ? ":" + a.sshPort : "") + " · " + a.username + " · " + a.folder;
}

// showGtNotes shows notes in the warning alert (alertEl/textEl), or hides
// it when there are none.
function showGtNotes(alertEl, textEl, notes) {
  alertEl.classList.toggle("hidden", notes.length === 0);
  textEl.textContent = notes.map(function (n) { return "• " + n; }).join("\n");
}

// ---- Exportar ---------------------------------------------------------------

// renderGtExport fills the Exportar checklist with items (RepoExportItems).
function renderGtExport(items) {
  gtExportList.innerHTML = "";
  gtExportRows = [];
  const accounts = items.accounts || [];
  gtExportEmpty.classList.toggle("hidden", accounts.length > 0);
  accounts.forEach(function (a) {
    const group = document.createElement("div");
    group.className = "mb-3";
    const head = gtBox(a.name, gtAccountDetail(a), "", false, false);
    group.appendChild(head.row);
    const entry = { name: a.name, box: head.box, repos: [] };
    (a.repos || []).forEach(function (r) {
      const row = gtBox(r.path, r.url, "", false, true);
      group.appendChild(row.row);
      entry.repos.push({ path: r.path, box: row.box });
    });
    if (!(a.repos || []).length) {
      const none = document.createElement("p");
      none.className = "text-xs opacity-70 ml-8";
      none.textContent = "Nenhum repositório com remote nesta conta: só os dados da conta são exportados.";
      group.appendChild(none);
    }
    gtLink(entry, updateGtExport);
    gtExportRows.push(entry);
    gtExportList.appendChild(group);
  });
  const skipped = items.skipped || [];
  showGtNotes(gtExportNote, gtExportNoteText, skipped.length
    ? ["Repositórios sem remote não são exportados (não há de onde cloná-los): " + skipped.join(", ") + "."]
    : []);
  updateGtExport();
}

function updateGtExport() {
  btnGtExport.disabled = gtCount(gtPicks(gtExportRows)) === 0;
}

function loadGtExport() {
  btnGtExport.disabled = true;
  App.GetGitRepoExportItems().then(latestOnly("gt-export", renderGtExport))
    .catch(function (err) { openError("Falha ao listar o que exportar: " + err); });
}

btnGtExport.addEventListener("click", function () {
  const picks = gtPicks(gtExportRows);
  if (gtCount(picks) === 0) return;
  App.PickGitRepoExportPath().then(function (path) {
    if (!path) return; // user cancelled the dialog
    return App.ExportGitRepos(path, picks).then(function () {
      const repos = picks.reduce(function (n, p) { return n + p.repos.length; }, 0);
      openMessage("Exportado", picks.length + " conta(s) e " + repos + " repositório(s) exportados para:\n" + path, false);
    });
  }).catch(function (err) { openError("Falha ao exportar: " + err); });
});

// ---- Importar ---------------------------------------------------------------

// gtImportBadge is the badge of an account of the import plan.
function gtImportBadge(p) {
  if (p.reason) return "";
  return p.existingId ? "já cadastrada — mescla com " + p.existingName : "conta nova";
}

// renderGtImport fills the Importar checklist with plans (ImportAccount).
// A new account gets a "Trocar pasta" button; an account or repository
// that can't be imported is disabled, with the reason.
function renderGtImport(plans) {
  gtImportList.innerHTML = "";
  gtImportRows = [];
  plans.forEach(function (p) {
    const group = document.createElement("div");
    group.className = "mb-3";
    const detail = p.reason ? "Indisponível: " + p.reason : gtAccountDetail(p);
    const head = gtBox(p.name, detail, gtImportBadge(p), !!p.reason, false);
    if (!p.existingId) {
      const change = document.createElement("button");
      change.className = "btn btn-xs mt-1";
      change.textContent = "Trocar pasta";
      change.addEventListener("click", function (e) {
        e.preventDefault();
        App.PickGitAccountFolder().then(function (folder) {
          if (!folder) return;
          gtFolders[p.name] = folder;
          loadGtImport(gtImportPath);
        }).catch(function (err) { openError("Falha ao escolher a pasta: " + err); });
      });
      head.text.appendChild(change);
    }
    group.appendChild(head.row);
    const entry = { name: p.name, box: head.box, repos: [] };
    (p.states || []).forEach(function (st, i) {
      const repo = (p.repos || [])[i] || {};
      let row;
      if (st.state === "new") row = gtBox(st.path, repo.url, "", !!p.reason, true);
      else if (st.state === "conflict") row = gtBox(st.path, "Ignorado: " + st.reason, "", true, true);
      else row = gtBox(st.path, repo.url, GT_STATE_LABELS[st.state] || st.state, true, true);
      group.appendChild(row.row);
      entry.repos.push({ path: st.path, box: row.box });
    });
    gtLink(entry, updateGtImport);
    gtImportRows.push(entry);
    gtImportList.appendChild(group);
  });
  updateGtImport();
}

function updateGtImport() {
  if (!gtImportPlans) {
    btnGtImport.disabled = true;
    return;
  }
  const picks = gtPicks(gtImportRows);
  const notes = [];
  picks.forEach(function (pick) {
    const plan = gtImportPlans.find(function (p) { return p.name === pick.account; });
    if (plan && !plan.existingId) {
      notes.push('"' + pick.account + '" é nova: ganha a pasta ' + plan.folder + ", uma chave SSH sem senha e a regra no ~/.gitconfig.");
    }
  });
  showGtNotes(gtImportNote, gtImportNoteText, notes);
  btnGtImport.disabled = running || picks.length === 0;
}

// resetGtImport clears the Importar tab back to "no file".
function resetGtImport() {
  invalidate("gt-import");
  gtImportPlans = null;
  gtImportPath = "";
  gtFolders = {};
  gtImportRows = [];
  gtImportPathEl.textContent = TRANSFER_NO_FILE;
  gtImportPathEl.classList.add("opacity-60");
  gtImportList.innerHTML = "";
  showGtNotes(gtImportNote, gtImportNoteText, []);
  btnGtImport.disabled = true;
}

// loadGtImport reads the export file at path, with the folders picked so
// far, and shows its accounts and repositories.
function loadGtImport(path) {
  App.PreviewGitRepoImport(path, gtFolders).then(latestOnly("gt-import", function (plans) {
    gtImportPlans = plans || [];
    gtImportPath = path;
    gtImportPathEl.textContent = path;
    gtImportPathEl.classList.remove("opacity-60");
    renderGtImport(gtImportPlans);
  })).catch(function (err) {
    resetGtImport();
    openError("Falha ao ler o arquivo: " + err);
  });
}

btnGtPick.addEventListener("click", function () {
  App.PickGitRepoImportPath().then(function (path) {
    if (!path) return;
    gtFolders = {};
    loadGtImport(path);
  }).catch(function (err) { openError("Falha ao selecionar o arquivo: " + err); });
});

btnGtImport.addEventListener("click", function () {
  if (running || !gtImportPlans) return;
  const picks = gtPicks(gtImportRows);
  if (picks.length === 0) return;
  const path = gtImportPath;
  const folders = Object.assign({}, gtFolders);
  const repos = picks.reduce(function (n, p) { return n + p.repos.length; }, 0);
  const msg = "Importar " + picks.length + " conta(s) e " + repos + " repositório(s).\n\n" +
    "Contas novas ganham a pasta, uma chave SSH sem senha e a regra no ~/.gitconfig; contas já cadastradas mantêm os dados delas. " +
    "Os repositórios ficam pendentes na tela de cada conta: nada é clonado agora. Continuar?";
  openConfirm("Importar repositórios", msg, function () {
    beginRun("Importar repositórios", function () {
      return App.ImportGitRepos(path, picks, folders);
    }, { buttons: [btnGtImport] });
  });
});

function renderGitTransferScreen() {
  loadGtExport();
  resetGtImport();
}

// refreshGitTransferScreen re-reads both lists after a run, keeping the
// file shown on the Importar tab, and rebuilds the menu (an import may
// add accounts).
function refreshGitTransferScreen() {
  loadMenu();
  loadGtExport();
  if (gtImportPath) loadGtImport(gtImportPath);
}

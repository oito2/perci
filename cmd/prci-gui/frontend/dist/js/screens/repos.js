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

// Dev Tools :: Repositórios.
//
// Classic script (not a module): shares one global scope with the other
// scripts under js/.
"use strict";

// ===========================================================================
// "Dev Tools :: Repositórios".
// ===========================================================================

// --- aba "Identidade Global" ------------------------------------------------

// One fetch feeds both the Identidade Global form and the cache the
// Clonar/Iniciar/Identidade Local forms pre-fill from — so applying a new
// global identity refreshes those too.
function renderReposIdentityTab() {
  App.GetGlobalGitIdentity().then(function (identity) {
    __reposGlobalIdentityCache = identity;
    const already = !!(identity.name && identity.email);
    reposIdentityCheckboxWrap.classList.toggle("hidden", !already);
    reposIdentityChange.checked = false;
    reposIdentityName.value = identity.name || "";
    reposIdentityEmail.value = identity.email || "";
    reposIdentityName.disabled = already;
    reposIdentityEmail.disabled = already;
    btnReposIdentityApply.disabled = already;
  });
}
reposIdentityChange.addEventListener("change", function () {
  const enable = reposIdentityChange.checked;
  reposIdentityName.disabled = !enable;
  reposIdentityEmail.disabled = !enable;
  btnReposIdentityApply.disabled = !enable;
});
btnReposIdentityApply.addEventListener("click", function () {
  if (running || btnReposIdentityApply.disabled) return;
  const name = reposIdentityName.value;
  const email = reposIdentityEmail.value;
  startExecution("Aplicar Identidade Global");
  App.ApplyGlobalGitIdentity(name, email).catch(failRun);
});

// --- "Repositórios" tab: list of known folders (cards) + horizontal menu
// — the remembered list is persisted (App.GetRepoFolders/AddRepoFolder),
// so it survives across sessions. ----------------------------------------

let reposFolderPath = "";
let reposFolderState = null;
let reposActiveSub = "clone";
let reposFolders = [];
let __reposGlobalIdentityCache = { name: "", email: "" };

function reposFolderName(path) {
  const parts = path.split("/").filter(Boolean);
  return parts.length ? parts[parts.length - 1] : path;
}

function renderReposFolderAlert() {
  if (!reposFolderPath) {
    reposFolderAlertText.innerHTML =
      "Nenhum repositório selecionado" +
      '<br><span class="text-xs opacity-70">Selecione um repositório acima (ou adicione um novo) para habilitar as opções abaixo</span>';
  } else {
    reposFolderAlertText.innerHTML =
      "<strong>" + escHTML(reposFolderName(reposFolderPath)) + "</strong>" +
      '<br><span class="text-xs opacity-70">' + escHTML(reposFolderPath) + "</span>";
  }
  reposSubmenu.classList.toggle("repos-submenu-disabled", !reposFolderPath);
}

function selectRepoFolder(path) {
  reposFolderPath = path;
  renderReposFolderAlert();
  refreshReposFolderState();
  renderRepoCards(); // reapplies the selected card's highlight
}

// buildRepoCard builds a "Card with no image" (daisyUI) for one
// remembered folder — split out of renderRepoCards so it can be reused by
// reconcileKeyedList.
function buildRepoCard(r) {
  const card = document.createElement("div");
  card.className = "card bg-base-200 border " + (r.path === reposFolderPath ? "border-primary" : "border-base-300");

  const body = document.createElement("div");
  body.className = "card-body";

  const title = document.createElement("h3");
  title.className = "card-title text-base";
  title.textContent = r.name;
  body.appendChild(title);

  const pathEl = document.createElement("p");
  pathEl.className = "text-xs opacity-70 break-all";
  pathEl.textContent = r.path;
  body.appendChild(pathEl);

  const actions = document.createElement("div");
  actions.className = "card-actions justify-end";
  // "Remover" only forgets the card (config.yaml's repo_folders) —
  // nothing on disk is touched, which the confirmation spells out.
  const removeBtn = document.createElement("button");
  removeBtn.type = "button";
  removeBtn.className = "btn btn-sm btn-ghost";
  removeBtn.textContent = "Remover";
  removeBtn.addEventListener("click", function () {
    openConfirm(
      "Remover da lista?",
      'Remover "' + r.name + '" da lista de repositórios?\n\nSó o cartão sai da lista — nenhum arquivo é apagado do disco.',
      function () {
        App.RemoveRepoFolder(r.path).then(function () {
          if (r.path === reposFolderPath) selectRepoFolder("");
          loadRepoFolders();
        }).catch(function (err) {
          openError("Falha ao remover da lista: " + err);
        });
      }
    );
  });
  actions.appendChild(removeBtn);
  const btn = document.createElement("button");
  btn.type = "button";
  btn.className = "btn btn-sm btn-outline";
  btn.textContent = "Selecionar";
  btn.addEventListener("click", function () { selectRepoFolder(r.path); });
  actions.appendChild(btn);
  body.appendChild(actions);

  card.appendChild(body);
  return card;
}

// One card per remembered folder (reconciled by key — path — instead of
// innerHTML="" + full rebuild on every call) plus one last "Centered card
// with neutral color" card to add a new one — that last one is always
// rebuilt fresh (static, no per-folder state) and re-appended last, which
// also keeps its position at the end of the list after reconciliation.
// reconcileKeyedList only manages keyed cards, so the previous "Novo
// repositório" card has to be removed here, or every re-render (adding a
// folder, "Selecionar") would leave one more behind.
let reposAddCard = null;

function renderRepoCards() {
  if (reposAddCard) reposAddCard.remove();
  reconcileKeyedList(
    reposListCards,
    reposFolders,
    function (r) { return r.path; },
    function (r) { return r.path + "|" + r.name + "|" + (r.path === reposFolderPath); },
    buildRepoCard
  );

  const addCard = document.createElement("div");
  addCard.className = "card bg-neutral text-neutral-content";
  const addBody = document.createElement("div");
  addBody.className = "card-body items-center text-center";
  const addTitle = document.createElement("h3");
  addTitle.className = "card-title";
  addTitle.textContent = "Novo repositório";
  const addDesc = document.createElement("p");
  addDesc.textContent = "Selecione uma pasta";
  const addActions = document.createElement("div");
  addActions.className = "card-actions";
  const addBtn = document.createElement("button");
  addBtn.type = "button";
  addBtn.className = "btn btn-primary btn-sm";
  addBtn.textContent = "Selecionar pasta";
  addBtn.addEventListener("click", function () {
    App.PickRepoWorkingFolder().then(function (path) {
      if (!path) return; // user cancelled the dialog
      App.AddRepoFolder(path)
        .then(function () { return App.GetRepoFolders(); })
        .then(function (folders) {
          reposFolders = folders || [];
          selectRepoFolder(path); // renders the cards with the updated list
        });
    });
  });
  addActions.appendChild(addBtn);
  addBody.appendChild(addTitle);
  addBody.appendChild(addDesc);
  addBody.appendChild(addActions);
  addCard.appendChild(addBody);
  reposListCards.appendChild(addCard);
  reposAddCard = addCard;
}

function loadRepoFolders() {
  App.GetRepoFolders().then(function (folders) {
    reposFolders = folders || [];
    renderRepoCards();
  });
}

function refreshReposFolderState() {
  if (!reposFolderPath) {
    reposFolderState = null;
    renderReposSubPanel();
    return;
  }
  App.GetRepoFolderState(reposFolderPath).then(function (state) {
    reposFolderState = state;
    renderReposSubPanel();
  }).catch(function (err) {
    reposFolderState = null;
    renderReposSubPanel();
    openError("Falha ao ler a pasta selecionada: " + err);
  });
}

function selectReposSub(sub) {
  if (!reposFolderPath) return;
  reposActiveSub = sub;
  document.querySelectorAll("#repos-submenu a").forEach(function (a) {
    a.classList.toggle("menu-active", a.dataset.sub === sub);
  });
  renderReposSubPanel();
}
document.querySelectorAll("#repos-submenu a").forEach(function (a) {
  a.addEventListener("click", function (ev) {
    ev.preventDefault();
    selectReposSub(a.dataset.sub);
  });
});

// repoAlert shows a one-line daisyUI alert in the sub-panel; html must
// already be escaped.
function repoAlert(kind, html) {
  reposSubPanel.innerHTML = '<div role="alert" class="alert alert-' + kind + ' alert-soft"><span>' + html + "</span></div>";
}

// renderFormPanel fills the sub-panel with text fields and one submit
// button; onSubmit gets { key: value } and runs only while nothing else is
// running. fields: [{ key, label, value }].
function renderFormPanel(idPrefix, fields, submitLabel, onSubmit) {
  reposSubPanel.innerHTML =
    fields.map(function (f) {
      return textFieldHTML(idPrefix + "-" + f.key, f.label, { value: f.value });
    }).join("") +
    '<button id="' + idPrefix + '-submit" type="button" class="btn btn-primary mt-2">' + escHTML(submitLabel) + "</button>";
  document.getElementById(idPrefix + "-submit").addEventListener("click", function () {
    if (running) return;
    const values = {};
    fields.forEach(function (f) { values[f.key] = document.getElementById(idPrefix + "-" + f.key).value; });
    onSubmit(values);
  });
}

function renderReposSubPanel() {
  if (!reposFolderPath || !reposFolderState) {
    reposSubPanel.innerHTML = "";
    return;
  }
  const st = reposFolderState;
  const gid = __reposGlobalIdentityCache;

  if (reposActiveSub === "clone") {
    if (st.isGitRepo && st.hasRemote) {
      repoAlert("success", "Já foi clonado nesta pasta. Remote: <code>" + escHTML(st.remoteUrl) + "</code>");
      return;
    }
    // git clone only clones into an empty folder (CloneRepo refuses too).
    if (!st.isEmpty) {
      repoAlert("warning", "Esta pasta não está vazia. " +
        "Para clonar, adicione uma pasta vazia pelo cartão <strong>Novo repositório</strong> " +
        "(o seletor permite criar uma pasta nova).");
      return;
    }
    renderFormPanel("repos-clone", [
      { key: "url", label: "URL do Repositório" },
      { key: "name", label: "Nome no Git", value: gid.name },
      { key: "email", label: "E-mail no Git", value: gid.email },
    ], "Clonar", function (v) {
      startExecution("Clonar repositório");
      App.CloneRepo(v.url, reposFolderPath, v.name, v.email).catch(failRun);
    });
    return;
  }

  if (reposActiveSub === "init") {
    if (st.isGitRepo) {
      repoAlert("success", "Este repositório já foi inicializado nesta pasta.");
      return;
    }
    renderFormPanel("repos-init", [
      { key: "name", label: "Nome Local (opcional)", value: gid.name },
      { key: "email", label: "E-mail Local (opcional)", value: gid.email },
    ], "Iniciar Repositório", function (v) {
      startExecution("Iniciar Repositório");
      App.InitRepoAt(reposFolderPath, v.name, v.email).catch(failRun);
    });
    return;
  }

  if (reposActiveSub === "ident") {
    if (!st.isGitRepo) {
      reposSubPanel.innerHTML = '<p class="opacity-70 text-sm">Clone ou inicie um repositório nesta pasta primeiro.</p>';
      return;
    }
    if (st.localName && st.localEmail) {
      repoAlert("success", "Identidade local já configurada: " +
        escHTML(st.localName) + " &lt;" + escHTML(st.localEmail) + "&gt;");
      return;
    }
    renderFormPanel("repos-ident", [
      { key: "name", label: "Nome Local", value: gid.name },
      { key: "email", label: "E-mail Local", value: gid.email },
    ], "Aplicar Identificação Local", function (v) {
      startExecution("Aplicar Identificação Local");
      App.ApplyLocalGitIdentityAt(reposFolderPath, v.name, v.email).catch(failRun);
    });
    return;
  }

  if (reposActiveSub === "files") {
    // The Code of Conduct needs the project's own reporting contact —
    // pre-filled with the folder's local Git e-mail, then the global one.
    reposSubPanel.innerHTML =
      '<div class="flex gap-2 flex-wrap mb-6">' +
      '<button id="btn-repos-gitignore" type="button" class="btn btn-outline">Criar/Atualizar .gitignore</button>' +
      "</div>" +
      '<div class="max-w-md">' +
      textFieldHTML("repos-conduct-email", "E-mail de contato para denúncias", { value: st.localEmail || gid.email || "" }) +
      '<p class="text-xs opacity-60 mb-2">Gera CODE_OF_CONDUCT.md na raiz e docs/pt-br/codigo-de-conduta.md.</p>' +
      '<button id="btn-repos-conduct" type="button" class="btn btn-outline">Criar/Atualizar Código de Conduta</button>' +
      "</div>";
    document.getElementById("btn-repos-gitignore").addEventListener("click", function () {
      if (running) return;
      startExecution("Criar/Atualizar .gitignore");
      App.GenerateGitignoreAt(reposFolderPath).catch(failRun);
    });
    document.getElementById("btn-repos-conduct").addEventListener("click", function () {
      if (running) return;
      const email = document.getElementById("repos-conduct-email").value.trim();
      if (!email) {
        openError("Informe o e-mail de contato para denúncias.");
        return;
      }
      startExecution("Criar/Atualizar Código de Conduta");
      App.CreateConductAt(reposFolderPath, email).catch(failRun);
    });
  }
}

function renderReposScreen() {
  reposFolderPath = "";
  reposFolderState = null;
  reposActiveSub = "clone";
  renderReposIdentityTab();
  renderReposFolderAlert();
  loadRepoFolders();
  document.querySelectorAll("#repos-submenu a").forEach(function (a) {
    a.classList.toggle("menu-active", a.dataset.sub === "clone");
  });
  reposSubPanel.innerHTML = "";
}

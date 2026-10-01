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

// Generic UI helpers shared by every screen: confirm/message/logs modals, status badges, escaping (escHTML/escAttr), form field builders and reconcileKeyedList.
//
// Classic script (not a module): every file under js/ shares one global
// scope, loaded in order by index.html — see js/bootstrap.js.
"use strict";

// --- generic modals (confirm / message / logs) --------------------------

let __confirmCallback = null;
btnConfirmOk.addEventListener("click", function () {
  confirmModal.close();
  const cb = __confirmCallback;
  __confirmCallback = null;
  if (cb) cb();
});
function openConfirm(title, message, onConfirm) {
  confirmModalTitle.textContent = title;
  confirmModalMessage.textContent = message;
  __confirmCallback = onConfirm;
  confirmModal.showModal();
}

// error-modal is reused as a generic message modal (title and color
// class vary) — avoids a 4th modal just for "success" (e.g. Exportar
// configurações, which doesn't go through the Execução tab).
const errorModalTitleEl = errorModal.querySelector("h3");
function openMessage(title, message, isError) {
  errorModalTitleEl.textContent = title;
  errorModalTitleEl.className = "font-bold text-lg mb-2" + (isError ? " text-error" : "");
  errorModalMessage.textContent = message;
  errorModal.showModal();
}
function openError(message) { openMessage("Erro", String(message), true); }

let __logsFolder = "";
// The fetched log itself, kept apart from what the modal shows: "Exportar"
// saves this, never a "Carregando..."/"(sem saída)"/error placeholder.
let __logsText = "";
function refreshLogs() {
  const folder = __logsFolder;
  __logsText = "";
  btnLogsExport.disabled = true;
  logsModalContent.textContent = "Carregando...";
  App.GetContainerLogsSnapshot(folder).then(function (text) {
    if (folder !== __logsFolder) return; // another container's logs opened meanwhile
    __logsText = text || "";
    btnLogsExport.disabled = __logsText === "";
    logsModalContent.textContent = __logsText || "(sem saída)";
  }).catch(function (err) {
    if (folder !== __logsFolder) return;
    logsModalContent.textContent = "Falha ao buscar logs: " + err;
  });
}
function openLogs(folder, title) {
  __logsFolder = folder;
  logsModalTitle.textContent = "Logs — " + title;
  logsModal.showModal();
  refreshLogs();
}
btnLogsRefresh.addEventListener("click", refreshLogs);
btnLogsExport.addEventListener("click", function () {
  App.PickLogsExportPath(__logsFolder + "-logs.txt").then(function (path) {
    if (!path) return;
    return App.SaveTextFile(path, __logsText);
  }).catch(function (err) { openError("Falha ao exportar logs: " + err); });
});

// startExecution is the shared "begin a long-running action" sequence,
// switching to the Execução tab — used by every runAction-based screen
// that has no dedicated "Executar" button of its own (Docker:
// Recriar/Editar/Backup/Restore/Importar; Dev Tools :: Repositórios:
// Aplicar Identidade Global/Clonar/Iniciar/Identidade Local/Arquivos —
// only Iniciar/Parar/Reiniciar/Remover on containers stay outside this,
// since those run synchronously with no terminal panel involved).
// buttonsToDisable/maxSteps are optional — omit for the single-step,
// no-extra-button case.
//
// announceText: the line written to the terminal sometimes differs from
// the default "Iniciando <label>..." pattern — a suffix (IDE/app-local-
// file/singleapp: "Zed Editor (instalar)...") or even a different verb
// (self-uninstall: "Desinstalando o Perci...", not "Iniciando..."), while
// termTitle always uses just label ("Executando: Zed Editor..."). This
// parameter lets those screens reuse the shared sequence instead of
// hand-rolling the whole block just for that one line; when omitted, the
// terminal uses the default pattern.
// latestOnly guards an async render against responses arriving out of
// order: each call starts a new request for key and returns fn wrapped so
// it only runs if no newer request for the same key started meanwhile.
// Without it, switching quickly between screens that share one container
// (the 7 checklist screens, the Docker table, the SKILLs/MCPs tables when
// changing scope) mixed items from two screens — and left "Executar" bound
// to whichever response arrived last.
const renderTokens = {};
function latestOnly(key, fn) {
  const token = (renderTokens[key] = (renderTokens[key] || 0) + 1);
  return function (value) {
    if (renderTokens[key] === token) return fn(value);
  };
}

function startExecution(label, buttonsToDisable, maxSteps, announceText) {
  running = true;
  const buttons = (buttonsToDisable || []).filter(Boolean);
  buttons.forEach(function (btn) { btn.disabled = true; });
  // Remembered so finishRun (core.js) releases exactly these buttons and
  // knows which screen started the run — see activeRun.
  activeRun = {
    actionId: window.__selectedItem ? window.__selectedItem.actionId : null,
    buttons: buttons,
  };
  term.clear();
  term.writeln(announceText || ("Iniciando " + label + "..."));
  termTitle.textContent = "Executando: " + label + "...";
  termProgress.value = 0;
  termProgress.max = maxSteps || 1;
  selectTab("terminal");
}

// --- status → "Status with ping animation" (docs.daisyui.com/components/status) ---

const CONTAINER_TYPE_LABELS = {
  nginx: "Nginx", mariadb: "MariaDB", moodle: "Moodle", php: "PHP",
  generic: "Genérico", node: "Node", php_node: "PHP + Node",
};

function statusInfo(status) {
  switch (status) {
    case "running": return { label: "rodando", color: "success", ping: true };
    case "restarting": return { label: "reiniciando", color: "warning", ping: true };
    case "paused": return { label: "pausado", color: "warning", ping: false };
    case "created": return { label: "criado", color: "info", ping: false };
    case "exited": return { label: "parado", color: "neutral", ping: false };
    case "dead": return { label: "morto", color: "error", ping: false };
    case "": return { label: "não encontrado", color: "error", ping: false };
    default: return { label: status, color: "neutral", ping: false };
  }
}
function statusBadgeHTML(status) {
  const info = statusInfo(status);
  const dot = "status status-" + info.color;
  const indicator = info.ping
    ? '<span class="inline-grid *:[grid-area:1/1]"><span class="' + dot + ' animate-ping"></span><span class="' + dot + '"></span></span>'
    : '<span class="' + dot + '"></span>';
  // The default label is Docker's raw status — escaped like any other
  // non-literal text.
  return '<span class="inline-flex items-center gap-2">' + indicator + "<span>" + escHTML(info.label) + "</span></span>";
}

// --- escaping (a convention for the whole file, not just this section) -
// Any text that isn't a fixed literal from index.html/catalog.go itself
// (a folder name, a path chosen in a native dialog, command output,
// whatever it is) must never go into innerHTML without passing through
// escHTML() (loose text) or escAttr() (inside an attribute, e.g.
// data-url="...") first. A folder path chosen via
// App.PickRepoWorkingFolder can contain any character on Linux
// (including `<`/`>`/`"`), and since this is a Wails app, injected script
// has direct access to every bound App method — it isn't "trapped" in a
// browser tab sandbox the way a typical XSS would be. Prefer
// .textContent/document.createElement when there's no real need for
// embedded HTML.
function escHTML(s) {
  return String(s)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}
// escAttr also escapes & — without it a value like "a&amp;b" (a password,
// a dev command) came back as "a&b" after an Editar round trip.
function escAttr(s) {
  return escHTML(s).replace(/"/g, "&quot;").replace(/'/g, "&#39;");
}

// --- dynamic per-type field construction (shared between Criar and the
// Editar modal) -----------------------------------------------------

// opts.secret renders a password input plus a "Mostrar"/"Ocultar" toggle
// (the delegated listener below) — a generated password still has to be
// readable once, to be written down.
function textFieldHTML(id, label, opts) {
  opts = opts || {};
  const input =
    '<input id="' + id + '" type="' + (opts.secret ? "password" : (opts.type || "text")) + '" class="input input-bordered' + (opts.secret ? " join-item flex-1" : "") + '" ' +
    (opts.placeholder ? 'placeholder="' + escAttr(opts.placeholder) + '" ' : "") +
    (opts.value !== undefined && opts.value !== null ? 'value="' + escAttr(opts.value) + '" ' : "") +
    (opts.secret ? 'autocomplete="off" ' : "") +
    "/>";
  return (
    '<label class="form-control">' +
    '<div class="label"><span class="label-text">' + label + "</span></div>" +
    (opts.secret
      ? '<div class="join w-full">' + input +
        '<button type="button" class="btn join-item" data-reveal="' + escAttr(id) + '" aria-pressed="false">Mostrar</button></div>'
      : input) +
    "</label>"
  );
}
document.addEventListener("click", function (ev) {
  const btn = ev.target.closest("button[data-reveal]");
  if (!btn) return;
  ev.preventDefault(); // inside a <label>: don't focus/submit anything else
  const input = document.getElementById(btn.dataset.reveal);
  if (!input) return;
  const show = input.type === "password";
  input.type = show ? "text" : "password";
  btn.textContent = show ? "Ocultar" : "Mostrar";
  btn.setAttribute("aria-pressed", show ? "true" : "false");
});
function selectFieldHTML(id, label, options, selected) {
  const opts = (options || []).map(function (v) {
    return '<option value="' + escAttr(v) + '"' + (v === selected ? " selected" : "") + ">" + escHTML(v) + "</option>";
  }).join("");
  return (
    '<label class="form-control">' +
    '<div class="label"><span class="label-text">' + label + "</span></div>" +
    '<select id="' + id + '" class="select select-bordered">' + opts + "</select></label>"
  );
}
function checkboxFieldHTML(id, label, checked, disabled, hint) {
  return (
    '<label class="label cursor-pointer justify-start gap-3">' +
    '<input id="' + id + '" type="checkbox" class="checkbox"' + (checked ? " checked" : "") + (disabled ? " disabled" : "") + " />" +
    '<span class="label-text">' + label + (hint ? '<br><span class="text-xs opacity-60">' + hint + "</span>" : "") + "</span>" +
    "</label>"
  );
}

// reconcileKeyedList keeps container in sync with items (array), avoiding
// innerHTML="" + full rebuild on every update — used by
// renderAgentSkillsTable/renderMCPServersTable/renderRepoCards. keyOf(item)
// identifies each node across calls (e.g. slug/path); signatureOf(item)
// summarizes every field that affects the node (content, button state,
// text) — when it hasn't changed since the last render, the existing node
// is left untouched (buildNode is never called); when it has changed, or
// the item is new, buildNode(item) builds a fresh node from scratch
// (handlers are always current, never at risk of referencing stale
// state); items that disappeared from items are removed. The final node
// order follows items' own order.
function reconcileKeyedList(container, items, keyOf, signatureOf, buildNode) {
  const existing = new Map();
  Array.prototype.forEach.call(container.children, function (el) {
    if (el.dataset && el.dataset.rkKey !== undefined) existing.set(el.dataset.rkKey, el);
  });

  let afterNode = null; // last node already in its right place in this pass
  items.forEach(function (item) {
    const key = String(keyOf(item));
    const sig = String(signatureOf(item));
    let node = existing.get(key);
    if (node) {
      existing.delete(key);
      if (node.dataset.rkSig !== sig) {
        const fresh = buildNode(item);
        fresh.dataset.rkKey = key;
        fresh.dataset.rkSig = sig;
        node.replaceWith(fresh);
        node = fresh;
      }
    } else {
      node = buildNode(item);
      node.dataset.rkKey = key;
      node.dataset.rkSig = sig;
    }
    const wantedNext = afterNode ? afterNode.nextSibling : container.firstChild;
    if (wantedNext !== node) container.insertBefore(node, wantedNext);
    afterNode = node;
  });

  // leftovers: keys that existed before and no longer appear in items
  existing.forEach(function (node) { node.remove(); });
}

// buildContainerFieldsHTML renders the fields for one final container
// kind ("nginx"|"mariadb"|"moodle"|"php"|"generic"|"node"|"php_node")
// into prefix-scoped ids (ex. "dc-folder", "edit-folder") — reused by
// both #dc-fields (Criar) and #edit-fields (Editar), values pre-fills
// when editing.
function buildContainerFieldsHTML(prefix, kind, catalog, values) {
  values = values || {};
  const p = prefix;
  if (kind === "nginx") {
    let html = '<p class="opacity-70 text-sm">O contêiner Nginx não tem parâmetros — ele roteia todos os *.localhost já configurados.</p>';
    // Warning only the first time (catalog.mkcertReady=false): creating
    // Nginx now will call "mkcert -install" to generate the *.localhost
    // certificate — that command can prompt for an admin password on its
    // own (outside Perci's pkexec), and the GUI has no way to answer that
    // prompt. After the first time the certificate already exists and
    // this warning disappears.
    if (catalog && !catalog.mkcertReady) {
      html +=
        '<div role="alert" class="alert alert-warning alert-soft mt-3 text-sm items-start">' +
        LUCIDE.svg("triangleAlert", { size: 18 }) +
        '<span>Primeira criação: o Perci vai gerar um certificado local via <code>mkcert</code>. ' +
        "Se isso pedir uma senha de administrador e travar, abra um terminal fora do Perci e rode " +
        "<code>mkcert -install</code> manualmente uma vez antes de clicar em Criar.</span>" +
        "</div>";
    }
    return html;
  }
  if (kind === "mariadb") {
    return (
      textFieldHTML(p + "-dbUser", "Usuário do banco", { value: values.dbUser || "dev_user" }) +
      textFieldHTML(p + "-dbPass", "Senha", { value: values.dbPass || "", secret: true })
    );
  }
  const isPHPFamily = kind === "moodle" || kind === "php" || kind === "generic";
  const isNodeFamily = kind === "node" || kind === "php_node";
  let html = "";
  html += textFieldHTML(p + "-name", "Nome do aplicativo", { value: values.name || "" });
  html += textFieldHTML(p + "-folder", "Pasta (nome técnico do contêiner)", { value: values.folder || "", placeholder: "meuapp" });
  html += textFieldHTML(p + "-url", "URL", { value: values.url || "" });

  if (kind === "moodle") {
    const mv = values.moodleVersion || catalog.moodleVersions[catalog.moodleVersions.length - 1];
    html += selectFieldHTML(p + "-moodleVersion", "Versão do Moodle", catalog.moodleVersions, mv);
  }
  if (isPHPFamily) {
    html += '<div id="' + p + '-phpVersion-wrap">' + selectFieldHTML(p + "-phpVersion", "Versão PHP", catalog.phpVersions, values.phpVersion || catalog.phpVersions[0]) + "</div>";
    html += textFieldHTML(p + "-phpMemoryLimit", "Limite de memória PHP (opcional)", { value: values.phpMemoryLimit || "", placeholder: "512M" });
  }
  if (isNodeFamily) {
    html += selectFieldHTML(p + "-nodeVersion", "Versão Node", catalog.nodeVersions, values.nodeVersion || catalog.nodeVersions[0]);
    html += textFieldHTML(p + "-devCommand", "Comando de start", { value: values.devCommand || "npm run dev" });
    html += textFieldHTML(p + "-devPort", "Porta", { type: "number", value: values.devPort || 5173 });
  }
  if (kind === "php_node") {
    html += textFieldHTML(p + "-workerCommand", "Comando do worker (opcional)", { value: values.workerCommand || "" });
  }
  if (isPHPFamily || kind === "php_node") {
    html += checkboxFieldHTML(
      p + "-dbAccess", "Habilitar acesso ao banco", !!values.dbAccess, !catalog.mariadbReady,
      catalog.mariadbReady ? "" : "Crie um Contêiner MariaDB primeiro para habilitar isso."
    );
  }
  return html;
}

// wireMoodlePHPFilter: "Versão PHP" only shows the versions compatible
// with the chosen "Versão do Moodle" (appstack.PHPVersionsForMoodleVersion).
function wireMoodlePHPFilter(prefix) {
  const moodleSel = document.getElementById(prefix + "-moodleVersion");
  const phpWrap = document.getElementById(prefix + "-phpVersion-wrap");
  if (!moodleSel || !phpWrap) return;
  function refresh() {
    const current = document.getElementById(prefix + "-phpVersion");
    const currentVal = current ? current.value : "";
    App.GetPHPVersionsForMoodleVersion(moodleSel.value).then(function (versions) {
      phpWrap.innerHTML = selectFieldHTML(prefix + "-phpVersion", "Versão PHP", versions, versions.indexOf(currentVal) >= 0 ? currentVal : versions[0]);
    });
  }
  moodleSel.addEventListener("change", refresh);
  refresh();
}

// wireFolderURLAutofill: "URL" follows "Pasta" (folder + ".localhost")
// until the user edits the URL by hand.
function wireFolderURLAutofill(prefix) {
  const folderEl = document.getElementById(prefix + "-folder");
  const urlEl = document.getElementById(prefix + "-url");
  if (!folderEl || !urlEl) return;
  let urlEdited = !!urlEl.value;
  urlEl.addEventListener("input", function () { urlEdited = true; });
  folderEl.addEventListener("input", function () {
    if (urlEdited) return;
    const slug = folderEl.value.trim().toLowerCase().replace(/[^a-z0-9-]/g, "-");
    urlEl.value = slug ? slug + ".localhost" : "";
  });
}

function readContainerFieldsValues(prefix, kind) {
  const p = prefix;
  const get = function (id) { const el = document.getElementById(id); return el ? el.value : ""; };
  const getChecked = function (id) { const el = document.getElementById(id); return !!(el && el.checked); };

  if (kind === "nginx") return { kind: "nginx" };
  if (kind === "mariadb") return { kind: "mariadb", dbUser: get(p + "-dbUser"), dbPass: get(p + "-dbPass") };

  const req = { kind: kind, name: get(p + "-name"), folder: get(p + "-folder"), url: get(p + "-url") };
  if (kind === "moodle") req.moodleVersion = get(p + "-moodleVersion");
  if (kind === "moodle" || kind === "php" || kind === "generic" || kind === "php_node") {
    req.phpVersion = get(p + "-phpVersion");
    req.phpMemoryLimit = get(p + "-phpMemoryLimit");
    req.dbAccess = getChecked(p + "-dbAccess");
  }
  if (kind === "node" || kind === "php_node") {
    req.nodeVersion = get(p + "-nodeVersion");
    req.devCommand = get(p + "-devCommand");
    req.devPort = parseInt(get(p + "-devPort"), 10) || 0;
  }
  if (kind === "php_node") req.workerCommand = get(p + "-workerCommand");
  return req;
}

// createScopedTable drives a "scope + table" screen (IA: SKILLs, IA: MCPs):
// a Global/Local select, the alert describing the scope (with the folder
// picker for Local) and a keyed table of rows fetched for that scope.
//   cfg.els      — { select, alertTitle, alertDesc, pickButton, rows }
//   cfg.noun     — "das SKILLs" / "dos MCPs", used in the alert text
//   cfg.latestKey, cfg.pickFolder(), cfg.fetchRows(global, folder)
//   cfg.keyOf(row), cfg.sigOf(row, state), cfg.buildRow(row, state)
//   cfg.onScope(state) — optional, after every scope/folder change
// Returns the state ({ scope, folder, isGlobal() }) plus render()/reset().
function createScopedTable(cfg) {
  const els = cfg.els;
  const state = {
    scope: "global", // "global" | "local"
    folder: "",
    isGlobal: function () { return state.scope === "global"; },
  };

  function isReady() {
    return state.isGlobal() || !!state.folder;
  }

  function renderAlert() {
    if (cfg.onScope) cfg.onScope(state);
    if (state.isGlobal()) {
      els.alertTitle.textContent = "Instalação Global";
      els.alertDesc.textContent = "A instalação " + cfg.noun + " será feita globalmente.";
      els.pickButton.classList.add("hidden");
      return;
    }
    const desc = "A instalação " + cfg.noun + " será feita exclusivamente para este projeto.";
    els.alertTitle.textContent = "Caminho da pasta";
    if (state.folder) {
      els.alertDesc.innerHTML = escHTML(desc) + '<br><span class="text-xs opacity-70">' + escHTML(state.folder) + "</span>";
    } else {
      els.alertDesc.textContent = desc;
    }
    els.pickButton.textContent = state.folder ? "Alterar Pasta" : "Selecionar Pasta";
    els.pickButton.classList.remove("hidden");
  }

  function render() {
    if (!isReady()) {
      els.rows.innerHTML = "";
      return;
    }
    cfg.fetchRows(state.isGlobal(), state.folder).then(latestOnly(cfg.latestKey, function (rows) {
      // Reconciled by key instead of innerHTML="" + full rebuild — a row
      // is only recreated when its signature changed.
      reconcileKeyedList(
        els.rows,
        rows || [],
        cfg.keyOf,
        function (row) { return cfg.sigOf(row, state); },
        function (row) { return cfg.buildRow(row, state); }
      );
    }));
  }

  els.select.addEventListener("change", function () {
    state.scope = els.select.value;
    renderAlert();
    render();
  });
  els.pickButton.addEventListener("click", function () {
    cfg.pickFolder().then(function (path) {
      if (!path) return; // user cancelled the dialog
      state.folder = path;
      renderAlert();
      render();
    });
  });

  state.render = render;
  state.reset = function () {
    state.scope = "global";
    state.folder = "";
    els.select.value = "global";
    renderAlert();
    render();
  };
  return state;
}

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

// Dev Tools :: IA: MCPs.
//
// Classic script (not a module): every file under js/ shares one global
// scope, loaded in order by index.html — see js/bootstrap.js.
"use strict";

// ===========================================================================
// "Dev Tools :: IA: MCPs" — backed by internal/dev/mcpservers (a
// different mechanism per agent — Claude Code/Codex via CLI, Antigravity
// by editing mcp_config.json — not a single tool like skills.sh).
// ===========================================================================

const mcpTable = createScopedTable({
  els: { select: mcpScopeSelect, alertTitle: mcpScopeAlertTitle, alertDesc: mcpScopeAlertDesc, pickButton: btnMcpPickFolder, rows: mcpRows },
  noun: "dos MCPs",
  latestKey: "mcpservers",
  pickFolder: function () { return App.PickMCPFolder(); },
  fetchRows: function (global, folder) { return App.GetMCPServerRows(global, folder); },
  keyOf: function (row) { return row.slug; },
  // Scope/folder are part of the signature: a row keeps the parameter
  // picked in it (currentParam) in its closure, which must not carry over
  // to another scope or project folder.
  sigOf: function (row, st) {
    return [st.scope, st.folder, row.slug, row.name, row.installed, row.param, row.paramKind, row.paramLabel, row.manual, row.manualUrl, row.manualNote].join("|");
  },
  buildRow: function (row) { return buildMCPServerRow(row); },
  onScope: function (st) { mcpCodexNote.classList.toggle("hidden", st.isGlobal()); },
});

function buildMCPServerRow(row) {
  const tr = document.createElement("tr");

  const tdName = document.createElement("td");
  tdName.textContent = row.name;
  tr.appendChild(tdName);

  const tdActions = document.createElement("td");

  // Godot Studio: no command to automate — just a link + instructions.
  if (row.manual) {
    const wrap = document.createElement("div");
    const a = document.createElement("a");
    a.href = "#";
    a.className = "link link-primary text-sm";
    a.textContent = "Ver instruções";
    a.addEventListener("click", function (ev) {
      ev.preventDefault();
      Browser.OpenURL(row.manualUrl);
    });
    const note = document.createElement("p");
    note.className = "text-xs opacity-70 mt-1";
    note.textContent = row.manualNote;
    wrap.appendChild(a);
    wrap.appendChild(note);
    tdActions.appendChild(wrap);
    tr.appendChild(tdActions);
    return tr;
  }

  const wrap = document.createElement("div");
  wrap.className = "flex flex-col gap-2";
  let currentParam = row.param || "";

  const btnToggle = document.createElement("button");
  btnToggle.type = "button";
  const btnUpdate = document.createElement("button");
  btnUpdate.type = "button";
  btnUpdate.className = "btn btn-sm btn-outline";
  btnUpdate.textContent = "Atualizar";

  function updateButtonsState() {
    const needsParam = !!row.paramKind && !currentParam;
    btnToggle.disabled = needsParam;
    btnUpdate.disabled = needsParam || !row.installed;
  }

  // Filesystem/SQLite: need an extra value (folder/file) before being
  // able to install/update — without it there's no fixed command to run.
  if (row.paramKind) {
    const paramRow = document.createElement("div");
    paramRow.className = "flex items-center gap-2 text-xs";
    const paramText = document.createElement("span");
    paramText.className = "opacity-70";
    paramText.textContent = row.paramLabel + ": " + (currentParam || "não definido");
    const btnPick = document.createElement("button");
    btnPick.type = "button";
    btnPick.className = "btn btn-xs btn-outline";
    btnPick.textContent = "Selecionar";
    btnPick.addEventListener("click", function () {
      App.PickMCPServerParam(row.slug).then(function (path) {
        if (!path) return;
        currentParam = path;
        paramText.textContent = row.paramLabel + ": " + currentParam;
        updateButtonsState();
      });
    });
    paramRow.appendChild(paramText);
    paramRow.appendChild(btnPick);
    wrap.appendChild(paramRow);
  }

  btnToggle.className = "btn btn-sm " + (row.installed ? "btn-outline btn-error" : "btn-primary");
  btnToggle.textContent = row.installed ? "Remover" : "Instalar";
  btnToggle.addEventListener("click", function () {
    if (running || btnToggle.disabled) return;
    const global = mcpTable.isGlobal();
    if (row.installed) {
      startExecution("Remover MCP: " + row.name);
      App.RemoveMCPServer(row.slug, global, mcpTable.folder).catch(failRun);
    } else {
      startExecution("Instalar MCP: " + row.name);
      App.InstallMCPServer(row.slug, global, mcpTable.folder, currentParam).catch(failRun);
    }
  });
  btnUpdate.addEventListener("click", function () {
    if (running || btnUpdate.disabled) return;
    startExecution("Atualizar MCP: " + row.name);
    App.UpdateMCPServer(row.slug, mcpTable.isGlobal(), mcpTable.folder, currentParam).catch(failRun);
  });

  updateButtonsState();

  const btnsRow = document.createElement("div");
  btnsRow.className = "flex gap-2";
  btnsRow.appendChild(btnToggle);
  btnsRow.appendChild(btnUpdate);
  wrap.appendChild(btnsRow);

  tdActions.appendChild(wrap);
  tr.appendChild(tdActions);
  return tr;
}

function renderMCPServersTable() { mcpTable.render(); }

function renderMCPServersScreen() { mcpTable.reset(); }

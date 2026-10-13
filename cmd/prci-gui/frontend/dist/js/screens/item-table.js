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

// Item table screens (Desenvolvimento :: Pré-requisitos and Linguagens e
// SDKs) — a table with one row per item and its own Instalar/Atualizar/
// Desinstalar buttons. Instalar is enabled only while the item isn't
// installed and nothing it requires is missing (the row then shows
// "Requer: ..."); Atualizar and Desinstalar only while it is installed.
// Desinstalar always asks for confirmation.
//
// Classic script (not a module): shares one global scope with the other
// scripts under js/.
"use strict";

// ITEM_TABLE_SCREENS maps actionId to the two bound methods the screen
// needs: fetch() brings the rows, run(id, op) runs "install", "update" or
// "uninstall" on one item. Each new screen of this type only needs one
// entry here.
const ITEM_TABLE_SCREENS = {
  prereqs: {
    header: "Pré-requisito",
    fetch: function () { return App.GetPrereqsInfo(); },
    run: function (id, op) { return App.RunPrereqAction(id, op); },
  },
  sdks: {
    header: "SDK",
    fetch: function () { return App.GetSDKsInfo(); },
    run: function (id, op) { return App.RunSDKAction(id, op); },
  },
};

// itemTableButtons holds the enabled buttons of the rows on screen — all of
// them are disabled while one action runs.
let itemTableButtons = [];

function itemTableButton(label, cls, enabled, onClick) {
  const btn = document.createElement("button");
  btn.type = "button";
  btn.className = "btn btn-sm " + cls;
  btn.textContent = label;
  btn.disabled = !enabled;
  btn.addEventListener("click", function () {
    if (running || btn.disabled) return;
    onClick();
  });
  if (enabled) itemTableButtons.push(btn);
  return btn;
}

function runItemTableAction(cfg, it, op, label) {
  beginRun(label + " " + it.label, function () { return cfg.run(it.id, op); }, { buttons: itemTableButtons });
}

// buildItemTableRow builds the <tr> of it: name, description and missing
// requirements in the first cell, the three buttons in the second.
function buildItemTableRow(cfg, it) {
  const tr = document.createElement("tr");

  const tdInfo = document.createElement("td");
  const name = document.createElement("p");
  name.className = "font-medium";
  name.textContent = it.label;
  tdInfo.appendChild(name);
  if (it.description) {
    const desc = document.createElement("p");
    desc.className = "text-sm opacity-70";
    desc.textContent = it.description;
    tdInfo.appendChild(desc);
  }
  const missing = it.missing || [];
  if (!it.installed && missing.length) {
    const hint = document.createElement("p");
    hint.className = "text-sm text-warning";
    hint.textContent = "Requer: " + missing.join(", ");
    tdInfo.appendChild(hint);
  }

  const tdActions = document.createElement("td");
  tdActions.className = "align-top";
  const wrap = document.createElement("div");
  wrap.className = "flex gap-2 justify-end";
  wrap.appendChild(itemTableButton("Instalar", "btn-primary", !it.installed && !missing.length, function () {
    runItemTableAction(cfg, it, "install", "Instalar");
  }));
  wrap.appendChild(itemTableButton("Atualizar", "btn-outline", it.installed, function () {
    runItemTableAction(cfg, it, "update", "Atualizar");
  }));
  wrap.appendChild(itemTableButton("Desinstalar", "btn-outline btn-error", it.installed, function () {
    const message = "Desinstalar " + it.label + "?" + (it.removeWarning ? "\n\n" + it.removeWarning : "");
    openConfirm("Confirmar remoção", message, function () {
      runItemTableAction(cfg, it, "uninstall", "Desinstalar");
    });
  }));
  tdActions.appendChild(wrap);

  tr.appendChild(tdInfo);
  tr.appendChild(tdActions);
  return tr;
}

function renderItemTableScreen(item) {
  const cfg = ITEM_TABLE_SCREENS[item.actionId];
  itemTableIntro.textContent = item.desc || "";
  itemTableHeader.textContent = cfg.header;
  itemTableRows.innerHTML = '<tr><td colspan="2" class="opacity-60">Verificando o que está instalado...</td></tr>';
  itemTableButtons = [];
  // One latestOnly guard for both outcomes: neither the rows nor the error
  // of an older request reach the table after a newer one started.
  const show = latestOnly("item-table", function (result) {
    itemTableRows.innerHTML = "";
    itemTableButtons = [];
    if (result.err !== undefined) {
      const tr = document.createElement("tr");
      const td = document.createElement("td");
      td.colSpan = 2;
      td.className = "text-error";
      td.textContent = "Falha ao verificar o que está instalado: " + result.err;
      tr.appendChild(td);
      itemTableRows.appendChild(tr);
      return;
    }
    (result.items || []).forEach(function (it) {
      itemTableRows.appendChild(buildItemTableRow(cfg, it));
    });
  });
  cfg.fetch().then(
    function (items) { show({ items: items }); },
    function (err) { show({ err: err }); }
  );
}

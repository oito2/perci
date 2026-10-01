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

// Dev Tools :: IA: SKILLs.
//
// Classic script (not a module): every file under js/ shares one global
// scope, loaded in order by index.html — see js/bootstrap.js.
"use strict";

// ===========================================================================
// "Dev Tools :: IA: SKILLs" — backed by internal/dev/agentskills
// (npx skills).
// ===========================================================================

asSkillsShLink.addEventListener("click", function (ev) {
  ev.preventDefault();
  Browser.OpenURL("https://www.skills.sh/");
});

const asTable = createScopedTable({
  els: { select: asScopeSelect, alertTitle: asScopeAlertTitle, alertDesc: asScopeAlertDesc, pickButton: btnAsPickFolder, rows: asRows },
  noun: "das SKILLs",
  latestKey: "agentskills",
  pickFolder: function () { return App.PickAgentSkillsFolder(); },
  fetchRows: function (global, folder) { return App.GetAgentSkillsRows(global, folder); },
  keyOf: function (row) { return row.slug; },
  sigOf: function (row) { return row.slug + "|" + row.name + "|" + row.installed; },
  buildRow: function (row) { return buildAgentSkillRow(row); },
});

function renderAgentSkillsTable() { asTable.render(); }

function buildAgentSkillRow(row) {
  const tr = document.createElement("tr");

  const tdName = document.createElement("td");
  tdName.textContent = row.name;
  tr.appendChild(tdName);

  const tdActions = document.createElement("td");
  const wrap = document.createElement("div");
  wrap.className = "flex gap-2";

  const btnToggle = document.createElement("button");
  btnToggle.type = "button";
  btnToggle.className = "btn btn-sm " + (row.installed ? "btn-outline btn-error" : "btn-primary");
  btnToggle.textContent = row.installed ? "Remover" : "Instalar";

  const btnUpdate = document.createElement("button");
  btnUpdate.type = "button";
  btnUpdate.className = "btn btn-sm btn-outline";
  btnUpdate.textContent = "Atualizar";
  btnUpdate.disabled = !row.installed;

  btnToggle.addEventListener("click", function () {
    if (running) return;
    const global = asTable.isGlobal();
    if (row.installed) {
      startExecution("Remover skill: " + row.name);
      App.RemoveAgentSkill(row.slug, global, asTable.folder).catch(failRun);
    } else {
      startExecution("Instalar skill: " + row.name);
      App.InstallAgentSkill(row.slug, global, asTable.folder).catch(failRun);
    }
  });
  btnUpdate.addEventListener("click", function () {
    if (running || btnUpdate.disabled) return;
    startExecution("Atualizar skill: " + row.name);
    App.UpdateAgentSkill(row.slug, asTable.isGlobal(), asTable.folder).catch(failRun);
  });

  wrap.appendChild(btnToggle);
  wrap.appendChild(btnUpdate);
  tdActions.appendChild(wrap);
  tr.appendChild(tdActions);
  return tr;
}

function renderAgentSkillsScreen() { asTable.reset(); }

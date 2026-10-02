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

// Dev Tools :: IA: Contextos.
//
// Classic script (not a module): shares one global scope with the other
// scripts under js/.
"use strict";

// ===========================================================================
// "Dev Tools :: IA: Contextos".
// ===========================================================================

let aicFolder = "";

function renderAicAlert() {
  if (!aicFolder) {
    aicAlertTitle.textContent = "Selecione a pasta de trabalho";
    aicAlertDesc.textContent = "As opções abaixo serão habilitadas ao selecionar a pasta de trabalho";
    btnAicPickFolder.textContent = "Selecionar Pasta";
    return;
  }
  aicAlertTitle.textContent = "Nome da pasta: " + reposFolderName(aicFolder);
  aicAlertDesc.textContent = aicFolder;
  btnAicPickFolder.textContent = "Alterar Pasta";
}

btnAicPickFolder.addEventListener("click", function () {
  App.PickAIContextFolder().then(function (path) {
    if (!path) return; // user cancelled the dialog
    aicFolder = path;
    renderAicAlert();
    renderAicItems();
  });
});

function renderAicItems() {
  aicItems.innerHTML = "";
  btnAicApply.disabled = true;
  btnAicApply.onclick = null;
  if (!aicFolder) return;

  App.GetAIContextItems(aicFolder).then(latestOnly("aicontext", function (items) {
    items = items || [];
    const checkboxes = {}; // it.id -> checkbox, captured at creation time
    items.forEach(function (it) {
      const built = buildChecklistItemLI("aic-check-", it, {});
      checkboxes[it.id] = built.checkbox;
      aicItems.appendChild(built.li);
    });

    btnAicApply.disabled = false;
    btnAicApply.onclick = function () {
      if (running) return;
      const selected = items
        .filter(function (it) { return checkboxes[it.id].checked; })
        .map(function (it) { return it.id; });
      startExecution("Aplicar Contextos de IA");
      App.ApplyAIContext(aicFolder, selected).catch(failRun);
    };
  }));
}

function renderAIContextScreen() {
  aicFolder = "";
  renderAicAlert();
  renderAicItems();
}

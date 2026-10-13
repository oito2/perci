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

// aicPresets holds the project types (name, models, hint) from the backend.
let aicPresets = [];

// setAicPresetHint shows the hint of the selected project type, if any.
function setAicPresetHint(preset) {
  const hint = preset && preset.hint ? preset.hint : "";
  aicPresetHint.textContent = hint;
  aicPresetHint.classList.toggle("hidden", !hint);
}

// renderAicPresetOptions fills the project type select: "Personalizado"
// plus one option per preset.
function renderAicPresetOptions() {
  aicPreset.innerHTML = "";
  const custom = document.createElement("option");
  custom.value = "";
  custom.textContent = "Personalizado";
  aicPreset.appendChild(custom);
  aicPresets.forEach(function (p) {
    const opt = document.createElement("option");
    opt.value = p.name;
    opt.textContent = p.name;
    aicPreset.appendChild(opt);
  });
  aicPreset.value = "";
  setAicPresetHint(null);
}

App.GetAIContextPresets().then(function (presets) {
  aicPresets = presets || [];
  renderAicPresetOptions();
}).catch(function () {
  aicPresets = [];
});

function renderAicItems() {
  aicItems.innerHTML = "";
  btnAicApply.disabled = true;
  btnAicApply.onclick = null;
  aicPreset.disabled = true;
  aicPreset.onchange = null;
  aicPreset.value = "";
  setAicPresetHint(null);
  if (!aicFolder) {
    invalidate("aicontext");
    return;
  }

  App.GetAIContextItems(aicFolder).then(latestOnly("aicontext", function (items) {
    items = items || [];
    const checkboxes = {}; // it.id -> checkbox, captured at creation time
    items.forEach(function (it) {
      const built = buildChecklistItemLI("aic-check-", it, {});
      checkboxes[it.id] = built.checkbox;
      // A manual change no longer matches the chosen project type.
      built.checkbox.addEventListener("change", function () {
        aicPreset.value = "";
        setAicPresetHint(null);
      });
      aicItems.appendChild(built.li);
    });

    aicPreset.disabled = aicPresets.length === 0;
    aicPreset.onchange = function () {
      const preset = aicPresets.find(function (p) { return p.name === aicPreset.value; });
      setAicPresetHint(preset);
      if (!preset) return;
      items.forEach(function (it) {
        checkboxes[it.id].checked = preset.models.indexOf(it.id) >= 0;
      });
    };

    btnAicApply.disabled = false;
    btnAicApply.onclick = function () {
      if (running) return;
      const selected = items
        .filter(function (it) { return checkboxes[it.id].checked; })
        .map(function (it) { return it.id; });
      beginRun("Aplicar Contextos de IA", function () { return App.ApplyAIContext(aicFolder, selected); });
    };
  }));
}

function renderAIContextScreen() {
  aicFolder = "";
  renderAicAlert();
  renderAicItems();
}

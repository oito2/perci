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

// aicFolderName is the last segment of path (path itself when it has none).
function aicFolderName(path) {
  const parts = path.split("/").filter(Boolean);
  return parts.length ? parts[parts.length - 1] : path;
}

function renderAicAlert() {
  if (!aicFolder) {
    aicAlertTitle.textContent = "Selecione a pasta de trabalho";
    aicAlertDesc.textContent = "As opções abaixo serão habilitadas ao selecionar a pasta de trabalho";
    btnAicPickFolder.textContent = "Selecionar Pasta";
    return;
  }
  aicAlertTitle.textContent = "Nome da pasta: " + aicFolderName(aicFolder);
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

// renderAicAgents fills the agent checkboxes from agents
// ({id,label,selected}).
function renderAicAgents(agents) {
  aicAgents.innerHTML = "";
  (agents || []).forEach(function (a) {
    const label = document.createElement("label");
    label.className = "label cursor-pointer gap-2";
    const box = document.createElement("input");
    box.type = "checkbox";
    box.className = "checkbox checkbox-sm";
    box.dataset.agent = a.id;
    box.checked = !!a.selected;
    label.appendChild(box);
    const text = document.createElement("span");
    text.className = "label-text";
    text.textContent = a.label;
    label.appendChild(text);
    aicAgents.appendChild(label);
  });
}

// aicCheckedAgents lists the IDs of the checked agents.
function aicCheckedAgents() {
  return Array.prototype.filter.call(aicAgents.querySelectorAll("input[data-agent]"), function (box) { return box.checked; })
    .map(function (box) { return box.dataset.agent; });
}

// AIC_STATUS describes each standard state (managerai.GuideStatus).
const AIC_STATUS = {
  applied: { label: "aplicado", cls: "badge-success", title: "Igual ao que esta versão do Perci gera." },
  outdated: { label: "desatualizado", cls: "badge-warning", title: "Gerado por uma versão anterior do Perci: Aplicar atualiza." },
  edited: { label: "editado", cls: "badge-error", title: "Editado à mão: Aplicar substitui (guardando um .perci-bak). Regras do projeto ficam em .instructions/PROJECT.md." },
};

// buildAicItem builds the row of one standard: checkbox, name, state
// badge and description; a complement is indented under its base.
function buildAicItem(it) {
  const built = buildChecklistItemLI("aic-check-", { id: it.id, label: it.label, description: it.description, installed: !!it.status }, { withDescription: true });
  if (it.base) built.li.className += " ml-8";
  const st = AIC_STATUS[it.status];
  if (st) {
    const badge = document.createElement("span");
    badge.className = "badge badge-sm ml-2 " + st.cls;
    badge.textContent = st.label;
    badge.title = st.title;
    built.li.children[1].children[0].appendChild(badge);
  }
  return built;
}

const AIC_ACTIONS = { create: "criar", update: "atualizar", remove: "remover" };

// aicPreviewMessage lists the changes of an Aplicar for the confirmation.
function aicPreviewMessage(changes) {
  const lines = changes.map(function (c) { return "• " + (AIC_ACTIONS[c.action] || c.action) + ": " + c.path; });
  return "Arquivos que serão alterados em " + aicFolder + ":\n\n" + lines.join("\n") +
    "\n\nArquivos substituídos ficam com uma cópia .perci-bak. Continuar?";
}

function renderAicItems() {
  aicItems.innerHTML = "";
  aicAgents.innerHTML = "";
  btnAicApply.disabled = true;
  btnAicApply.onclick = null;
  aicPreset.disabled = true;
  aicPreset.onchange = null;
  aicPreset.value = "";
  setAicPresetHint(null);
  if (!aicFolder) {
    invalidate("aicontext");
    invalidate("aicontext-agents");
    invalidate("aicontext-suggest");
    return;
  }

  App.GetAIContextAgents(aicFolder).then(latestOnly("aicontext-agents", renderAicAgents));
  App.GetAIContextItems(aicFolder).then(latestOnly("aicontext", function (items) {
    items = items || [];
    const checkboxes = {}; // it.id -> checkbox, captured at creation time
    items.forEach(function (it) {
      const built = buildAicItem(it);
      checkboxes[it.id] = built.checkbox;
      built.checkbox.addEventListener("change", function () {
        // A complement needs its base.
        if (built.checkbox.checked && it.base && checkboxes[it.base]) checkboxes[it.base].checked = true;
        // A manual change no longer matches the chosen project type.
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

    // A folder with no standard yet starts on the project type its files
    // point to.
    if (!items.some(function (it) { return it.status; })) {
      const folder = aicFolder;
      App.GetAIContextSuggestedPreset(folder).then(latestOnly("aicontext-suggest", function (name) {
        const preset = aicPresets.find(function (p) { return p.name === name; });
        if (!preset || folder !== aicFolder) return;
        aicPreset.value = name;
        aicPreset.onchange();
        aicPresetHint.textContent = "Sugerido pelos arquivos da pasta." + (preset.hint ? " " + preset.hint : "");
        aicPresetHint.classList.remove("hidden");
      }));
    }

    btnAicApply.disabled = false;
    btnAicApply.onclick = function () {
      if (running) return;
      const selected = items
        .filter(function (it) { return checkboxes[it.id].checked; })
        .map(function (it) { return it.id; });
      const agents = aicCheckedAgents();
      if (agents.length === 0) {
        openError("Marque ao menos um agente de IA.");
        return;
      }
      const folder = aicFolder;
      App.PreviewAIContext(folder, selected, agents).then(function (changes) {
        changes = changes || [];
        if (changes.length === 0) {
          openMessage("Nada a mudar", "Os arquivos de " + folder + " já estão como esta seleção gera.", false);
          return;
        }
        openConfirm("Aplicar Contextos de IA", aicPreviewMessage(changes), function () {
          beginRun("Aplicar Contextos de IA", function () { return App.ApplyAIContext(folder, selected, agents); });
        });
      }).catch(function (err) { openError("Falha ao calcular as mudanças: " + err); });
    };
  }));
}

function renderAIContextScreen() {
  aicFolder = "";
  renderAicAlert();
  renderAicItems();
}

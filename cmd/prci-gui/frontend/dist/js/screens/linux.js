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

// Linux :: Pós-instalação and Linux :: Atualizar Sistema.
//
// Classic script (not a module): shares one global scope with the other
// scripts under js/.
"use strict";

// --- "Linux :: Pós-instalação" — one row per item of the detected
// profile, each with its own "Aplicar" button. An applied item shows a
// disabled "Aplicado" button; an item whose dependsOn isn't applied yet
// has its button disabled with a "Requer: <item>" hint. ------------------

// postinstallButtons holds the "Aplicar" buttons of the rows on screen —
// all of them are disabled while one item runs.
let postinstallButtons = [];

// buildPostinstallRow builds the <li> of act: label, description and,
// when blocked by its dependency, the "Requer" hint, plus the button.
// applied maps action ID -> applied; labels maps action ID -> label.
function buildPostinstallRow(act, applied, labels) {
  const li = document.createElement("li");
  li.className = "flex items-start justify-between gap-4";

  const textWrap = document.createElement("div");
  const label = document.createElement("p");
  label.className = "font-medium";
  label.textContent = act.label;
  textWrap.appendChild(label);
  if (act.description) {
    const desc = document.createElement("p");
    desc.className = "text-sm opacity-70";
    desc.textContent = act.description;
    textWrap.appendChild(desc);
  }

  const btn = document.createElement("button");
  btn.className = "btn btn-sm btn-primary shrink-0";
  btn.id = "pi-apply-" + act.id;
  const blocked = !!act.dependsOn && !applied[act.dependsOn];
  if (applied[act.id]) {
    btn.textContent = "Aplicado";
    btn.disabled = true;
  } else {
    btn.textContent = "Aplicar";
    btn.disabled = blocked;
    if (blocked) {
      const hint = document.createElement("p");
      hint.className = "text-sm text-warning";
      hint.textContent = "Requer: " + (labels[act.dependsOn] || act.dependsOn);
      textWrap.appendChild(hint);
    }
    btn.addEventListener("click", function () {
      beginRun(act.label, function () { return App.RunPostinstallAction(act.id); }, { buttons: postinstallButtons, maxSteps: 1 });
    });
  }

  li.appendChild(textWrap);
  li.appendChild(btn);
  return { li: li, button: btn, enabled: !btn.disabled };
}

function renderPostinstallScreen() {
  postinstallUnsupported.classList.add("hidden");
  postinstallSupported.classList.add("hidden");
  postinstallActionsList.innerHTML = "";
  postinstallButtons = [];

  const status = App.GetPostinstallStatus().catch(function () { return {}; });
  Promise.all([getPostinstallProfile(), status]).then(latestOnly("postinstall", function (results) {
    const profile = results[0];
    const applied = results[1] || {};
    if (!profile || !profile.supported) {
      postinstallUnsupported.classList.remove("hidden");
      return;
    }
    postinstallSupported.classList.remove("hidden");
    postinstallTitle.textContent = "Pós instalação do " + profile.label;

    const actions = profile.actions || [];
    const labels = {};
    actions.forEach(function (act) { labels[act.id] = act.label; });
    postinstallActionsList.innerHTML = "";
    actions.forEach(function (act) {
      const row = buildPostinstallRow(act, applied, labels);
      // Only the buttons that can run are re-enabled when a run ends.
      if (row.enabled) postinstallButtons.push(row.button);
      postinstallActionsList.appendChild(row.li);
    });
  }));
}

// --- "Linux :: Atualizar Sistema" — a status bar with just "Sistema",
// fed by the same source (GetPostinstallProfile) as the "Sistema" stat on
// Home :: Visão Geral. #simple-container is only used by this screen
// today (every other item with an ActionID falls into one of the
// dedicated modes below), so this runs without checking which item is
// selected. --------------------------------------------------------------
function loadSimpleSystemStat() {
  simpleSystemLabel.textContent = "—";
  getPostinstallProfile().then(function (profile) {
    simpleSystemLabel.textContent = (profile && profile.supported) ? profile.label : "Não identificado";
  });
}

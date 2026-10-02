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

// --- "Linux :: Pós-instalação" — checklist with dependencies between
// actions. Different mode from the simple one above: several
// independent actions, some only unlockable to check after another is
// already checked, everything checked by default, unchecking a
// prerequisite cascades to uncheck whatever depends on it. --------------

// applyPostinstallDependencies disables/unchecks in cascade any
// checkbox whose prerequisite (dependsOn) isn't checked — repeats until
// it stabilizes, to cover chains (A depends on B, which depends on C).
// checkboxes (act.id -> checkbox element) is captured once at render
// time by the caller, instead of two getElementById lookups per action
// on every pass of the while loop.
function applyPostinstallDependencies(actions, checkboxes) {
  let changed = true;
  while (changed) {
    changed = false;
    actions.forEach(function (act) {
      if (!act.dependsOn) return;
      const cb = checkboxes[act.id];
      const depCb = checkboxes[act.dependsOn];
      const depOk = !!(depCb && depCb.checked);
      cb.disabled = !depOk;
      if (!depOk && cb.checked) {
        cb.checked = false;
        changed = true;
      }
    });
  }
}

function renderPostinstallScreen() {
  postinstallUnsupported.classList.add("hidden");
  postinstallSupported.classList.add("hidden");
  postinstallActionsList.innerHTML = "";

  getPostinstallProfile().then(latestOnly("postinstall", function (profile) {
    if (!profile || !profile.supported) {
      postinstallUnsupported.classList.remove("hidden");
      return;
    }
    postinstallSupported.classList.remove("hidden");
    postinstallTitle.textContent = "Pós instalação do " + profile.label;

    const actions = profile.actions || [];
    const checkboxes = {}; // act.id -> checkbox, captured at creation time
    actions.forEach(function (act) {
      // All checked by default.
      const built = buildChecklistItemLI("pi-check-", act, {
        checked: true,
        withDescription: true,
        onChange: function () { applyPostinstallDependencies(actions, checkboxes); },
      });
      checkboxes[act.id] = built.checkbox;
      postinstallActionsList.appendChild(built.li);
    });

    applyPostinstallDependencies(actions, checkboxes);

    btnExecutarPostinstall.onclick = function () {
      if (running) return;
      const selected = actions
        .filter(function (act) { return checkboxes[act.id].checked; })
        .map(function (act) { return act.id; });
      if (selected.length === 0) return;

      startExecution("pós-instalação", [btnExecutarPostinstall], selected.length);
      App.RunPostinstall(selected).catch(failRun);
    };
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

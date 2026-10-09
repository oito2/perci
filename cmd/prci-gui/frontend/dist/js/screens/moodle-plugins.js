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

// Dev Tools :: Plugins Moodle.
//
// Classic script (not a module): shares one global scope with the other
// scripts under js/.
"use strict";

// ===========================================================================
// "Dev Tools :: Plugins Moodle".
// ===========================================================================

let mpDev = "";

const MP_NO_FOLDER = "Nenhuma pasta selecionada";

// MP_STATUS maps each mount status to its badge label, color and tooltip.
const MP_STATUS = {
  applied: ["Aplicado", "badge-success", "O contêiner do Moodle e o Nginx já enxergam o plugin."],
  pending: ["Reinício pendente", "badge-warning", "Montado no host. Reinicie o contêiner para aplicar."],
  stopped: ["Contêiner parado", "badge-ghost", "Montado no host. Será aplicado quando o contêiner iniciar."],
  inactive: ["Montagem inativa", "badge-error", "A montagem no host não está ativa. Remova e crie de novo."],
  missing: ["Contêiner não encontrado", "badge-error", "O contêiner deste Moodle não existe mais."],
};

// setMpStatus shows text in the status alert with the daisyUI alert kind
// ("info", "warning", "error"), or hides it when text is empty.
function setMpStatus(text, kind) {
  mpStatus.classList.remove("alert-info", "alert-warning", "alert-error");
  mpStatus.classList.toggle("hidden", !text);
  if (!text) return;
  mpStatus.classList.add("alert-" + kind);
  mpStatusText.textContent = text;
}

// fillMpSelect replaces select's options with values (value = label),
// after an optional placeholder, keeping the previous value when present.
function fillMpSelect(select, options, placeholder) {
  const previous = select.value;
  select.innerHTML = "";
  if (placeholder) {
    const opt = document.createElement("option");
    opt.value = "";
    opt.textContent = placeholder;
    select.appendChild(opt);
  }
  options.forEach(function (o) {
    const opt = document.createElement("option");
    opt.value = o.value;
    opt.textContent = o.label;
    select.appendChild(opt);
  });
  if (options.some(function (o) { return o.value === previous; })) select.value = previous;
}

// setMpLocked locks the type/name fields to the values read from the
// development folder's version.php, or unlocks them.
function setMpLocked(check) {
  const locked = !!(check && check.locked);
  if (locked) {
    if (!Array.prototype.some.call(mpType.options, function (o) { return o.value === check.type; })) {
      fillMpSelect(mpType, [{ value: check.type, label: check.type }], "");
    }
    mpType.value = check.type;
    mpName.value = check.name;
    mpLocked.textContent = "Tipo e nome lidos do version.php da pasta de desenvolvimento: " + check.type + "_" + check.name + ".";
  }
  mpType.disabled = locked;
  mpName.disabled = locked;
  mpLocked.classList.toggle("hidden", !locked);
}

// checkMpForm asks the backend whether the form can be created and
// enables "Criar" only when it can.
function checkMpForm() {
  btnMpCreate.disabled = true;
  if (!mpDev) {
    setMpLocked(null);
    setMpStatus("", "");
    return;
  }
  App.InspectMoodlePlugin(mpDev, mpApp.value, mpType.value, mpName.value.trim()).then(latestOnly("moodleplugins-check", function (check) {
    setMpLocked(check);
    if (check.ok) {
      setMpStatus("Será montado em: " + check.destLabel, "info");
      btnMpCreate.disabled = running;
      return;
    }
    setMpStatus(check.message, "warning");
  })).catch(function (err) { setMpStatus(String(err), "error"); });
}

// loadMpTypes fills "Tipo do plugin" with the types the selected Moodle
// accepts.
function loadMpTypes() {
  if (!mpApp.value) {
    fillMpSelect(mpType, [], "Selecione o Moodle");
    checkMpForm();
    return;
  }
  App.GetMoodlePluginTypes(mpApp.value).then(latestOnly("moodleplugins-types", function (types) {
    fillMpSelect(mpType, (types || []).map(function (t) { return { value: t, label: t }; }), "Selecione o tipo");
    checkMpForm();
  })).catch(function (err) {
    fillMpSelect(mpType, [], "Tipos indisponíveis");
    setMpStatus(String(err), "error");
  });
}

btnMpPickDev.addEventListener("click", function () {
  App.PickMoodlePluginDevFolder().then(function (path) {
    if (!path) return; // user cancelled the dialog
    mpDev = path;
    mpDevPath.textContent = path;
    mpDevPath.classList.remove("opacity-60");
    checkMpForm();
  });
});
mpApp.addEventListener("change", loadMpTypes);
mpType.addEventListener("change", checkMpForm);
mpName.addEventListener("input", checkMpForm);

btnMpCreate.addEventListener("click", function () {
  if (running || !mpDev) return;
  startExecution("Montar plugin Moodle", [btnMpCreate]);
  App.AddMoodlePlugin(mpDev, mpApp.value, mpType.value, mpName.value.trim()).catch(failRun);
});

// buildMpRow returns the table row for one mounted plugin.
function buildMpRow(row) {
  const tr = document.createElement("tr");

  const tdPlugin = document.createElement("td");
  tdPlugin.textContent = row.component;
  if (row.declaredAs) {
    const warn = document.createElement("div");
    warn.className = "text-xs text-warning";
    warn.textContent = "version.php declara " + row.declaredAs;
    tdPlugin.appendChild(warn);
  }
  tr.appendChild(tdPlugin);

  const tdApp = document.createElement("td");
  tdApp.textContent = row.appLabel;
  tdApp.title = row.destLabel;
  tr.appendChild(tdApp);

  const tdDev = document.createElement("td");
  tdDev.className = "break-all";
  tdDev.textContent = row.devLabel;
  tdDev.title = row.devPath;
  tr.appendChild(tdDev);

  const info = MP_STATUS[row.status] || [row.status, "badge-ghost", ""];
  const tdStatus = document.createElement("td");
  const badge = document.createElement("span");
  badge.className = "badge badge-sm whitespace-nowrap " + info[1];
  badge.textContent = info[0];
  badge.title = info[2];
  tdStatus.appendChild(badge);
  tr.appendChild(tdStatus);

  const tdActions = document.createElement("td");
  const wrap = document.createElement("div");
  wrap.className = "flex gap-1";

  const btnRestart = iconButton("refreshCw", "Reiniciar contêiner");
  btnRestart.addEventListener("click", function () {
    if (running) return;
    startExecution("Reiniciar " + row.appLabel);
    App.ApplyMoodlePlugins(row.app).catch(failRun);
  });
  wrap.appendChild(btnRestart);

  const btnRemove = iconButton("trash2", "Remover");
  btnRemove.addEventListener("click", function () {
    if (running) return;
    openConfirm("Remover plugin montado",
      'Desmontar "' + row.component + '" do Moodle ' + row.appLabel + "? A pasta de desenvolvimento não é alterada, e os contêineres do Moodle e do Nginx serão reiniciados.",
      function () {
        if (running) return; // re-checked: the confirm dialog is async
        startExecution("Remover " + row.component);
        App.RemoveMoodlePlugin(row.app, row.relPath).catch(failRun);
      });
  });
  wrap.appendChild(btnRemove);

  tdActions.appendChild(wrap);
  tr.appendChild(tdActions);
  return tr;
}

// loadMpRows fills the table with every mounted plugin and its status.
function loadMpRows() {
  App.GetMoodlePlugins().then(latestOnly("moodleplugins-rows", function (screen) {
    mpRows.innerHTML = "";
    const rows = screen.rows || [];
    rows.forEach(function (row) { mpRows.appendChild(buildMpRow(row)); });
    mpEmpty.classList.toggle("hidden", rows.length > 0);
  })).catch(function (err) { openError("Falha ao listar os plugins montados: " + err); });
}

function renderMoodlePluginsScreen() {
  mpDev = "";
  mpDevPath.textContent = MP_NO_FOLDER;
  mpDevPath.classList.add("opacity-60");
  mpName.value = "";
  setMpLocked(null);
  setMpStatus("", "");
  btnMpCreate.disabled = true;
  App.GetMoodlePlugins().then(latestOnly("moodleplugins-rows", function (screen) {
    const apps = (screen.apps || []).map(function (a) { return { value: a.folder, label: a.label }; });
    fillMpSelect(mpApp, apps, apps.length ? "" : "Nenhum contêiner Moodle — crie em Docker → Criar Contêiner");
    if (apps.length && !mpApp.value) mpApp.value = apps[0].value;
    mpApp.disabled = !apps.length;
    loadMpTypes();
    mpRows.innerHTML = "";
    const rows = screen.rows || [];
    rows.forEach(function (row) { mpRows.appendChild(buildMpRow(row)); });
    mpEmpty.classList.toggle("hidden", rows.length > 0);
  })).catch(function (err) { openError("Falha ao carregar a tela Plugins Moodle: " + err); });
}

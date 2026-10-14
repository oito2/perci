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

// Run lifecycle (core.js/widgets.js), screen resolution (navigation.js) and
// the async races of the screens that share a container.

import { test } from "node:test";
import assert from "node:assert/strict";
import { FakeElement, evalIn, loadScripts, scriptOrder } from "./harness.mjs";

const tick = () => new Promise((r) => setTimeout(r, 0));
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

// deferred returns a promise and the functions that settle it.
function deferred() {
  const d = {};
  d.promise = new Promise((resolve, reject) => { d.resolve = resolve; d.reject = reject; });
  return d;
}

// queued makes a fixture that hands out the given deferreds, one per call.
function queued(list) {
  let i = 0;
  return () => list[i++].promise;
}

function load(fixtures = {}) {
  return loadScripts(scriptOrder(), { fixtures });
}

const state = (ctx, expr) => evalIn(ctx, expr);

test("startExecution -> finishRun releases the buttons and reports the result", () => {
  const { ctx, element } = load();
  const b1 = new FakeElement("button");
  const b2 = new FakeElement("button");
  ctx.startExecution("Fazer", [b1, null, b2], 3);
  assert.equal(state(ctx, "running"), true);
  assert.ok(b1.disabled && b2.disabled);
  assert.equal(element("term-title").textContent, "Executando: Fazer...");
  assert.equal(element("tab-radio-terminal").checked, true);

  ctx.finishRun({ ok: true });
  assert.equal(state(ctx, "running"), false);
  assert.equal(state(ctx, "activeRun"), null);
  assert.ok(!b1.disabled && !b2.disabled);
  assert.equal(element("term-title").textContent, "Concluído.");
});

test("failRun ends only a run this window started", () => {
  const { ctx, element } = load();
  ctx.failRun(new Error("sem efeito"));
  assert.equal(element("term-title").textContent, "");
  ctx.startExecution("Fazer");
  ctx.failRun(new Error("recusada"));
  assert.equal(state(ctx, "running"), false);
  assert.equal(element("term-title").textContent, "Falhou.");
});

test("action-done and step events reach only the window that runs", () => {
  const { ctx, element, emit } = load();
  emit("action-done", { ok: true, error: "" }); // another window's run
  assert.equal(element("term-title").textContent, "");
  ctx.startExecution("Fazer");
  emit("step", { index: 1, total: 4, label: "passo" });
  assert.equal(element("term-title").textContent, "Executando: passo");
  assert.equal(element("term-progress").max, 4);
  emit("action-done", { ok: false, error: "x" });
  assert.equal(element("term-title").textContent, "Falhou.");
  assert.equal(state(ctx, "running"), false);
});

test("beginRun: one run at a time, refusals and sync errors end the run", async () => {
  const { ctx, element } = load();
  let calls = 0;
  const call = () => { calls++; return Promise.resolve(); };
  assert.equal(ctx.beginRun("A", call), true);
  assert.equal(ctx.beginRun("B", call), false);
  assert.equal(calls, 1);
  ctx.finishRun({ ok: true });

  assert.equal(ctx.beginRun("C", () => Promise.reject(new Error("outra ação já está em execução"))), true);
  await tick();
  assert.equal(state(ctx, "running"), false);
  assert.equal(element("term-title").textContent, "Falhou.");

  assert.equal(ctx.beginRun("D", () => { throw new Error("boom"); }), true);
  assert.equal(state(ctx, "running"), false);
  assert.equal(element("term-title").textContent, "Falhou.");

  const b = new FakeElement("button");
  ctx.beginRun("E", () => Promise.resolve(), { buttons: [b], maxSteps: 2, announce: "Olá" });
  assert.ok(b.disabled);
  assert.equal(element("term-progress").max, 2);
  ctx.finishRun({ ok: true });
  assert.ok(!b.disabled);
});

test("doneHandlerFor picks the handler by screen family", () => {
  const { ctx } = load({ GetMegaSyncInfo: { installed: false }, GetIDEInfo: { installed: true } });
  assert.equal(ctx.doneHandlerFor("unknown-screen"), ctx.updateExecutarButton);
  const run = { buttons: [], actionId: "x" };
  for (const id of ["terminals", "megasync", "ide-zed", "androidstudio", "prereqs"]) {
    assert.equal(typeof ctx.doneHandlerFor(id), "function", id);
    assert.notEqual(ctx.doneHandlerFor(id), ctx.updateExecutarButton, id);
    ctx.__selectedItem = { actionId: id, title: id, desc: "" };
    ctx.doneHandlerFor(id)(run);
  }
});

test("screenFor resolves each family to its container", () => {
  const { ctx, element } = load();
  const el = (item) => ctx.screenFor(item).el;
  assert.equal(el({ actionId: "dashboard" }), element("dashboard-container"));
  assert.equal(el({ actionId: "fonts" }), element("multiselect-container"));
  assert.equal(el({ actionId: "megasync" }), element("singleapp-container"));
  assert.equal(el({ actionId: "ide-zed" }), element("ide-container"));
  assert.equal(el({ actionId: "androidstudio" }), element("fileapp-container"));
  assert.equal(el({ actionId: "prereqs" }), element("itemtable-container"));
  assert.equal(el({ actionId: "system-update" }), element("simple-container"));
  assert.equal(ctx.screenFor({ actionId: "repos" }).alsoShow[0], element("tab-label-repos-list"));
  assert.equal(ctx.screenFor({ actionId: "moodleplugins" }).alsoShow[0], element("tab-label-mp-mounted"));
  assert.equal(ctx.screenFor({ actionId: "self-config" }).noTerminal, true);
  assert.equal(evalIn(ctx, 'typeof SINGLE_APP_SCREENS["linuxtoys"]'), "undefined");
});

test("the screen-only tab labels are hidden when another screen is selected", () => {
  const { ctx, element } = load();
  const hidden = ctx.screenContainers();
  assert.ok(hidden.includes(element("tab-label-mp-mounted")));
  assert.ok(hidden.includes(element("tab-label-repos-list")));
});

test("quit-blocked: the window running the action opens its Execução tab", () => {
  const { ctx, element, emit } = load();
  ctx.startExecution("Atualizar Sistema", []);
  element("tab-radio-terminal").checked = false;
  emit("quit-blocked");
  assert.equal(element("tab-radio-terminal").checked, true);
  assert.match(element("error-modal-message").textContent, /Aguarde ela terminar na aba Execução/);
});

test("quit-blocked: the main window without the action points to the tray window", () => {
  const { element, emit } = load();
  element("tab-radio-terminal").checked = false;
  emit("quit-blocked");
  assert.notEqual(element("tab-radio-terminal").checked, true);
  assert.match(element("error-modal-message").textContent, /janela da bandeja/);
});

test("showDialog opens a closed dialog once and leaves an open one alone", () => {
  const { ctx } = load();
  let opened = 0;
  const dialog = { open: false, showModal() { opened++; this.open = true; } };
  ctx.showDialog(dialog);
  ctx.showDialog(dialog);
  assert.equal(opened, 1);
});

// FE-01: the checklist's "Executar" is not armed with the previous screen's
// handler while the new items load.
test("checklist: Executar is disabled and unarmed until the items arrive", async () => {
  const slow = deferred();
  const { ctx, element, calls } = load({
    GetFontsInfo: [{ id: "f1", label: "F1", installed: true }],
    GetTemplatesInfo: queued([slow]),
  });
  ctx.renderMultiselectScreen({ actionId: "fonts", title: "Fontes" });
  await tick();
  const btn = element("btn-executar-multiselect");
  assert.equal(typeof btn.onclick, "function");
  assert.equal(btn.disabled, false);

  ctx.renderMultiselectScreen({ actionId: "templates", title: "Templates" });
  assert.equal(btn.disabled, true);
  assert.equal(btn.onclick, null);

  slow.resolve([{ id: "t1", label: "T1" }]);
  await tick();
  assert.equal(btn.disabled, false);
  btn.onclick();
  assert.equal(calls.filter((c) => c.name === "RunFonts").length, 0, "never runs the old screen");
  assert.equal(calls.filter((c) => c.name === "RunTemplates").length, 1);
});

// FE-02: a late answer cannot overwrite the buttons of a newer one.
test("single app: a stale fetch is dropped, also from the done handler", async () => {
  const first = deferred();
  const second = deferred();
  const third = deferred();
  const { ctx, element } = load({ GetMegaSyncInfo: queued([first, second, third]) });
  ctx.renderSingleAppScreen({ actionId: "megasync", title: "MEGA", desc: "" });
  ctx.renderSingleAppScreen({ actionId: "megasync", title: "MEGA", desc: "" });
  second.resolve({ installed: false });
  await tick();
  first.resolve({ installed: true });
  await tick();
  assert.equal(element("btn-singleapp-install").disabled, false);
  assert.equal(element("btn-singleapp-uninstall").disabled, true);

  ctx.doneHandlerFor("megasync")({ buttons: [] });
  third.resolve({ installed: true });
  await tick();
  assert.equal(element("btn-singleapp-install").disabled, true);
  assert.equal(element("btn-singleapp-uninstall").disabled, false);
});

test("IDE screen: a stale fetch is dropped", async () => {
  const first = deferred();
  const second = deferred();
  const { ctx, element } = load({ GetIDEInfo: queued([first, second]) });
  ctx.renderIDEScreen({ actionId: "ide-zed", desc: "" });
  ctx.renderIDEScreen({ actionId: "ide-code", desc: "" });
  second.resolve({ installed: false });
  await tick();
  first.resolve({ installed: true });
  await tick();
  assert.equal(element("btn-ide-uninstall").disabled, true);
});

test("item table: a failed query shows the error instead of waiting forever", async () => {
  const { ctx, element } = load({ GetPrereqsInfo: () => Promise.reject(new Error("sem rede")) });
  ctx.renderItemTableScreen({ actionId: "prereqs", desc: "" });
  await tick();
  await tick();
  const cell = element("itemtable-rows").children[0].children[0];
  assert.match(cell.textContent, /Falha ao verificar o que está instalado: Error: sem rede/);
});

test("item table: the error of an older query does not replace a newer screen", async () => {
  const first = deferred();
  const second = deferred();
  const { ctx, element } = load({ GetPrereqsInfo: queued([first, second]) });
  ctx.renderItemTableScreen({ actionId: "prereqs", desc: "" });
  ctx.renderItemTableScreen({ actionId: "prereqs", desc: "" });
  second.resolve([{ id: "Git", label: "Git", description: "", installed: true }]);
  await tick();
  first.reject(new Error("antigo"));
  await tick();
  await tick();
  const rows = element("itemtable-rows").children;
  assert.equal(rows.length, 1);
  assert.doesNotMatch(rows[0].children[0].textContent, /antigo/);
});

// FE-03: an early return of the render invalidates the pending response.
test("scoped table: a slow Global answer does not fill the Local screen without a folder", async () => {
  const slow = deferred();
  const { ctx } = load();
  const els = {
    select: new FakeElement("select"), alertTitle: new FakeElement(), alertDesc: new FakeElement(),
    pickButton: new FakeElement("button"), rows: new FakeElement("tbody"),
  };
  const table = ctx.createScopedTable({
    els, noun: "x", latestKey: "race-test",
    pickFolder: () => Promise.resolve(""),
    fetchRows: () => slow.promise,
    keyOf: (r) => r.slug, sigOf: (r) => r.slug,
    buildRow: () => new FakeElement("tr"),
  });
  table.render(); // Global, response pending
  els.select.value = "local";
  els.select.listeners.change[0](); // Local without a folder
  slow.resolve([{ slug: "a" }]);
  await tick();
  assert.equal(els.rows.children.length, 0);
});

test("IA Contextos: an answer for a folder that was cleared is dropped", async () => {
  const slow = deferred();
  const { ctx, element } = load({ GetAIContextItems: queued([slow]) });
  evalIn(ctx, 'aicFolder = "/proj"');
  ctx.renderAicItems();
  evalIn(ctx, 'aicFolder = ""');
  ctx.renderAicItems();
  slow.resolve([{ id: "a", label: "A" }]);
  await tick();
  assert.equal(element("aic-items").children.length, 0);
  assert.equal(element("btn-aic-apply").disabled, true);
});

// FE-04
test("repos: the state of the previous folder is not applied to the new one", async () => {
  const a = deferred();
  const b = deferred();
  const { ctx } = load({ GetRepoFolderState: queued([a, b]) });
  evalIn(ctx, 'reposFolderPath = "/a"');
  ctx.refreshReposFolderState();
  evalIn(ctx, 'reposFolderPath = "/b"');
  ctx.refreshReposFolderState();
  b.resolve({ isEmpty: true, marker: "b" });
  await tick();
  a.resolve({ isEmpty: false, marker: "a" });
  await tick();
  assert.equal(evalIn(ctx, "reposFolderState.marker"), "b");
});

test("repos: a failure for a folder that is no longer selected stays silent", async () => {
  const a = deferred();
  const { ctx, element } = load({ GetRepoFolderState: queued([a]) });
  evalIn(ctx, 'reposFolderPath = "/a"');
  ctx.refreshReposFolderState();
  evalIn(ctx, 'reposFolderPath = "/b"');
  a.reject(new Error("old"));
  await tick();
  await tick();
  assert.equal(element("error-modal-message").textContent, "");
});

// FE-05
test("an update that ends outside Visão Geral makes the next visit check again", () => {
  const { ctx } = load();
  evalIn(ctx, 'selfUpdateCheckedAt = 12345; dashboardRunningAction = "self-update"');
  ctx.__selectedItem = { actionId: "dashboard" };
  ctx.startExecution("Atualizar Perci");
  ctx.__selectedItem = { actionId: "fonts" };
  ctx.finishRun({ ok: true });
  assert.equal(evalIn(ctx, "selfUpdateCheckedAt"), 0);
});

test("the update check shows a failure instead of staying on Verificando", async () => {
  const { ctx, element } = load({ GetSelfUpdateInfo: () => Promise.reject(new Error("offline")) });
  ctx.loadSelfUpdateStatus(true);
  await tick();
  await tick();
  assert.match(element("su-status").textContent, /Falha ao verificar atualizações: Error: offline/);
  assert.equal(evalIn(ctx, "selfUpdateCheckedAt"), 0);
});

// FE-06
test("theme grid: two renders in a row leave one button per theme", async () => {
  const { ctx, element } = load({ GetGUIThemes: ["a", "b", "c"] });
  ctx.renderThemeGrid("a");
  ctx.renderThemeGrid("b");
  await tick();
  await tick();
  assert.equal(element("cfg-theme-grid").children.length, 3);
});

// FE-09
test("Moodle PHP versions: a stale answer does not replace a newer one", async () => {
  const slow = deferred();
  const fast = deferred();
  const { ctx, element } = load({ GetPHPVersionsForMoodleVersion: queued([slow, fast]) });
  ctx.wireMoodlePHPFilter("dc"); // first refresh, pending
  element("dc-moodleVersion").listeners.change[0](); // second refresh
  fast.resolve(["8.3"]);
  await tick();
  slow.resolve(["7.4"]);
  await tick();
  assert.ok(element("dc-phpVersion-wrap").innerHTML.includes("8.3"));
  assert.ok(!element("dc-phpVersion-wrap").innerHTML.includes("7.4"));
});

// FE-10
test("Plugins Moodle: typing the name checks the backend once, after a pause", async () => {
  const { ctx, element, calls } = load({
    InspectMoodlePlugin: { ok: true, destLabel: "x", locked: false },
  });
  element("mp-type").options = [];
  evalIn(ctx, 'mpDev = "/ws/dev"');
  const input = element("mp-name").listeners.input[0];
  input();
  input();
  input();
  assert.equal(element("btn-mp-create").disabled, true);
  assert.equal(calls.filter((c) => c.name === "InspectMoodlePlugin").length, 0);
  await sleep(350);
  assert.equal(calls.filter((c) => c.name === "InspectMoodlePlugin").length, 1);
});

test("Plugins Moodle: the screen load and the table reload use separate request keys", async () => {
  const screen = deferred();
  const rows = deferred();
  const { ctx, element } = load({ GetMoodlePlugins: queued([screen, rows]) });
  element("mp-app").options = [];
  ctx.renderMoodlePluginsScreen();
  ctx.loadMpRows();
  rows.resolve({ rows: [] });
  await tick();
  screen.resolve({ apps: [{ folder: "m", label: "M" }], rows: [] });
  await tick();
  assert.equal(element("mp-app").disabled, false, "the screen's own answer was not discarded");
});

// FE-11
test("form field helpers escape id, label and hint", () => {
  const { ctx } = load();
  const bad = `x"><img src=x onerror=1>`;
  for (const html of [
    ctx.textFieldHTML(bad, bad, {}),
    ctx.selectFieldHTML(bad, bad, [], ""),
    ctx.checkboxFieldHTML(bad, bad, false, false, bad),
  ]) {
    assert.ok(!html.includes("<img"), html);
  }
});

// FE-12
test("accordion checkboxes are labelled and the active menu item has aria-current", async () => {
  const cats = [
    { label: "Home", items: [{ title: "Visão Geral", actionId: "dashboard", desc: "" }, { title: "Outro", actionId: "x", desc: "" }] },
  ];
  const { ctx, element, query } = load({ GetCategories: cats });
  await tick();
  const checkbox = element("accordion").children[0].children[0];
  assert.equal(checkbox.getAttribute("aria-label"), "Home");

  const anchors = [0, 1].map((i) => {
    const a = new FakeElement("a");
    a.dataset.cat = "0";
    a.dataset.item = String(i);
    return a;
  });
  query("#accordion a[data-cat]", anchors);
  ctx.selectItem(cats[0], cats[0].items[1], 0, 1);
  assert.equal(anchors[1].getAttribute("aria-current"), "page");
  assert.equal(anchors[0].getAttribute("aria-current"), null);
  ctx.selectItem(cats[0], cats[0].items[0], 0, 0);
  assert.equal(anchors[1].getAttribute("aria-current"), null);
  assert.equal(anchors[0].getAttribute("aria-current"), "page");
});

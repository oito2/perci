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

import { test } from "node:test";
import assert from "node:assert/strict";
import { evalIn, loadScripts, scriptOrder } from "./harness.mjs";

const tick = () => new Promise((r) => setTimeout(r, 0));

const PRESETS = [
  { name: "Plugin Moodle", models: ["Moodle", "Documentação de projeto"] },
  { name: "Aplicativo Rust + Tauri v2", models: ["Rust + Tauri v2"], hint: "Marque também o complemento do frontend." },
];
const ITEMS = [
  { id: "Moodle", label: "Moodle", description: "Plugins Moodle.", base: "", status: "" },
  { id: "Rust + Tauri v2", label: "Rust + Tauri v2", description: "Tauri.", base: "", status: "applied" },
  { id: "Documentação de projeto", label: "Documentação de projeto", description: "Docs.", base: "", status: "" },
];

const AGENTS = [
  { id: "claude", label: "Claude Code", selected: true },
  { id: "codex", label: "ChatGPT CLI (Codex)", selected: false },
];

async function render(fixtures = {}) {
  const loaded = loadScripts(scriptOrder(), {
    fixtures: {
      GetAIContextPresets: PRESETS, GetAIContextItems: ITEMS, GetAIContextAgents: AGENTS,
      PreviewAIContext: [{ path: "AGENTS.md", action: "update" }], ...fixtures,
    },
  });
  await tick();
  evalIn(loaded.ctx, 'aicFolder = "/ws/proj";');
  loaded.ctx.renderAicItems();
  await tick();
  await tick();
  const checked = () => loaded.element("aic-items").children.map((li) => li.children[0].checked);
  return { ...loaded, checked };
}

test("IA: Contextos: a project type checks exactly its models and shows its hint", async () => {
  const { element, checked } = await render();
  const select = element("aic-preset");
  assert.equal(select.disabled, false);
  assert.deepEqual(select.children.map((o) => o.textContent), ["Personalizado", "Plugin Moodle", "Aplicativo Rust + Tauri v2"]);

  select.value = "Plugin Moodle";
  select.onchange();
  assert.deepEqual(checked(), [true, false, true]);

  select.value = "Aplicativo Rust + Tauri v2";
  select.onchange();
  assert.deepEqual(checked(), [false, true, false]);
  assert.match(element("aic-preset-hint").textContent, /complemento do frontend/);
});

test("IA: Contextos: changing a checkbox by hand returns to Personalizado", async () => {
  const { element } = await render();
  const select = element("aic-preset");
  select.value = "Plugin Moodle";
  select.onchange();
  const firstBox = element("aic-items").children[0].children[0];
  firstBox.listeners.change.forEach((fn) => fn());
  assert.equal(select.value, "");
  assert.equal(element("aic-preset-hint").textContent, "");
});

test("IA: Contextos: Aplicar sends the checked agents and needs at least one", async () => {
  const { ctx, element, calls } = await render();
  const boxes = element("aic-agents").children.map((label) => label.children[0]);
  assert.deepEqual(boxes.map((b) => [b.dataset.agent, b.checked]), [["claude", true], ["codex", false]]);
  boxes[1].checked = true;
  element("btn-aic-apply").onclick();
  await tick();
  assert.match(element("confirm-modal-message").textContent, /atualizar: AGENTS\.md/);
  element("btn-confirm-ok").listeners.click.forEach((fn) => fn({}));
  const apply = calls.find((c) => c.name === "ApplyAIContext");
  assert.deepEqual(JSON.parse(JSON.stringify(apply.args[2])), ["claude", "codex"]);

  evalIn(ctx, "running = false; activeRun = null");
  boxes.forEach((b) => { b.checked = false; });
  element("btn-aic-apply").onclick();
  assert.equal(calls.filter((c) => c.name === "ApplyAIContext").length, 1, "no agent checked: nothing runs");
  assert.match(element("error-modal-message").textContent, /ao menos um agente/);
});

test("IA: Contextos: complements sit under their base, which a complement checks, and states show as badges", async () => {
  const items = [
    { id: "Go", label: "Go", description: "Go.", base: "", status: "outdated" },
    { id: "Go: Wails v3", label: "Go: Wails v3", description: "Wails.", base: "Go", status: "" },
  ];
  const { element } = await render({ GetAIContextItems: items });
  const [goLi, wailsLi] = element("aic-items").children;
  assert.match(wailsLi.className, /ml-8/);
  assert.doesNotMatch(goLi.className, /ml-8/);
  const badge = goLi.children[1].children[0].children[0];
  assert.equal(badge.textContent, "desatualizado");
  goLi.children[0].checked = false;
  wailsLi.children[0].checked = true;
  wailsLi.children[0].listeners.change.forEach((fn) => fn());
  assert.equal(goLi.children[0].checked, true, "checking a complement checks its base");
});

test("IA: Contextos: a folder without standards starts on the suggested project type", async () => {
  const fresh = ITEMS.map((it) => ({ ...it, status: "" }));
  const { element, checked } = await render({ GetAIContextItems: fresh, GetAIContextSuggestedPreset: "Plugin Moodle" });
  await tick();
  assert.equal(element("aic-preset").value, "Plugin Moodle");
  assert.deepEqual(checked(), [true, false, true]);
  assert.match(element("aic-preset-hint").textContent, /Sugerido pelos arquivos/);

  const applied = await render({ GetAIContextSuggestedPreset: "Plugin Moodle" });
  await tick();
  assert.equal(applied.element("aic-preset").value, "", "a folder with standards keeps its own choice");
});

test("IA: Contextos: Aplicar with nothing to change says so and runs nothing", async () => {
  const { element, calls } = await render({ PreviewAIContext: [] });
  element("btn-aic-apply").onclick();
  await tick();
  assert.match(element("error-modal-title").textContent || element("error-modal-message").textContent, /Nada a mudar|já estão/);
  assert.equal(calls.filter((c) => c.name === "ApplyAIContext").length, 0);
});


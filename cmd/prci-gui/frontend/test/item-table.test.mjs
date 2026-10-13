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
import { loadScripts, scriptOrder } from "./harness.mjs";

const tick = () => new Promise((r) => setTimeout(r, 0));

const ITEMS = [
  { id: "Curl", label: "Curl", description: "URL.", installed: true },
  { id: "Node.js", label: "Node.js", description: "JS.", installed: false },
  { id: "GitHub CLI (gh)", label: "GitHub CLI (gh)", description: "gh.", installed: false, missing: ["Curl"] },
  { id: "Docker Engine", label: "Docker Engine", description: "Docker.", installed: true, removeWarning: "Leva junto." },
];

async function render(actionId = "prereqs", items = ITEMS) {
  const loaded = loadScripts(scriptOrder(), { fixtures: { GetPrereqsInfo: items, GetSDKsInfo: items } });
  loaded.ctx.renderItemTableScreen({ actionId, desc: "intro" });
  await tick();
  await tick();
  const rows = loaded.element("itemtable-rows").children;
  const buttons = (i) => rows[i].children[1].children[0].children;
  const state = (i) => buttons(i).map((b) => (b.disabled ? "-" : "+") + b.textContent);
  const texts = (i) => rows[i].children[0].children.map((c) => c.textContent);
  return { ...loaded, rows, buttons, state, texts };
}

test("Item table: buttons follow the installed state and missing requirements", async () => {
  const { rows, state, texts } = await render();
  assert.equal(rows.length, 4);
  assert.deepEqual(state(0), ["-Instalar", "+Atualizar", "+Desinstalar"]);
  assert.deepEqual(state(1), ["+Instalar", "-Atualizar", "-Desinstalar"]);
  assert.deepEqual(state(2), ["-Instalar", "-Atualizar", "-Desinstalar"]);
  assert.deepEqual(texts(2), ["GitHub CLI (gh)", "gh.", "Requer: Curl"]);
  assert.deepEqual(texts(1), ["Node.js", "JS."]);
});

test("Item table: each button runs its own operation; Desinstalar confirms first", async () => {
  const { ctx, buttons, calls, element } = await render();
  buttons(1)[0].listeners.click[0]();
  let runs = calls.filter((c) => c.name === "RunPrereqAction").map((c) => c.args);
  assert.deepEqual(runs, [["Node.js", "install"]]);

  // Release the run so the next click is accepted.
  ctx.finishRun({ ok: true });
  buttons(3)[2].listeners.click[0]();
  runs = calls.filter((c) => c.name === "RunPrereqAction").map((c) => c.args);
  assert.equal(runs.length, 1, "uninstall waits for the confirmation");
  assert.match(element("confirm-modal-message").textContent, /Desinstalar Docker Engine\?\n\nLeva junto\./);
});

test("Item table: Linguagens e SDKs uses its own bound methods and header", async () => {
  const { buttons, calls, element } = await render("sdks");
  assert.equal(element("itemtable-header").textContent, "SDK");
  assert.ok(calls.some((c) => c.name === "GetSDKsInfo"));
  buttons(1)[0].listeners.click[0]();
  const runs = calls.filter((c) => c.name === "RunSDKAction").map((c) => c.args);
  assert.deepEqual(runs, [["Node.js", "install"]]);
  assert.ok(!calls.some((c) => c.name === "RunPrereqAction"));
});

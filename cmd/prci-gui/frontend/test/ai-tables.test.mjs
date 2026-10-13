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
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { FakeElement, evalIn, loadScripts, scriptOrder } from "./harness.mjs";

const tick = () => new Promise((r) => setTimeout(r, 0));
const jsDir = path.join(path.dirname(fileURLToPath(import.meta.url)), "..", "dist", "js");

const SKILLS = [
  { slug: "a", name: "Skill A", installed: true, partial: false, agents: ["Claude", "OpenCode", "Antigravity", "Codex"] },
  { slug: "b", name: "Skill B", installed: true, partial: true, agents: ["Claude", "Codex"] },
  { slug: "c", name: "Skill C", installed: true, partial: false, agents: [] },
  { slug: "d", name: "Skill D", installed: false, partial: false, agents: [] },
];

const SERVERS = [
  { slug: "filesystem", name: "Filesystem", installed: true, partial: true, agents: ["Claude", "Antigravity"], param: "/p", paramKind: "directory", paramLabel: "Pasta permitida" },
  { slug: "sqlite", name: "SQLite", installed: true, partial: false, agents: ["Claude", "Codex", "Antigravity"], param: "/d", paramKind: "file", paramLabel: "Arquivo" },
  { slug: "dart-flutter", name: "Dart + Flutter", installed: false, partial: false, agents: [] },
];

// The name cell of each row, and the text of the tags inside it.
const nameCells = (rows) => rows.map((tr) => tr.children[0]);
const tags = (cell) => cell.children.map((c) => c.textContent);

test("SKILLs: a partial record shows which agents accepted it", async () => {
  const { ctx, element } = loadScripts(scriptOrder(), { fixtures: { GetAgentSkillsRows: SKILLS } });
  ctx.renderAgentSkillsScreen();
  await tick();
  const cells = nameCells(element("as-rows").children);
  assert.equal(cells.length, 4);
  assert.deepEqual(tags(cells[0]), [], "a complete record has no tag");
  assert.deepEqual(tags(cells[1]), ["parcial: Claude, Codex"]);
  assert.deepEqual(tags(cells[2]), [], "a record from before the agents were stored is not partial");
  assert.deepEqual(tags(cells[3]), []);
  assert.equal(cells[1].textContent, "Skill B");
});

test("MCPs: a partial registration shows which agents accepted it", async () => {
  const { ctx, element } = loadScripts(scriptOrder(), { fixtures: { GetMCPServerRows: SERVERS } });
  ctx.renderMCPServersScreen();
  await tick();
  const cells = nameCells(element("mcp-rows").children);
  assert.equal(cells.length, 3);
  assert.deepEqual(tags(cells[0]), ["parcial: Claude, Antigravity"]);
  assert.deepEqual(tags(cells[1]), []);
  assert.deepEqual(tags(cells[2]), []);
});

test("partialBadge: only for a partial row, named after the agents that took it", () => {
  const { ctx } = loadScripts(scriptOrder());
  assert.equal(ctx.partialBadge({ partial: false, agents: ["Claude"] }), null);
  assert.equal(ctx.partialBadge({}), null);
  assert.equal(ctx.partialBadge({ partial: true, agents: ["Claude", "Codex"] }).textContent, "parcial: Claude, Codex");
});

test("SKILLs: the intro says the content is from third parties", () => {
  const { ctx, element } = loadScripts(scriptOrder());
  const container = new FakeElement("div");
  const intro = new FakeElement("p");
  container.appendChild(intro);
  intro.appendChild(element("as-skills-sh-link"));
  const source = fs.readFileSync(path.join(jsDir, "screens", "agent-skills.js"), "utf8");
  const snippet = source.match(/\(function addThirdPartyNote\(\) \{[\s\S]*?\n\}\)\(\);/);
  assert.ok(snippet, "the note's code is in agent-skills.js");
  evalIn(ctx, snippet[0]);
  assert.equal(container.children.length, 2);
  assert.match(container.children[1].textContent, /conteúdo de terceiros/);
  assert.match(container.children[1].textContent, /commit/);
});

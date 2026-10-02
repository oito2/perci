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
import { FakeElement, evalIn, loadScripts, scriptOrder } from "./harness.mjs";

const { ctx, calls, element } = loadScripts(scriptOrder(), {
  fixtures: {
    GetPostinstallProfile: { supported: true, label: "Ubuntu" },
    GetRepoFolders: [{ path: "/home/u/a", name: "a" }, { path: "/home/u/b", name: "b" }],
  },
});
const tick = () => new Promise((r) => setTimeout(r, 0));

test("escHTML/escAttr escape every special character", () => {
  assert.equal(ctx.escHTML(`<a href="x">&</a>`), `&lt;a href="x"&gt;&amp;&lt;/a&gt;`);
  assert.equal(ctx.escAttr(`a&b "c" 'd' <e>`), "a&amp;b &quot;c&quot; &#39;d&#39; &lt;e&gt;");
  assert.equal(ctx.escHTML(42), "42");
});

test("linkifyDesc escapes text and URLs exactly once", () => {
  const html = ctx.linkifyDesc(`Baixe em https://x.io/a?b=1&c=2 e <b>leia</b>.`);
  assert.equal(
    html,
    `Baixe em <a href="#" class="link link-primary" data-url="https://x.io/a?b=1&amp;c=2">https://x.io/a?b=1&amp;c=2</a> e &lt;b&gt;leia&lt;/b&gt;.`,
  );
  assert.equal(ctx.linkifyDesc(`no link & "quotes"`), `no link &amp; "quotes"`);
  // A quote ends the URL: it can't reach, and break out of, data-url.
  assert.ok(ctx.linkifyDesc(`https://x.io/"onclick="alert(1)`).startsWith(`<a href="#" class="link link-primary" data-url="https://x.io/">`));
});

test("statusBadgeHTML escapes an unknown Docker status", () => {
  assert.ok(ctx.statusBadgeHTML("running").includes("rodando"));
  const html = ctx.statusBadgeHTML("<img src=x>");
  assert.ok(html.includes("&lt;img src=x&gt;") && !html.includes("<img"));
});

test("textFieldHTML keeps values intact and hides secrets", () => {
  const plain = ctx.textFieldHTML("f", "Campo", { value: `a&amp;b"` });
  assert.ok(plain.includes(`value="a&amp;amp;b&quot;"`), plain);
  const secret = ctx.textFieldHTML("p", "Senha", { value: "x", secret: true });
  assert.ok(secret.includes(`type="password"`) && secret.includes(`data-reveal="p"`), secret);
});

test("selectFieldHTML escapes option labels and marks the selected one", () => {
  const html = ctx.selectFieldHTML("s", "Versão", ["8.3", "<9>"], "8.3");
  assert.ok(html.includes(`<option value="8.3" selected>8.3</option>`), html);
  assert.ok(html.includes(`>&lt;9&gt;</option>`), html);
});

test("latestOnly drops every response but the newest", () => {
  const seen = [];
  const first = ctx.latestOnly("k", (v) => seen.push(v));
  const second = ctx.latestOnly("k", (v) => seen.push(v));
  first("old");
  second("new");
  assert.deepEqual(seen, ["new"]);
});

// reconcileKeyedList keeps unchanged rows, rebuilds changed ones, follows
// the new order and drops the missing.
test("reconcileKeyedList", () => {
  const container = new FakeElement("tbody");
  let builds = 0;
  const build = (item) => {
    builds++;
    const el = new FakeElement("tr");
    el.textContent = item.label;
    return el;
  };
  const run = (items) => ctx.reconcileKeyedList(container, items, (i) => i.id, (i) => i.label, build);
  const labels = () => container.children.map((c) => c.textContent);

  run([{ id: 1, label: "a" }, { id: 2, label: "b" }, { id: 3, label: "c" }]);
  assert.deepEqual(labels(), ["a", "b", "c"]);
  const keptNode = container.children[0];
  builds = 0;

  run([{ id: 3, label: "c" }, { id: 1, label: "a" }, { id: 2, label: "B" }]);
  assert.deepEqual(labels(), ["c", "a", "B"]);
  assert.equal(builds, 1, "only the changed row is rebuilt");
  assert.equal(container.children[1], keptNode, "an unchanged row is the same node");

  run([{ id: 1, label: "a" }]);
  assert.deepEqual(labels(), ["a"]);
});

test("getPostinstallProfile asks the backend once", async () => {
  const before = calls.filter((c) => c.name === "GetPostinstallProfile").length;
  const [a, b] = await Promise.all([ctx.getPostinstallProfile(), ctx.getPostinstallProfile()]);
  assert.equal(a.label, "Ubuntu");
  assert.equal(a, b);
  const after = calls.filter((c) => c.name === "GetPostinstallProfile").length;
  assert.ok(after - before <= 1, `called ${after - before} times`);
});

test("runScreenAction does nothing while another action runs", async () => {
  let ran = 0;
  evalIn(ctx, "running = true");
  ctx.runScreenAction({ title: "X" }, "instalar", [], () => { ran++; return Promise.resolve(); });
  assert.equal(ran, 0);
  evalIn(ctx, "running = false; activeRun = null");
  ctx.runScreenAction({ title: "X" }, "instalar", [], () => { ran++; return Promise.resolve(); });
  assert.equal(ran, 1);
  assert.equal(evalIn(ctx, "running"), true);
  evalIn(ctx, "running = false; activeRun = null");
  await tick();
});

// createScopedTable fetches nothing for Local until a folder is picked,
// then fetches for that folder; reset goes back to Global.
test("createScopedTable", async () => {
  const els = {
    select: new FakeElement("select"), alertTitle: new FakeElement(), alertDesc: new FakeElement(),
    pickButton: new FakeElement("button"), rows: new FakeElement("tbody"),
  };
  const fetched = [];
  const table = ctx.createScopedTable({
    els, noun: "das SKILLs", latestKey: "scoped-test",
    pickFolder: () => Promise.resolve("/proj/a&b"),
    fetchRows: (global, folder) => { fetched.push([global, folder]); return Promise.resolve([{ slug: "s", name: "S" }]); },
    keyOf: (r) => r.slug, sigOf: (r) => r.name,
    buildRow: () => new FakeElement("tr"),
  });

  table.reset();
  await tick();
  assert.deepEqual(fetched, [[true, ""]]);
  assert.equal(els.rows.children.length, 1);

  els.select.value = "local";
  els.select.listeners.change[0]();
  await tick();
  assert.equal(fetched.length, 1, "Local without a folder fetches nothing");
  assert.equal(els.rows.innerHTML, "");

  els.pickButton.listeners.click[0]();
  await tick();
  await tick();
  assert.deepEqual(fetched.at(-1), [false, "/proj/a&b"]);
  assert.ok(els.alertDesc.innerHTML.includes("/proj/a&amp;b"), els.alertDesc.innerHTML);

  table.reset();
  assert.equal(table.scope, "global");
  assert.equal(table.folder, "");
});

// The terminals screen gets one button per extras entry; "Remover Starship"
// asks for confirmation first, and a run disables every button of the
// screen until it finishes.
test("multiselect extras", async () => {
  ctx.renderMultiselectScreen({ actionId: "terminals", title: "Aplicativos: Terminais" });
  await tick();
  const buttons = evalIn(ctx, "multiselectExtraButtons");
  assert.deepEqual([...buttons].map((b) => b.textContent), ["Aplicar Starship", "Remover Starship"]);

  buttons[1].onclick();
  assert.equal(calls.filter((c) => c.name === "RemoveStarship").length, 0, "nothing runs before confirming");
  evalIn(ctx, "__confirmCallback()");
  assert.equal(calls.filter((c) => c.name === "RemoveStarship").length, 1);
  assert.ok(evalIn(ctx, "multiselectButtons()").every((b) => b.disabled));

  evalIn(ctx, "running = false; activeRun = null");
  ctx.doneHandlerFor("terminals")();
  assert.ok(evalIn(ctx, "multiselectButtons()").every((b) => !b.disabled));

  buttons[0].onclick();
  assert.equal(calls.filter((c) => c.name === "ApplyStarship").length, 1, "Aplicar runs without confirmation");
  evalIn(ctx, "running = false; activeRun = null");
  await tick();
});

test("repos cards keep a single Novo repositório card", async () => {
  const cards = element("repos-list-cards").children;
  const addCards = () => cards.filter((c) => c.children[0].children[0].textContent === "Novo repositório");

  ctx.renderReposScreen({ actionId: "repos" });
  await tick();
  ctx.renderReposScreen({ actionId: "repos" }); // reopening the screen
  await tick();
  ctx.selectRepoFolder("/home/u/a"); // "Selecionar"
  await tick();
  ctx.loadRepoFolders(); // after adding/removing a folder
  await tick();

  assert.equal(addCards().length, 1);
  assert.equal(cards.length, 3, "two folders plus the add card");
  assert.equal(cards[cards.length - 1], addCards()[0], "the add card stays last");
});

test("Moodle version select keeps a saved value that is no longer offered", () => {
  const catalog = { moodleVersions: ["3.x", "4.1", "5.1+"], phpVersions: ["8.1"], nodeVersions: [], mariadbReady: true };
  const legacy = ctx.buildContainerFieldsHTML("edit", "moodle", catalog, { moodleVersion: "4.x" });
  assert.match(legacy, /<option value="4\.x" selected>4\.x<\/option>/);

  const current = ctx.buildContainerFieldsHTML("dc", "moodle", catalog, { moodleVersion: "4.1" });
  assert.match(current, /<option value="4\.1" selected>/);
  assert.doesNotMatch(current, /value="4\.x"/);
});


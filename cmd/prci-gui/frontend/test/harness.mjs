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

// Loads the frontend's classic scripts into a Node vm
// context, the way the page loads them into one global scope, with just
// enough of the browser stubbed for their top-level code to run: every
// DOM element, the Wails bindings (App/Events/Browser) and xterm are inert
// stand-ins. Tests then call the scripts' own functions from the returned
// context. No dependencies: node:vm + a tiny element implementation.

import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import { fileURLToPath } from "node:url";

const jsDir = path.join(path.dirname(fileURLToPath(import.meta.url)), "..", "dist", "js");

// inert is a value any property access or call on returns itself — for
// the parts of the page a test doesn't look at (classList, style, ...).
function makeInert() {
  const target = function () {};
  const inert = new Proxy(target, {
    get(_, key) {
      if (typeof key === "symbol") return undefined;
      if (key === "then") return undefined; // never mistaken for a promise
      return inert;
    },
    set() { return true; },
    apply() { return inert; },
    construct() { return inert; },
  });
  return inert;
}

// FakeElement implements the DOM surface the list helpers actually use;
// anything else falls back to inert.
export class FakeElement {
  constructor(tag = "div") {
    this.tagName = tag.toUpperCase();
    this.children = [];
    this.parentNode = null;
    this.dataset = {};
    this.textContent = "";
    this.innerHTML = "";
    this.attributes = {};
    this.listeners = {};
    this.classList = { add() {}, remove() {}, toggle() {}, contains() { return false; } };
    this.style = {};
    return new Proxy(this, {
      get(t, key, receiver) {
        // Reflect.get with receiver: getters (firstChild, nextSibling)
        // must see the proxy, which is what lists hold.
        if (key in t || typeof key === "symbol") return Reflect.get(t, key, receiver);
        return makeInert();
      },
    });
  }
  get firstChild() { return this.children[0] || null; }
  get nextSibling() {
    if (!this.parentNode) return null;
    const siblings = this.parentNode.children;
    return siblings[siblings.indexOf(this) + 1] || null;
  }
  appendChild(node) { return this.insertBefore(node, null); }
  insertBefore(node, ref) {
    if (node.parentNode) node.remove();
    const i = ref ? this.children.indexOf(ref) : -1;
    if (i < 0) this.children.push(node); else this.children.splice(i, 0, node);
    node.parentNode = this;
    return node;
  }
  remove() {
    if (!this.parentNode) return;
    const siblings = this.parentNode.children;
    siblings.splice(siblings.indexOf(this), 1);
    this.parentNode = null;
  }
  replaceWith(node) {
    const parent = this.parentNode;
    parent.insertBefore(node, this);
    this.remove();
  }
  setAttribute(k, v) { this.attributes[k] = String(v); }
  getAttribute(k) { return this.attributes[k] ?? null; }
  addEventListener(type, fn) { (this.listeners[type] ||= []).push(fn); }
}

// loadScripts runs files (relative to the scripts folder), in order, in one
// context. calls records every App.<method>(args); fixtures maps a method name to
// the value its promise resolves to.
export function loadScripts(files, { fixtures = {} } = {}) {
  const calls = [];
  const byId = new Map();
  const element = (id) => {
    if (!byId.has(id)) byId.set(id, new FakeElement());
    return byId.get(id);
  };
  const inert = makeInert();
  const App = new Proxy({}, {
    get(_, name) {
      return (...args) => {
        calls.push({ name, args });
        return Promise.resolve(name in fixtures ? fixtures[name] : []);
      };
    },
  });
  const context = {
    console, URLSearchParams, setTimeout, clearTimeout, Promise,
    App,
    Events: { On() {}, Emit() {} },
    Browser: { OpenURL() {} },
    Terminal: class { constructor() { return inert; } },
    FitAddon: { FitAddon: class { fit() {} } },
    ResizeObserver: class { observe() {} },
    LUCIDE: { svg: () => "<svg></svg>" },
    requestAnimationFrame: (fn) => setTimeout(fn, 0),
    location: { search: "" },
    localStorage: { getItem: () => null, setItem() {} },
    addEventListener() {},
    document: {
      getElementById: element,
      querySelector: () => new FakeElement(),
      querySelectorAll: () => [],
      createElement: (tag) => new FakeElement(tag),
      addEventListener() {},
      documentElement: new FakeElement("html"),
      body: new FakeElement("body"),
    },
  };
  context.window = context;
  vm.createContext(context);
  for (const f of files) {
    vm.runInContext(fs.readFileSync(path.join(jsDir, f), "utf8"), context, { filename: f });
  }
  return { ctx: context, calls, element };
}

// Every classic script, in page load order.
export function scriptOrder() {
  const html = fs.readFileSync(path.join(jsDir, "..", "index.html"), "utf8");
  return [...html.matchAll(/<script defer src="\.\/js\/([^"]+)"/g)].map((m) => m[1]);
}

// evalIn runs code in a loaded context — needed to read or set the
// scripts' top-level let/const bindings (e.g. `running`), which, as in a
// browser, aren't properties of the global object.
export function evalIn(ctx, code) {
  return vm.runInContext(code, ctx);
}

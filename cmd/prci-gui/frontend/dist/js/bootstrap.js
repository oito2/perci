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

// The only ES module of the frontend: imports the generated Wails bindings
// (frontend/dist/bindings/) and the runtime (/wails/runtime.js, a virtual
// path served by the Wails process itself), and exposes them as globals
// for the classic scripts under js/ that make up the rest of the app.
//
// Why classic scripts and not modules: the frontend used to be a single
// inline <script type="module"> (~2600 lines, one closure). Split into
// files that share one global scope, it keeps the exact same semantics —
// every screen reads and updates the same shared state (running, caches,
// DOM references) — without a bundler (a deliberate choice of the
// project). Load order is the order of the <script> tags in index.html:
// module and `defer` scripts all run after parsing, in document order,
// this one first.
//
// App is a merge of the 6 domain services (cmd/prci-gui/service_*.go:
// HomeService/LinuxService/DevSetupService/DockerService/DevToolsService/
// TrayService) — Object.assign keeps every App.Method(...) call working
// without naming each service.
import {
  HomeService, LinuxService, DevSetupService, DockerService, DevToolsService, TrayService,
} from "../bindings/github.com/oito2/perci/cmd/prci-gui/index.js";
import { Events, Browser } from "/wails/runtime.js";

window.App = Object.assign({}, HomeService, LinuxService, DevSetupService, DockerService, DevToolsService, TrayService);
window.Events = Events;
window.Browser = Browser;

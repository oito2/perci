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

// Sidebar accordion + collapsed rail flyout, and selectItem (which screen shows for each menu item).
//
// Classic script (not a module): shares one global scope with the other
// scripts under js/.
"use strict";

// --- accordion (expanded) + rail flyout (collapsed) ------------------------
// Each category has 2 representations in the DOM, one visible at a time
// (CSS, based on #sidebar[data-collapsed]): the normal one (accordion,
// inline items) and the rail one (just the category icon, items in a
// flyout dropdown beside it).
function catInitials(label) {
  return label.split(" ").map(function (w) { return w[0]; }).join("").slice(0, 2).toUpperCase();
}

// Rail icon per category — a category missing
// here falls back to its initials.
const CATEGORY_ICONS = {
  Home: "house",
  Linux: "terminal",
  Desenvolvimento: "code",
  Docker: "container",
  "Dev Tools": "wrench",
};

// Builds the <ul class="menu"> of one category's items — reused both by
// the accordion and the rail dropdown, so the two views always stay in
// sync (same content, same click).
function buildItemsMenu(cat, catIdx, fullClass) {
  const ul = document.createElement("ul");
  ul.className = fullClass;
  cat.items.forEach(function (item, itemIdx) {
    const li = document.createElement("li");
    li.className = "w-full";
    const a = document.createElement("a");
    a.className = "w-full";
    a.href = "#"; // focusable and activated by Enter, like any link
    a.textContent = item.title;
    a.dataset.cat = catIdx;
    a.dataset.item = itemIdx;
    a.addEventListener("click", function (ev) {
      ev.preventDefault();
      selectItem(cat, item, catIdx, itemIdx);
    });
    li.appendChild(a);
    ul.appendChild(li);
  });
  return ul;
}

App.GetCategories().then(function (cats) {
  cats.forEach(function (cat, catIdx) {
    // --- normal view: accordion, checkbox (not radio) so every
    // category can be closed, not just swapped for another. No border
    // between categories — just daisyUI's collapse-arrow, no
    // border-b/border-base-300.
    const accWrap = document.createElement("div");
    accWrap.className = "accordion-view collapse collapse-arrow";

    const checkbox = document.createElement("input");
    checkbox.type = "checkbox";
    if (catIdx === 0) checkbox.checked = true;
    checkbox.addEventListener("change", function () {
      if (!checkbox.checked) return;
      document.querySelectorAll("#accordion .accordion-view > input[type=checkbox]").forEach(function (other) {
        if (other !== checkbox) other.checked = false;
      });
    });

    const accTitle = document.createElement("div");
    accTitle.className = "collapse-title font-medium";
    accTitle.textContent = cat.label;

    const accContent = document.createElement("div");
    accContent.className = "collapse-content";
    accContent.appendChild(buildItemsMenu(cat, catIdx, "menu menu-md w-full p-0"));

    accWrap.appendChild(checkbox);
    accWrap.appendChild(accTitle);
    accWrap.appendChild(accContent);

    // --- rail view: icon + flyout with the items (custom CSS, not
    // daisyUI's `dropdown` component).
    const railWrap = document.createElement("div");
    railWrap.className = "rail-view justify-center p-2";

    const railItem = document.createElement("div");
    railItem.className = "rail-item";
    railItem.tabIndex = 0;
    railItem.setAttribute("aria-label", cat.label);

    const railTrigger = document.createElement("div");
    railTrigger.title = cat.label;
    railTrigger.className = "cat-badge cursor-pointer";
    railTrigger.setAttribute("aria-hidden", "true"); // railItem carries the label
    const icon = CATEGORY_ICONS[cat.label];
    if (icon) railTrigger.innerHTML = LUCIDE.svg(icon, { size: 22 });
    else railTrigger.textContent = catInitials(cat.label);

    const railMenu = buildItemsMenu(cat, catIdx, "rail-flyout menu menu-md bg-base-200 rounded-box w-56 p-2 shadow border border-base-300");

    railItem.appendChild(railTrigger);
    railItem.appendChild(railMenu);
    railWrap.appendChild(railItem);

    accordionEl.appendChild(accWrap);
    accordionEl.appendChild(railWrap);
  });

  if (compactAction) {
    activateCompactMode(cats);
  }
});

// Screen registry: which container each menu item shows and how it renders
// (the counterpart of ACTION_DONE_HANDLERS). Built on demand —
// the containers and render functions are declared in scripts that load
// after this one. Screens sharing one container (checklists, single apps,
// IDEs, local-file apps) resolve by family; anything else is the "simple"
// screen (description + Executar).
function screenTable() {
  return {
    dashboard: { el: dashboardContainer, render: renderDashboardScreen },
    "self-config": { el: selfconfigContainer, noTerminal: true, render: renderSelfConfigScreen },
    postinstall: { el: postinstallContainer, render: renderPostinstallScreen },
    "docker-create": { el: dockercreateContainer, render: renderDockerCreateScreen },
    "docker-manage": { el: dockerlistContainer, render: renderDockerListScreen },
    repos: { el: reposContainer, alsoShow: [tabLabelReposList], render: renderReposScreen },
    agentskills: { el: agentskillsContainer, render: renderAgentSkillsScreen },
    aicontext: { el: aicontextContainer, render: renderAIContextScreen },
    mcpservers: { el: mcpserversContainer, render: renderMCPServersScreen },
  };
}

function screenFor(item) {
  const id = item.actionId;
  const fixed = screenTable()[id];
  if (fixed) return fixed;
  if (MULTISELECT_SCREENS[id]) return { el: multiselectContainer, render: renderMultiselectScreen };
  if (SINGLE_APP_SCREENS[id]) return { el: singleappContainer, render: renderSingleAppScreen };
  if (IDE_SCREENS[id]) return { el: ideContainer, render: renderIDEScreen };
  if (FILEAPP_SCREENS[id]) return { el: fileappContainer, render: renderFileAppScreen };
  return {
    el: simpleContainer,
    render: function (it) {
      contentDesc.textContent = it.desc;
      loadSimpleSystemStat();
      updateExecutarButton();
    },
  };
}

// Every screen container (plus the repos-only tab label) — all hidden before
// the selected screen shows its own.
function screenContainers() {
  const table = screenTable();
  return Object.keys(table).map(function (k) { return table[k].el; }).concat([
    simpleContainer, multiselectContainer, singleappContainer, ideContainer, fileappContainer, tabLabelReposList,
  ]);
}

function selectItem(cat, item, catIdx, itemIdx) {
  breadcrumb.innerHTML = "<li>" + escHTML(cat.label) + "</li><li>" + escHTML(item.title) + "</li>";
  tabDescLabel.textContent = item.tab || "Perci";
  window.__selectedItem = item;

  screenContainers().forEach(function (el) { el.classList.add("hidden"); });
  // "Execução" is visible by default (almost every screen goes through
  // it) — only "Home :: Configurações" hides it (noTerminal), since none of
  // its actions are async/streamed (Tema/Logo/Ícone/Flatpak apply
  // immediately, Workspace persists on its own when "Criar Workspace"
  // is clicked).
  tabLabelTerminal.classList.remove("hidden");

  const screen = screenFor(item);
  screen.el.classList.remove("hidden");
  (screen.alsoShow || []).forEach(function (el) { el.classList.remove("hidden"); });
  if (screen.noTerminal) {
    tabLabelTerminal.classList.add("hidden");
    selectTab("desc");
  }
  screen.render(item);
  selectTab("desc");

  document.querySelectorAll("#accordion a[data-cat]").forEach(function (el) {
    const active = el.dataset.cat == String(catIdx) && el.dataset.item == String(itemIdx);
    el.classList.toggle("menu-active", active);
  });
}

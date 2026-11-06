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
import { loadScripts, scriptOrder, evalIn } from "./harness.mjs";

const tick = () => new Promise((r) => setTimeout(r, 0));

const account = {
  id: "oito2", name: "OITO2", provider: "github", host: "github.com", sshPort: 0,
  username: "oito2", gitName: "OITO2", gitEmail: "dev@oito2.com", folder: "/srv/oito2",
  sshKey: "/home/u/.ssh/perci_oito2", keyGenerated: true, hasToken: false,
  keyState: "ready", publicKey: "ssh-ed25519 AAAA dev@oito2.com", pinnedHost: true,
  hostTrusted: true, keysUrl: "https://github.com/settings/keys",
};

const cats = [
  { label: "Home", items: [{ title: "Visão Geral", actionId: "dashboard" }] },
  { label: "Repositórios", items: [
    { title: "Configurar Conta", actionId: "git-accounts" },
    { title: "OITO2", actionId: "git-account", accountId: "oito2" },
  ] },
];

function load(fixtures = {}) {
  return loadScripts(scriptOrder(), {
    fixtures: { GetCategories: cats, GetGitAccount: account, ListGitAccounts: [account], ...fixtures },
  });
}

const click = (el) => el.listeners.click.forEach((fn) => fn({ preventDefault() {} }));

test("Configurar Conta: the form keeps the current key and sends the edited fields", () => {
  const { ctx, element } = load();
  ctx.gaShowForm(account);
  assert.equal(element("ga-key-mode").value, "keep");
  assert.equal(element("ga-host").value, "", "a public host is left to the provider default");
  element("ga-name").value = "OITO2 Labs";
  const form = ctx.gaFormInput();
  assert.equal(form.error, undefined);
  assert.equal(form.input.id, "oito2");
  assert.equal(form.input.name, "OITO2 Labs");
  assert.equal(form.input.keyMode, "");
  assert.equal(form.input.folder, "/srv/oito2");
  assert.equal(form.input.passphrase, "");
  assert.equal(form.input.sshPort, 0);
});

test("Configurar Conta: a new key needs the passphrase typed twice the same", () => {
  const { ctx, element } = load();
  ctx.gaShowForm(null);
  assert.equal(element("ga-key-mode").value, "generate");
  element("ga-pass").value = "frase";
  element("ga-pass2").value = "outra";
  assert.ok(ctx.gaFormInput().error);
  element("ga-pass2").value = "frase";
  const form = ctx.gaFormInput();
  assert.equal(form.input.id, "");
  assert.equal(form.input.keyMode, "generate");
  assert.equal(form.input.passphrase, "frase");
});

test("Configurar Conta: saving reloads the menu and shows the account's details", async () => {
  const { ctx, element, calls } = load({ SaveGitAccount: account });
  await tick();
  ctx.gaShowForm(null);
  const before = calls.filter((c) => c.name === "GetCategories").length;
  element("btn-ga-save").disabled = false;
  click(element("btn-ga-save"));
  await tick();
  await tick();
  assert.ok(calls.some((c) => c.name === "SaveGitAccount"));
  assert.equal(calls.filter((c) => c.name === "GetCategories").length, before + 1);
  assert.equal(element("ga-detail-title").textContent, "OITO2");
  assert.equal(element("ga-pubkey").textContent, account.publicKey);
  assert.equal(element("btn-ga-test").disabled, false);
});

test("Configurar Conta: a locked key or an untrusted server disables the connection test", () => {
  const { ctx, element } = load();
  ctx.gaShowDetail("oito2", { ...account, keyState: "locked" });
  assert.equal(element("btn-ga-test").disabled, true);
  assert.equal(element("ga-key-badge").textContent, "bloqueada");
  ctx.gaShowDetail("oito2", { ...account, hostTrusted: false });
  assert.equal(element("btn-ga-test").disabled, true);
  ctx.gaShowDetail("oito2", account);
  assert.equal(element("btn-ga-test").disabled, false);
});

test("Configurar Conta: removing asks first, then reloads the menu", async () => {
  const { ctx, element, calls } = load();
  await tick();
  ctx.gaConfirmRemove(account);
  assert.ok(!calls.some((c) => c.name === "RemoveGitAccount"), "nothing removed before confirming");
  assert.match(element("confirm-modal-message").textContent, /perci_oito2/);
  click(element("btn-confirm-ok"));
  await tick();
  assert.ok(calls.some((c) => c.name === "RemoveGitAccount" && c.args[0] === "oito2"));
  assert.equal(calls.filter((c) => c.name === "GetCategories").length, 2);
});

test("menu: an account item is reached by its key and stays active across a reload", async () => {
  const { ctx, element } = load();
  await tick();
  assert.equal(ctx.selectMenuItem("git-account|oito2"), true);
  assert.equal(element("breadcrumb").innerHTML, "<li>Repositórios</li><li>OITO2</li>");
  assert.equal(ctx.selectMenuItem("git-account|missing"), false);
  await ctx.loadMenu();
  assert.equal(ctx.__selectedItem.accountId, "oito2");
  assert.equal(ctx.menuKey(ctx.__selectedItem), "git-account|oito2");
});

test("account screen: Configurar conta opens the account's details", async () => {
  const { ctx, element } = load();
  await tick();
  ctx.selectMenuItem("git-account|oito2");
  click(element("btn-gac-configure"));
  await tick();
  assert.equal(ctx.__selectedItem.actionId, "git-accounts");
  assert.equal(element("ga-detail-title").textContent, "OITO2");
});

const repo = (extra) => ({
  dir: "/srv/oito2/labs/perci", path: "labs/perci", branch: "main", detached: false, upstream: "origin/main",
  ahead: 0, behind: 0, modified: 0, untracked: 0, lastCommit: "2026-11-01T10:00:00-03:00", lastSubject: "x",
  remote: "git@github.com:oito2/perci.git", remoteClass: "ok", localName: "", localEmail: "",
  identityDiverges: false, lastFetch: "", ...extra,
});

test("account screen: the Alterações badges come from the local state", () => {
  const { ctx } = load();
  const labels = (st) => [...ctx.gacChanges(st).map((c) => c.label)];
  assert.deepEqual(labels(repo()), ["em dia"]);
  assert.deepEqual(labels(repo({ behind: 2, ahead: 1, modified: 3, untracked: 1 })), ["↓ 2", "↑ 1", "3 modificado(s)", "1 novo(s)"]);
  assert.deepEqual(labels(repo({ remoteClass: "none", remote: "", upstream: "" })), ["sem remote"]);
  assert.deepEqual(labels(repo({ remoteClass: "https", identityDiverges: true })), ["URL HTTPS", "identidade divergente"]);
  assert.deepEqual(labels(repo({ upstream: "" })), ["sem upstream"]);
  assert.deepEqual(labels(repo({ error: "git status: falhou" })), ["erro ao ler"]);
});

test("relTime: minutes, hours, days, then a date", () => {
  const { ctx } = load();
  const now = Date.parse("2026-11-03T12:00:00Z");
  assert.equal(ctx.relTime("2026-11-03T11:59:30Z", now), "agora");
  assert.equal(ctx.relTime("2026-11-03T11:55:00Z", now), "há 5 min");
  assert.equal(ctx.relTime("2026-11-03T09:00:00Z", now), "há 3 h");
  assert.equal(ctx.relTime("2026-11-02T12:00:00Z", now), "ontem");
  assert.equal(ctx.relTime("2026-10-24T12:00:00Z", now), "há 10 dias");
  assert.match(ctx.relTime("2026-08-01T12:00:00Z", now), /^\d\d\/08\/2026$/);
  assert.equal(ctx.relTime("", now), "");
});

test("account screen: one row per repository, and the hidden ones listed apart", async () => {
  const { ctx, element } = load({
    ListGitAccountRepos: { repos: [repo(), repo({ path: "apps/web" })], hidden: [repo({ path: "third", remote: "git@github.com:x/y.git", remoteClass: "foreign" })] },
  });
  await tick();
  ctx.selectMenuItem("git-account|oito2");
  await tick();
  assert.equal(element("gac-rows").children.length, 2);
  assert.equal(element("gac-hidden-list").children.length, 1);
  assert.equal(element("gac-hidden-list").children[0].textContent, "third — git@github.com:x/y.git");
});

test("account screen: an older answer is dropped when the list was reloaded", async () => {
  const answers = [];
  const { ctx, element } = load({
    ListGitAccountRepos: () => new Promise((resolve) => answers.push(resolve)),
  });
  await tick();
  ctx.selectMenuItem("git-account|oito2");
  await tick();
  ctx.loadGacRepos();
  answers[1]({ repos: [repo()], hidden: [] });
  await tick();
  answers[0]({ repos: [repo(), repo({ path: "b" })], hidden: [] });
  await tick();
  assert.equal(element("gac-rows").children.length, 1);
});

test("account screen: Verificar and Baixar run for all or one row, and a not-ready account can't start them", async () => {
  const { ctx, element, calls } = load({ ListGitAccountRepos: { repos: [repo()], hidden: [] } });
  await tick();
  ctx.selectMenuItem("git-account|oito2");
  await tick();
  await tick();
  assert.equal(element("btn-gac-verify").disabled, false);
  click(element("btn-gac-verify"));
  assert.ok(calls.some((c) => c.name === "VerifyGitAccountRepos" && c.args[0] === "oito2" && c.args[1] === ""));
  ctx.finishRun({ ok: true }); // re-reads the list
  await tick();

  const row = element("gac-rows").children[0];
  const pullBtn = row.children[4].children[1];
  assert.equal(pullBtn.disabled, false);
  click(pullBtn);
  assert.ok(calls.some((c) => c.name === "PullGitAccountRepos" && c.args[1] === "/srv/oito2/labs/perci"));
  ctx.finishRun({ ok: true });

  const locked = load({ GetGitAccount: { ...account, keyState: "locked" }, ListGitAccountRepos: { repos: [repo()], hidden: [] } });
  await tick();
  locked.ctx.selectMenuItem("git-account|oito2");
  await tick();
  await tick();
  assert.equal(locked.element("btn-gac-verify").disabled, true);
  assert.equal(locked.element("gac-rows").children[0].children[4].children[0].disabled, true);
});

// openAccount shows the OITO2 account screen with repos listed.
async function openAccount(fixtures) {
  const loaded = load({ ListGitAccountRepos: { repos: [repo()], hidden: [] }, ...fixtures });
  await tick();
  loaded.ctx.selectMenuItem("git-account|oito2");
  await tick();
  await tick();
  return loaded;
}

test("repository actions: Definir remote previews and sends the name, then re-reads the list", async () => {
  const { ctx, element, calls } = await openAccount({ SetGitAccountRepoRemote: "git@github.com:oito2/draft.git" });
  const draft = repo({ path: "apps/draft", dir: "/srv/oito2/apps/draft", remoteClass: "none", remote: "", upstream: "" });
  ctx.openRepoActions(draft);
  assert.equal(element("ra-remote-name").value, "draft");
  assert.equal(element("ra-remote-preview").textContent, "Remote: git@github.com:oito2/draft.git");
  element("ra-remote-name").value = "draft-app.git";
  element("ra-remote-name").listeners.input[0]();
  assert.equal(element("ra-remote-preview").textContent, "Remote: git@github.com:oito2/draft-app.git");
  element("btn-ra-remote").disabled = false;
  const before = calls.filter((c) => c.name === "ListGitAccountRepos").length;
  click(element("btn-ra-remote"));
  await tick();
  await tick();
  assert.ok(calls.some((c) => c.name === "SetGitAccountRepoRemote" && c.args[1] === "/srv/oito2/apps/draft" && c.args[2] === "draft-app.git"));
  assert.equal(calls.filter((c) => c.name === "ListGitAccountRepos").length, before + 1);
});

test("repository actions: the SSH preview carries a custom port", () => {
  const { ctx } = load();
  assert.equal(ctx.gacSSHURL({ ...account, host: "git.example.com", sshPort: 2222 }, "app"), "ssh://git@git.example.com:2222/oito2/app.git");
});

test("repository actions: Corrigir identidade asks first", async () => {
  const { ctx, element, calls } = await openAccount();
  ctx.openRepoActions(repo({ identityDiverges: true, localEmail: "other@example.com" }));
  assert.match(element("ra-ident-text").textContent, /other@example\.com/);
  click(element("btn-ra-ident"));
  assert.ok(!calls.some((c) => c.name === "ResetGitAccountRepoIdentity"));
  click(element("btn-confirm-ok"));
  await tick();
  assert.ok(calls.some((c) => c.name === "ResetGitAccountRepoIdentity" && c.args[1] === "/srv/oito2/labs/perci"));
});

test("repository actions: Trocar para SSH runs on the Execução tab; an SSH remote is just reported", async () => {
  const { ctx, element, calls } = await openAccount();
  const https = repo({ remoteClass: "https", remote: "https://github.com/oito2/perci.git", sshUrl: "git@github.com:oito2/perci.git" });
  ctx.openRepoActions(https);
  assert.match(element("ra-url-text").textContent, /git@github\.com:oito2\/perci\.git/);
  element("btn-ra-url").disabled = false;
  click(element("btn-ra-url"));
  assert.ok(calls.some((c) => c.name === "FixGitAccountRepoURLs" && c.args[1] === "/srv/oito2/labs/perci"));
  ctx.finishRun({ ok: true });

  ctx.openRepoActions(repo());
  assert.match(element("ra-ssh-ok-text").textContent, /git@github\.com:oito2\/perci\.git/);

  element("btn-gac-fix-urls").disabled = false;
  click(element("btn-gac-fix-urls"));
  assert.ok(calls.some((c) => c.name === "FixGitAccountRepoURLs" && c.args[1] === ""));
});

const fileRow = (extra) => ({ id: "pr-template", label: ".github/PULL_REQUEST_TEMPLATE.md", files: [".github/PULL_REQUEST_TEMPLATE.md"], state: "missing", ...extra });

const filesView = (extra) => ({
  rows: [
    fileRow({ id: "gitignore", label: ".gitignore", files: [".gitignore"], state: "differs", missing: 3 }),
    fileRow({ id: "license", label: "LICENSE", files: ["LICENSE"], state: "differs", license: "MIT", notes: ["Também existe COPYING."] }),
    fileRow({ id: "conduct", label: "CODE_OF_CONDUCT.md", files: ["CODE_OF_CONDUCT.md", "docs/pt-br/codigo-de-conduta.md"], state: "present", needsEmail: true }),
    fileRow({ id: "support", label: "SUPPORT.md", files: ["SUPPORT.md", "docs/pt-br/suporte.md"], state: "missing", needsEmail: true }),
    fileRow({ id: "issue-templates", label: ".github/ISSUE_TEMPLATE/", files: [".github/ISSUE_TEMPLATE/bug_report.yml"], state: "current" }),
    fileRow(),
    fileRow({ id: "funding", label: ".github/FUNDING.yml", files: [".github/FUNDING.yml"], state: "disabled" }),
  ],
  licenses: [
    { id: "GPL-3.0", name: "GNU GPL v3.0", copyright: false },
    { id: "MIT", name: "MIT", copyright: true },
  ],
  license: "GPL-3.0", holder: "Oito Dois", year: 2026,
  contactEmail: "", accountEmail: "", repoEmail: "", webUrl: "https://github.com/oito2/perci",
  ...extra,
});

// fileButtons returns the action buttons of the files table row id.
function fileButtons(element, id) {
  const rows = element("ra-files-rows").children;
  const tr = rows.find((r) => r.children[2].children.some((b) => b.dataset.row === id))
    || rows.find((r) => r.children[0].children[0].textContent === id);
  return tr ? tr.children[2].children : [];
}

test("repository dialog: the plus button opens it on Configurações and reads the files", async () => {
  const { ctx, element, calls } = await openAccount({ GetGitAccountRepoFiles: filesView({ accountEmail: "acc@example.org", contactEmail: "acc@example.org" }) });
  const more = element("gac-rows").children[0].children[4].children[2];
  assert.equal(more.innerHTML, "<svg></svg>", "the button shows an icon");
  element("ra-tab-settings").checked = false;
  click(more);
  await tick();
  assert.equal(element("ra-tab-settings").checked, true);
  assert.equal(element("ra-title").textContent, "labs/perci");
  assert.ok(calls.some((c) => c.name === "GetGitAccountRepoFiles" && c.args[0] === "oito2" && c.args[1] === "/srv/oito2/labs/perci" && c.args[2] === ""));
  assert.equal(element("ra-account-email").value, "acc@example.org");
  assert.equal(element("ra-repo-email").value, "");
  assert.match(element("ra-email-effective").textContent, /acc@example\.org \(padrão da conta\)/);
  assert.equal(element("ra-license").value, "GPL-3.0");
  assert.equal(element("ra-license-holder").value, "Oito Dois");
  assert.equal(element("ra-license-year").value, "2026");
});

test("repository dialog: Status and actions follow each row's state", async () => {
  const { ctx, element } = await openAccount({ GetGitAccountRepoFiles: filesView() });
  ctx.openRepoActions(repo());
  await tick();
  const view = filesView();
  const status = (id) => ctx.raStatus(view.rows.find((r) => r.id === id), view).label;
  assert.equal(status("gitignore"), "faltam 3 padrão(ões)");
  assert.equal(status("license"), "MIT");
  assert.equal(status("conduct"), "existe");
  assert.equal(status("support"), "não existe");
  assert.equal(status("issue-templates"), "em dia");
  assert.equal(status("funding"), "em breve");
  assert.equal(ctx.raStatus(fileRow({ id: "license", state: "current", license: "GPL-3.0" }), view).label, "GPL-3.0 — em dia");
  assert.equal(ctx.raStatus(fileRow({ id: "license", state: "differs", license: "" }), view).label, "licença não reconhecida");

  const labels = (id) => fileButtons(element, id).map((b) => b.textContent);
  assert.deepEqual(labels("gitignore"), ["Atualizar", "Remover"]);
  assert.deepEqual(labels("support"), ["Criar"]);
  assert.deepEqual(labels("issue-templates"), ["Remover"]);
  assert.deepEqual(labels(".github/FUNDING.yml"), []);
  assert.equal(fileButtons(element, "support")[0].disabled, true, "no e-mail, no SUPPORT.md");
  assert.equal(fileButtons(element, "conduct")[0].disabled, true);
  assert.equal(fileButtons(element, "conduct")[1].disabled, false, "removing needs no e-mail");
  const notes = element("ra-files-rows").children[1].children[0].children.map((c) => c.textContent);
  assert.ok(notes.includes("Também existe COPYING."));
});

test("repository dialog: Criar and the .gitignore run at once; replacing a file asks first", async () => {
  const { ctx, element, calls } = await openAccount({
    GetGitAccountRepoFiles: filesView({ contactEmail: "acc@example.org" }),
    ApplyGitAccountRepoFile: "gravado.",
  });
  ctx.openRepoActions(repo());
  await tick();
  const applies = () => calls.filter((c) => c.name === "ApplyGitAccountRepoFile");

  click(fileButtons(element, "support")[0]);
  await tick();
  assert.equal(applies().length, 1);
  assert.deepEqual([...applies()[0].args.slice(0, 3)], ["oito2", "/srv/oito2/labs/perci", "support"]);
  assert.equal(element("ra-files-msg").textContent, "gravado.");
  const reloads = calls.filter((c) => c.name === "GetGitAccountRepoFiles");
  assert.equal(reloads[reloads.length - 1].args[2], "GPL-3.0", "the reload keeps the chosen license");

  click(fileButtons(element, "gitignore")[0]);
  await tick();
  assert.equal(applies().length, 2, "the .gitignore only gains patterns: no confirmation");

  click(fileButtons(element, "conduct")[0]);
  assert.equal(applies().length, 2, "replacing waits for the confirmation");
  assert.match(element("confirm-modal-message").textContent, /\.perci-bak/);
  click(element("btn-confirm-ok"));
  await tick();
  assert.equal(applies().length, 3);

  element("ra-license-holder").value = " Outro ";
  element("ra-license-year").value = "2025";
  click(fileButtons(element, "license")[0]);
  assert.match(element("confirm-modal-message").textContent, /Substituir a licença MIT por GPL-3\.0/);
  click(element("btn-confirm-ok"));
  await tick();
  assert.deepEqual({ ...applies()[3].args[3] }, { license: "GPL-3.0", holder: "Outro", year: 2025 });
});

test("repository dialog: Remover asks first, and an error stays in the dialog", async () => {
  const { ctx, element, calls } = await openAccount({
    GetGitAccountRepoFiles: filesView(),
    RemoveGitAccountRepoFile: () => Promise.reject(new Error("sem permissão")),
  });
  ctx.openRepoActions(repo());
  await tick();
  click(fileButtons(element, "issue-templates")[0]);
  assert.ok(!calls.some((c) => c.name === "RemoveGitAccountRepoFile"));
  assert.match(element("confirm-modal-message").textContent, /bug_report\.yml/);
  click(element("btn-confirm-ok"));
  await tick();
  await tick();
  assert.ok(calls.some((c) => c.name === "RemoveGitAccountRepoFile" && c.args[2] === "issue-templates"));
  assert.match(element("ra-files-msg").textContent, /sem permissão/);
});

test("repository dialog: the e-mails are saved for the account or the repository, and the license choice reloads", async () => {
  const { ctx, element, calls } = await openAccount({ GetGitAccountRepoFiles: filesView() });
  ctx.openRepoActions(repo());
  await tick();
  element("ra-account-email").value = " acc@example.org ";
  element("btn-ra-account-email").disabled = false;
  click(element("btn-ra-account-email"));
  await tick();
  element("ra-repo-email").value = "perci@example.org";
  element("btn-ra-repo-email").disabled = false;
  click(element("btn-ra-repo-email"));
  await tick();
  const saves = calls.filter((c) => c.name === "SetGitAccountContactEmail").map((c) => [...c.args]);
  assert.deepEqual(saves, [["oito2", "", "acc@example.org"], ["oito2", "/srv/oito2/labs/perci", "perci@example.org"]]);

  element("ra-license-holder").value = "Typed Holder";
  element("ra-license").value = "MIT";
  element("ra-license").listeners.change[0]();
  await tick();
  const reloads = calls.filter((c) => c.name === "GetGitAccountRepoFiles");
  assert.equal(reloads[reloads.length - 1].args[2], "MIT");
  assert.equal(element("ra-license-holder").value, "Typed Holder", "a reload keeps what was typed");
});

test("repository dialog: an older answer is dropped when the files were reloaded", async () => {
  let resolveFirst;
  let n = 0;
  const { ctx, element } = await openAccount({
    GetGitAccountRepoFiles: () => (++n === 1 ? new Promise((r) => { resolveFirst = r; }) : filesView({ license: "MIT" })),
  });
  ctx.openRepoActions(repo());
  ctx.raLoadFiles("MIT");
  await tick();
  resolveFirst(filesView({ license: "GPL-3.0" }));
  await tick();
  assert.equal(element("ra-license").value, "MIT");
});

test("Novo Repositório: pick from the account's list, the folder follows, and the clone runs after the check", async () => {
  const { ctx, element, calls } = await openAccount({
    ListGitAccountRemoteRepos: { withToken: true, repos: [
      { name: "perci", private: true, sshUrl: "git@github.com:oito2/perci.git", localPath: "labs/perci" },
      { name: "write82", private: false, sshUrl: "git@github.com:oito2/write82.git", description: "editor" },
    ] },
    CheckGitAccountClone: "/srv/oito2/write82",
  });
  ctx.openNewRepo();
  element("btn-nr-load").disabled = false;
  click(element("btn-nr-load"));
  await tick();
  const items = element("nr-list").children;
  assert.equal(items.length, 2);
  assert.equal(items[0].children[0].children[1].textContent, "já clonado em labs/perci");
  click(items[0].children[0]); // already cloned: not picked
  assert.equal(element("btn-nr-go").disabled, true);
  click(items[1].children[0]);
  assert.equal(element("nr-dest").value, "write82");
  assert.equal(element("btn-nr-go").disabled, false);

  element("nr-dest").value = "apps/write82";
  element("nr-dest").listeners.input[0]();
  assert.equal(element("nr-dest-preview").textContent, "/srv/oito2/apps/write82");
  click(element("btn-nr-go"));
  await tick();
  await tick();
  assert.ok(calls.some((c) => c.name === "CheckGitAccountClone" && c.args[1] === "git@github.com:oito2/write82.git" && c.args[2] === "apps/write82"));
  assert.ok(calls.some((c) => c.name === "CloneGitAccountRepo" && c.args[2] === "apps/write82"));
});

test("Novo Repositório: a refused clone keeps the dialog and never starts", async () => {
  const { ctx, element, calls } = await openAccount({
    CheckGitAccountClone: () => Promise.reject(new Error("apps/x já existe e não está vazia")),
  });
  ctx.openNewRepo();
  element("nr-mode").value = "clone";
  element("nr-source").value = "https://github.com/oito2/perci.git";
  element("nr-source").listeners.input[0]();
  assert.equal(element("nr-dest").value, "perci");
  click(element("btn-nr-go"));
  await tick();
  await tick();
  assert.ok(!calls.some((c) => c.name === "CloneGitAccountRepo"));
  assert.match(element("error-modal-message").textContent, /não está vazia/);
});

test("Novo Repositório: Iniciar creates the repository and re-reads the list", async () => {
  const { ctx, element, calls } = await openAccount({ InitGitAccountRepo: "/srv/oito2/apps/draft" });
  ctx.openNewRepo();
  element("nr-mode").value = "init";
  element("nr-mode").listeners.change[0]();
  assert.equal(element("btn-nr-go").textContent, "Iniciar");
  element("nr-dest").value = "apps/draft";
  element("nr-dest").listeners.input[0]();
  element("nr-remote-name").value = "draft";
  element("nr-remote-name").listeners.input[0]();
  assert.equal(element("nr-dest").value, "apps/draft", "a typed folder is kept");
  const before = calls.filter((c) => c.name === "ListGitAccountRepos").length;
  click(element("btn-nr-go"));
  await tick();
  await tick();
  assert.ok(calls.some((c) => c.name === "InitGitAccountRepo" && c.args[1] === "apps/draft" && c.args[2] === "draft"));
  assert.equal(calls.filter((c) => c.name === "ListGitAccountRepos").length, before + 1);
});

// --- Gerenciar Repositórios -------------------------------------------------

const change = (box, checked) => { box.checked = checked; box.listeners.change.forEach((fn) => fn({})); };

const exportItems = {
  accounts: [
    { name: "OITO2", provider: "github", host: "github.com", username: "oito2", folder: "/srv/oito2",
      repos: [{ path: "labs/perci", url: "git@github.com:oito2/perci.git" }, { path: "web", url: "git@github.com:oito2/web.git" }] },
    { name: "Kadu", provider: "gitlab", host: "gitlab.com", username: "kadu", folder: "/srv/kadu", repos: [] },
  ],
  skipped: ["OITO2: draft"],
};

test("Gerenciar Repositórios: the export checklist ties repositories to their account", async () => {
  const { ctx, element, calls } = load({ GetGitRepoExportItems: exportItems, PickGitRepoExportPath: "/tmp/r.yaml" });
  ctx.renderGitTransferScreen();
  await tick();
  const rows = evalIn(ctx, "gtExportRows");
  assert.equal(rows.length, 2);
  assert.deepEqual(JSON.parse(JSON.stringify(ctx.gtPicks(rows))), [
    { account: "OITO2", repos: ["labs/perci", "web"] }, { account: "Kadu", repos: [] },
  ]);
  change(rows[0].box, false);
  assert.equal(rows[0].repos[0].box.checked, false, "unchecking an account unchecks its repositories");
  change(rows[0].repos[1].box, true);
  assert.equal(rows[0].box.checked, true, "checking a repository checks its account");
  change(rows[1].box, false);
  assert.match(element("gt-export-note-text").textContent, /OITO2: draft/);

  click(element("btn-gt-export"));
  await tick();
  await tick();
  const exp = calls.find((c) => c.name === "ExportGitRepos");
  assert.equal(exp.args[0], "/tmp/r.yaml");
  assert.deepEqual(JSON.parse(JSON.stringify(exp.args[1])), [{ account: "OITO2", repos: ["web"] }]);
});

const importPlans = [
  { name: "OITO2", provider: "github", host: "github.com", username: "oito2", folder: "/srv/oito2",
    existingId: "oito2", existingName: "OITO2", reason: "",
    repos: [{ path: "labs/perci", url: "u1" }, { path: "web", url: "u2" }, { path: "full", url: "u3" }],
    states: [{ path: "labs/perci", state: "present" }, { path: "web", state: "new" }, { path: "full", state: "conflict", reason: "não está vazia" }] },
  { name: "Kadu", provider: "gitlab", host: "gitlab.com", username: "kadu", folder: "/srv/kadu",
    existingId: "", reason: "", repos: [{ path: "a", url: "u4" }], states: [{ path: "a", state: "new" }] },
  { name: "Minha", provider: "github", host: "github.com", username: "outra", folder: "/srv/outra",
    existingId: "", reason: "já existe uma conta chamada \"Minha\"", repos: [], states: [] },
];

test("Gerenciar Repositórios: the import checklist skips what can't be imported and sends the picked folders", async () => {
  const { ctx, element, calls } = load({
    PickGitRepoImportPath: "/tmp/r.yaml", PreviewGitRepoImport: importPlans, PickGitAccountFolder: "/home/u/kadu",
  });
  ctx.renderGitTransferScreen();
  click(element("btn-gt-pick"));
  await tick();
  await tick();
  const rows = evalIn(ctx, "gtImportRows");
  assert.equal(rows[2].box.disabled, true, "an account with a reason is disabled");
  assert.deepEqual(JSON.parse(JSON.stringify(ctx.gtPicks(rows))), [
    { account: "OITO2", repos: ["web"] }, { account: "Kadu", repos: ["a"] },
  ]);
  assert.equal(element("btn-gt-import").disabled, false);

  // "Trocar pasta" of the new account re-reads the file with the picked folder.
  const kaduButtons = rows[1].box.parentNode.children[1].children.filter((c) => c.tagName === "BUTTON");
  click(kaduButtons[0]);
  await tick();
  await tick();
  const previews = calls.filter((c) => c.name === "PreviewGitRepoImport");
  assert.equal(previews.length, 2);
  assert.deepEqual(JSON.parse(JSON.stringify(previews[1].args[1])), { Kadu: "/home/u/kadu" });

  click(element("btn-gt-import"));
  click(element("btn-confirm-ok"));
  await tick();
  const imp = calls.find((c) => c.name === "ImportGitRepos");
  assert.equal(imp.args[0], "/tmp/r.yaml");
  assert.deepEqual(JSON.parse(JSON.stringify(imp.args[2])), { Kadu: "/home/u/kadu" });
  assert.equal(imp.args[1].length, 2);
});

test("account screen: pending repositories can be cloned or discarded", async () => {
  const pending = [{ path: "web", url: "git@github.com:oito2/web.git", reason: "" }];
  const { ctx, element, calls } = load({ ListGitAccountRepos: { repos: [], hidden: [], pending } });
  await tick();
  ctx.selectMenuItem("git-account|oito2");
  await tick();
  await tick();
  await tick();
  assert.equal(element("gac-pending-list").children.length, 1);
  const discard = element("gac-pending-list").children[0].children.find((c) => c.tagName === "BUTTON");
  click(discard);
  click(element("btn-confirm-ok"));
  await tick();
  assert.deepEqual(calls.find((c) => c.name === "DiscardGitAccountPending").args, ["oito2", "web"]);

  element("btn-gac-clone-pending").disabled = false;
  click(element("btn-gac-clone-pending"));
  click(element("btn-confirm-ok"));
  await tick();
  assert.deepEqual(calls.find((c) => c.name === "CloneGitAccountPending").args, ["oito2"]);
});

test("Desinstalar: the repositories backup has its own picker, and cancelling it stops the uninstall", async () => {
  const run = load({ PickUninstallBackupPath: "/tmp/b.yaml", PickUninstallReposBackupPath: "/tmp/r.yaml" });
  run.element("su2-backup-config").checked = true;
  run.element("su2-backup-repos").checked = true;
  run.element("su2-backup-ai").checked = false;
  run.element("su2-remove-docker").checked = false;
  run.element("su2-remove-config").checked = true;
  click(run.element("btn-su2-uninstall"));
  click(run.element("btn-confirm-ok"));
  await tick();
  await tick();
  await tick();
  assert.deepEqual(run.calls.find((c) => c.name === "RunSelfUninstall").args, [true, false, "/tmp/b.yaml", "/tmp/r.yaml", ""]);

  const cancel = load({ PickUninstallReposBackupPath: "" });
  cancel.element("su2-backup-config").checked = false;
  cancel.element("su2-backup-repos").checked = true;
  cancel.element("su2-backup-ai").checked = true;
  cancel.element("su2-remove-docker").checked = false;
  cancel.element("su2-remove-config").checked = false;
  click(cancel.element("btn-su2-uninstall"));
  click(cancel.element("btn-confirm-ok"));
  await tick();
  await tick();
  await tick();
  assert.ok(!cancel.calls.some((c) => c.name === "RunSelfUninstall"));
  assert.ok(!cancel.calls.some((c) => c.name === "PickUninstallBackupPath"));
  assert.ok(!cancel.calls.some((c) => c.name === "PickUninstallAIContextsBackupPath"), "no dialog after a cancelled one");
});

test("Desinstalar: the AI contexts backup has its own picker, and cancelling it stops the uninstall", async () => {
  const start = (fixtures, config) => {
    const run = load(fixtures);
    run.element("su2-backup-config").checked = config;
    run.element("su2-backup-repos").checked = false;
    run.element("su2-backup-ai").checked = true;
    run.element("su2-remove-docker").checked = false;
    run.element("su2-remove-config").checked = false;
    click(run.element("btn-su2-uninstall"));
    assert.match(run.element("confirm-modal-message").textContent, /backup dos contextos de IA/);
    click(run.element("btn-confirm-ok"));
    return run;
  };
  const settle = async () => { for (let i = 0; i < 6; i++) await tick(); };

  const run = start({ PickUninstallBackupPath: "/tmp/b.yaml", PickUninstallAIContextsBackupPath: "/tmp/ia.yaml" }, true);
  await settle();
  assert.deepEqual(run.calls.find((c) => c.name === "RunSelfUninstall").args, [false, false, "/tmp/b.yaml", "", "/tmp/ia.yaml"]);
  assert.ok(!run.calls.some((c) => c.name === "PickUninstallReposBackupPath"));

  const cancel = start({ PickUninstallAIContextsBackupPath: "" }, false);
  await settle();
  assert.ok(!cancel.calls.some((c) => c.name === "RunSelfUninstall"));
  assert.match(cancel.element("error-modal-message").textContent, /desinstalação não foi iniciada/);
});

test("Configurar Conta: the form explains the SSH key and the token, and opens the provider's pages", () => {
  const { ctx, element } = load();
  const opened = [];
  ctx.Browser.OpenURL = (url) => opened.push(url);
  ctx.gaShowForm(null);
  assert.match(element("ga-key-hint").textContent, /Authentication Key/);
  assert.match(element("ga-token-steps").textContent, /Metadata: Read-only/);
  click(element("btn-ga-form-keys-page"));
  click(element("btn-ga-token-page"));
  assert.deepEqual(opened, ["https://github.com/settings/keys", "https://github.com/settings/personal-access-tokens/new"]);

  element("ga-provider").value = "gitlab";
  element("ga-host").value = "Git.Example.com";
  ctx.gaUpdateForm();
  assert.match(element("ga-token-steps").textContent, /read_api/);
  click(element("btn-ga-token-page"));
  assert.equal(opened[2], "https://git.example.com/-/user_settings/personal_access_tokens");

  element("ga-host").value = "javascript:alert(1)";
  ctx.gaUpdateForm();
  assert.equal(element("btn-ga-token-page").disabled, true);
  click(element("btn-ga-token-page"));
  assert.equal(opened.length, 3, "an invalid host is never opened");
});

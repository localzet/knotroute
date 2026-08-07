"use strict";

type Peer = { address: string; expected_id?: string };
type Service = { name: string; target: string; description?: string; allow?: string[]; publish?: boolean; identity_file?: string; intro_count?: number; metadata?: Record<string,string> };
type Forward = { listen: string; node: string; service: string };
type Alias = { name: string; node?: string; service_id?: string; description?: string };
type Config = {
  network_id: string;
  identity_file: string;
  listen: string[];
  advertise?: string[];
  peers?: Peer[];
  services?: Service[];
  forwards?: Forward[];
  aliases?: Alias[];
  dashboard: string;
  proxy: { socks: string; http: string; direct: boolean; default_http_service: string; default_https_service: string };
  ca: { enabled: boolean; directory: string; intercept_https: boolean };
  discovery: { enabled: boolean; beacons?: string[]; lan: boolean; peer_exchange: boolean; cache_file?: string; interval: string };
  directory: { replicas: number; descriptor_ttl: string; publish_interval: string; lookup_timeout: string };
  privacy: { circuit_hops: number; circuit_timeout: string };
  routing: { lsa_interval: string; lsa_ttl: string; max_hops: number };
};
type Status = any;
type Field = { key: string; label: string; value?: string; checked?: boolean; type?: "text"|"checkbox"|"number"; placeholder?: string; cls?: string };

const byId = <T extends HTMLElement = HTMLElement>(id: string): T => {
  const node = document.getElementById(id);
  if (!node) throw new Error(`missing element #${id}`);
  return node as T;
};
let lastStatus: Status | null = null;
let currentConfig: Config | null = null;
let dirty = false;

const short = (id: string | null | undefined) => id && id.length > 18 ? `${id.slice(0, 10)}…${id.slice(-4)}` : id ?? "—";
const bytes = (n: number) => { if (!n) return "0 B"; const u = ["B", "KiB", "MiB", "GiB", "TiB"]; const i = Math.min(Math.floor(Math.log(n) / Math.log(1024)), u.length - 1); return `${(n / 1024 ** i).toFixed(i ? 1 : 0)} ${u[i]}`; };
const duration = (seconds: number) => { const n = Math.max(0, Math.floor(seconds)), d = Math.floor(n / 86400), h = Math.floor(n % 86400 / 3600), m = Math.floor(n % 3600 / 60), s = n % 60; return [[d, "d"], [h, "h"], [m, "m"], [s, "s"]].filter(([v]) => Number(v) > 0 || s === Number(v)).map(([v, u]) => `${v}${u}`).join(" "); };
const esc = (value: unknown) => String(value ?? "").replace(/[&<>'"]/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", "'": "&#39;", '"': "&quot;" }[c] ?? c));
const lines = (value: string) => value.split(/\r?\n/).map(x => x.trim()).filter(Boolean);
const csv = (value: string) => value.split(",").map(x => x.trim()).filter(Boolean);

function setSaveState(message: string, state = "") {
  const bar = document.querySelector<HTMLElement>(".savebar");
  if (bar) bar.className = `savebar panel ${state}`;
  byId("saveState").textContent = message;
}
function markDirty() { dirty = true; setSaveState("Unsaved changes"); }

function renderRoutes(items: any[]) {
  const root = byId("routes");
  if (!items.length) { root.className = "table-wrap empty"; root.textContent = "No routes yet"; return; }
  root.className = "table-wrap";
  root.innerHTML = `<table><thead><tr><th>Destination</th><th>Path</th><th>Hops</th><th>Services</th></tr></thead><tbody>${items.map(r => `<tr><td><code title="${esc(r.destination)}">${esc(r.domain)}</code></td><td class="path">${r.path.map(short).map(esc).join(" → ")}</td><td>${r.hops}</td><td>${r.services.length ? r.services.map((x: string) => `<span class="service-pill">${esc(x)}</span>`).join("") : "—"}</td></tr>`).join("")}</tbody></table>`;
}
function renderPeers(items: any[]) {
  const root = byId("peers");
  if (!items.length) { root.className = "cards empty"; root.textContent = "No peers connected"; return; }
  root.className = "cards";
  root.innerHTML = items.map(p => `<div class="card"><div class="card-top"><strong title="${esc(p.id)}">${esc(p.short_id)}</strong><span class="direction">${esc(p.direction)}</span></div><p>${esc(p.remote_addr)}${p.advertise?.length ? ` · ${esc(p.advertise.join(", "))}` : ""}</p></div>`).join("");
}
function renderEndpoints(services: any[], forwards: any[]) {
  const root = byId("endpoints");
  const all = [...services.map(x => ({ title: x.domain || x.name, sub: `${x.name} → ${x.target}${x.description ? ` · ${x.description}` : ""}${x.introduction_points?.length ? ` · ${x.introduction_points.length} intros` : ""}`, tag: x.published ? "hidden service" : "direct service", ok: true })), ...forwards.map(x => ({ title: x.listen, sub: `${short(x.node)} / ${x.service}${x.error ? ` · ${x.error}` : ""}`, tag: "forward", ok: x.active }))];
  if (!all.length) { root.className = "cards empty"; root.textContent = "Nothing configured"; return; }
  root.className = "cards";
  root.innerHTML = all.map(x => `<div class="card"><div class="card-top"><strong>${esc(x.title)}</strong><span class="state ${x.ok ? "ok" : "bad"}">${esc(x.tag)}</span></div><p>${esc(x.sub)}</p></div>`).join("");
}
function renderEvents(items: any[]) {
  const root = byId("events"), show = items.slice(-80).reverse();
  if (!show.length) { root.className = "events empty"; root.textContent = "No events"; return; }
  root.className = "events";
  root.innerHTML = show.map(e => `<div class="event"><time>${new Date(e.time).toLocaleTimeString()}</time><span class="level ${esc(e.level)}">${esc(e.level)}</span><span>${esc(e.message)}</span></div>`).join("");
}
function renderStatus(s: Status) {
  lastStatus = s;
  byId("domain").textContent = s.domain;
  byId("nodeId").textContent = s.node_id;
  byId("networkId").textContent = s.network_id ?? "—";
  byId("listen").textContent = s.listen.length ? `Listening on ${s.listen.join(", ")}` : "No overlay listener";
  byId("peerCount").textContent = String(s.peers.length);
  byId("routeCount").textContent = String(s.routes.length);
  byId("streamCount").textContent = `${s.active_streams} / ${s.active_circuits ?? 0}`;
  byId("traffic").textContent = bytes(s.bytes_sent + s.bytes_received);
  byId("frames").textContent = `${s.frames_sent + s.frames_received} frames · ↑ ${bytes(s.bytes_sent)} ↓ ${bytes(s.bytes_received)}`;
  byId("uptime").textContent = duration((Date.now() - new Date(s.started_at).getTime()) / 1000);
  byId("version").textContent = `KnotRoute ${s.version}`;
  byId("serviceCount").textContent = String(s.services.filter((x: any) => x.published).length);
  byId("descriptorCount").textContent = `service identities · ${s.descriptors ?? 0} descriptors`;
  byId("gatewayCount").textContent = String(s.proxy.listeners.length);
  byId("topologyBadge").textContent = `${s.routes.length} nodes`;
  byId("socksAddress").textContent = s.proxy.socks ? `socks5://${s.proxy.socks}` : "disabled";
  byId("httpAddress").textContent = s.proxy.http ? `http://${s.proxy.http}` : "disabled";
  byId("pacAddress").textContent = s.proxy.pac || "disabled";
  renderRoutes(s.routes); renderPeers(s.peers); renderEndpoints(s.services, s.forwards); renderEvents(s.events);
  const health = document.querySelector<HTMLElement>(".health"); if (health) health.className = "health online";
  byId("healthText").textContent = "Online";
}
async function refresh() {
  try {
    const response = await fetch("/api/status", { cache: "no-store" });
    if (!response.ok) throw new Error(response.statusText);
    renderStatus(await response.json());
  } catch {
    const health = document.querySelector<HTMLElement>(".health"); if (health) health.className = "health offline";
    byId("healthText").textContent = "Disconnected";
  }
}

function makeRow(root: HTMLElement, fields: Field[]) {
  const row = document.createElement("div"); row.className = "editor-row";
  for (const field of fields) {
    const label = document.createElement("label"); if (field.cls) label.className = field.cls;
    const span = document.createElement("span"); span.textContent = field.label;
    const input = document.createElement("input"); input.dataset.key = field.key; input.type = field.type ?? "text"; if (input.type === "checkbox") input.checked = field.checked ?? false; else input.value = field.value ?? ""; input.placeholder = field.placeholder ?? "";
    input.addEventListener("input", markDirty); label.append(span, input); row.append(label);
  }
  const remove = document.createElement("button"); remove.className = "remove"; remove.type = "button"; remove.textContent = "×"; remove.title = "Remove";
  remove.addEventListener("click", () => { row.remove(); markDirty(); }); row.append(remove); root.append(row);
}
function value(row: Element, key: string) { return (row.querySelector<HTMLInputElement>(`[data-key="${key}"]`)?.value ?? "").trim(); }
function checked(row: Element, key: string) { return row.querySelector<HTMLInputElement>(`[data-key="${key}"]`)?.checked ?? false; }
function rows(id: string) { return [...byId(id).querySelectorAll(".editor-row")]; }
function renderConfig(cfg: Config) {
  currentConfig = cfg;
  const peerRoot = byId("peerEditor"); peerRoot.replaceChildren();
  for (const p of cfg.peers ?? []) makeRow(peerRoot, [{ key: "address", label: "Address", value: p.address, placeholder: "seed.example:7447", cls: "wide" }, { key: "expected", label: "Expected node ID", value: p.expected_id, placeholder: "kr_…", cls: "wide" }]);
  const serviceRoot = byId("serviceEditor"); serviceRoot.replaceChildren();
  for (const s of cfg.services ?? []) makeRow(serviceRoot, [{ key: "name", label: "Name", value: s.name, placeholder: "web", cls: "narrow" }, { key: "target", label: "Local target", value: s.target, placeholder: "127.0.0.1:8080" }, { key: "publish", label: "Publish identity", type: "checkbox", checked: s.publish ?? false, cls: "narrow" }, { key: "intros", label: "Intro points", type: "number", value: String(s.intro_count ?? 3), cls: "narrow" }, { key: "description", label: "Description", value: s.description }, { key: "allow", label: "Direct-mode node ACL", value: (s.allow ?? []).join(","), placeholder: "*", cls: "wide" }]);
  const forwardRoot = byId("forwardEditor"); forwardRoot.replaceChildren();
  for (const f of cfg.forwards ?? []) makeRow(forwardRoot, [{ key: "listen", label: "Local listener", value: f.listen, placeholder: "127.0.0.1:2222" }, { key: "node", label: "Remote node ID", value: f.node, placeholder: "kr_…", cls: "wide" }, { key: "service", label: "Service", value: f.service, placeholder: "ssh", cls: "narrow" }]);
  const aliasRoot = byId("aliasEditor"); aliasRoot.replaceChildren();
  for (const a of cfg.aliases ?? []) makeRow(aliasRoot, [{ key: "name", label: "Alias", value: a.name, placeholder: "server", cls: "narrow" }, { key: "node", label: "Node target", value: a.node, placeholder: "kr_… or node.knot", cls: "wide" }, { key: "serviceId", label: "Service target", value: a.service_id, placeholder: "ks_… or service.knot", cls: "wide" }, { key: "description", label: "Description", value: a.description }]);
  byId<HTMLTextAreaElement>("listenInput").value = cfg.listen.join("\n");
  byId<HTMLTextAreaElement>("advertiseInput").value = (cfg.advertise ?? []).join("\n");
  byId<HTMLInputElement>("networkIdInput").value = cfg.network_id;
  byId<HTMLInputElement>("discoveryEnabled").checked = cfg.discovery.enabled;
  byId<HTMLInputElement>("discoveryLan").checked = cfg.discovery.lan;
  byId<HTMLInputElement>("discoveryPex").checked = cfg.discovery.peer_exchange;
  byId<HTMLInputElement>("discoveryInterval").value = cfg.discovery.interval;
  byId<HTMLTextAreaElement>("beaconInput").value = (cfg.discovery.beacons ?? []).join("\n");
  byId<HTMLInputElement>("circuitHops").value = String(cfg.privacy.circuit_hops);
  byId<HTMLInputElement>("circuitTimeout").value = cfg.privacy.circuit_timeout;
  byId<HTMLInputElement>("directoryReplicas").value = String(cfg.directory.replicas);
  byId<HTMLInputElement>("descriptorTtl").value = cfg.directory.descriptor_ttl;
  byId<HTMLInputElement>("descriptorPublish").value = cfg.directory.publish_interval;
  byId<HTMLInputElement>("descriptorLookup").value = cfg.directory.lookup_timeout;
  byId<HTMLInputElement>("caEnabled").checked = cfg.ca.enabled;
  byId<HTMLInputElement>("caIntercept").checked = cfg.ca.intercept_https;
  byId<HTMLInputElement>("caDirectory").value = cfg.ca.directory;
  byId<HTMLInputElement>("proxySocks").value = cfg.proxy.socks;
  byId<HTMLInputElement>("proxyHttp").value = cfg.proxy.http;
  byId<HTMLInputElement>("defaultHttp").value = cfg.proxy.default_http_service;
  byId<HTMLInputElement>("defaultHttps").value = cfg.proxy.default_https_service;
  byId<HTMLInputElement>("proxyDirect").checked = cfg.proxy.direct;
  byId<HTMLInputElement>("dashboardInput").value = cfg.dashboard;
  byId<HTMLInputElement>("maxHops").value = String(cfg.routing.max_hops);
  byId<HTMLInputElement>("lsaInterval").value = cfg.routing.lsa_interval;
  byId<HTMLInputElement>("lsaTtl").value = cfg.routing.lsa_ttl;
  dirty = false; setSaveState("Configuration loaded", "success");
}
async function loadConfig() {
  const response = await fetch("/api/config", { cache: "no-store" });
  if (!response.ok) throw new Error(await response.text());
  renderConfig(await response.json());
}
function collectConfig(): Config {
  if (!currentConfig) throw new Error("configuration is not loaded");
  return {
    ...currentConfig,
    listen: lines(byId<HTMLTextAreaElement>("listenInput").value),
    advertise: lines(byId<HTMLTextAreaElement>("advertiseInput").value),
    peers: rows("peerEditor").map(row => ({ address: value(row, "address"), ...(value(row, "expected") ? { expected_id: value(row, "expected") } : {}) })).filter(x => x.address),
    network_id: byId<HTMLInputElement>("networkIdInput").value.trim(),
    services: rows("serviceEditor").map(row => ({ name: value(row, "name"), target: value(row, "target"), publish: checked(row, "publish"), intro_count: Number(value(row, "intros") || "3"), ...(value(row, "description") ? { description: value(row, "description") } : {}), ...(csv(value(row, "allow")).length ? { allow: csv(value(row, "allow")) } : {}) })).filter(x => x.name || x.target),
    forwards: rows("forwardEditor").map(row => ({ listen: value(row, "listen"), node: value(row, "node"), service: value(row, "service") })).filter(x => x.listen || x.node || x.service),
    aliases: rows("aliasEditor").map(row => ({ name: value(row, "name"), ...(value(row, "node") ? { node: value(row, "node") } : {}), ...(value(row, "serviceId") ? { service_id: value(row, "serviceId") } : {}), ...(value(row, "description") ? { description: value(row, "description") } : {}) })).filter(x => x.name || x.node || x.service_id),
    discovery: { ...currentConfig.discovery, enabled: byId<HTMLInputElement>("discoveryEnabled").checked, lan: byId<HTMLInputElement>("discoveryLan").checked, peer_exchange: byId<HTMLInputElement>("discoveryPex").checked, interval: byId<HTMLInputElement>("discoveryInterval").value.trim(), beacons: lines(byId<HTMLTextAreaElement>("beaconInput").value) },
    privacy: { circuit_hops: Number(byId<HTMLInputElement>("circuitHops").value), circuit_timeout: byId<HTMLInputElement>("circuitTimeout").value.trim() },
    directory: { replicas: Number(byId<HTMLInputElement>("directoryReplicas").value), descriptor_ttl: byId<HTMLInputElement>("descriptorTtl").value.trim(), publish_interval: byId<HTMLInputElement>("descriptorPublish").value.trim(), lookup_timeout: byId<HTMLInputElement>("descriptorLookup").value.trim() },
    ca: { enabled: byId<HTMLInputElement>("caEnabled").checked, directory: byId<HTMLInputElement>("caDirectory").value.trim(), intercept_https: byId<HTMLInputElement>("caIntercept").checked },
    dashboard: byId<HTMLInputElement>("dashboardInput").value.trim(),
    proxy: { socks: byId<HTMLInputElement>("proxySocks").value.trim(), http: byId<HTMLInputElement>("proxyHttp").value.trim(), direct: byId<HTMLInputElement>("proxyDirect").checked, default_http_service: byId<HTMLInputElement>("defaultHttp").value.trim(), default_https_service: byId<HTMLInputElement>("defaultHttps").value.trim() },
    routing: { lsa_interval: byId<HTMLInputElement>("lsaInterval").value.trim(), lsa_ttl: byId<HTMLInputElement>("lsaTtl").value.trim(), max_hops: Number(byId<HTMLInputElement>("maxHops").value) }
  };
}
async function saveConfig() {
  const button = byId<HTMLButtonElement>("saveConfig"); button.disabled = true; setSaveState("Validating and saving…");
  try {
    const next = collectConfig();
    const response = await fetch("/api/config", { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify(next) });
    if (!response.ok) throw new Error((await response.text()).trim());
    currentConfig = next; dirty = false; setSaveState("Saved. Restarting node…", "success");
    await fetch("/api/reload", { method: "POST", headers: { "Content-Type": "application/json" }, body: "{}" });
    setTimeout(() => void loadConfig().catch(() => undefined), 2200);
  } catch (error) { setSaveState(error instanceof Error ? error.message : String(error), "error"); }
  finally { button.disabled = false; }
}

for (const tab of document.querySelectorAll<HTMLButtonElement>(".tab")) tab.addEventListener("click", () => {
  for (const x of document.querySelectorAll(".tab")) x.classList.toggle("active", x === tab);
  for (const page of document.querySelectorAll<HTMLElement>(".tab-page")) page.classList.toggle("active", page.dataset.page === tab.dataset.tab);
});
for (const input of document.querySelectorAll("input,textarea")) input.addEventListener("input", markDirty);
for (const button of document.querySelectorAll<HTMLButtonElement>("[data-copy]")) button.addEventListener("click", async () => {
  const target = byId(button.dataset.copy ?? ""); await navigator.clipboard.writeText(target.textContent ?? ""); const old = button.textContent; button.textContent = "Copied"; setTimeout(() => button.textContent = old, 1200);
});
byId("addPeer").addEventListener("click", () => makeRow(byId("peerEditor"), [{ key: "address", label: "Address", placeholder: "seed.example:7447", cls: "wide" }, { key: "expected", label: "Expected node ID", placeholder: "kr_…", cls: "wide" }]));
byId("addService").addEventListener("click", () => makeRow(byId("serviceEditor"), [{ key: "name", label: "Name", placeholder: "web", cls: "narrow" }, { key: "target", label: "Local target", placeholder: "127.0.0.1:8080" }, { key: "publish", label: "Publish identity", type: "checkbox", checked: true, cls: "narrow" }, { key: "intros", label: "Intro points", type: "number", value: "3", cls: "narrow" }, { key: "description", label: "Description" }, { key: "allow", label: "Direct-mode node ACL", placeholder: "*", cls: "wide" }]));
byId("addForward").addEventListener("click", () => makeRow(byId("forwardEditor"), [{ key: "listen", label: "Local listener", placeholder: "127.0.0.1:2222" }, { key: "node", label: "Remote node ID", placeholder: "kr_…", cls: "wide" }, { key: "service", label: "Service", placeholder: "ssh", cls: "narrow" }]));
byId("addAlias").addEventListener("click", () => makeRow(byId("aliasEditor"), [{ key: "name", label: "Alias", placeholder: "server", cls: "narrow" }, { key: "node", label: "Node target", placeholder: "kr_… or node.knot", cls: "wide" }, { key: "serviceId", label: "Service target", placeholder: "ks_… or service.knot", cls: "wide" }, { key: "description", label: "Description" }]));
byId("saveConfig").addEventListener("click", () => void saveConfig());
byId("restartNode").addEventListener("click", async () => { await fetch("/api/reload", { method: "POST", headers: { "Content-Type": "application/json" }, body: "{}" }); setSaveState("Restart requested", "success"); });
byId("stopNode").addEventListener("click", async () => { if (!confirm("Stop the KnotRoute node? The tray controller can start it again.")) return; await fetch("/api/shutdown", { method: "POST", headers: { "Content-Type": "application/json" }, body: "{}" }); setSaveState("Node stopped", "success"); });
window.addEventListener("beforeunload", event => { if (dirty) { event.preventDefault(); event.returnValue = ""; } });

void refresh(); void loadConfig().catch(error => setSaveState(error instanceof Error ? error.message : String(error), "error"));
setInterval(() => void refresh(), 2000);

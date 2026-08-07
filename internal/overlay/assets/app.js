"use strict";
const byId = (id) => {
    const node = document.getElementById(id);
    if (!node)
        throw new Error(`missing element #${id}`);
    return node;
};
let last = null;
const short = (id) => id && id.length > 18 ? `${id.slice(0, 10)}…${id.slice(-4)}` : id ?? "—";
const bytes = (n) => { if (!n)
    return "0 B"; const u = ["B", "KiB", "MiB", "GiB", "TiB"]; const i = Math.min(Math.floor(Math.log(n) / Math.log(1024)), u.length - 1); return `${(n / 1024 ** i).toFixed(i ? 1 : 0)} ${u[i]}`; };
const duration = (seconds) => { const n = Math.max(0, Math.floor(seconds)), d = Math.floor(n / 86400), h = Math.floor(n % 86400 / 3600), m = Math.floor(n % 3600 / 60), s = n % 60; return [[d, "d"], [h, "h"], [m, "m"], [s, "s"]].filter(([v]) => Number(v) > 0 || s === Number(v)).map(([v, u]) => `${v}${u}`).join(" "); };
const esc = (value) => String(value ?? "").replace(/[&<>'"]/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", "'": "&#39;", '"': "&quot;" }[c] ?? c));
function renderRoutes(items) {
    const root = byId("routes");
    if (!items.length) {
        root.className = "table-wrap empty";
        root.textContent = "No routes yet";
        return;
    }
    root.className = "table-wrap";
    root.innerHTML = `<table><thead><tr><th>Destination</th><th>Path</th><th>Hops</th><th>Services</th></tr></thead><tbody>${items.map(r => `<tr><td><code title="${esc(r.destination)}">${esc(r.short_id)}</code></td><td class="path">${r.path.map(short).map(esc).join(" → ")}</td><td>${r.hops}</td><td>${r.services.length ? r.services.map(x => `<span class="service-pill">${esc(x)}</span>`).join("") : "—"}</td></tr>`).join("")}</tbody></table>`;
}
function renderPeers(items) {
    const root = byId("peers");
    if (!items.length) {
        root.className = "cards empty";
        root.textContent = "No peers connected";
        return;
    }
    root.className = "cards";
    root.innerHTML = items.map(p => `<div class="card"><div class="card-top"><strong title="${esc(p.id)}">${esc(p.short_id)}</strong><span class="direction">${esc(p.direction)}</span></div><p>${esc(p.remote_addr)}${p.advertise?.length ? ` · ${esc(p.advertise.join(", "))}` : ""}</p></div>`).join("");
}
function renderEndpoints(services, forwards) {
    const root = byId("endpoints");
    const all = [...services.map(x => ({ title: x.name, sub: x.target + (x.description ? ` · ${x.description}` : ""), tag: "service", ok: true })), ...forwards.map(x => ({ title: x.listen, sub: `${short(x.node)} / ${x.service}${x.error ? ` · ${x.error}` : ""}`, tag: "forward", ok: x.active }))];
    if (!all.length) {
        root.className = "cards empty";
        root.textContent = "Nothing configured";
        return;
    }
    root.className = "cards";
    root.innerHTML = all.map(x => `<div class="card"><div class="card-top"><strong>${esc(x.title)}</strong><span class="state ${x.ok ? "ok" : "bad"}">${esc(x.tag)}</span></div><p>${esc(x.sub)}</p></div>`).join("");
}
function renderEvents(items) {
    const root = byId("events"), show = items.slice(-80).reverse();
    if (!show.length) {
        root.className = "events empty";
        root.textContent = "No events";
        return;
    }
    root.className = "events";
    root.innerHTML = show.map(e => `<div class="event"><time>${new Date(e.time).toLocaleTimeString()}</time><span class="level ${esc(e.level)}">${esc(e.level)}</span><span>${esc(e.message)}</span></div>`).join("");
}
function render(s) {
    last = s;
    byId("nodeId").textContent = s.node_id;
    byId("listen").textContent = s.listen.length ? `Listening on ${s.listen.join(", ")}` : "No listener";
    byId("peerCount").textContent = String(s.peers.length);
    byId("routeCount").textContent = String(s.routes.length);
    byId("streamCount").textContent = String(s.active_streams);
    byId("traffic").textContent = bytes(s.bytes_sent + s.bytes_received);
    byId("frames").textContent = `${s.frames_sent + s.frames_received} frames · ↑ ${bytes(s.bytes_sent)} ↓ ${bytes(s.bytes_received)}`;
    byId("uptime").textContent = duration((Date.now() - new Date(s.started_at).getTime()) / 1000);
    byId("version").textContent = `KnotRoute ${s.version}`;
    byId("serviceCount").textContent = String(s.services.length);
    byId("forwardCount").textContent = String(s.forwards.filter(x => x.active).length);
    byId("topologyBadge").textContent = `${s.routes.length} nodes`;
    renderRoutes(s.routes);
    renderPeers(s.peers);
    renderEndpoints(s.services, s.forwards);
    renderEvents(s.events);
    document.querySelector(".health").className = "health online";
    byId("healthText").textContent = "Online";
}
async function refresh() { try {
    const response = await fetch("/api/status", { cache: "no-store" });
    if (!response.ok)
        throw new Error(response.statusText);
    render(await response.json());
}
catch {
    document.querySelector(".health").className = "health offline";
    byId("healthText").textContent = "Disconnected";
} }
byId("copyId").addEventListener("click", async () => { if (!last)
    return; await navigator.clipboard.writeText(last.node_id); byId("copyId").textContent = "Copied"; setTimeout(() => byId("copyId").textContent = "Copy", 1200); });
void refresh();
setInterval(() => void refresh(), 2000);

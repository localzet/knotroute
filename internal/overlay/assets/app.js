"use strict";
const byId = (id) => {
    const node = document.getElementById(id);
    if (!node)
        throw new Error(`missing element #${id}`);
    return node;
};
let lastStatus = null;
let currentConfig = null;
let dirty = false;
const short = (id) => id && id.length > 18 ? `${id.slice(0, 10)}…${id.slice(-4)}` : id ?? "—";
const bytes = (n) => { if (!n)
    return "0 B"; const u = ["B", "KiB", "MiB", "GiB", "TiB"]; const i = Math.min(Math.floor(Math.log(n) / Math.log(1024)), u.length - 1); return `${(n / 1024 ** i).toFixed(i ? 1 : 0)} ${u[i]}`; };
const duration = (seconds) => { const n = Math.max(0, Math.floor(seconds)), d = Math.floor(n / 86400), h = Math.floor(n % 86400 / 3600), m = Math.floor(n % 3600 / 60), s = n % 60; return [[d, "д"], [h, "ч"], [m, "м"], [s, "с"]].filter(([v]) => Number(v) > 0 || s === Number(v)).map(([v, u]) => `${v}${u}`).join(" "); };
const esc = (value) => String(value ?? "").replace(/[&<>'"]/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", "'": "&#39;", '"': "&quot;" }[c] ?? c));
const lines = (value) => value.split(/\r?\n/).map(x => x.trim()).filter(Boolean);
const csv = (value) => value.split(",").map(x => x.trim()).filter(Boolean);
const asArray = (value) => Array.isArray(value) ? value : [];
const asObject = (value) => value && typeof value === "object" && !Array.isArray(value) ? value : {};
function normalizeStatus(value) {
    const raw = asObject(value);
    const proxy = asObject(raw.proxy);
    return {
        ...raw,
        listen: asArray(raw.listen),
        peers: asArray(raw.peers).map(p => ({ ...asObject(p), advertise: asArray(asObject(p).advertise) })),
        routes: asArray(raw.routes).map(r => ({ ...asObject(r), path: asArray(asObject(r).path), services: asArray(asObject(r).services) })),
        services: asArray(raw.services).map(service => ({ ...asObject(service), introduction_points: asArray(asObject(service).introduction_points) })),
        known_services: asArray(raw.known_services).map(service => ({ ...asObject(service), introduction_points: asArray(asObject(service).introduction_points), metadata: asObject(asObject(service).metadata) })),
        forwards: asArray(raw.forwards),
        aliases: asArray(raw.aliases),
        events: asArray(raw.events),
        proxy: { ...proxy, listeners: asArray(proxy.listeners) },
    };
}
function setSaveState(message, state = "") {
    const bar = document.querySelector(".savebar");
    if (bar)
        bar.className = `savebar panel ${state}`;
    byId("saveState").textContent = message;
}
function markDirty() { dirty = true; setSaveState("Есть несохранённые изменения"); }
function nodeServiceDomain(service, nodeDomain) {
    return `${service}.${nodeDomain}`;
}
function renderRoutes(items) {
    const root = byId("routes");
    if (!items.length) {
        root.className = "table-wrap empty";
        root.textContent = "Маршрутов пока нет";
        return;
    }
    root.className = "table-wrap";
    root.innerHTML = `<table><thead><tr><th>Узел</th><th>Путь</th><th>Хопы</th><th>Объявленные сервисы</th></tr></thead><tbody>${items.map(r => `<tr><td><code title="${esc(r.destination)}">${esc(r.domain)}</code></td><td class="path">${r.path.map(short).map(esc).join(" → ")}</td><td>${r.hops}</td><td>${r.services.length ? r.services.map((x) => { const domain = nodeServiceDomain(x, r.domain); return `<a class="service-pill route-service-link" href="http://${esc(domain)}/" target="_blank" rel="noreferrer" title="Открыть ${esc(domain)}">${esc(x)}</a>`; }).join("") : "—"}</td></tr>`).join("")}</tbody></table>`;
}
function catalogEntries(s) {
    const entries = [];
    const seen = new Set();
    for (const route of s.routes) {
        for (const service of route.services) {
            const domain = nodeServiceDomain(service, route.domain);
            if (seen.has(domain)) continue;
            seen.add(domain);
            entries.push({ kind: "node", domain, title: service, description: `Сервис узла ${route.short_id || short(route.destination)} · ${route.hops} hop`, tags: ["node-service"], href: `http://${domain}/` });
        }
    }
    for (const item of s.known_services) {
        if (!item.domain || seen.has(item.domain)) continue;
        seen.add(item.domain);
        const metadata = asObject(item.metadata);
        const tags = String(metadata.tags || "").split(",").map(x => x.trim()).filter(Boolean);
        const scheme = String(metadata.scheme || "http").toLowerCase() === "https" ? "https" : "http";
        entries.push({ kind: "identity", domain: item.domain, title: metadata.title || metadata.name || item.domain, description: metadata.description || `Переносимая service identity · revision ${item.revision ?? "?"} · intro: ${item.introduction_points.length}`, tags, href: `${scheme}://${item.domain}/` });
    }
    return entries;
}
function renderCatalog(s) {
    const root = byId("catalog");
    const query = (byId("catalogSearch").value || "").trim().toLowerCase();
    const all = catalogEntries(s);
    const filtered = !query ? all : all.filter(x => [x.title, x.domain, x.description, ...x.tags].join(" ").toLowerCase().includes(query));
    byId("catalogCount").textContent = `${filtered.length} из ${all.length}`;
    if (!filtered.length) {
        root.className = "catalog-grid empty";
        root.textContent = all.length ? "Ничего не найдено" : "Сервисы пока не обнаружены";
        return;
    }
    root.className = "catalog-grid";
    root.innerHTML = filtered.map(x => `<article class="catalog-card"><div class="catalog-head"><strong>${esc(x.title)}</strong><span class="catalog-kind ${x.kind === "identity" ? "identity" : ""}">${x.kind === "identity" ? "service identity" : "узел"}</span></div><code>${esc(x.domain)}</code><p>${esc(x.description)}</p>${x.tags.length ? `<div class="catalog-meta">${x.tags.map(tag => `<span class="service-pill">${esc(tag)}</span>`).join("")}</div>` : ""}<div class="catalog-actions"><a class="button" href="${esc(x.href)}" target="_blank" rel="noreferrer">Открыть</a><button class="button ghost catalog-copy" type="button" data-domain="${esc(x.domain)}">Копировать адрес</button></div></article>`).join("");
    for (const button of root.querySelectorAll(".catalog-copy")) button.addEventListener("click", async () => { await navigator.clipboard.writeText(button.dataset.domain || ""); const old = button.textContent; button.textContent = "Скопировано"; setTimeout(() => button.textContent = old, 1000); });
}
function renderPeers(items) {
    const root = byId("peers");
    if (!items.length) {
        root.className = "cards empty";
        root.textContent = "Нет подключённых пиров";
        return;
    }
    root.className = "cards";
    root.innerHTML = items.map(p => `<div class="card"><div class="card-top"><strong title="${esc(p.id)}">${esc(p.short_id)}</strong><span class="direction">${esc(p.direction === "inbound" ? "входящий" : p.direction === "outbound" ? "исходящий" : p.direction)}</span></div><p>${esc(p.remote_addr)}${p.advertise?.length ? ` · ${esc(p.advertise.join(", "))}` : ""}</p></div>`).join("");
}
function renderEndpoints(services, forwards) {
    const root = byId("endpoints");
    const all = [...services.map(x => ({ title: x.domain || x.name, sub: `${x.name} → ${x.target}${x.description ? ` · ${x.description}` : ""}${x.introduction_points?.length ? ` · introduction points: ${x.introduction_points.length}` : ""}`, tag: x.published ? "скрытый сервис" : "прямой сервис", ok: true })), ...forwards.map(x => ({ title: x.listen, sub: `${short(x.node)} / ${x.service}${x.error ? ` · ${x.error}` : ""}`, tag: "проброс", ok: x.active }))];
    if (!all.length) {
        root.className = "cards empty";
        root.textContent = "Ничего не настроено";
        return;
    }
    root.className = "cards";
    root.innerHTML = all.map(x => `<div class="card"><div class="card-top"><strong>${esc(x.title)}</strong><span class="state ${x.ok ? "ok" : "bad"}">${esc(x.tag)}</span></div><p>${esc(x.sub)}</p></div>`).join("");
}
function renderEvents(items) {
    const root = byId("events"), show = items.slice(-80).reverse();
    if (!show.length) {
        root.className = "events empty";
        root.textContent = "Событий пока нет";
        return;
    }
    root.className = "events";
    root.innerHTML = show.map(e => `<div class="event"><time>${new Date(e.time).toLocaleTimeString()}</time><span class="level ${esc(e.level)}">${esc(e.level)}</span><span>${esc(e.message)}</span></div>`).join("");
}
function renderStatus(s) {
    lastStatus = s;
    byId("domain").textContent = s.domain;
    byId("nodeId").textContent = s.node_id;
    byId("networkId").textContent = s.network_id ?? "—";
    byId("listen").textContent = s.listen.length ? `Слушает: ${s.listen.join(", ")}` : "Overlay listener отключён";
    byId("peerCount").textContent = String(s.peers.length);
    byId("routeCount").textContent = String(s.routes.length);
    byId("streamCount").textContent = `${s.active_streams} / ${s.active_circuits ?? 0}`;
    byId("traffic").textContent = bytes(s.bytes_sent + s.bytes_received);
    byId("frames").textContent = `${s.frames_sent + s.frames_received} кадров · ↑ ${bytes(s.bytes_sent)} ↓ ${bytes(s.bytes_received)}`;
    byId("uptime").textContent = duration((Date.now() - new Date(s.started_at).getTime()) / 1000);
    byId("version").textContent = `KnotRoute ${s.version}`;
    byId("serviceCount").textContent = String(s.services.filter((x) => x.published).length);
    byId("descriptorCount").textContent = `идентичности сервисов · дескрипторов: ${s.descriptors ?? 0}`;
    byId("gatewayCount").textContent = String(s.proxy.listeners.length);
    byId("topologyBadge").textContent = `${s.routes.length} узлов`;
    byId("socksAddress").textContent = s.proxy.socks ? `socks5://${s.proxy.socks}` : "выключено";
    byId("httpAddress").textContent = s.proxy.http ? `http://${s.proxy.http}` : "выключено";
    byId("pacAddress").textContent = s.proxy.pac || "выключено";
    renderRoutes(s.routes);
    renderPeers(s.peers);
    renderEndpoints(s.services, s.forwards);
    renderCatalog(s);
    renderEvents(s.events);
    const health = document.querySelector(".health");
    if (health) {
        health.className = "health online";
        health.title = s.peers.length ? `Подключено пиров: ${s.peers.length}` : "Локальный узел работает, но активных пиров пока нет";
    }
    byId("healthText").textContent = s.peers.length ? "Сеть подключена" : "Узел работает · нет пиров";
}
async function refresh() {
    let payload;
    try {
        const response = await fetch("/api/status", { cache: "no-store" });
        if (!response.ok)
            throw new Error(`HTTP ${response.status} ${response.statusText}`.trim());
        payload = await response.json();
    }
    catch (error) {
        const detail = error instanceof Error ? error.message : String(error);
        const health = document.querySelector(".health");
        if (health) {
            health.className = "health offline";
            health.title = detail;
        }
        byId("healthText").textContent = "Локальный узел недоступен";
        const previous = Array.isArray(lastStatus?.events) ? lastStatus.events : [];
        renderEvents([...previous, {
                time: new Date().toISOString(),
                level: "error",
                message: `Панель не может получить /api/status: ${detail}. Проверьте %LOCALAPPDATA%\\KnotRoute\\knotroute.log и desktop.log.`,
            }]);
        return;
    }
    try {
        renderStatus(normalizeStatus(payload));
    }
    catch (error) {
        const detail = error instanceof Error ? error.message : String(error);
        const status = normalizeStatus(payload);
        lastStatus = status;
        const health = document.querySelector(".health");
        if (health) {
            health.className = "health offline";
            health.title = `Узел отвечает, но панель не смогла отобразить status: ${detail}`;
        }
        byId("healthText").textContent = "Узел отвечает · ошибка интерфейса";
        renderEvents([...status.events, {
                time: new Date().toISOString(),
                level: "error",
                message: `Ошибка отображения /api/status: ${detail}. Это ошибка панели, а не признак завершения knotroute.exe.`,
            }]);
    }
}
function makeRow(root, fields) {
    const row = document.createElement("div");
    row.className = "editor-row";
    const advanced = fields.filter(field => field.advanced);
    const renderField = (field, parent) => {
        const label = document.createElement("label");
        if (field.cls)
            label.className = field.cls;
        const span = document.createElement("span");
        span.textContent = field.label;
        const input = document.createElement("input");
        input.dataset.key = field.key;
        input.type = field.type ?? "text";
        if (input.type === "checkbox")
            input.checked = field.checked ?? false;
        else
            input.value = field.value ?? "";
        input.placeholder = field.placeholder ?? "";
        input.addEventListener("input", markDirty);
        input.addEventListener("change", markDirty);
        label.append(span, input);
        parent.append(label);
    };
    for (const field of fields.filter(field => !field.advanced))
        renderField(field, row);
    if (advanced.length) {
        const details = document.createElement("details");
        details.className = "row-advanced";
        const summary = document.createElement("summary");
        summary.textContent = "Дополнительно";
        details.append(summary);
        const body = document.createElement("div");
        body.className = "row-advanced-body";
        for (const field of advanced)
            renderField(field, body);
        details.append(body);
        row.append(details);
    }
    const remove = document.createElement("button");
    remove.className = "remove";
    remove.type = "button";
    remove.textContent = "×";
    remove.title = "Удалить";
    remove.addEventListener("click", () => { row.remove(); markDirty(); });
    row.append(remove);
    root.append(row);
}
function value(row, key) { return (row.querySelector(`[data-key="${key}"]`)?.value ?? "").trim(); }
function checked(row, key) { return row.querySelector(`[data-key="${key}"]`)?.checked ?? false; }
function rows(id) { return [...byId(id).querySelectorAll(".editor-row")]; }
function renderConfig(cfg) {
    currentConfig = cfg;
    const peerRoot = byId("peerEditor");
    peerRoot.replaceChildren();
    for (const p of cfg.peers ?? [])
        makeRow(peerRoot, [{ key: "address", label: "Адрес", value: p.address, placeholder: "seed.example:7447", cls: "wide" }, { key: "expected", label: "Ожидаемый ID узла", value: p.expected_id, placeholder: "kr_…", cls: "wide", advanced: true }]);
    const serviceRoot = byId("serviceEditor");
    serviceRoot.replaceChildren();
    for (const s of cfg.services ?? [])
        makeRow(serviceRoot, [{ key: "name", label: "Имя", value: s.name, placeholder: "web", cls: "narrow" }, { key: "target", label: "Локальная цель", value: s.target, placeholder: "127.0.0.1:8080" }, { key: "publish", label: "Опубликовать identity", type: "checkbox", checked: s.publish ?? false, cls: "narrow" }, { key: "intros", label: "Точек входа", type: "number", value: String(s.intro_count ?? 3), cls: "narrow", advanced: true }, { key: "description", label: "Описание", value: s.description, advanced: true }, { key: "allow", label: "ACL узлов (прямой режим)", value: (s.allow ?? []).join(","), placeholder: "*", cls: "wide", advanced: true }]);
    const forwardRoot = byId("forwardEditor");
    forwardRoot.replaceChildren();
    for (const f of cfg.forwards ?? [])
        makeRow(forwardRoot, [{ key: "listen", label: "Локальный listener", value: f.listen, placeholder: "127.0.0.1:2222" }, { key: "node", label: "ID удалённого узла", value: f.node, placeholder: "kr_…", cls: "wide" }, { key: "service", label: "Сервис", value: f.service, placeholder: "ssh", cls: "narrow" }]);
    const aliasRoot = byId("aliasEditor");
    aliasRoot.replaceChildren();
    for (const a of cfg.aliases ?? [])
        makeRow(aliasRoot, [{ key: "name", label: "Алиас", value: a.name, placeholder: "server", cls: "narrow" }, { key: "node", label: "Целевой узел", value: a.node, placeholder: "kr_… or node.knot", cls: "wide" }, { key: "serviceId", label: "Целевой сервис", value: a.service_id, placeholder: "ks_… or service.knot", cls: "wide" }, { key: "description", label: "Описание", value: a.description, advanced: true }]);
    byId("listenInput").value = cfg.listen.join("\n");
    byId("advertiseInput").value = (cfg.advertise ?? []).join("\n");
    byId("networkIdInput").value = cfg.network_id;
    byId("discoveryEnabled").checked = cfg.discovery.enabled;
    byId("discoveryLan").checked = cfg.discovery.lan;
    byId("discoveryPex").checked = cfg.discovery.peer_exchange;
    byId("discoveryInterval").value = cfg.discovery.interval;
    byId("beaconInput").value = (cfg.discovery.beacons ?? []).join("\n");
    byId("circuitHops").value = String(cfg.privacy.circuit_hops);
    byId("circuitTimeout").value = cfg.privacy.circuit_timeout;
    byId("directoryReplicas").value = String(cfg.directory.replicas);
    byId("descriptorTtl").value = cfg.directory.descriptor_ttl;
    byId("descriptorPublish").value = cfg.directory.publish_interval;
    byId("descriptorLookup").value = cfg.directory.lookup_timeout;
    byId("caEnabled").checked = cfg.ca.enabled;
    byId("caIntercept").checked = cfg.ca.intercept_https;
    byId("caDirectory").value = cfg.ca.directory;
    byId("proxySocks").value = cfg.proxy.socks;
    byId("proxyHttp").value = cfg.proxy.http;
    byId("defaultHttp").value = cfg.proxy.default_http_service;
    byId("defaultHttps").value = cfg.proxy.default_https_service;
    byId("proxyDirect").checked = cfg.proxy.direct;
    byId("dashboardInput").value = cfg.dashboard;
    byId("maxHops").value = String(cfg.routing.max_hops);
    byId("lsaInterval").value = cfg.routing.lsa_interval;
    byId("lsaTtl").value = cfg.routing.lsa_ttl;
    dirty = false;
    setSaveState("Конфигурация загружена", "success");
}
async function loadConfig() {
    const response = await fetch("/api/config", { cache: "no-store" });
    if (!response.ok)
        throw new Error(await response.text());
    renderConfig(await response.json());
}
function collectConfig() {
    if (!currentConfig)
        throw new Error("конфигурация ещё не загружена");
    return {
        ...currentConfig,
        listen: lines(byId("listenInput").value),
        advertise: lines(byId("advertiseInput").value),
        peers: rows("peerEditor").map(row => ({ address: value(row, "address"), ...(value(row, "expected") ? { expected_id: value(row, "expected") } : {}) })).filter(x => x.address),
        network_id: byId("networkIdInput").value.trim(),
        services: rows("serviceEditor").map(row => ({ name: value(row, "name"), target: value(row, "target"), publish: checked(row, "publish"), intro_count: Number(value(row, "intros") || "3"), ...(value(row, "description") ? { description: value(row, "description") } : {}), ...(csv(value(row, "allow")).length ? { allow: csv(value(row, "allow")) } : {}) })).filter(x => x.name || x.target),
        forwards: rows("forwardEditor").map(row => ({ listen: value(row, "listen"), node: value(row, "node"), service: value(row, "service") })).filter(x => x.listen || x.node || x.service),
        aliases: rows("aliasEditor").map(row => ({ name: value(row, "name"), ...(value(row, "node") ? { node: value(row, "node") } : {}), ...(value(row, "serviceId") ? { service_id: value(row, "serviceId") } : {}), ...(value(row, "description") ? { description: value(row, "description") } : {}) })).filter(x => x.name || x.node || x.service_id),
        discovery: { ...currentConfig.discovery, enabled: byId("discoveryEnabled").checked, lan: byId("discoveryLan").checked, peer_exchange: byId("discoveryPex").checked, interval: byId("discoveryInterval").value.trim(), beacons: lines(byId("beaconInput").value) },
        privacy: { circuit_hops: Number(byId("circuitHops").value), circuit_timeout: byId("circuitTimeout").value.trim() },
        directory: { replicas: Number(byId("directoryReplicas").value), descriptor_ttl: byId("descriptorTtl").value.trim(), publish_interval: byId("descriptorPublish").value.trim(), lookup_timeout: byId("descriptorLookup").value.trim() },
        ca: { enabled: byId("caEnabled").checked, directory: byId("caDirectory").value.trim(), intercept_https: byId("caIntercept").checked },
        dashboard: byId("dashboardInput").value.trim(),
        proxy: { socks: byId("proxySocks").value.trim(), http: byId("proxyHttp").value.trim(), direct: byId("proxyDirect").checked, default_http_service: byId("defaultHttp").value.trim(), default_https_service: byId("defaultHttps").value.trim() },
        routing: { lsa_interval: byId("lsaInterval").value.trim(), lsa_ttl: byId("lsaTtl").value.trim(), max_hops: Number(byId("maxHops").value) }
    };
}
async function saveConfig() {
    const button = byId("saveConfig");
    button.disabled = true;
    setSaveState("Проверка и сохранение…");
    try {
        const next = collectConfig();
        const response = await fetch("/api/config", { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify(next) });
        if (!response.ok)
            throw new Error((await response.text()).trim());
        currentConfig = next;
        dirty = false;
        setSaveState("Сохранено. Перезапускаю узел…", "success");
        await fetch("/api/reload", { method: "POST", headers: { "Content-Type": "application/json" }, body: "{}" });
        setTimeout(() => void loadConfig().catch(() => undefined), 2200);
    }
    catch (error) {
        setSaveState(error instanceof Error ? error.message : String(error), "error");
    }
    finally {
        button.disabled = false;
    }
}
for (const tab of document.querySelectorAll(".tab"))
    tab.addEventListener("click", () => {
        for (const x of document.querySelectorAll(".tab"))
            x.classList.toggle("active", x === tab);
        for (const page of document.querySelectorAll(".tab-page"))
            page.classList.toggle("active", page.dataset.page === tab.dataset.tab);
    });
for (const input of document.querySelectorAll("input,textarea"))
    input.addEventListener("input", markDirty);
for (const button of document.querySelectorAll("[data-copy]"))
    button.addEventListener("click", async () => {
        const target = byId(button.dataset.copy ?? "");
        await navigator.clipboard.writeText(target.textContent ?? "");
        const old = button.textContent;
        button.textContent = "Скопировано";
        setTimeout(() => button.textContent = old, 1200);
    });
byId("addPeer").addEventListener("click", () => makeRow(byId("peerEditor"), [{ key: "address", label: "Адрес", placeholder: "seed.example:7447", cls: "wide" }, { key: "expected", label: "Ожидаемый ID узла", placeholder: "kr_…", cls: "wide", advanced: true }]));
byId("addService").addEventListener("click", () => makeRow(byId("serviceEditor"), [{ key: "name", label: "Имя", placeholder: "web", cls: "narrow" }, { key: "target", label: "Локальная цель", placeholder: "127.0.0.1:8080" }, { key: "publish", label: "Опубликовать identity", type: "checkbox", checked: true, cls: "narrow" }, { key: "intros", label: "Точек входа", type: "number", value: "3", cls: "narrow", advanced: true }, { key: "description", label: "Описание", advanced: true }, { key: "allow", label: "ACL узлов (прямой режим)", placeholder: "*", cls: "wide", advanced: true }]));
byId("addForward").addEventListener("click", () => makeRow(byId("forwardEditor"), [{ key: "listen", label: "Локальный listener", placeholder: "127.0.0.1:2222" }, { key: "node", label: "ID удалённого узла", placeholder: "kr_…", cls: "wide" }, { key: "service", label: "Сервис", placeholder: "ssh", cls: "narrow" }]));
byId("addAlias").addEventListener("click", () => makeRow(byId("aliasEditor"), [{ key: "name", label: "Алиас", placeholder: "server", cls: "narrow" }, { key: "node", label: "Целевой узел", placeholder: "kr_… or node.knot", cls: "wide" }, { key: "serviceId", label: "Целевой сервис", placeholder: "ks_… or service.knot", cls: "wide" }, { key: "description", label: "Описание", advanced: true }]));
byId("catalogSearch").addEventListener("input", () => { if (lastStatus) renderCatalog(lastStatus); });
byId("saveConfig").addEventListener("click", () => void saveConfig());
byId("restartNode").addEventListener("click", async () => { await fetch("/api/reload", { method: "POST", headers: { "Content-Type": "application/json" }, body: "{}" }); setSaveState("Перезапуск запрошен", "success"); });
byId("stopNode").addEventListener("click", async () => { if (!confirm("Остановить узел KnotRoute? Его можно снова запустить из приложения в трее."))
    return; await fetch("/api/shutdown", { method: "POST", headers: { "Content-Type": "application/json" }, body: "{}" }); setSaveState("Узел остановлен", "success"); });
window.addEventListener("beforeunload", event => { if (dirty) {
    event.preventDefault();
    event.returnValue = "";
} });
void refresh();
void loadConfig().catch(error => setSaveState(error instanceof Error ? error.message : String(error), "error"));
setInterval(() => void refresh(), 2000);

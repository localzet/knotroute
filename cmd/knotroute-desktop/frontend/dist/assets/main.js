import { bridge } from './bridge.js';
import { icon } from './icons.js';
const state = {
    snapshot: null, page: 'home', activeContact: '', catalogQuery: '', busy: false,
    preview: new URLSearchParams(location.search).has('preview') || !window.go, pendingSnapshot: null
};
const root = document.getElementById('app');
const esc = (v) => String(v ?? '').replace(/[&<>'"]/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;' }[c]));
const short = (v, n = 9) => v.length > n * 2 + 1 ? `${v.slice(0, n)}…${v.slice(-n)}` : v;
const time = (unix) => unix ? new Date(unix * 1000).toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' }) : '';
const dateTime = (unix) => unix ? new Date(unix * 1000).toLocaleString('ru-RU', { day: '2-digit', month: 'short', hour: '2-digit', minute: '2-digit' }) : '';
const bytes = (n) => n < 1024 ? `${n} Б` : n < 1048576 ? `${(n / 1024).toFixed(1)} КБ` : `${(n / 1048576).toFixed(1)} МБ`;
const nav = [
    ['home', 'Главная', 'home'], ['chats', 'Чаты', 'chats'], ['feed', 'Лента', 'feed'], ['catalog', 'Каталог', 'catalog'], ['internet', 'Интернет', 'globe'], ['network', 'Сеть', 'network'], ['developer', 'Разработчикам', 'code'], ['settings', 'Настройки', 'settings']
];
function previewSnapshot() {
    const now = Math.floor(Date.now() / 1000);
    return {
        version: '4.0.0-alpha.3', running: true, integration: true, startup: true,
        overview: { state: 'online', state_detail: 'Связь работает · 3 соседа · 8 маршрутов', peer_count: 3, route_count: 8, service_count: 12, transport: 'direct', node_domain: 'agyamy7uflycwefpg4uwakhimcmfkk6ensdqbbtvlvdsvdgffbrestt3.knot', network_id: 'kn_ptfefojtpg5izk6csbodjczvkgz5j3lba5ebace4b6kso6cpbdsq', bytes_sent: 128442, bytes_recv: 512930 },
        profile: { id: 'ku_bgop6mixbugj67afz7r2vplk4uikqujntg5paqa', display_name: 'Gervase', bio: 'Связь, проекты и немного хаоса.' },
        contacts: [
            { id: 'ku_friend1', node: 'kr_lgb5regpswlz7j34uudo3k4nojeudpmtin7p4xkzdk3rff2z6kfa', display_name: 'Алексей', alias: 'alex', bio: 'в сети', last_message: 'Ну вот теперь выглядит уже интереснее', last_unix: now - 120, messages: [{ id: 'm1', mine: false, body: 'Как там твоя сеть?', unix: now - 620, sender_id: 'ku_friend1' }, { id: 'm2', mine: true, body: 'Переделываю клиент. Теперь это уже не админка :)', unix: now - 510, sender_id: 'me' }, { id: 'm3', mine: false, body: 'Ну вот теперь выглядит уже интереснее', unix: now - 120, sender_id: 'ku_friend1' }] },
            { id: 'ku_friend2', node: 'kr_eisi6pw76ycvf7wcydsyqcfvcggg2uhvwso6jkl3vcntadfgkqea', display_name: 'Иван', bio: 'через Xray', last_message: 'Закинул ссылку', last_unix: now - 3600, messages: [] }
        ],
        posts: [
            { id: 'p1', author_id: 'ku_friend1', author: 'Алексей', text: 'Проверяю ленту в KnotRoute. Если это дошло без центрального сервера — уже неплохо.', tags: ['knotroute', 'test'], unix: now - 900, mine: false },
            { id: 'p2', author_id: 'me', author: 'Gervase', text: 'v4 становится приложением, а не сетевой панелью. Следом — звонки и нормальный transport manager.', tags: ['v4', 'dev'], unix: now - 5200, mine: true }
        ],
        catalog: [
            { kind: 'identity', title: 'Localzet', description: 'Основной сайт и проекты Localzet в KnotRoute.', domain: 'all4l7wsjnjm6tw5xb6p77el3o2twwpmuhtfrl5tqfrrmy774mtw3zhu.knot', url: 'https://all4l7wsjnjm6tw5xb6p77el3o2twwpmuhtfrl5tqfrrmy774mtw3zhu.knot/', tags: ['software', 'development'] },
            { kind: 'node', title: 'Flowsint', description: 'Тестовый frontend на соседнем узле.', domain: 'web.aercjdz6373akux6ylaolcaiwuiyy3kq6w2j3zfjpoujwmamuzkaqmhc.knot', url: 'https://web.aercjdz6373akux6ylaolcaiwuiyy3kq6w2j3zfjpoujwmamuzkaqmhc.knot/', tags: ['node-service'], hops: 2 },
            { kind: 'identity', title: 'KnotDrop', description: 'Обмен файлами между участниками сети.', domain: 'adrop2otw3zhu7e4y5i6u7o8p9q0asfghjklzxcvbnmqwertyuio.knot', url: 'https://adrop2otw3zhu7e4y5i6u7o8p9q0asfghjklzxcvbnmqwertyuio.knot/', tags: ['files', 'utility'] }
        ],
        network: { network_id: 'kn_ptfefojtpg5izk6csbodjczvkgz5j3lba5ebace4b6kso6cpbdsq', beacons: ['https://beacon.localzet.com'], circuit_hops: 3, transport_mode: 'auto', direct_first: true, xray_socks: '127.0.0.1:10808', peers: [{ id: 'kr_lgb5regpswlz7j34uudo3k4nojeudpmtin7p4xkzdk3rff2z6kfa', short_id: 'kr_lgb5reg…6kfa', direction: 'outbound', remote_addr: '198.18.0.25:7447', advertise: ['beacon.localzet.com:7447'] }], routes: [{ destination: 'kr_eisi6...', domain: 'aercjdz6373akux6ylaolcaiwuiyy3kq6w2j3zfjpoujwmamuzkaqmhc.knot', short_id: 'kr_eisi6pw…kqea', next_hop: 'kr_lgb5reg...', hops: 2, path: ['kr_me', 'kr_lgb', 'kr_eisi'], services: ['web'] }], transport_attempts: [] },
        developer: { node_id: 'kr_wadgh5bk6avrblzxffqcr2datbksxrdmq4aim5k5i4vizrjimjeq', node_domain: 'agyamy7uflycwefpg4uwakhimcmfkk6ensdqbbtvlvdsvdgffbrestt3.knot', listen: ['[::]:7447'], services: [{ name: 'flowsint', target: '127.0.0.1:5173', description: 'Local preview', published: true, service_id: 'ks_qfefh3skycutsd6bzbcsajapw7kxpgtugyk3lwvj6fwqdckbg5bq', domain: 'akauqu7ojlaksoipyheekibeb635k542oq3blno2vhyw2amjie3uheq3.knot', introduction_points: ['kr_lgb...'] }], known_services: [], aliases: [], forwards: [], events: [{ time: new Date().toISOString(), level: 'info', message: 'auto-peer kr_lgb5reg…6kfa connected via beacon.localzet.com:7447' }], active_streams: 1, active_circuits: 2, descriptors: 4 },
        ca: { enabled: true, intercept_https: true, validity_days: 3650, common_name: 'Localzet KnotRoute Root CA', organization: ['Localzet'], organizational_unit: ['KnotRoute Network'], country: ['RU'], province: ['Moscow'], locality: ['Moscow'], street_address: [], postal_code: [], fingerprint: '67:8A:39:24:AB:EF:10:91:22:74:DD', subject: 'CN=Localzet KnotRoute Root CA, O=Localzet, OU=KnotRoute Network, C=RU', issuer: 'CN=Localzet KnotRoute Root CA, O=Localzet, OU=KnotRoute Network, C=RU', not_after: '2036-08-08T00:00:00Z' },
        paths: { data: 'C:\\Users\\user\\AppData\\Local\\KnotRoute', config: 'C:\\Users\\user\\AppData\\Local\\KnotRoute\\knotroute.json', identity: 'C:\\Users\\user\\AppData\\Local\\KnotRoute\\identity.json', user_identity: 'C:\\Users\\user\\AppData\\Local\\KnotRoute\\user-identity.json', social: 'C:\\Users\\user\\AppData\\Local\\KnotRoute\\social.json', ca: 'C:\\Users\\user\\AppData\\Local\\KnotRoute\\ca' }
    };
}
function statusPill(s) {
    const label = s.overview.state === 'online' ? 'Связь работает' : s.overview.state === 'starting' ? 'Подключаемся' : 'Сеть остановлена';
    return `<div class="status-pill ${s.overview.state}"><span class="status-dot"></span><span>${esc(label)}</span></div>`;
}
function avatar(name, cls = '') { const letters = name.trim().split(/\s+/).slice(0, 2).map(x => x[0]).join('').toUpperCase() || 'K'; return `<div class="avatar ${cls}">${esc(letters)}</div>`; }
function shell(s, body) {
    const navHtml = nav.map(([page, label, ic]) => `<button class="nav-item ${state.page === page ? 'active' : ''}" data-page="${page}">${icon(ic, 18)}<span>${label}</span>${page === 'chats' && s.contacts.length ? `<span class="nav-badge">${s.contacts.length}</span>` : ''}</button>`).join('');
    return `<div class="app-shell">
    <aside class="sidebar">
      <div class="brand"><div class="brand-mark">K</div><div><strong>KnotRoute</strong><span>связь, которая находит дорогу</span></div></div>
      <nav class="nav">${navHtml}</nav>
      <div class="sidebar-bottom">${statusPill(s)}<div class="version">v${esc(s.version || 'dev')}</div></div>
    </aside>
    <main class="content"><div class="content-inner">${body}</div></main>
    ${s.operation ? `<div class="operation"><span class="spinner"></span>${esc(s.operation)}</div>` : ''}
  </div>`;
}
function pageHeader(title, subtitle, actions = '') { return `<header class="page-header"><div><h1>${title}</h1><p>${subtitle}</p></div><div class="page-actions">${actions}</div></header>`; }
function button(label, action, kind = 'secondary', ic = '') { return `<button class="btn ${kind}" data-action="${action}">${ic ? icon(ic, 16) : ''}<span>${label}</span></button>`; }
function copyButton(value) { return `<button class="icon-btn" data-copy="${esc(value)}" title="Копировать">${icon('copy', 15)}</button>`; }
function home(s) {
    const c = s.contacts[0];
    const p = s.posts[0];
    const heroState = s.overview.state === 'online' ? 'online' : s.overview.state === 'starting' ? 'starting' : 'offline';
    return `${pageHeader('Главная', 'Состояние связи и то, что важно прямо сейчас', statusPill(s))}
    <section class="hero-card ${heroState}">
      <div class="hero-icon">${icon('shield', 26)}</div>
      <div class="hero-copy"><span class="eyebrow">СОЕДИНЕНИЕ</span><h2>${esc(s.overview.state === 'online' ? 'Всё работает' : s.overview.state === 'starting' ? 'Ищем маршрут' : 'Сеть остановлена')}</h2><p>${esc(s.overview.state_detail)}</p></div>
      <div class="hero-actions">${button(s.running ? 'Остановить' : 'Запустить', 'toggle-network', s.running ? 'ghost' : 'primary')}</div>
    </section>
    <div class="metric-row">
      <div class="metric"><span>Соседи</span><strong>${s.overview.peer_count}</strong><small>прямые соединения</small></div>
      <div class="metric"><span>Маршруты</span><strong>${s.overview.route_count}</strong><small>доступно сейчас</small></div>
      <div class="metric"><span>Транспорт</span><strong class="metric-word">${esc(s.overview.transport || '—')}</strong><small>выбран автоматически</small></div>
      <div class="metric"><span>Трафик</span><strong class="metric-word">${bytes(s.overview.bytes_recv)}</strong><small>получено · ${bytes(s.overview.bytes_sent)} отправлено</small></div>
    </div>
    <div class="home-grid">
      <section class="surface continue-card"><div class="section-head"><div><span class="eyebrow">ПРОДОЛЖИТЬ</span><h3>${c ? 'Последний диалог' : 'Чаты'}</h3></div>${button('Все чаты', 'goto-chats', 'text', 'arrow')}</div>
        ${c ? `<button class="conversation-preview" data-contact="${esc(c.id)}">${avatar(c.display_name)}<div><strong>${esc(c.display_name)}</strong><span>${esc(c.last_message || 'Начните разговор')}</span></div><time>${time(c.last_unix)}</time></button>` : `<div class="empty-small">Добавьте контакт — он появится здесь.</div>`}
      </section>
      <section class="surface quick-card"><div class="section-head"><div><span class="eyebrow">БЫСТРЫЙ ДОСТУП</span><h3>Возможности</h3></div></div>
        <div class="quick-grid"><button data-page="catalog">${icon('catalog', 20)}<span>Каталог</span><small>.knot-сервисы</small></button><button data-page="chats">${icon('chats', 20)}<span>Новый чат</span><small>по identity</small></button><button data-page="internet">${icon('globe', 20)}<span>Интернет</span><small>автомаршрут</small></button><button data-page="network">${icon('network', 20)}<span>Сеть</span><small>${s.overview.peer_count} соседей</small></button></div>
      </section>
      <section class="surface feed-preview"><div class="section-head"><div><span class="eyebrow">ЛЕНТА</span><h3>Последнее</h3></div>${button('Открыть ленту', 'goto-feed', 'text', 'arrow')}</div>
        ${p ? postCard(p, true) : `<div class="empty-small">В ленте пока тихо.</div>`}
      </section>
      <section class="surface identity-card"><span class="eyebrow">ВАША IDENTITY</span><div class="identity-main">${avatar(s.profile.display_name, 'large')}<div><h3>${esc(s.profile.display_name)}</h3><p>${esc(s.profile.bio || 'Добавьте пару слов о себе в настройках.')}</p></div></div><div class="identity-id"><code>${esc(short(s.profile.id, 14))}</code>${copyButton(s.profile.id)}</div></section>
    </div>`;
}
function chats(s) {
    const active = s.contacts.find(c => c.id === state.activeContact) || s.contacts[0];
    if (active)
        state.activeContact = active.id;
    const list = s.contacts.map(c => `<button class="contact-row ${active?.id === c.id ? 'active' : ''}" data-contact="${esc(c.id)}">${avatar(c.display_name)}<div class="contact-main"><div><strong>${esc(c.display_name)}</strong><time>${time(c.last_unix)}</time></div><span>${esc(c.last_message || c.bio || 'Нет сообщений')}</span></div></button>`).join('');
    const messages = active?.messages.map(m => `<div class="message-row ${m.mine ? 'mine' : ''}"><div class="bubble"><div>${esc(m.body)}</div><time>${time(m.unix)}</time></div></div>`).join('') || '';
    return `${pageHeader('Чаты', 'Личные сообщения поверх KnotRoute circuits', button('Добавить контакт', 'open-add-contact', 'primary', 'plus'))}
  <section class="messenger surface">
    <aside class="chat-list"><div class="search-box">${icon('catalog', 16)}<input id="chat-search" placeholder="Поиск по чатам" /></div><div class="contact-list">${list || '<div class="empty-small pad">Контактов пока нет</div>'}</div></aside>
    <div class="chat-pane">${active ? `<header class="chat-head">${avatar(active.display_name)}<div><strong>${esc(active.display_name)}</strong><span>${esc(active.alias ? `@${active.alias} · ` : '')}${esc(short(active.node, 12))}</span></div><button class="icon-btn">${icon('more', 18)}</button></header><div class="message-list">${messages || '<div class="empty-center"><strong>Начните разговор</strong><span>Сообщения подписываются вашей ku_ identity.</span></div>'}</div><form class="composer" id="message-form"><textarea id="message-input" rows="1" placeholder="Сообщение…"></textarea><button class="send-btn" title="Отправить">${icon('send', 19)}</button></form>` : `<div class="empty-center"><div class="empty-icon">${icon('chats', 26)}</div><strong>Выберите чат</strong><span>Или добавьте первый контакт по его node address.</span></div>`}</div>
  </section>`;
}
function postCard(p, compact = false) { return `<article class="post-card ${compact ? 'compact' : ''}"><div class="post-head">${avatar(p.author)}<div><strong>${esc(p.author)}</strong><span>${dateTime(p.unix)}</span></div>${p.mine ? '<span class="soft-badge">ваш пост</span>' : ''}</div><p>${esc(p.text)}</p>${p.tags.length ? `<div class="tags">${p.tags.map(t => `<span>#${esc(t)}</span>`).join('')}</div>` : ''}</article>`; }
function feed(s) { return `${pageHeader('Лента', 'Подписанные публикации ваших контактов', button('Обновить контакты', 'refresh-all-feed', 'secondary'))}<div class="feed-layout"><section class="compose-post surface"><span class="eyebrow">НОВАЯ ПУБЛИКАЦИЯ</span><textarea id="post-text" placeholder="Что нового?" rows="4"></textarea><div class="compose-bottom"><input id="post-tags" placeholder="теги через запятую"/>${button('Опубликовать', 'publish-post', 'primary', 'send')}</div></section><section class="feed-list">${s.posts.length ? s.posts.map(p => postCard(p)).join('') : '<div class="surface empty-center tall"><strong>Лента пока пустая</strong><span>Добавьте контакты или опубликуйте первый пост.</span></div>'}</section></div>`; }
function catalog(s) { const q = state.catalogQuery.trim().toLowerCase(); const items = s.catalog.filter(x => !q || [x.title, x.description, x.domain, ...x.tags].join(' ').toLowerCase().includes(q)); return `${pageHeader('Каталог', 'Обнаруженные сервисы и опубликованные service identities')}<div class="catalog-toolbar surface"><div class="search-box large">${icon('catalog', 18)}<input id="catalog-search" value="${esc(state.catalogQuery)}" placeholder="Название, тег или .knot-адрес" /></div><span class="muted">${items.length} из ${s.catalog.length}</span></div><div class="catalog-grid">${items.map(x => `<article class="service-card surface"><div class="service-top"><div class="service-logo">${esc(x.title.slice(0, 1).toUpperCase())}</div><span class="kind-badge ${x.kind}">${x.kind === 'identity' ? 'service identity' : 'сервис узла'}</span></div><h3>${esc(x.title)}</h3><p>${esc(x.description)}</p><div class="service-domain"><code>${esc(short(x.domain, 18))}</code>${copyButton(x.domain)}</div>${x.tags.length ? `<div class="tags">${x.tags.map(t => `<span>${esc(t)}</span>`).join('')}</div>` : ''}<div class="service-actions">${button('Открыть', 'open-service primary', 'primary', 'external')}<button class="btn ghost" data-copy="${esc(x.domain)}">${icon('copy', 15)}Копировать</button></div><span class="hidden-url">${esc(x.url)}</span></article>`).join('') || '<div class="surface empty-center tall span-all"><strong>Ничего не найдено</strong><span>Каталог содержит только реально обнаруженные или добровольно опубликованные сервисы.</span></div>'}</div>`; }
function internet(s) { return `${pageHeader('Интернет', 'Один режим подключения — KnotRoute сам выбирает доступный транспорт')}<section class="internet-hero surface"><div class="internet-orbit ${s.running ? 'active' : ''}"><div>${icon('globe', 36)}</div><span></span><span></span></div><div><span class="eyebrow">АВТОМАТИЧЕСКИЙ МАРШРУТ</span><h2>${s.running ? 'Интернет защищён' : 'Маршрутизация остановлена'}</h2><p>Обычный интернет может идти напрямую или через разрешённый gateway/transport. .knot всегда остаётся внутри KnotRoute.</p></div></section><div class="settings-grid"><section class="surface setting-card"><div class="setting-row"><div><strong>.knot в браузерах</strong><span>Chrome, Edge, Firefox и другие браузеры получают PAC + локальный Root CA.</span></div><button class="switch ${s.integration ? 'on' : ''}" data-action="toggle-integration" aria-label="Переключить интеграцию"><span></span></button></div><div class="callout ${s.integration ? 'success' : ''}">${s.integration ? `${icon('shield', 17)} Интеграция включена. Открывайте https://*.knot прямо в браузере.` : `${icon('shield', 17)} Интеграция выключена. Сервисы можно открыть из каталога после включения.`}</div></section><section class="surface setting-card"><span class="eyebrow">ТРАНСПОРТ</span><h3>${esc(s.overview.transport || 'auto')}</h3><p>Режим настраивается в разделе «Сеть». Xray может использоваться как резервный SOCKS5 transport для overlay-пиров.</p>${button('Настроить сеть', 'goto-network', 'secondary', 'network')}</section></div>`; }
function network(s) { const n = s.network; return `${pageHeader('Сеть', 'Повседневные настройки подключения; низкоуровневые детали спрятаны')}<div class="network-layout"><section class="surface form-card"><div class="section-head"><div><span class="eyebrow">ОСНОВНОЕ</span><h3>Подключение</h3></div><span class="soft-badge">${s.overview.peer_count} соседей</span></div><label>Network ID<input id="network-id" value="${esc(n.network_id)}" spellcheck="false"/></label><label>Beacon URL <small>по одному на строку</small><textarea id="beacons" rows="3" spellcheck="false">${esc(n.beacons.join('\n'))}</textarea></label><div class="form-two"><label>Режим транспорта<select id="transport-mode"><option value="auto" ${n.transport_mode === 'auto' ? 'selected' : ''}>Автоматически</option><option value="direct" ${n.transport_mode === 'direct' ? 'selected' : ''}>Только direct</option><option value="proxy" ${n.transport_mode === 'proxy' ? 'selected' : ''}>Только proxy</option></select></label><label>Хопов в circuit<input id="circuit-hops" type="number" min="1" max="8" value="${n.circuit_hops}"/></label></div><details><summary>Дополнительные параметры</summary><div class="details-body"><label>Xray / SOCKS5 transport<input id="xray-socks" value="${esc(n.xray_socks)}" placeholder="127.0.0.1:10808"/></label><label class="check"><input id="direct-first" type="checkbox" ${n.direct_first ? 'checked' : ''}/><span><strong>Пробовать direct первым</strong><small>При неудаче KnotRoute перейдёт к доступному proxy transport.</small></span></label></div></details><div class="form-actions">${button('Сохранить и переподключиться', 'save-network', 'primary')}</div></section><section class="surface topology-card"><div class="section-head"><div><span class="eyebrow">СЕЙЧАС</span><h3>Маршруты</h3></div></div><div class="peer-summary"><div><span>Соседи</span><strong>${n.peers.length}</strong></div><div><span>Маршруты</span><strong>${n.routes.length}</strong></div><div><span>Transport</span><strong>${esc(s.overview.transport || '—')}</strong></div></div><div class="mini-list">${n.peers.slice(0, 5).map(p => `<div><span class="online-dot"></span><div><strong>${esc(p.short_id)}</strong><span>${esc(p.remote_addr)}</span></div><small>${esc(p.direction)}</small></div>`).join('') || '<div class="empty-small">Нет прямых соседей</div>'}</div>${button('Подробнее для разработчиков', 'goto-developer', 'text', 'arrow')}</section></div>`; }
function developer(s) { const d = s.developer; return `${pageHeader('Разработчикам', 'Identity, сервисы, маршруты и диагностика — всё техническое живёт здесь', button('Диагностика', 'diagnostics', 'secondary'))}<div class="dev-intro surface"><div>${icon('code', 24)}</div><div><strong>Эта секция не нужна для обычного использования KnotRoute.</strong><p>Здесь можно публиковать локальные сервисы, разбирать адреса, проверять circuits и диагностировать transport layer.</p></div></div><div class="dev-grid"><section class="surface dev-card"><span class="eyebrow">IDENTITY</span><h3>Узел</h3>${kv('Node ID', d.node_id, true)}${kv('Node domain', d.node_domain, true)}${kv('Listen', d.listen.join(', ') || '—')}<details><summary>Что это означает?</summary><p><code>kr_…</code> — криптографическая identity конкретного KnotRoute node. <code>node.knot</code> адресует узел, но сам по себе не является «сайтом».</p></details></section><section class="surface dev-card"><span class="eyebrow">SERVICE PLANE</span><h3>Опубликованные сервисы</h3>${d.services.length ? d.services.map(x => `<div class="dev-service"><div><strong>${esc(x.name)}</strong><span>${esc(x.target)}</span></div><code>${esc(x.domain || 'не опубликован')}</code></div>`).join('') : '<div class="empty-small">Нет локальных сервисов</div>'}<details><summary>Как работает service identity?</summary><p><code>ks_…</code> и canonical <code>*.knot</code> принадлежат сервису, а не конкретному узлу. Introduction points позволяют менять хост без смены адреса.</p></details></section><section class="surface dev-card wide"><div class="section-head"><div><span class="eyebrow">ЖУРНАЛ</span><h3>Последние события</h3></div><span class="soft-badge">circuits ${d.active_circuits}</span></div><div class="event-log">${d.events.slice(-40).reverse().map(e => `<div><time>${esc(new Date(e.time).toLocaleTimeString('ru-RU'))}</time><span class="log-level ${esc(e.level)}">${esc(e.level)}</span><p>${esc(e.message)}</p></div>`).join('') || '<div class="empty-small">Событий пока нет</div>'}</div></section><section class="surface dev-card wide"><span class="eyebrow">ПУТИ</span><h3>Файлы клиента</h3>${Object.entries(s.paths).map(([k, v]) => kv(k, v, true)).join('')}${button('Открыть папку данных', 'open-data', 'secondary')}</section></div>`; }
function kv(k, v, copy = false) { return `<div class="kv"><span>${esc(k)}</span><code>${esc(v || '—')}</code>${copy && v ? copyButton(v) : ''}</div>`; }
function settings(s) { const ca = s.ca; return `${pageHeader('Настройки', 'Профиль, запуск приложения и ваша локальная инфраструктура доверия')}<div class="settings-layout"><section class="surface form-card"><span class="eyebrow">ПРОФИЛЬ</span><h3>Как вас видят контакты</h3><label>Имя<input id="profile-name" value="${esc(s.profile.display_name)}"/></label><label>О себе<textarea id="profile-bio" rows="3">${esc(s.profile.bio)}</textarea></label><div class="identity-id"><code>${esc(short(s.profile.id, 18))}</code>${copyButton(s.profile.id)}</div>${button('Сохранить профиль', 'save-profile', 'primary')}</section><section class="surface setting-card"><span class="eyebrow">ПРИЛОЖЕНИЕ</span><h3>Поведение</h3><div class="setting-row"><div><strong>Запускать вместе с Windows</strong><span>KnotRoute будет готов до открытия браузера.</span></div><button class="switch ${s.startup ? 'on' : ''}" data-action="toggle-startup"><span></span></button></div><div class="setting-row"><div><strong>.knot в браузерах</strong><span>PAC и Root CA для текущего пользователя.</span></div><button class="switch ${s.integration ? 'on' : ''}" data-action="toggle-integration"><span></span></button></div></section><section class="surface form-card wide"><div class="section-head"><div><span class="eyebrow">ROOT CA</span><h3>Локальный центр сертификации</h3><p>Issuer self-signed Root CA совпадает с Subject. Изменения профиля применяются к новому сертификату только после ротации.</p></div><span class="soft-badge">${ca.enabled ? 'включён' : 'выключен'}</span></div><div class="form-two"><label>Common Name<input id="ca-cn" value="${esc(ca.common_name)}"/></label><label>Organization<input id="ca-o" value="${esc(ca.organization.join(', '))}"/></label><label>Organizational Unit<input id="ca-ou" value="${esc(ca.organizational_unit.join(', '))}"/></label><label>Country<input id="ca-country" value="${esc(ca.country.join(', '))}"/></label><label>State / Province<input id="ca-state" value="${esc(ca.province.join(', '))}"/></label><label>Locality<input id="ca-locality" value="${esc(ca.locality.join(', '))}"/></label></div><details><summary>Дополнительные поля и параметры</summary><div class="details-body form-two"><label>Street address<input id="ca-street" value="${esc(ca.street_address.join(', '))}"/></label><label>Postal code<input id="ca-postal" value="${esc(ca.postal_code.join(', '))}"/></label><label>Срок действия, дней<input id="ca-validity" type="number" min="30" max="7300" value="${ca.validity_days}"/></label><label class="check"><input id="ca-https" type="checkbox" ${ca.intercept_https ? 'checked' : ''}/><span><strong>HTTPS для .knot</strong><small>Выпускать локальные сертификаты для браузеров.</small></span></label></div></details>${ca.fingerprint ? `<div class="cert-info">${kv('Subject', ca.subject || '')}${kv('Issuer', ca.issuer || '')}${kv('SHA-256', ca.fingerprint || '', true)}</div>` : ''}<div class="form-actions split">${button('Сохранить профиль CA', 'save-ca', 'secondary')}${button('Установить Root CA', 'install-ca', 'secondary')}${button('Экспортировать .crt', 'export-ca', 'secondary')}${button('Перевыпустить Root CA', 'rotate-ca', 'danger')}</div></section></div>`; }
function render() { const s = state.snapshot; if (!s) {
    root.innerHTML = '<div class="boot"><div class="brand-mark large">K</div><span class="spinner"></span><strong>Запускаем KnotRoute</strong></div>';
    return;
} let body = ''; switch (state.page) {
    case 'home':
        body = home(s);
        break;
    case 'chats':
        body = chats(s);
        break;
    case 'feed':
        body = feed(s);
        break;
    case 'catalog':
        body = catalog(s);
        break;
    case 'internet':
        body = internet(s);
        break;
    case 'network':
        body = network(s);
        break;
    case 'developer':
        body = developer(s);
        break;
    case 'settings':
        body = settings(s);
        break;
} root.innerHTML = shell(s, body); if (s.error)
    toast(s.error, 'error'); }
function toast(msg, kind = 'info') { const host = document.getElementById('toast-root'); const el = document.createElement('div'); el.className = `toast ${kind}`; el.textContent = msg; host.appendChild(el); setTimeout(() => el.classList.add('show'), 10); setTimeout(() => { el.classList.remove('show'); setTimeout(() => el.remove(), 250); }, 4500); }
async function action(fn, success = '') { if (state.preview) {
    toast('Preview mode: действие не отправляется в runtime');
    return;
} if (state.busy)
    return; state.busy = true; try {
    await fn();
    if (success)
        toast(success, 'success');
}
catch (e) {
    toast(e instanceof Error ? e.message : String(e), 'error');
}
finally {
    state.busy = false;
} }
const val = (id) => document.getElementById(id)?.value ?? '';
const checked = (id) => document.getElementById(id)?.checked ?? false;
const csv = (v) => v.split(',').map(x => x.trim()).filter(Boolean);
document.addEventListener('click', e => {
    const el = e.target.closest('[data-page],[data-action],[data-contact],[data-copy]');
    if (!el)
        return;
    const page = el.dataset.page;
    if (page) {
        state.page = page;
        render();
        return;
    }
    if (el.dataset.contact) {
        state.activeContact = el.dataset.contact;
        state.page = 'chats';
        render();
        return;
    }
    if (el.dataset.copy !== undefined) {
        if (state.preview)
            navigator.clipboard?.writeText(el.dataset.copy).catch(() => { });
        else
            void action(() => bridge.copy(el.dataset.copy));
        toast('Скопировано', 'success');
        return;
    }
    const a = el.dataset.action;
    const s = state.snapshot;
    if (a === 'goto-chats') {
        state.page = 'chats';
        render();
    }
    else if (a === 'goto-feed') {
        state.page = 'feed';
        render();
    }
    else if (a === 'goto-network') {
        state.page = 'network';
        render();
    }
    else if (a === 'goto-developer') {
        state.page = 'developer';
        render();
    }
    else if (a === 'toggle-network')
        void action(async () => { state.snapshot = await bridge.toggleNetwork(); render(); });
    else if (a === 'toggle-integration') {
        const enabling = !s.integration;
        if (enabling && !confirm('Включить .knot в браузерах? KnotRoute установит локальный Root CA для текущего пользователя и настроит PAC. Это позволит открывать HTTPS .knot в обычных браузерах.'))
            return;
        void action(async () => { state.snapshot = await bridge.browserIntegration(enabling); render(); }, enabling ? '.knot включён в браузерах' : 'Интеграция отключена');
    }
    else if (a === 'toggle-startup')
        void action(async () => { state.snapshot = await bridge.startup(!s.startup); render(); });
    else if (a === 'open-add-contact')
        openContactDialog();
    else if (a === 'publish-post') {
        const text = val('post-text');
        const tags = csv(val('post-tags'));
        void action(async () => { state.snapshot = await bridge.createPost(text, tags); render(); }, 'Опубликовано');
    }
    else if (a === 'refresh-all-feed')
        void action(async () => { for (const c of s.contacts) {
            try {
                state.snapshot = await bridge.refreshFeed(c.id);
            }
            catch { }
        } render(); }, 'Лента обновлена');
    else if (a === 'save-profile')
        void action(async () => { state.snapshot = await bridge.saveProfile(val('profile-name'), val('profile-bio')); render(); }, 'Профиль сохранён');
    else if (a === 'save-network') {
        const input = { network_id: val('network-id'), beacons: val('beacons').split(/\r?\n/).map(x => x.trim()).filter(Boolean), circuit_hops: Number(val('circuit-hops') || 3), transport_mode: val('transport-mode'), direct_first: checked('direct-first'), xray_socks: val('xray-socks') };
        void action(async () => { state.snapshot = await bridge.saveNetwork(input); render(); }, 'Сеть сохранена');
    }
    else if (a === 'save-ca')
        void saveCA(false);
    else if (a === 'rotate-ca') {
        if (confirm('Перевыпустить Root CA? Новый корневой сертификат потребуется заново установить на устройствах.'))
            void saveCA(true);
    }
    else if (a === 'install-ca')
        void action(async () => { state.snapshot = await bridge.installCA(); render(); }, 'Root CA установлен');
    else if (a === 'export-ca')
        void action(async () => { const p = await bridge.exportCA(); if (p)
            toast(`Сохранено: ${p}`, 'success'); });
    else if (a === 'open-data')
        void action(() => bridge.openData());
    else if (a === 'diagnostics')
        void action(async () => { const p = await bridge.diagnostics(); toast(`Диагностика: ${p}`, 'success'); });
    else if (a?.startsWith('open-service')) {
        const card = el.closest('.service-card');
        const url = card?.querySelector('.hidden-url')?.textContent || '';
        if (url)
            void action(() => bridge.openURL(url));
    }
});
document.addEventListener('submit', e => { if (e.target.id !== 'message-form')
    return; e.preventDefault(); const body = val('message-input').trim(); if (!body || !state.activeContact)
    return; void action(async () => { state.snapshot = await bridge.sendMessage(state.activeContact, body); render(); }); });
document.addEventListener('input', e => { const t = e.target; if (t.id === 'catalog-search') {
    state.catalogQuery = t.value;
    render();
    const n = document.getElementById('catalog-search');
    n?.focus();
    n?.setSelectionRange(n.value.length, n.value.length);
} });
function openContactDialog() { const modal = document.createElement('div'); modal.className = 'modal-backdrop'; modal.innerHTML = `<form class="modal surface"><div class="modal-head"><div><span class="eyebrow">НОВЫЙ КОНТАКТ</span><h3>Добавить человека</h3><p>Укажите node ID или .knot-адрес его узла. Профиль ku_ будет получен и проверен автоматически.</p></div><button type="button" class="icon-btn" data-close>×</button></div><label>Node / .knot<input id="contact-node" autofocus placeholder="kr_… или node.knot"/></label><label>Локальный псевдоним <small>необязательно</small><input id="contact-alias" placeholder="alex"/></label><div class="form-actions">${button('Отмена', 'close-modal', 'ghost')}${button('Добавить', 'submit-contact', 'primary', 'plus')}</div></form>`; document.body.appendChild(modal); modal.querySelector('[data-close]')?.addEventListener('click', () => modal.remove()); modal.querySelector('[data-action="close-modal"]')?.addEventListener('click', () => modal.remove()); modal.querySelector('[data-action="submit-contact"]')?.addEventListener('click', e => { e.preventDefault(); const node = modal.querySelector('#contact-node').value; const alias = modal.querySelector('#contact-alias').value; void action(async () => { state.snapshot = await bridge.addContact(node, alias); modal.remove(); state.page = 'chats'; render(); }, 'Контакт добавлен'); }); }
async function saveCA(rotate) { const input = { enabled: true, intercept_https: checked('ca-https'), validity_days: Number(val('ca-validity') || 3650), common_name: val('ca-cn'), organization: csv(val('ca-o')), organizational_unit: csv(val('ca-ou')), country: csv(val('ca-country')), province: csv(val('ca-state')), locality: csv(val('ca-locality')), street_address: csv(val('ca-street')), postal_code: csv(val('ca-postal')) }; await action(async () => { state.snapshot = await bridge.saveCA(input); if (rotate)
    state.snapshot = await bridge.rotateCA(); render(); }, rotate ? 'Root CA перевыпущен и установлен' : 'Профиль CA сохранён'); }
function hasEditableFocus() { const active = document.activeElement; if (!active)
    return false; return active.matches('input, textarea, select, [contenteditable="true"]'); }
function applySnapshot(s) { state.snapshot = s; if (hasEditableFocus()) {
    state.pendingSnapshot = s;
    return;
} state.pendingSnapshot = null; render(); }
document.addEventListener('focusout', () => { if (!state.pendingSnapshot)
    return; queueMicrotask(() => { if (hasEditableFocus() || !state.pendingSnapshot)
    return; const pending = state.pendingSnapshot; state.pendingSnapshot = null; state.snapshot = pending; render(); }); });
async function boot() { if (state.preview) {
    state.snapshot = previewSnapshot();
    render();
    return;
} try {
    state.snapshot = await bridge.snapshot();
    render();
    bridge.onSnapshot(applySnapshot);
}
catch (e) {
    root.innerHTML = `<div class="fatal"><div class="brand-mark large">K</div><h1>Не удалось запустить интерфейс</h1><p>${esc(e)}</p></div>`;
} }
void boot();

window.KNOT_DOCS={
ru:{groups:[
{title:'Начало',pages:[['start','С чего начать'],['concepts','Как устроен KnotRoute'],['install','Установка клиента']]},
{title:'Клиенты',pages:[['windows','Windows'],['android','Android'],['client-troubleshooting','Проблемы подключения']]},
{title:'Публикация',pages:[['publish-docker','Docker-сайт'],['traefik','Сайт вместе с Traefik'],['service-identity','Service identity и перенос']]},
{title:'Инфраструктура',pages:[['self-hosted','Полное self-hosted развёртывание'],['beacon','Beacon + bootstrap relay'],['relay','Relay'],['control','KnotRoute Control'],['agent','KnotRoute Agent']]},
{title:'Разработка',pages:[['sdk','Client SDK'],['server-sdk','Server SDK']]},
{title:'Безопасность',pages:[['security','Модель безопасности'],['backup','Резервные копии и восстановление']]}
],pages:{
start:{tag:'START HERE',title:'KnotRoute без боли',lead:'Выберите, что именно вы хотите сделать. Документация начинается с задач и готовых конфигураций, а не с устройства протокола.',html:`<div class="quick-grid"><a class="quick-card" href="#/windows"><strong>Открыть .knot-сайт</strong><span>Windows-клиент, системная интеграция, локальный CA.</span></a><a class="quick-card" href="#/android"><strong>Подключиться с Android</strong><span>Профиль сети, QR/deep link, встроенный браузер.</span></a><a class="quick-card" href="#/publish-docker"><strong>Опубликовать Docker-сервис</strong><span>Sidecar рядом с существующим контейнером, без публичного порта.</span></a><a class="quick-card" href="#/self-hosted"><strong>Развернуть свою сеть</strong><span>Network ID, Beacon, relay, Control и Agent.</span></a></div><div class="callout"><strong>KnotRoute</strong> — encrypted overlay для сервисов с собственными <code>.knot</code>-идентичностями. Обычный Интернет используется как транспорт между узлами.</div><h2>Самый короткий путь</h2><div class="steps"><div class="step"><h3>Получите профиль сети</h3><p>Network ID + Beacon URL. KnotRoute Control умеет выдавать QR и инструкции.</p></div><div class="step"><h3>Запустите клиент</h3><p>Desktop/Android либо встройте SDK в приложение.</p></div><div class="step"><h3>Откройте сервис</h3><p>Используйте canonical <code>…​.knot</code> address или локальный alias.</p></div></div>`},
concepts:{tag:'CONCEPTS',title:'Как устроен KnotRoute',lead:'Минимум терминов, которые полезно понимать при эксплуатации.',html:`<h2>Сеть и узлы</h2><p><code>network_id</code> разделяет независимые overlay-сети. Это идентификатор пространства, а не пароль членства.</p><h2>Service identity</h2><p>Опубликованный сервис имеет собственный Ed25519 identity и стабильный canonical <code>.knot</code>-адрес. Его можно переносить между хостами, сохраняя identity-файл.</p><h2>Discovery</h2><p>Узлы могут находить друг друга через Beacon, LAN multicast, PEX, peer cache и static seeds. Beacon не является каталогом сайтов.</p><h2>Circuits и rendezvous</h2><p>Клиент строит multi-hop circuit, сервис держит introduction points, а соединение встречается на rendezvous. Payload дополнительно защищён end-to-end.</p><div class="callout warn"><strong>Важно:</strong> v3 не следует позиционировать как Tor-equivalent anonymity. Timing correlation, Sybil и глобальный пассивный противник требуют отдельного исследования и аудита.</div>`},
install:{tag:'CLIENT',title:'Установка клиента',lead:'Desktop-клиент нужен только приложениям, которые сами не встраивают KnotRoute SDK.',html:`<h2>Windows</h2><pre><code>powershell -ExecutionPolicy Bypass -File .\\Install-KnotRoute.ps1</code></pre><p>Запускайте <code>knotroute-desktop.exe</code>. Он управляет daemon, tray, dashboard, proxy/PAC и локальным CA.</p><h2>Linux/macOS</h2><pre><code>knotroute init --config knotroute.json
knotroute doctor --config knotroute.json --probe
knotroute run --config knotroute.json</code></pre><p>Для браузеров укажите KnotRoute SOCKS5/HTTP proxy либо используйте системную интеграцию платформы.</p>`},
windows:{tag:'WINDOWS',title:'Windows: подключение и браузер',lead:'Нормальный пользовательский сценарий — tray-приложение, без TUN-интерфейса.',html:`<div class="steps"><div class="step"><h3>Установите и запустите</h3><p>Откройте <code>knotroute-desktop.exe</code>.</p></div><div class="step"><h3>Импортируйте параметры сети</h3><p>Задайте Network ID и Beacon URLs либо импортируйте invitation/profile.</p></div><div class="step"><h3>Включите .knot integration</h3><p>Tray установит локальный CA текущего пользователя и PAC, который отправляет только <code>.knot</code> в локальный proxy.</p></div><div class="step"><h3>Откройте сайт</h3><p><code>https://&lt;service-identity&gt;.knot/</code></p></div></div><h2>Совместимость с VPN/TUN</h2><p>В стандартном режиме KnotRoute не создаёт TUN. Обычные хосты возвращаются PAC как <code>DIRECT</code> и дальше следуют обычной таблице маршрутов ОС.</p><h2>Диагностика</h2><pre><code>knotroute doctor --config knotroute.json --probe
knotroute address --config knotroute.json</code></pre>`},
android:{tag:'ANDROID',title:'Android',lead:'Android-клиент содержит KnotRoute core внутри APK и не требует отдельного VPN/TUN.',html:`<div class="steps"><div class="step"><h3>Добавьте сеть</h3><p>Откройте <code>knotroute://join</code> deep link из QR или заполните Network ID/Beacon вручную.</p></div><div class="step"><h3>Подключитесь</h3><p>Foreground service поддерживает overlay, пока клиент включён.</p></div><div class="step"><h3>Для HTTPS установите локальный CA</h3><p>Приложение открывает штатный Android certificate installer и никогда не обходит TLS error.</p></div><div class="step"><h3>Используйте Browser</h3><p>WebView получает process-local KnotRoute proxy.</p></div></div><h2>Приложения без standalone-клиента</h2><p>Используйте AAR из release. Ваше приложение может поднять embedded KnotRoute core и локальный forward самостоятельно.</p>`},
'client-troubleshooting':{tag:'TROUBLESHOOTING',title:'Клиент не подключается',lead:'Проверяйте цепочку от network profile до конкретного сервиса.',html:`<h2>Нет peers</h2><ol><li>Сверьте <code>network_id</code>.</li><li>Проверьте доступность HTTPS Beacon.</li><li>Проверьте TCP endpoint bootstrap relay.</li><li>Запустите <code>doctor --probe</code>.</li></ol><h2>.knot не открывается</h2><ol><li>Убедитесь, что proxy/PAC реально используется приложением.</li><li>Для SOCKS включите remote DNS.</li><li>Проверьте, что descriptor сервиса успел распространиться.</li></ol><h2>TLS error</h2><p>Не нажимайте «продолжить». Переустановите local KnotRoute CA и проверьте trust store браузера.</p>`},
'publish-docker':{tag:'PUBLISH',title:'Опубликовать Docker-сайт',lead:'Самый простой production-сценарий: sidecar подключается к той же Docker network, что и приложение.',html:`<pre><code>services:
  app:
    image: your/app:latest
    networks: [private]

  knotroute:
    image: ghcr.io/localzet/knotroute-sidecar:3.1.0
    restart: unless-stopped
    networks: [private]
    volumes:
      - knotroute-data:/data
    environment:
      KNOTROUTE_NETWORK_ID: "kn_..."
      KNOTROUTE_BEACONS: "https://beacon-a.example,https://beacon-b.example"
      KNOTROUTE_SERVICE_NAME: "web"
      KNOTROUTE_SERVICE_TARGET: "app:8080"

networks:
  private:
volumes:
  knotroute-data:</code></pre><div class="callout"><strong>Не удаляйте volume.</strong> В <code>/data</code> хранится service identity. Потеря identity = новый canonical <code>.knot</code>-адрес.</div><p>Приложению не нужен host <code>ports:</code>, если оно доступно только через KnotRoute.</p>`},
traefik:{tag:'DOCKER',title:'Публичный Traefik + приватный KnotRoute',lead:'Один контейнер может быть одновременно доступен через обычный домен и через .knot.',html:`<pre><code>Internet -> Traefik -> app:8080
                       ^
                       |
KnotRoute -> sidecar --+</code></pre><p>Подключите приложение к публичной Traefik network и к отдельной private network с sidecar. Sidecar целится в <code>app:8080</code>; Traefik продолжает работать независимо.</p><p>Для hidden-only сервиса не подключайте приложение к Traefik network вообще.</p>`},
'service-identity':{tag:'IDENTITY',title:'Service identity и перенос сервиса',lead:'Адрес принадлежит сервису, а не машине.',html:`<h2>Что сохранять</h2><p>Service identity file под persistent <code>/data</code>. Именно его публичный ключ определяет canonical address.</p><h2>Миграция</h2><ol><li>Остановите старый publisher.</li><li>Безопасно перенесите service identity.</li><li>Запустите sidecar на новом сервере с новым target.</li><li>Дождитесь новой revision descriptor.</li><li>Проверьте, что canonical address не изменился.</li></ol>`},
'self-hosted':{tag:'SELF-HOSTED',title:'Полное self-hosted развёртывание',lead:'Рекомендуемая схема: несколько Beacon/relay + Control + Agent на управляемых Docker-хостах.',html:`<pre><code>                    KnotRoute Control
                         ^
                         | outbound signed polling
              +----------+----------+
              |                     |
          Agent A                 Agent B
             |                       |
      Beacon + relay            sidecars
             |                       |
             +------ overlay --------+</code></pre><div class="steps"><div class="step"><h3>Создайте Network ID</h3><p><code>knotroute network create</code> и сохраните значение.</p></div><div class="step"><h3>Поднимите Control</h3><p>Панель хранит inventory и deployment jobs, но не участвует в маршрутизации.</p></div><div class="step"><h3>Подключите Agents</h3><p>Agents инициируют исходящие HTTPS-запросы к Control и подписывают их Ed25519.</p></div><div class="step"><h3>Разверните 2+ Beacon</h3><p>На разных failure domains. Control поставит job нужному agent.</p></div><div class="step"><h3>Публикуйте sidecars</h3><p>Выберите server, Docker network и target. Persistent identity volume обязателен.</p></div><div class="step"><h3>Генерируйте onboarding</h3><p>Windows/Android/Linux/Docker/Go инструкции и QR из одной панели.</p></div></div><h2>Минимальный compose Control</h2><pre><code>services:
  control:
    image: ghcr.io/localzet/knotroute-control:3.1.0
    restart: unless-stopped
    environment:
      KNOTROUTE_CONTROL_ADMIN_PASSWORD: "change-me"
      KNOTROUTE_CONTROL_ENROLL_TOKEN: "long-random-token"
    volumes:
      - control-data:/data
    expose: ["8080"]
volumes:
  control-data:</code></pre>`},
beacon:{tag:'DISCOVERY',title:'Beacon + bootstrap relay',lead:'Beacon даёт peer candidates, bundled relay — первый TCP edge в overlay.',html:`<p>HTTP Beacon следует публиковать через HTTPS reverse proxy. Relay <code>:7447/tcp</code> — raw TCP endpoint.</p><pre><code>KNOTROUTE_NETWORK_ID=kn_...
KNOTROUTE_BEACON_LISTEN=0.0.0.0:8080
KNOTROUTE_BEACON_RELAY=true
KNOTROUTE_BEACON_RELAY_LISTEN=0.0.0.0:7447
KNOTROUTE_BEACON_RELAY_ADVERTISE=relay.example.net:7447</code></pre><p>Beacon не знает список <code>.knot</code>-сервисов и не становится naming authority.</p>`},
relay:{tag:'ROUTING',title:'Relay',lead:'Always-on relay повышает связность, path diversity и доступность directory/rendezvous.',html:`<p>Для входящих peers откройте <code>7447/tcp</code> и задайте реальный <code>advertise</code>. Client-only узлы могут быть outbound-only.</p><p>Для устойчивой независимой сети имеет смысл держать несколько relay на независимых хостах/провайдерах.</p>`},
control:{tag:'OPERATIONS',title:'KnotRoute Control',lead:'Web control plane для inventory, deployment, onboarding и наблюдаемости. Не является частью overlay data plane.',html:`<h2>Что умеет</h2><ul><li>Network profiles;</li><li>agent inventory и heartbeat;</li><li>Beacon/sidecar deployment jobs;</li><li>restart/remove только managed components;</li><li>генератор подключения и QR;</li><li>RU/EN интерфейс.</li></ul><h2>Безопасность</h2><p>Веб-панель требует admin password, использует HttpOnly SameSite session cookie, same-origin checks и security headers. Размещайте её за HTTPS reverse proxy.</p>`},
agent:{tag:'OPERATIONS',title:'KnotRoute Agent',lead:'Минимальный server-side executor. Он сам инициирует connection к Control.',html:`<p>При первом запуске agent генерирует Ed25519 identity и использует одноразовый/ограниченно распространяемый enrollment token. Дальнейшие heartbeat/jobs подписываются identity.</p><pre><code>services:
  agent:
    image: ghcr.io/localzet/knotroute-agent:3.1.0
    restart: unless-stopped
    environment:
      KNOTROUTE_CONTROL_URL: https://control.example.net
      KNOTROUTE_CONTROL_ENROLL_TOKEN: "..."
      KNOTROUTE_AGENT_DOCKER: "true"
    volumes:
      - agent-data:/data
      - /var/run/docker.sock:/var/run/docker.sock</code></pre><div class="callout warn"><strong>Docker socket — host-admin capability.</strong> Монтируйте его только в Agent, никогда не в Control. Agent ограничивает операции managed labels/stacks, но сам по себе остаётся высокопривилегированным компонентом.</div>`},
sdk:{tag:'DEVELOPMENT',title:'Client SDK',lead:'Standalone KnotRoute не обязателен: приложение может включать core непосредственно.',html:`<pre><code>client, err := knotclient.New(knotclient.Options{
    DataDir:     "./knot-data",
    NetworkID:   "kn_...",
    Beacons:     []string{"https://beacon.example"},
    CircuitHops: 3,
})
client.Start(ctx)
conn, err := client.Dial(ctx, "... .knot")</code></pre><p>Android release содержит AAR с gomobile binding.</p>`},
'server-sdk':{tag:'DEVELOPMENT',title:'Server SDK',lead:'Нативное приложение может публиковать handler как service identity без внешнего nginx.',html:`<p>Используйте server SDK для in-process TCP/RPC handlers и persistent service identity. Для существующих контейнеров sidecar обычно проще.</p>`},
security:{tag:'SECURITY',title:'Модель безопасности',lead:'Что защищает KnotRoute и чего он сознательно не обещает.',html:`<h2>Защищает</h2><ul><li>direct-link TLS и identity authentication;</li><li>multi-hop circuit encryption;</li><li>service identity authentication;</li><li>end-to-end rendezvous payload encryption;</li><li>signed expiring descriptors.</li></ul><h2>Не является</h2><ul><li>гарантией anonymity уровня Tor;</li><li>network-wide membership PKI;</li><li>заменой authentication внутри приложения;</li><li>защитой от endpoint compromise, global traffic correlation и DoS.</li></ul><p><code>network_id</code> — namespace/isolation identifier, не пароль.</p>`},
backup:{tag:'OPERATIONS',title:'Backup и восстановление',lead:'Ключи важнее контейнеров.',html:`<table><thead><tr><th>Что</th><th>Почему</th></tr></thead><tbody><tr><td>Service identity</td><td>Определяет canonical .knot address.</td></tr><tr><td>Node identity</td><td>Нужна для continuity node ID.</td></tr><tr><td>Control /data</td><td>Inventory, jobs, network profiles.</td></tr><tr><td>Agent /data</td><td>Agent identity и managed compose stacks.</td></tr></tbody></table><p>Не делайте <code>docker compose down -v</code> для production sidecar без осознанного удаления identity.</p>`}
}},
en:{groups:[
{title:'Start',pages:[['start','Start here'],['concepts','How KnotRoute works'],['install','Install a client']]},
{title:'Clients',pages:[['windows','Windows'],['android','Android'],['client-troubleshooting','Troubleshooting']]},
{title:'Publishing',pages:[['publish-docker','Docker service'],['traefik','Traefik + KnotRoute'],['service-identity','Service identity & migration']]},
{title:'Infrastructure',pages:[['self-hosted','Full self-hosted deployment'],['beacon','Beacon + bootstrap relay'],['relay','Relay'],['control','KnotRoute Control'],['agent','KnotRoute Agent']]},
{title:'Development',pages:[['sdk','Client SDK'],['server-sdk','Server SDK']]},
{title:'Security',pages:[['security','Security model'],['backup','Backup & recovery']]}
],pages:{}}
};
// English pages deliberately mirror the operational structure while keeping the shipped bundle compact.
for(const [id,p] of Object.entries(window.KNOT_DOCS.ru.pages)){window.KNOT_DOCS.en.pages[id]={...p,tag:p.tag,title:({start:'KnotRoute without the pain',concepts:'How KnotRoute works',install:'Install a client',windows:'Windows: connect and browse',android:'Android', 'client-troubleshooting':'Client troubleshooting','publish-docker':'Publish a Docker service',traefik:'Public Traefik + private KnotRoute','service-identity':'Service identity and migration','self-hosted':'Full self-hosted deployment',beacon:'Beacon + bootstrap relay',relay:'Relay',control:'KnotRoute Control',agent:'KnotRoute Agent',sdk:'Client SDK','server-sdk':'Server SDK',security:'Security model',backup:'Backup and recovery'})[id]||p.title,lead:p.lead,html:p.html};}
Object.assign(window.KNOT_DOCS.en.pages, {
start:{tag:'START HERE',title:'KnotRoute without the pain',lead:'Choose the task you need to complete. These docs start with deployable workflows instead of protocol internals.',html:`<div class="quick-grid"><a class="quick-card" href="#/windows"><strong>Open a .knot site</strong><span>Windows client, system integration and local CA.</span></a><a class="quick-card" href="#/android"><strong>Connect from Android</strong><span>Network profile, QR/deep link and built-in browser.</span></a><a class="quick-card" href="#/publish-docker"><strong>Publish a Docker service</strong><span>Run a sidecar next to an existing container without exposing its private port.</span></a><a class="quick-card" href="#/self-hosted"><strong>Deploy your own network</strong><span>Network ID, Beacons, relays, Control and Agents.</span></a></div><div class="callout"><strong>KnotRoute</strong> is an encrypted service overlay with self-authenticating <code>.knot</code> identities. The ordinary Internet is only the transport between nodes.</div><h2>Shortest path</h2><div class="steps"><div class="step"><h3>Get a network profile</h3><p>Network ID plus Beacon URLs. KnotRoute Control can generate a QR/deep link.</p></div><div class="step"><h3>Start a client</h3><p>Use desktop/Android or embed the SDK.</p></div><div class="step"><h3>Open a service</h3><p>Use its canonical <code>…​.knot</code> address or a local alias.</p></div></div>`},
concepts:{tag:'CONCEPTS',title:'How KnotRoute works',lead:'The minimum set of concepts useful for operating the network.',html:`<h2>Networks and nodes</h2><p><code>network_id</code> separates independent overlays. It is a namespace identifier, not a membership password.</p><h2>Service identities</h2><p>A published service owns an independent Ed25519 identity and stable canonical <code>.knot</code> address. Move the identity file with the service to keep the address.</p><h2>Discovery</h2><p>Nodes can learn peers through Beacons, LAN multicast, PEX, persistent cache and static seeds. A Beacon is not a site directory.</p><h2>Circuits and rendezvous</h2><p>Clients build multi-hop circuits, services maintain introduction points and both sides meet at a rendezvous node. The service payload has a separate end-to-end protection layer.</p><div class="callout warn"><strong>Important:</strong> do not market v3/v3.1 as Tor-equivalent anonymity. Timing correlation, Sybil resistance and global-adversary analysis require independent research and audit.</div>`},
install:{tag:'CLIENT',title:'Install a client',lead:'The standalone client is needed only when the application does not embed the KnotRoute SDK itself.',html:`<h2>Windows</h2><pre><code>powershell -ExecutionPolicy Bypass -File .\\Install-KnotRoute.ps1</code></pre><p>Launch <code>knotroute-desktop.exe</code>. It controls the daemon, tray, dashboard, local proxies/PAC and the per-device CA.</p><h2>Linux/macOS</h2><pre><code>knotroute init --config knotroute.json
knotroute doctor --config knotroute.json --probe
knotroute run --config knotroute.json</code></pre>`},
windows:{tag:'WINDOWS',title:'Windows: connect and browse',lead:'Normal interactive use is the tray application; KnotRoute does not need a TUN adapter by default.',html:`<div class="steps"><div class="step"><h3>Install and start</h3><p>Run <code>knotroute-desktop.exe</code>.</p></div><div class="step"><h3>Import network settings</h3><p>Set the Network ID and Beacon URLs or import a profile.</p></div><div class="step"><h3>Enable .knot integration</h3><p>The tray installs the current user's local CA and a PAC that routes only <code>.knot</code> web traffic to KnotRoute.</p></div><div class="step"><h3>Open the service</h3><p><code>https://&lt;service-identity&gt;.knot/</code></p></div></div><h2>VPN coexistence</h2><p>Ordinary hosts are <code>DIRECT</code> from KnotRoute's PAC and continue through the operating system's routing stack.</p>`},
android:{tag:'ANDROID',title:'Android',lead:'The Android app embeds the KnotRoute core directly and does not require VpnService/TUN.',html:`<div class="steps"><div class="step"><h3>Add a network</h3><p>Open a <code>knotroute://join</code> QR/deep link or enter the Network ID and Beacons manually.</p></div><div class="step"><h3>Connect</h3><p>A foreground service keeps the overlay core running.</p></div><div class="step"><h3>Install the local CA for HTTPS</h3><p>The app opens Android's system certificate installer and never bypasses TLS errors.</p></div><div class="step"><h3>Use Browser</h3><p>The built-in WebView receives a process-local KnotRoute proxy.</p></div></div>`},
'client-troubleshooting':{tag:'TROUBLESHOOTING',title:'Client troubleshooting',lead:'Check the path from network profile to the target service.',html:`<h2>No peers</h2><ol><li>Verify <code>network_id</code>.</li><li>Check the HTTPS Beacon endpoint.</li><li>Check the bootstrap relay TCP endpoint.</li><li>Run <code>doctor --probe</code>.</li></ol><h2>.knot does not open</h2><ol><li>Confirm the application actually uses KnotRoute proxy/PAC.</li><li>Enable remote DNS for SOCKS clients.</li><li>Allow time for the service descriptor to propagate.</li></ol><h2>TLS error</h2><p>Do not click through it. Reinstall the local KnotRoute CA and verify the browser trust store.</p>`},
'publish-docker':{tag:'PUBLISH',title:'Publish a Docker service',lead:'Run a KnotRoute sidecar on the same Docker network as the application.',html:`<pre><code>services:
  app:
    image: your/app:latest
    networks: [private]
  knotroute:
    image: ghcr.io/localzet/knotroute-sidecar:3.1.0
    restart: unless-stopped
    networks: [private]
    volumes: [knotroute-data:/data]
    environment:
      KNOTROUTE_NETWORK_ID: "kn_..."
      KNOTROUTE_BEACONS: "https://beacon-a.example,https://beacon-b.example"
      KNOTROUTE_SERVICE_NAME: "web"
      KNOTROUTE_SERVICE_TARGET: "app:8080"
networks: { private: {} }
volumes: { knotroute-data: {} }</code></pre><div class="callout"><strong>Keep the volume.</strong> <code>/data</code> contains the service identity. Losing it changes the canonical <code>.knot</code> address.</div>`},
traefik:{tag:'DOCKER',title:'Public Traefik + private KnotRoute',lead:'The same application can have a normal public hostname and an independent .knot identity.',html:`<pre><code>Internet -> Traefik -> app:8080
                       ^
                       |
KnotRoute -> sidecar --+</code></pre><p>Attach the application to its normal reverse-proxy network and to a separate private network shared with the KnotRoute sidecar. Hidden-only applications do not need the public network at all.</p>`},
'service-identity':{tag:'IDENTITY',title:'Service identity and migration',lead:'The address belongs to the service rather than the machine.',html:`<h2>Keep the identity</h2><p>The service identity under persistent <code>/data</code> determines the canonical address.</p><h2>Move the service</h2><ol><li>Stop the old publisher.</li><li>Move the service identity securely.</li><li>Start the publisher on the new host with the new local target.</li><li>Wait for the new signed descriptor revision.</li><li>Confirm the canonical address is unchanged.</li></ol>`},
'self-hosted':{tag:'SELF-HOSTED',title:'Full self-hosted deployment',lead:'Recommended layout: multiple Beacon/relay nodes plus Control and outbound Agents on managed Docker hosts.',html:`<pre><code>                    KnotRoute Control
                         ^
                         | signed outbound polling
              +----------+----------+
              |                     |
          Agent A                 Agent B
             |                       |
      Beacon + relay            sidecars
             |                       |
             +------ overlay --------+</code></pre><div class="steps"><div class="step"><h3>Create a Network ID</h3><p>Run <code>knotroute network create</code>.</p></div><div class="step"><h3>Deploy Control</h3><p>Control manages inventory and jobs but is not part of the data plane.</p></div><div class="step"><h3>Enroll Agents</h3><p>Agents make outbound HTTPS requests and sign them with Ed25519.</p></div><div class="step"><h3>Deploy 2+ Beacons</h3><p>Use different failure domains.</p></div><div class="step"><h3>Publish sidecars</h3><p>Select the server, Docker network and target.</p></div><div class="step"><h3>Generate onboarding</h3><p>Share platform-specific steps and a QR/deep link.</p></div></div>`},
beacon:{tag:'DISCOVERY',title:'Beacon + bootstrap relay',lead:'The Beacon returns peer candidates; the bundled relay provides the first raw TCP overlay edge.',html:`<p>Publish the HTTP Beacon through an HTTPS reverse proxy. Expose relay <code>:7447/tcp</code> as raw TCP.</p><pre><code>KNOTROUTE_NETWORK_ID=kn_...
KNOTROUTE_BEACON_LISTEN=0.0.0.0:8080
KNOTROUTE_BEACON_RELAY=true
KNOTROUTE_BEACON_RELAY_LISTEN=0.0.0.0:7447
KNOTROUTE_BEACON_RELAY_ADVERTISE=relay.example.net:7447</code></pre><p>The Beacon is neither a service directory nor a naming authority.</p>`},
relay:{tag:'ROUTING',title:'Relay',lead:'Always-on relays improve connectivity, directory availability and path diversity.',html:`<p>For inbound peers expose <code>7447/tcp</code> and configure a reachable <code>advertise</code> endpoint. Client-only nodes can remain outbound-only.</p>`},
control:{tag:'OPERATIONS',title:'KnotRoute Control',lead:'Optional web control plane for inventory, deployments and onboarding. It is not part of overlay routing.',html:`<h2>Features</h2><ul><li>network profiles;</li><li>Agent inventory and health;</li><li>Beacon/sidecar deployment jobs;</li><li>restart/remove for managed components only;</li><li>onboarding and QR generation;</li><li>RU/EN UI.</li></ul><h2>Security</h2><p>Control requires an admin password, uses HttpOnly SameSite sessions, same-origin checks and browser security headers. Place it behind HTTPS.</p>`},
agent:{tag:'OPERATIONS',title:'KnotRoute Agent',lead:'Small server-side executor that initiates outbound connections to Control.',html:`<p>On first start the Agent generates an Ed25519 identity. Enrollment uses a separate token; subsequent requests are signed.</p><pre><code>agent:
  image: ghcr.io/localzet/knotroute-agent:3.1.0
  environment:
    KNOTROUTE_CONTROL_URL: https://control.example.net
    KNOTROUTE_CONTROL_ENROLL_TOKEN: "..."
    KNOTROUTE_AGENT_DOCKER: "true"
  volumes:
    - agent-data:/data
    - /var/run/docker.sock:/var/run/docker.sock</code></pre><div class="callout warn"><strong>Docker socket is host-admin capability.</strong> Mount it only into Agent, never into Control.</div>`},
sdk:{tag:'DEVELOPMENT',title:'Client SDK',lead:'Applications can embed KnotRoute and work without a separately installed client.',html:`<pre><code>client, err := knotclient.New(knotclient.Options{
    DataDir: "./knot-data",
    NetworkID: "kn_...",
    Beacons: []string{"https://beacon.example"},
    CircuitHops: 3,
})
client.Start(ctx)
conn, err := client.Dial(ctx, "... .knot")</code></pre><p>Android releases include the gomobile AAR binding.</p>`},
'server-sdk':{tag:'DEVELOPMENT',title:'Server SDK',lead:'Native applications can publish in-process handlers under a stable service identity.',html:`<p>Use the server SDK for embedded TCP/RPC handlers. For existing containers, a sidecar is usually simpler.</p>`},
security:{tag:'SECURITY',title:'Security model',lead:'What KnotRoute protects and what it deliberately does not claim.',html:`<h2>Provides</h2><ul><li>authenticated direct-link encryption;</li><li>multi-hop circuit encryption;</li><li>service identity authentication;</li><li>end-to-end rendezvous payload encryption;</li><li>signed expiring service descriptors.</li></ul><h2>Does not provide</h2><ul><li>Tor-equivalent anonymity guarantees;</li><li>network-wide membership PKI;</li><li>application authentication;</li><li>protection from endpoint compromise, global correlation or denial of service.</li></ul>`},
backup:{tag:'OPERATIONS',title:'Backup and recovery',lead:'Keys matter more than containers.',html:`<table><thead><tr><th>Data</th><th>Why</th></tr></thead><tbody><tr><td>Service identity</td><td>Determines canonical .knot address.</td></tr><tr><td>Node identity</td><td>Preserves node ID continuity.</td></tr><tr><td>Control /data</td><td>Inventory, jobs and network profiles.</td></tr><tr><td>Agent /data</td><td>Agent identity and managed compose stacks.</td></tr></tbody></table>`}
});

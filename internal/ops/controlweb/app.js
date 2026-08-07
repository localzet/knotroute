const state = { lang: localStorage.krLang || 'ru', data: null, page: 'overview', onboarding: null }
const $ = selector => document.querySelector(selector)
const $$ = selector => [...document.querySelectorAll(selector)]

const messages = {
  ru: {
    loginHint: 'Управление KnotRoute-инфраструктурой.', password: 'Пароль', login: 'Войти', overview: 'Обзор', networks: 'Сети', infrastructure: 'Инфраструктура', services: 'Сервисы', onboarding: 'Подключение', jobs: 'Задачи', logout: 'Выйти', refresh: 'Обновить',
    networkNamespaces: 'сетевых профилей', agents: 'Агенты', managedComponents: 'управляемых компонентов', publishedServices: 'опубликованных сервисов', agentHealth: 'Состояние агентов', recentJobs: 'Последние задачи',
    networksHint: 'Здесь хранится конфигурация сети для Control. Beacon может быть уже поднят вручную — просто добавьте его HTTP(S) URL.', newNetwork: 'Добавить сеть', infraHint: 'Агенты и компоненты, которые Control развернул сам. Уже работающий внешний Beacon не обязан быть управляемым Control.', servicesHint: 'Sidecar-сервисы, развернутые через подключенные агенты.',
    publishService: 'Опубликовать сервис', connectInstruction: 'Инструкция подключения', network: 'Сеть', platform: 'Платформа', publicName: 'Название профиля для пользователя', generate: 'Сгенерировать', copy: 'Копировать', cancel: 'Отмена', save: 'Сохранить', online: 'онлайн', offline: 'не в сети',
    noAgents: 'Агенты ещё не подключены. Для существующего Beacon агент не нужен.', noJobs: 'Задач пока нет.', noNetworks: 'Добавьте существующую сеть или создайте новую.', noServices: 'Управляемых sidecar-сервисов пока нет.', name: 'Название', description: 'Описание', networkId: 'Network ID', beacons: 'Beacon HTTP(S) URL — по одному на строку',
    agent: 'Агент', publicHost: 'Публичный relay host:port', httpHost: 'Beacon HTTP(S) URL', serviceName: 'Имя сервиса', target: 'Target контейнер:порт', dockerNetwork: 'Docker network', advertise: 'Advertise host:port', status: 'Статус', lastSeen: 'Последняя связь', docker: 'Docker', components: 'Компоненты', restart: 'Перезапустить', remove: 'Удалить', jobQueued: 'Задача поставлена в очередь.', networkSaved: 'Сеть сохранена.',
    advanced: 'Дополнительно', existingBeaconHelp: 'Укажите URL HTTP API Beacon, например https://beacon.example.net. Порт 7447 — relay transport и сюда не подходит.', deployManagedBeacon: 'Развернуть новый Beacon', checkBeacon: 'Проверить Beacon', beaconHealthy: 'Beacon доступен', edit: 'Изменить', noDockerAgents: 'Нет подключенных агентов с включенным Docker-управлением.', noNetwork: 'Сначала добавьте сеть.', networkHasNoBeacon: 'У этой сети нет Beacon URL. Добавьте уже работающий Beacon в настройки сети; Control не обязан разворачивать его сам.',
    validationName: 'Введите название.', unauthorized: 'Сессия истекла. Войдите снова.', requestFailed: 'Запрос не выполнен', registeredBeacons: 'Зарегистрированные Beacon', unmanaged: 'внешний / не управляется Control', noRegisteredBeacons: 'В профилях сетей пока нет внешних Beacon URL.', registerExistingBeacon: 'Добавить существующий Beacon', existingBeaconTitle: 'Существующий Beacon', beaconAdded: 'Beacon добавлен в профиль сети.',
  },
  en: {
    loginHint: 'KnotRoute infrastructure control plane.', password: 'Password', login: 'Sign in', overview: 'Overview', networks: 'Networks', infrastructure: 'Infrastructure', services: 'Services', onboarding: 'Onboarding', jobs: 'Jobs', logout: 'Sign out', refresh: 'Refresh',
    networkNamespaces: 'network profiles', agents: 'Agents', managedComponents: 'managed components', publishedServices: 'published services', agentHealth: 'Agent health', recentJobs: 'Recent jobs',
    networksHint: 'Control stores network profiles here. A Beacon may already exist outside Control — just add its HTTP(S) URL.', newNetwork: 'Add network', infraHint: 'Agents and components deployed by Control. An existing external Beacon does not need to be Control-managed.', servicesHint: 'Sidecars deployed through enrolled agents.',
    publishService: 'Publish service', connectInstruction: 'Connection instructions', network: 'Network', platform: 'Platform', publicName: 'Profile name for users', generate: 'Generate', copy: 'Copy', cancel: 'Cancel', save: 'Save', online: 'online', offline: 'offline',
    noAgents: 'No agents enrolled. Existing external Beacons do not require an agent.', noJobs: 'No jobs yet.', noNetworks: 'Add an existing network or create one.', noServices: 'No managed sidecar services yet.', name: 'Name', description: 'Description', networkId: 'Network ID', beacons: 'Beacon HTTP(S) URLs — one per line',
    agent: 'Agent', publicHost: 'Public relay host:port', httpHost: 'Beacon HTTP(S) URL', serviceName: 'Service name', target: 'Target container:port', dockerNetwork: 'Docker network', advertise: 'Advertise host:port', status: 'Status', lastSeen: 'Last seen', docker: 'Docker', components: 'Components', restart: 'Restart', remove: 'Remove', jobQueued: 'Job queued.', networkSaved: 'Network saved.',
    advanced: 'Advanced', existingBeaconHelp: 'Use the Beacon HTTP API URL, for example https://beacon.example.net. Port 7447 is the relay transport and does not belong here.', deployManagedBeacon: 'Deploy new Beacon', checkBeacon: 'Check Beacon', beaconHealthy: 'Beacon is reachable', edit: 'Edit', noDockerAgents: 'No connected agents with Docker management enabled.', noNetwork: 'Add a network first.', networkHasNoBeacon: 'This network has no Beacon URL. Add the already-running Beacon to the network profile; Control does not need to deploy it.',
    validationName: 'Enter a name.', unauthorized: 'Session expired. Sign in again.', requestFailed: 'Request failed', registeredBeacons: 'Registered Beacons', unmanaged: 'external / not managed by Control', noRegisteredBeacons: 'No external Beacon URLs are registered in network profiles yet.', registerExistingBeacon: 'Add existing Beacon', existingBeaconTitle: 'Existing Beacon', beaconAdded: 'Beacon added to the network profile.',
  },
}

const tr = key => messages[state.lang]?.[key] || messages.en[key] || key
const esc = value => String(value ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]))

function applyI18n() {
  document.documentElement.lang = state.lang
  $$('[data-i18n]').forEach(el => { el.textContent = tr(el.dataset.i18n) })
  $('#langBtn').textContent = state.lang === 'ru' ? 'RU / EN' : 'EN / RU'
  render()
}

async function api(path, options = {}) {
  let response
  try {
    response = await fetch(path, {
      headers: { 'Content-Type': 'application/json', ...(options.headers || {}) },
      credentials: 'same-origin', cache: 'no-store', ...options,
    })
  } catch (error) {
    throw new Error(`${tr('requestFailed')}: ${error.message || error}`)
  }
  const contentType = response.headers.get('content-type') || ''
  const body = contentType.includes('application/json') ? await response.json() : await response.text()
  if (response.status === 401) throw Object.assign(new Error(tr('unauthorized')), { unauthorized: true })
  if (!response.ok) {
    const message = typeof body === 'object' ? body.error : body
    const hint = typeof body === 'object' ? body.hint : ''
    const error = new Error([message || response.statusText, hint].filter(Boolean).join('\n'))
    if (typeof body === 'object') Object.assign(error, { code: body.code, field: body.field })
    throw error
  }
  return body
}

let noticeTimer
function notice(message, bad = false) {
  clearTimeout(noticeTimer)
  const node = $('#notice')
  node.className = `notice${bad ? ' error' : ''}`
  node.textContent = message
  noticeTimer = setTimeout(() => { node.className = ''; node.textContent = '' }, 6500)
}

async function boot() {
  try {
    await api('/api/v1/session')
    $('#login').classList.add('hidden')
    $('#app').classList.remove('hidden')
    await refresh()
  } catch {
    $('#app').classList.add('hidden')
    $('#login').classList.remove('hidden')
  }
}

async function refresh() {
  try {
    state.data = await api('/api/v1/overview')
    $('#refreshState').textContent = new Date().toLocaleTimeString()
    render()
  } catch (error) {
    if (error.unauthorized) return boot()
    notice(error.message, true)
  }
}

function isOnline(agent) { return Date.now() - new Date(agent.last_seen).getTime() < 45000 }

function displayStatus(value) {
  const labels = state.lang === 'ru' ? { running: 'работает', exited: 'остановлен', created: 'создан', restarting: 'перезапуск', paused: 'пауза', dead: 'ошибка', unknown: 'неизвестно', pending: 'в очереди', running_job: 'выполняется', succeeded: 'успешно', failed: 'ошибка' } : {}
  return labels[value] || value || (state.lang === 'ru' ? 'неизвестно' : 'unknown')
}
function displayJobKind(value) {
  if (state.lang !== 'ru') return value
  return ({ deploy_beacon: 'Развернуть Beacon', deploy_sidecar: 'Опубликовать сервис', restart_component: 'Перезапустить компонент', remove_component: 'Удалить компонент' })[value] || value
}
function displayKind(value) {
  if (state.lang !== 'ru') return value
  return ({ beacon: 'Beacon', relay: 'Relay', sidecar: 'Sidecar', service: 'Сервис' })[value] || value
}
function fmtDate(value) { return value ? new Date(value).toLocaleString() : '—' }

function render() {
  if (!state.data) return
  const networks = Object.values(state.data.networks || {})
  const agents = Object.values(state.data.agents || {})
  const jobs = Object.values(state.data.jobs || {}).sort((a, b) => new Date(b.created_at) - new Date(a.created_at))
  const components = agents.flatMap(agent => (agent.components || []).map(component => ({ ...component, agent })))
  const infrastructure = components.filter(component => component.kind === 'beacon' || component.kind === 'relay')
  const services = components.filter(component => component.kind === 'sidecar' || component.kind === 'service')

  $('#metricNetworks').textContent = networks.length
  $('#metricAgents').textContent = agents.length
  $('#metricAgentsOnline').textContent = `${agents.filter(isOnline).length} ${tr('online')}`
  $('#metricInfra').textContent = infrastructure.length
  $('#metricServices').textContent = services.length
  $('#pageTitle').textContent = tr(state.page === 'overview' ? 'overview' : state.page)
  renderAgents(agents)
  renderExternalBeacons(networks)
  renderJobs(jobs)
  renderNetworks(networks)
  renderServices(services)
  fillNetworkSelect(networks)
}


function renderExternalBeacons(networks) {
  const items = networks.flatMap(network => (network.beacons || []).map(url => ({ network, url })))
  $('#externalBeaconsList').innerHTML = items.length ? items.map(item => `
    <article class="compact-card" data-network-id="${esc(item.network.id)}">
      <div class="card-head"><div><span class="eyebrow">${esc(item.network.name)}</span><h3>${esc(item.url)}</h3><p class="muted">${tr('unmanaged')}</p></div><span class="badge ok">HTTP API</span></div>
      <div class="actions"><button class="ghost" data-external-beacon="${esc(item.url)}">${tr('checkBeacon')}</button></div>
    </article>`).join('') : `<article>${tr('noRegisteredBeacons')}</article>`
}

function renderAgents(agents) {
  $('#overviewAgents').innerHTML = agents.length ? agents.slice(0, 8).map(agent => `
    <div class="list-row"><div><strong>${esc(agent.name || agent.hostname || agent.id)}</strong><div class="muted">${esc(agent.hostname || agent.id)}</div></div>
    <span class="badge ${isOnline(agent) ? 'ok' : 'bad'}">${isOnline(agent) ? tr('online') : tr('offline')}</span></div>`).join('') : `<div class="muted">${tr('noAgents')}</div>`
  $('#agentsList').innerHTML = agents.length ? agents.map(agent => `
    <article><div class="card-head"><div><span class="eyebrow">${esc(agent.id)}</span><h3>${esc(agent.name || agent.hostname || agent.id)}</h3></div><span class="badge ${isOnline(agent) ? 'ok' : 'bad'}">${isOnline(agent) ? tr('online') : tr('offline')}</span></div>
    <div class="card-meta"><div><span>${tr('lastSeen')}</span><strong>${fmtDate(agent.last_seen)}</strong></div><div><span>${tr('docker')}</span><strong>${agent.docker_available ? esc(agent.docker_version || (state.lang === 'ru' ? 'доступен' : 'available')) : '—'}</strong></div><div><span>${tr('components')}</span><strong>${(agent.components || []).length}</strong></div><div><span>${state.lang === 'ru' ? 'Версия' : 'Version'}</span><strong>${esc(agent.version || '—')}</strong></div></div>
    ${(agent.components || []).map(component => `<div class="list-row"><div><strong>${esc(component.name)}</strong><div class="muted">${esc(displayKind(component.kind))} · ${esc(component.image || component.target || '')}</div></div><span class="badge ${component.status === 'running' ? 'ok' : 'warn'}">${esc(displayStatus(component.status))}</span></div>`).join('')}</article>`).join('') : `<article>${tr('noAgents')}</article>`
}

function jobRow(job) {
  const cls = job.status === 'succeeded' ? 'ok' : job.status === 'failed' ? 'bad' : 'warn'
  return `<div class="list-row"><div><strong>${esc(displayJobKind(job.kind))}</strong><div class="muted">${esc(job.agent_id)} · ${fmtDate(job.created_at)}</div></div><span class="badge ${cls}">${esc(displayStatus(job.status))}</span></div>`
}
function renderJobs(jobs) {
  $('#overviewJobs').innerHTML = jobs.length ? jobs.slice(0, 8).map(jobRow).join('') : `<div class="muted">${tr('noJobs')}</div>`
  $('#jobsList').innerHTML = jobs.length ? jobs.map(job => `<article>${jobRow(job)}${job.result ? `<pre>${esc(job.result)}</pre>` : ''}</article>`).join('') : `<article>${tr('noJobs')}</article>`
}

function renderNetworks(networks) {
  $('#networksList').innerHTML = networks.length ? networks.map(network => `
    <article data-network-id="${esc(network.id)}">
      <div class="card-head"><div><span class="eyebrow">${state.lang === 'ru' ? 'СЕТЬ' : 'NETWORK'}</span><h3>${esc(network.name)}</h3>${network.description ? `<p class="muted">${esc(network.description)}</p>` : ''}</div></div>
      <div class="card-meta compact"><div><span>Network ID</span><code>${esc(network.id)}</code></div><div><span>Beacon</span><strong>${(network.beacons || []).length || '—'}</strong></div></div>
      ${(network.beacons || []).length ? `<div class="endpoint-list">${network.beacons.map(url => `<code>${esc(url)}</code>`).join('')}</div>` : `<p class="hint warn-text">${tr('networkHasNoBeacon')}</p>`}
      <div class="actions"><button class="ghost" data-action="edit-network">${tr('edit')}</button>${(network.beacons || []).length ? `<button class="ghost" data-action="check-beacon">${tr('checkBeacon')}</button>` : ''}<button class="ghost" data-action="onboard-network">${tr('onboarding')}</button></div>
    </article>`).join('') : `<article>${tr('noNetworks')}</article>`
}

function renderServices(services) {
  $('#servicesList').innerHTML = services.length ? services.map(component => `
    <article><div class="card-head"><div><span class="eyebrow">${esc(component.agent.name || component.agent.id)}</span><h3>${esc(component.name)}</h3></div><span class="badge ${component.status === 'running' ? 'ok' : 'warn'}">${esc(displayStatus(component.status))}</span></div>
    <div class="card-meta"><div><span>.knot</span><code>${esc(component.service || component.address || (state.lang === 'ru' ? 'ожидание' : 'pending'))}</code></div><div><span>${state.lang === 'ru' ? 'Цель' : 'Target'}</span><strong>${esc(component.target || '—')}</strong></div><div><span>${state.lang === 'ru' ? 'Контейнер' : 'Container'}</span><strong>${esc(component.container || '—')}</strong></div><div><span>${state.lang === 'ru' ? 'Образ' : 'Image'}</span><strong>${esc(component.image || '—')}</strong></div></div>
    <div class="actions"><button class="ghost" data-component-action="restart_component" data-agent="${esc(component.agent.id)}" data-component="${esc(component.id)}">${tr('restart')}</button><button class="ghost danger" data-component-action="remove_component" data-agent="${esc(component.agent.id)}" data-component="${esc(component.id)}">${tr('remove')}</button></div></article>`).join('') : `<article>${tr('noServices')}</article>`
}

function fillNetworkSelect(networks) {
  const select = $('#onboardNetwork')
  const current = select.value
  select.innerHTML = networks.map(network => `<option value="${esc(network.id)}">${esc(network.name)} · ${esc(network.id.slice(0, 14))}…</option>`).join('')
  if (networks.some(network => network.id === current)) select.value = current
}

function showPage(page) {
  state.page = page
  $$('#nav button').forEach(button => button.classList.toggle('active', button.dataset.page === page))
  $$('.page').forEach(section => section.classList.toggle('active', section.id === `page-${page}`))
  render()
}

function modal(title, body, submit, submitText = tr('save')) {
  const dialog = $('#modal')
  const form = $('#modalForm')
  const submitButton = $('#modalSubmit')
  const error = $('#modalError')
  $('#modalTitle').textContent = title
  $('#modalBody').innerHTML = body
  submitButton.textContent = submitText
  submitButton.disabled = false
  error.textContent = ''
  form.onsubmit = async event => {
    event.preventDefault()
    if (submitButton.disabled) return
    submitButton.disabled = true
    error.textContent = ''
    try {
      await submit(new FormData(form))
      dialog.close()
      await refresh()
    } catch (failure) {
      error.textContent = failure.message || String(failure)
    } finally {
      submitButton.disabled = false
    }
  }
  const close = () => { if (!submitButton.disabled) dialog.close() }
  $('#modalClose').onclick = close
  $('#modalCancel').onclick = close
  dialog.oncancel = event => { if (submitButton.disabled) event.preventDefault() }
  dialog.showModal()
}

function networkByID(id) { return Object.values(state.data?.networks || {}).find(network => network.id === id) }
function dockerAgents() { return Object.values(state.data?.agents || {}).filter(agent => agent.docker_available && isOnline(agent)) }

function editNetwork(network) {
  modal(network?.name || tr('newNetwork'), `
    <label><span>${tr('name')}</span><input name="name" required maxlength="80" value="${esc(network?.name || '')}" autofocus></label>
    <label><span>${tr('beacons')}</span><textarea name="beacons" rows="3" placeholder="https://beacon.example.net">${esc((network?.beacons || []).join('\n'))}</textarea><small class="field-help">${tr('existingBeaconHelp')}</small></label>
    <details class="advanced"><summary>${tr('advanced')}</summary>
      <label><span>${tr('description')}</span><input name="description" maxlength="300" value="${esc(network?.description || '')}"></label>
      <label><span>${tr('networkId')}</span><input name="id" ${network ? 'readonly' : ''} placeholder="${state.lang === 'ru' ? 'оставьте пустым для генерации' : 'leave blank to generate'}" value="${esc(network?.id || '')}"></label>
    </details>`, async data => {
      const name = data.get('name').trim()
      if (!name) throw new Error(tr('validationName'))
      await api('/api/v1/networks', { method: 'POST', body: JSON.stringify({
        id: data.get('id').trim(), name, description: data.get('description').trim(),
        beacons: data.get('beacons').split(/\s+/).filter(Boolean),
      }) })
      notice(tr('networkSaved'))
    })
}

async function checkBeacon(network) {
  const urls = network.beacons || []
  if (!urls.length) return notice(tr('networkHasNoBeacon'), true)
  try {
    const results = []
    for (const url of urls) {
      const out = await api(`/api/v1/beacons/check?url=${encodeURIComponent(url)}`)
      results.push(`${out.url}: ${out.latency_ms} ms`)
    }
    notice(`${tr('beaconHealthy')}: ${results.join(' · ')}`)
  } catch (error) { notice(error.message, true) }
}

function registerExistingBeacon() {
  const networks = Object.values(state.data?.networks || {})
  if (!networks.length) return notice(tr('noNetwork'), true)
  modal(tr('existingBeaconTitle'), `
    <label><span>${tr('network')}</span><select name="network">${networks.map(n => `<option value="${esc(n.id)}">${esc(n.name)}</option>`).join('')}</select></label>
    <label><span>${tr('httpHost')}</span><input name="beacon_url" placeholder="https://beacon.example.net" required><small class="field-help">${tr('existingBeaconHelp')}</small></label>`, async data => {
      const network = networkByID(data.get('network'))
      if (!network) throw new Error(tr('noNetwork'))
      const beaconURL = data.get('beacon_url').trim()
      await api(`/api/v1/beacons/check?url=${encodeURIComponent(beaconURL)}`)
      await api('/api/v1/networks', { method: 'POST', body: JSON.stringify({
        id: network.id, name: network.name, description: network.description || '',
        beacons: [...new Set([...(network.beacons || []), beaconURL])],
      }) })
      notice(tr('beaconAdded'))
    }, tr('save'))
}

function deployBeacon() {
  const agents = dockerAgents()
  const networks = Object.values(state.data?.networks || {})
  if (!agents.length) return notice(tr('noDockerAgents'), true)
  if (!networks.length) return notice(tr('noNetwork'), true)
  modal(tr('deployManagedBeacon'), `
    <label><span>${tr('agent')}</span><select name="agent">${agents.map(a => `<option value="${esc(a.id)}">${esc(a.name || a.hostname)}</option>`).join('')}</select></label>
    <label><span>${tr('network')}</span><select name="network">${networks.map(n => `<option value="${esc(n.id)}">${esc(n.name)}</option>`).join('')}</select></label>
    <label><span>${tr('name')}</span><input name="name" required value="beacon-a"></label>
    <label><span>${tr('httpHost')}</span><input name="beacon_url" placeholder="https://beacon.example.net" required><small class="field-help">${tr('existingBeaconHelp')}</small></label>
    <label><span>${tr('publicHost')}</span><input name="advertise" placeholder="relay.example.net:7447" required></label>
    <details class="advanced"><summary>${tr('advanced')}</summary>
      <label><span>HTTP host port</span><input name="http_port" value="18080" inputmode="numeric"></label>
      <small class="field-help">Relay host port берётся из Advertise host:port. DNS/TLS для Beacon URL настраиваются во внешнем reverse proxy.</small>
    </details>`, async data => {
      await createJob(data.get('agent'), 'deploy_beacon', {
        name: data.get('name'), network_id: data.get('network'), beacon_url: data.get('beacon_url'), advertise: data.get('advertise'),
        http_port: Number(data.get('http_port')),
      })
    }, tr('deployManagedBeacon'))
}

function deploySidecar() {
  const agents = dockerAgents()
  const networks = Object.values(state.data?.networks || {})
  if (!agents.length) return notice(tr('noDockerAgents'), true)
  if (!networks.length) return notice(tr('noNetwork'), true)
  modal(tr('publishService'), `
    <label><span>${tr('agent')}</span><select name="agent">${agents.map(a => `<option value="${esc(a.id)}">${esc(a.name || a.hostname)}</option>`).join('')}</select></label>
    <label><span>${tr('network')}</span><select name="network">${networks.map(n => `<option value="${esc(n.id)}">${esc(n.name)}</option>`).join('')}</select></label>
    <label><span>${tr('serviceName')}</span><input name="name" required value="web"></label>
    <label><span>${tr('target')}</span><input name="target" required placeholder="app:8080"></label>
    <label><span>${tr('dockerNetwork')}</span><input name="docker_network" required placeholder="my_stack_private"></label>
    <details class="advanced"><summary>${tr('advanced')}</summary><label><span>${tr('advertise')}</span><input name="advertise" placeholder="node.example.net:7447"></label></details>`, async data => {
      const network = networks.find(item => item.id === data.get('network'))
      if (!network?.beacons?.length) throw new Error(tr('networkHasNoBeacon'))
      await createJob(data.get('agent'), 'deploy_sidecar', {
        name: data.get('name'), network_id: data.get('network'), beacons: network.beacons,
        target: data.get('target'), docker_network: data.get('docker_network'), advertise: data.get('advertise'),
      })
    }, tr('publishService'))
}

async function createJob(agent, kind, payload) {
  await api('/api/v1/jobs', { method: 'POST', body: JSON.stringify({ agent_id: agent, kind, payload }) })
  notice(tr('jobQueued'))
}

async function componentJob(agent, kind, id) {
  if (kind === 'remove_component' && !confirm(`${tr('remove')} ${id}?`)) return
  try { await createJob(agent, kind, { component_id: id }); await refresh() } catch (error) { notice(error.message, true) }
}

function onboardNetwork(id) {
  showPage('onboarding')
  $('#onboardNetwork').value = id
  void generateOnboarding()
}
async function generateOnboarding() {
  try {
    if (!$('#onboardNetwork').value) throw new Error(tr('noNetwork'))
    const body = { network_id: $('#onboardNetwork').value, platform: $('#onboardPlatform').value, name: $('#onboardName').value.trim(), language: state.lang }
    state.onboarding = await api('/api/v1/onboarding/render', { method: 'POST', body: JSON.stringify(body) })
    $('#onboardTitle').textContent = state.onboarding.title
    $('#onboardText').textContent = state.onboarding.instructions
    $('#onboardURI').textContent = state.onboarding.uri || ''
    $('#qrBox').innerHTML = state.onboarding.uri ? `<img alt="QR" src="/api/v1/onboarding/qr?payload=${encodeURIComponent(state.onboarding.uri)}">` : '<span>QR</span>'
  } catch (error) { notice(error.message, true) }
}

$('#networksList').addEventListener('click', event => {
  const button = event.target.closest('button[data-action]')
  const card = event.target.closest('[data-network-id]')
  if (!button || !card) return
  const network = networkByID(card.dataset.networkId)
  if (!network) return
  if (button.dataset.action === 'edit-network') editNetwork(network)
  if (button.dataset.action === 'check-beacon') void checkBeacon(network)
  if (button.dataset.action === 'onboard-network') onboardNetwork(network.id)
})
$('#servicesList').addEventListener('click', event => {
  const button = event.target.closest('button[data-component-action]')
  if (button) void componentJob(button.dataset.agent, button.dataset.componentAction, button.dataset.component)
})
$('#loginForm').addEventListener('submit', async event => {
  event.preventDefault()
  try { await api('/api/v1/session', { method: 'POST', body: JSON.stringify({ password: $('#loginPassword').value }) }); $('#loginError').textContent = ''; await boot() }
  catch (error) { $('#loginError').textContent = error.message }
})
$('#logoutBtn').onclick = async () => { await api('/api/v1/session', { method: 'DELETE' }).catch(() => {}); await boot() }
$('#refreshBtn').onclick = refresh
$('#langBtn').onclick = () => { state.lang = state.lang === 'ru' ? 'en' : 'ru'; localStorage.krLang = state.lang; applyI18n() }
$$('#nav button').forEach(button => { button.onclick = () => showPage(button.dataset.page) })
$('#newNetworkBtn').onclick = () => editNetwork(null)
$('#registerBeaconBtn').onclick = registerExistingBeacon
$('#deployBeaconBtn').onclick = deployBeacon
$('#deploySidecarBtn').onclick = deploySidecar
$('#generateOnboardingBtn').onclick = generateOnboarding
$('#copyOnboardingBtn').onclick = () => navigator.clipboard.writeText([state.onboarding?.instructions, state.onboarding?.uri].filter(Boolean).join('\n\n'))

applyI18n()
void boot()
setInterval(() => { if (!$('#app').classList.contains('hidden')) void refresh() }, 15000)

$('#externalBeaconsList').addEventListener('click', event => {
  const button = event.target.closest('button[data-external-beacon]')
  if (!button) return
  void api(`/api/v1/beacons/check?url=${encodeURIComponent(button.dataset.externalBeacon)}`)
    .then(out => notice(`${tr('beaconHealthy')}: ${out.url} · ${out.latency_ms} ms`))
    .catch(error => notice(error.message, true))
})

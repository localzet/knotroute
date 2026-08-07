export const navigation = {
  ru: [
    { title: 'Начало', links: [
      { title: 'Обзор', href: '/ru', description: 'Что такое KnotRoute и с чего начать.' },
      { title: 'Основные понятия', href: '/ru/getting-started/concepts', description: 'Node, service, Beacon, relay, circuit, rendezvous и network_id.' },
      { title: 'Быстрый старт', href: '/ru/getting-started/quickstart', description: 'Минимальный путь от установки до первого .knot-сервиса.' },
    ]},
    { title: 'Клиенты', links: [
      { title: 'Windows', href: '/ru/client/windows', description: 'Desktop-клиент, PAC, CA и диагностика.' },
      { title: 'Android', href: '/ru/client/android', description: 'Профили сети, QR/deep-link и встроенный браузер.' },
      { title: 'Linux и macOS', href: '/ru/client/linux-macos', description: 'CLI-клиент, SOCKS5 и HTTP proxy.' },
      { title: 'HTTPS и локальный CA', href: '/ru/client/https', description: 'Почему .knot HTTPS работает без публичного WebPKI.' },
      { title: 'Проблемы подключения', href: '/ru/client/troubleshooting', description: 'Что проверять, если .knot не открывается.' },
    ]},
    { title: 'Self-hosted', links: [
      { title: 'Архитектура развёртывания', href: '/ru/self-hosted/overview', description: 'Control plane, data plane и рекомендуемая топология.' },
      { title: 'Первая сеть с нуля', href: '/ru/self-hosted/first-network', description: 'Network ID, Beacon, relay, Control и первый сервис.' },
      { title: 'Control', href: '/ru/self-hosted/control', description: 'Панель управления, onboarding и deployment jobs.' },
      { title: 'Agent', href: '/ru/self-hosted/agent', description: 'Подключение серверов и безопасное Docker-управление.' },
      { title: 'Beacon', href: '/ru/self-hosted/beacon', description: 'Discovery API и bootstrap relay.' },
      { title: 'Relay', href: '/ru/self-hosted/relay', description: 'Постоянные маршрутизирующие узлы сети.' },
      { title: 'Docker sidecar', href: '/ru/self-hosted/docker', description: 'Публикация контейнерного сервиса в .knot.' },
      { title: 'Traefik + KnotRoute', href: '/ru/self-hosted/traefik', description: 'Публичный HTTPS и .knot рядом.' },
      { title: 'Service identity и миграция', href: '/ru/self-hosted/service-identity', description: 'Стабильный адрес, перенос между серверами и резервные копии.' },
      { title: 'Мониторинг', href: '/ru/self-hosted/monitoring', description: 'Health checks, synthetic probes и признаки деградации.' },
      { title: 'Бэкапы и обновления', href: '/ru/self-hosted/backup-updates', description: 'Что сохранять и как обновлять без смены .knot.' },
      { title: 'Диагностический runbook', href: '/ru/self-hosted/diagnostics', description: 'Пошаговая проверка Control → Beacon → relay → client → sidecar → rendezvous.' },
      { title: 'Troubleshooting серверов', href: '/ru/self-hosted/troubleshooting', description: 'Beacon, sidecar, Agent, relay и Docker проблемы.' },
    ]},
    { title: 'Разработка', links: [
      { title: 'SDK: обзор', href: '/ru/development/sdk', description: 'Встраивание KnotRoute без отдельного клиента.' },
      { title: 'Go client/server SDK', href: '/ru/development/go', description: 'Dial, HTTP transport и публикация in-process сервисов.' },
      { title: 'Android AAR', href: '/ru/development/android-aar', description: 'gomobile binding и интеграция core в своё Android-приложение.' },
      { title: 'RPC, PUBSUB, OBJECT, mailbox', href: '/ru/development/primitives', description: 'Application primitives поверх KnotRoute.' },
    ]},
    { title: 'Справочник', links: [
      { title: 'Архитектура протокола', href: '/ru/reference/architecture', description: 'Circuits, directory, introduction и rendezvous.' },
      { title: 'Конфигурация', href: '/ru/reference/config', description: 'Поля knotroute.json и безопасные значения.' },
      { title: 'CLI', href: '/ru/reference/cli', description: 'Команды knotroute и типовые сценарии.' },
      { title: 'Порты и сетевые потоки', href: '/ru/reference/ports', description: 'Что слушает локально и что открывать наружу.' },
      { title: 'Безопасность', href: '/ru/security', description: 'Модель угроз и границы гарантий.' },
      { title: 'Ограничения v3', href: '/ru/reference/limitations', description: 'Что проект пока намеренно не обещает.' },
    ]},
  ],
  en: [
    { title: 'Start', links: [
      { title: 'Overview', href: '/en', description: 'What KnotRoute is and where to start.' },
      { title: 'Core concepts', href: '/en/getting-started/concepts', description: 'Node, service, Beacon, relay, circuit, rendezvous and network_id.' },
      { title: 'Quick start', href: '/en/getting-started/quickstart', description: 'Shortest path from installation to the first .knot service.' },
    ]},
    { title: 'Clients', links: [
      { title: 'Windows', href: '/en/client/windows', description: 'Desktop client, PAC, CA and diagnostics.' },
      { title: 'Android', href: '/en/client/android', description: 'Network profiles, QR/deep-link and embedded browser.' },
      { title: 'Linux and macOS', href: '/en/client/linux-macos', description: 'CLI client, SOCKS5 and HTTP proxy.' },
      { title: 'HTTPS and local CA', href: '/en/client/https', description: 'How .knot HTTPS works without public WebPKI.' },
      { title: 'Connection troubleshooting', href: '/en/client/troubleshooting', description: 'Checklist for a .knot service that does not open.' },
    ]},
    { title: 'Self-hosted', links: [
      { title: 'Deployment architecture', href: '/en/self-hosted/overview', description: 'Control plane, data plane and recommended topology.' },
      { title: 'First network from scratch', href: '/en/self-hosted/first-network', description: 'Network ID, Beacon, relay, Control and first service.' },
      { title: 'Control', href: '/en/self-hosted/control', description: 'Management UI, onboarding and deployment jobs.' },
      { title: 'Agent', href: '/en/self-hosted/agent', description: 'Attach servers and safely execute managed Docker jobs.' },
      { title: 'Beacon', href: '/en/self-hosted/beacon', description: 'Discovery API and bootstrap relay.' },
      { title: 'Relay', href: '/en/self-hosted/relay', description: 'Stable routing nodes for an overlay.' },
      { title: 'Docker sidecar', href: '/en/self-hosted/docker', description: 'Publish a container service to .knot.' },
      { title: 'Traefik + KnotRoute', href: '/en/self-hosted/traefik', description: 'Public HTTPS and .knot side by side.' },
      { title: 'Service identity and migration', href: '/en/self-hosted/service-identity', description: 'Stable addresses, migrations and backups.' },
      { title: 'Monitoring', href: '/en/self-hosted/monitoring', description: 'Health checks, synthetic probes and degradation signals.' },
      { title: 'Backups and upgrades', href: '/en/self-hosted/backup-updates', description: 'What to preserve and how to upgrade without changing .knot.' },
      { title: 'Diagnostic runbook', href: '/en/self-hosted/diagnostics', description: 'Layer-by-layer Control, Beacon, relay, client, sidecar and rendezvous checks.' },
      { title: 'Server troubleshooting', href: '/en/self-hosted/troubleshooting', description: 'Beacon, sidecar, Agent, relay and Docker failures.' },
    ]},
    { title: 'Development', links: [
      { title: 'SDK overview', href: '/en/development/sdk', description: 'Embed KnotRoute without a separate installed client.' },
      { title: 'Go client/server SDK', href: '/en/development/go', description: 'Dial, HTTP transport and in-process publishing.' },
      { title: 'Android AAR', href: '/en/development/android-aar', description: 'gomobile binding and embedding the core into Android.' },
      { title: 'RPC, PUBSUB, OBJECT, mailbox', href: '/en/development/primitives', description: 'Application primitives over KnotRoute.' },
    ]},
    { title: 'Reference', links: [
      { title: 'Protocol architecture', href: '/en/reference/architecture', description: 'Circuits, directory, introduction and rendezvous.' },
      { title: 'Configuration', href: '/en/reference/config', description: 'knotroute.json fields and safe defaults.' },
      { title: 'CLI', href: '/en/reference/cli', description: 'knotroute commands and common workflows.' },
      { title: 'Ports and network flows', href: '/en/reference/ports', description: 'Local listeners and externally reachable ports.' },
      { title: 'Security', href: '/en/security', description: 'Threat model and security boundaries.' },
      { title: 'v3 limitations', href: '/en/reference/limitations', description: 'What the project does not currently claim.' },
    ]},
  ],
}

export function flatNavigation(language) {
  return (navigation[language] || navigation.en).flatMap(section =>
    section.links.map(link => ({ ...link, section: section.title })),
  )
}

export function pageNeighbors(language, pathname) {
  const flat = flatNavigation(language)
  const index = flat.findIndex(item => item.href === pathname)
  return {
    previous: index > 0 ? flat[index - 1] : null,
    next: index >= 0 && index < flat.length - 1 ? flat[index + 1] : null,
  }
}

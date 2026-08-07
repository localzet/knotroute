export const navigation = {
  ru: [
    { title: 'Начало', links: [
      { title: 'Что такое KnotRoute', href: '/ru' },
      { title: 'Windows-клиент', href: '/ru/client/windows' },
      { title: 'Android-клиент', href: '/ru/client/android' },
    ]},
    { title: 'Self-hosted', links: [
      { title: 'Схема развёртывания', href: '/ru/self-hosted/overview' },
      { title: 'Control и Agent', href: '/ru/self-hosted/control' },
      { title: 'Beacon и relays', href: '/ru/self-hosted/beacon' },
      { title: 'Docker sidecar', href: '/ru/self-hosted/docker' },
      { title: 'Traefik + KnotRoute', href: '/ru/self-hosted/traefik' },
    ]},
    { title: 'Разработка', links: [
      { title: 'SDK и embedded client', href: '/ru/development/sdk' },
      { title: 'Безопасность', href: '/ru/security' },
    ]},
  ],
  en: [
    { title: 'Start', links: [
      { title: 'What is KnotRoute', href: '/en' },
      { title: 'Windows client', href: '/en/client/windows' },
      { title: 'Android client', href: '/en/client/android' },
    ]},
    { title: 'Self-hosted', links: [
      { title: 'Deployment overview', href: '/en/self-hosted/overview' },
      { title: 'Control and Agent', href: '/en/self-hosted/control' },
      { title: 'Beacon and relays', href: '/en/self-hosted/beacon' },
      { title: 'Docker sidecar', href: '/en/self-hosted/docker' },
      { title: 'Traefik + KnotRoute', href: '/en/self-hosted/traefik' },
    ]},
    { title: 'Development', links: [
      { title: 'SDK and embedded client', href: '/en/development/sdk' },
      { title: 'Security', href: '/en/security' },
    ]},
  ],
}

export function flatNavigation(language) {
  return (navigation[language] || navigation.en).flatMap(section =>
    section.links.map(link => ({ ...link, section: section.title })),
  )
}

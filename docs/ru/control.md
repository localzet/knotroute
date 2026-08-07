# KnotRoute Control и Agent

## Control

`knotroute-control` — optional control plane. Он не маршрутизирует KnotRoute traffic и не является обязательным для работы overlay.

Функции:

- network profiles;
- Agent inventory/health;
- Beacon и sidecar deployment jobs;
- managed component restart/remove;
- service inventory;
- onboarding generator;
- QR/deep-link generation;
- RU/EN web UI.

Обязательные переменные:

```text
KNOTROUTE_CONTROL_ADMIN_PASSWORD
KNOTROUTE_CONTROL_ENROLL_TOKEN
```

Размещайте панель только через HTTPS.

## Agent

При первом старте Agent генерирует Ed25519 identity. Enrollment использует отдельный token; после регистрации каждый heartbeat/job request подписывается identity агента.

```yaml
agent:
  image: ghcr.io/localzet/knotroute-agent:3.1.0
  environment:
    KNOTROUTE_CONTROL_URL: "https://control.example.net"
    KNOTROUTE_CONTROL_ENROLL_TOKEN: "..."
    KNOTROUTE_AGENT_DOCKER: "true"
  volumes:
    - agent-data:/data
    - /var/run/docker.sock:/var/run/docker.sock
```

Agent не открывает management HTTP port наружу.

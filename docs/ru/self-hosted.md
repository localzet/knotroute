# Полностью self-hosted KnotRoute

Базовая production-схема:

```text
                  KnotRoute Control
                       ^
                       | signed agent polling
          +------------+------------+
          |                         |
       Agent A                    Agent B
          |                         |
  Beacon + relay              sidecars/relays
          |                         |
          +--------- overlay -------+
```

## 1. Network ID

Создайте отдельный Network ID:

```bash
knotroute network create
```

`network_id` — namespace/isolation identifier, а не пароль членства.

## 2. Control + Docs

Разместите оба web-контейнера за HTTPS reverse proxy.

## 3. Agents

На каждом управляемом Docker-host запустите Agent. Remote Agent сам подключается к публичному HTTPS Control URL.

Для Docker deployment agent'у требуется `/var/run/docker.sock`. Это высокий уровень привилегий, поэтому socket никогда не монтируется в Control.

## 4. Beacon

Через Control создайте минимум два Beacon/relay на разных failure domains.

- HTTPS endpoint — discovery API;
- `7447/tcp` — raw KnotRoute relay endpoint.

## 5. Services

В Control выберите Agent → Docker network → `container:port`. Agent создаст отдельный managed sidecar stack.

## 6. Onboarding

Control генерирует инструкции и `knotroute://join` QR/deep-link для Windows, Android, Linux, Docker и SDK.

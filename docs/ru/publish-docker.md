# Опубликовать Docker-сервис

Рекомендуемый способ — `knotroute-sidecar` в той же Docker network, что и приложение.

```yaml
services:
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
  knotroute-data:
```

Если приложение должно быть только `.knot`-сервисом, host `ports:` ему вообще не нужен.

Если оно уже опубликовано через Traefik, оставьте текущую proxy network и добавьте вторую private network для KnotRoute sidecar.

`/data` содержит service identity. Не удаляйте volume при обычном redeploy/update.

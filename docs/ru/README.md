# KnotRoute — русская документация

Эта папка — краткая task-first документация, которая также публикуется отдельным контейнером `ghcr.io/localzet/knotroute-docs`.

Начинайте не с протокола, а с задачи:

- [Подключить Windows/Android-клиент](clients.md)
- [Опубликовать Docker-сервис](publish-docker.md)
- [Развернуть собственную KnotRoute-сеть](self-hosted.md)
- [Управлять инфраструктурой через KnotRoute Control](control.md)
- [Резервные копии и security baseline](security.md)

Полный интерактивный сайт документации запускается контейнером:

```bash
docker run --rm -p 8080:8080 ghcr.io/localzet/knotroute-docs:3.1.0
```

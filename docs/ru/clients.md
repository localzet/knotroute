# Клиенты

## Windows

1. Распакуйте release archive.
2. Запустите `Install-KnotRoute.ps1`.
3. Откройте `knotroute-desktop.exe`.
4. Добавьте Network ID и Beacon URL из профиля сети.
5. В tray включите `.knot` integration.
6. При запросе подтвердите установку локального KnotRoute Root CA текущего пользователя.
7. Откройте `https://<service-identity>.knot/`.

KnotRoute по умолчанию не создаёт TUN. Обычный трафик остаётся в системном routing stack.

## Android

Android-клиент содержит KnotRoute core внутри APK.

Поддерживаются:

- RU/EN UI;
- несколько network profiles;
- `knotroute://join?...` deep links;
- QR onboarding через системный QR scanner/camera;
- встроенный `.knot` browser;
- foreground overlay service;
- установка локального CA через штатный Android UI;
- diagnostics screen.

QR содержит только Network ID, имя профиля и Beacon URL. Приватные ключи через него не передаются.

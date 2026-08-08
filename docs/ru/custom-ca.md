# Собственный Root CA для `.knot`

KnotRoute завершает браузерный TLS локально: браузер видит сертификат конкретного `.knot`, подписанный локальным Root CA клиента, а HTTP внутри уже передаётся через зашифрованную KnotRoute-сессию.

## Что можно изменить

В `ca.subject` доступны:

- `common_name` — Common Name;
- `organization` — Organization;
- `organizational_unit` — Organizational Unit;
- `country` — Country;
- `province` — State/Province;
- `locality` — Locality;
- `street_address` — адрес;
- `postal_code` — индекс.

`validity_days` задаёт срок действия root.

Пример:

```json
"ca": {
  "enabled": true,
  "directory": "ca",
  "intercept_https": true,
  "validity_days": 1825,
  "subject": {
    "common_name": "Localzet KnotRoute Root CA",
    "organization": ["Localzet"],
    "organizational_unit": ["Private Network"],
    "country": ["RU"],
    "province": ["Moscow"],
    "locality": ["Moscow"]
  }
}
```

## Почему Issuer нельзя задать отдельно

Root CA — self-signed certificate. Поэтому для него `Issuer == Subject`: сертификат подписывает сам себя. Для выдаваемого `.knot` leaf-сертификата Issuer уже будет Subject этого Root CA.

## Как изменить уже созданный Root CA

Нельзя отредактировать подписанный X.509 сертификат на месте. Сначала сохраните новый профиль, затем выполните явную ротацию:

```powershell
knotroute-cli.exe ca info --config "$env:LOCALAPPDATA\KnotRoute\knotroute.json"
knotroute-cli.exe ca rotate --yes --config "$env:LOCALAPPDATA\KnotRoute\knotroute.json"
```

После CLI-ротации перезапустите KnotRoute и установите новый сертификат. В Windows v4 UI кнопка «Перевыпустить Root CA» делает ротацию, устанавливает новый root текущему пользователю и перезапускает встроенный runtime.

## Android / Xiaomi

На Android 11+ приложение не может установить CA через старый `KeyChain.createInstallIntent()`. KnotRoute экспортирует `.crt`, после чего сертификат нужно установить через системные настройки безопасности. На Xiaomi/HyperOS название пунктов зависит от версии прошивки; искать нужно установку сертификата CA из хранилища устройства.

Если конкретный браузер не доверяет пользовательским CA, это ограничение политики браузера/Android, а не возможность KnotRoute обойти системное хранилище без root/device-owner управления.

## Никогда не копируйте private key

Файл `root-ca-key.pem` даёт возможность подписывать сертификаты, которым доверяет устройство. Его нельзя раздавать друзьям или класть в Docker image/репозиторий. Между устройствами распространяют только публичный root certificate, если архитектура действительно предполагает общий CA; текущая модель KnotRoute безопаснее как локальный CA на каждом клиенте.

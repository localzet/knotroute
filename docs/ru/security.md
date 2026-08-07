# Security baseline и backup

## Не путать свойства

- `network_id` не является membership password.
- Неизвестный `.knot` address не является access control.
- Hidden service не должен полагаться на node-ID ACL клиента: используйте application authentication.
- KnotRoute v3/v3.1 не следует рекламировать как Tor-equivalent anonymity.

## Что резервировать

1. service identity volume/file;
2. node identity, если важен стабильный node ID;
3. Control `/data`;
4. Agent `/data`;
5. canonical network profile и список Beacon.

## Docker socket

Docker socket даёт почти административный доступ к хосту. Его получает только `knotroute-agent`, когда требуется автоматический deployment. Control никогда не получает Docker socket.

#!/usr/bin/env sh
set -eu
BINARY="${1:-./knotroute}"
CONFIG_DIR="/etc/knotroute"
STATE_DIR="/var/lib/knotroute"
id knotroute >/dev/null 2>&1 || useradd --system --home "$STATE_DIR" --shell /usr/sbin/nologin knotroute
install -m 0755 "$BINARY" /usr/local/bin/knotroute
install -d -m 0750 -o knotroute -g knotroute "$CONFIG_DIR" "$STATE_DIR"
if [ ! -f "$CONFIG_DIR/knotroute.json" ]; then
  /usr/local/bin/knotroute init --config "$CONFIG_DIR/knotroute.json"
  chown knotroute:knotroute "$CONFIG_DIR/knotroute.json" "$CONFIG_DIR/identity.json"
  chmod 0600 "$CONFIG_DIR/identity.json"
fi
install -m 0644 "$(dirname "$0")/knotroute.service" /etc/systemd/system/knotroute.service
systemctl daemon-reload
systemctl enable --now knotroute

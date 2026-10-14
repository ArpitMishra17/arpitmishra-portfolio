#!/usr/bin/env bash
# Build the TUI for Linux and push it to the server.
#
#   TUI_HOST=ubuntu@<ip> ./deploy/deploy.sh
#
# Env: TUI_HOST (required), SSH_PORT (default 2222), GOARCH (default arm64;
# use amd64 for the VM.Standard.E2.1.Micro shape).
set -euo pipefail

: "${TUI_HOST:?set TUI_HOST=ubuntu@<server-ip>}"
SSH_PORT=${SSH_PORT:-2222}
GOARCH=${GOARCH:-arm64}

cd "$(dirname "$0")/.."

echo "building linux/$GOARCH..."
CGO_ENABLED=0 GOOS=linux GOARCH=$GOARCH go build -trimpath -ldflags="-s -w" -o deploy/tui-linux .

ssh_() { ssh -p "$SSH_PORT" "$TUI_HOST" "$@"; }

echo "uploading..."
scp -P "$SSH_PORT" -q deploy/tui-linux "$TUI_HOST":/tmp/tui.new

# Ship the local host key on first deploy so the fingerprint matches what
# you've been testing with; never overwrite one already on the server.
if ! ssh_ sudo test -f /opt/tui/.ssh/host_ed25519 && [[ -f .ssh/host_ed25519 ]]; then
	echo "installing host key..."
	scp -P "$SSH_PORT" -q .ssh/host_ed25519 .ssh/host_ed25519.pub "$TUI_HOST":/tmp/
	ssh_ 'sudo install -o tui -g tui -m 600 /tmp/host_ed25519 /opt/tui/.ssh/host_ed25519 &&
		sudo install -o tui -g tui -m 644 /tmp/host_ed25519.pub /opt/tui/.ssh/host_ed25519.pub &&
		rm /tmp/host_ed25519 /tmp/host_ed25519.pub'
fi

echo "restarting service..."
ssh_ 'sudo install -o root -g root -m 755 /tmp/tui.new /opt/tui/tui && rm /tmp/tui.new &&
	sudo systemctl restart tui && sleep 1 && systemctl is-active tui'

rm deploy/tui-linux
echo "done -> ssh ${TUI_HOST#*@}"

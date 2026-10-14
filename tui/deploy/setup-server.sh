#!/usr/bin/env bash
# One-time server setup for an Oracle Cloud Ubuntu 24.04 VM. Run as root.
# Run it next to tui.service (stage2 installs that file from this directory).
#
#   stage1  open :2222 in the host firewall and make sshd listen on 22 AND 2222
#           -> then verify `ssh -p 2222 ubuntu@<ip>` works before stage2
#   stage2  drop sshd from :22, create the tui user/dir, install the service
set -euo pipefail

[[ $EUID -eq 0 ]] || { echo "run with sudo" >&2; exit 1; }

ADMIN_PORT=2222
SSHD_DROPIN=/etc/ssh/sshd_config.d/10-ports.conf

reload_sshd() {
	# 24.04 uses socket activation; the socket's ports are generated from sshd_config.
	systemctl daemon-reload
	if systemctl is-enabled --quiet ssh.socket 2>/dev/null; then
		systemctl restart ssh.socket
	fi
	systemctl restart ssh.service
}

stage1() {
	# The E2.1.Micro shape has 1 GB RAM and no swap; give apt & co. some headroom.
	if ! swapon --show | grep -q /swapfile; then
		fallocate -l 1G /swapfile
		chmod 600 /swapfile
		mkswap /swapfile
		swapon /swapfile
		grep -q '^/swapfile' /etc/fstab || echo '/swapfile none swap sw 0 0' >>/etc/fstab
	fi

	# Oracle's Ubuntu images ship iptables rules ending in a REJECT; insert ahead of it.
	for port in 22 "$ADMIN_PORT"; do
		iptables -C INPUT -p tcp --dport "$port" -m conntrack --ctstate NEW -j ACCEPT 2>/dev/null ||
			iptables -I INPUT -p tcp --dport "$port" -m conntrack --ctstate NEW -j ACCEPT
	done
	if command -v netfilter-persistent >/dev/null; then
		netfilter-persistent save
	else
		DEBIAN_FRONTEND=noninteractive apt-get install -y iptables-persistent
		netfilter-persistent save
	fi

	printf 'Port 22\nPort %s\n' "$ADMIN_PORT" >"$SSHD_DROPIN"
	sshd -t
	reload_sshd
	ss -tlnp | grep -E ":(22|$ADMIN_PORT)\b" || true
	echo
	echo "stage1 done. From your laptop, confirm: ssh -p $ADMIN_PORT ubuntu@<ip>"
	echo "Only then run stage2."
}

stage2() {
	printf 'Port %s\n' "$ADMIN_PORT" >"$SSHD_DROPIN"
	sshd -t
	reload_sshd

	id tui >/dev/null 2>&1 || useradd --system --home-dir /opt/tui --shell /usr/sbin/nologin tui
	install -d -o tui -g tui -m 755 /opt/tui
	install -d -o tui -g tui -m 700 /opt/tui/.ssh

	install -m 644 "$(dirname "$0")/tui.service" /etc/systemd/system/tui.service
	systemctl daemon-reload
	systemctl enable tui.service
	echo "stage2 done. sshd is on :$ADMIN_PORT only; tui.service is enabled (starts once the binary is deployed)."
}

case "${1:-}" in
stage1) stage1 ;;
stage2) stage2 ;;
*) echo "usage: sudo $0 stage1|stage2" >&2; exit 1 ;;
esac

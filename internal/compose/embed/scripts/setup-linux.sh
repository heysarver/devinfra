#!/usr/bin/env bash
set -euo pipefail

info()  { echo -e "\033[0;36m[INFO]\033[0m $*"; }
ok()    { echo -e "\033[0;32m[OK]\033[0m $*"; }
warn()  { echo -e "\033[0;33m[WARN]\033[0m $*"; }
fail()  { echo -e "\033[0;31m[FAIL]\033[0m $*"; }

info "Running Linux setup..."

info "Installing mkcert and NSS tools..."
sudo apt-get update -qq
sudo apt-get install -y -qq libnss3-tools mkcert

info "Installing local CA..."
mkcert -install

info "Configuring DNS for .test domains..."

if ! systemctl is-active --quiet NetworkManager 2>/dev/null; then
  warn "NetworkManager not active. Using standalone dnsmasq approach."

  sudo mkdir -p /etc/systemd/resolved.conf.d
  sudo tee /etc/systemd/resolved.conf.d/disable-stub.conf > /dev/null <<'EOF'
[Resolve]
DNSStubListener=no
DNS=127.0.0.1
EOF
  sudo ln -sf /run/systemd/resolve/resolv.conf /etc/resolv.conf

  sudo apt-get install -y -qq dnsmasq
  sudo tee /etc/dnsmasq.d/test-domain.conf > /dev/null <<'EOF'
address=/test/127.0.0.1
server=8.8.8.8
server=8.8.4.4
EOF
  sudo systemctl restart systemd-resolved
  sudo systemctl enable --now dnsmasq

else
  if ! grep -q "dns=dnsmasq" /etc/NetworkManager/NetworkManager.conf 2>/dev/null; then
    sudo sed -i '/^\[main\]/a dns=dnsmasq' /etc/NetworkManager/NetworkManager.conf
  fi

  sudo mkdir -p /etc/NetworkManager/dnsmasq.d
  sudo tee /etc/NetworkManager/dnsmasq.d/local-dns.conf > /dev/null <<'EOF'
listen-address=127.0.0.2
bind-interfaces
EOF

  sudo tee /etc/NetworkManager/dnsmasq.d/test-domain.conf > /dev/null <<'EOF'
address=/test/127.0.0.1
EOF

  sudo mkdir -p /etc/systemd/resolved.conf.d
  sudo tee /etc/systemd/resolved.conf.d/test-dns.conf > /dev/null <<'EOF'
[Resolve]
DNS=127.0.0.2
Domains=~test
EOF

  sudo systemctl restart NetworkManager
  sudo systemctl restart systemd-resolved
fi

info "Note: On Linux, host-mode projects require 'extra_hosts: host.docker.internal:host-gateway'"
ok "Linux setup complete."

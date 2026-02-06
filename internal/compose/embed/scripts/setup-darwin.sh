#!/usr/bin/env bash
set -euo pipefail

info()  { echo -e "\033[0;36m[INFO]\033[0m $*"; }
ok()    { echo -e "\033[0;32m[OK]\033[0m $*"; }
warn()  { echo -e "\033[0;33m[WARN]\033[0m $*"; }
fail()  { echo -e "\033[0;31m[FAIL]\033[0m $*"; }

DNS_PORT="${DNS_PORT:-5354}"

info "Running macOS setup..."

if ! command -v brew &>/dev/null; then
  fail "Homebrew is required. Install from https://brew.sh"
  exit 1
fi

info "Installing dependencies..."
brew install mkcert nss 2>/dev/null || brew upgrade mkcert nss 2>/dev/null || true

info "Installing local CA..."
mkcert -install

info "Configuring macOS DNS resolver for .test domains..."

sudo mkdir -p /etc/resolver
printf "nameserver 127.0.0.1\nport %s\n" "$DNS_PORT" | sudo tee /etc/resolver/test > /dev/null
sudo dscacheutil -flushcache
sudo killall -HUP mDNSResponder 2>/dev/null || true

ok "macOS setup complete."

#!/usr/bin/env bash
# Run once on a fresh VPS to install Docker and clone the repo.
# Usage: bash setup.sh <your-github-ssh-private-key-path>
set -euo pipefail

REPO="git@github.com:padington/tgbase.git"
DEPLOY_DIR="/opt/tgbase"

echo "==> Installing Docker"
curl -fsSL https://get.docker.com | sh
systemctl enable --now docker

echo "==> Cloning repo to $DEPLOY_DIR"
git clone "$REPO" "$DEPLOY_DIR"

echo "==> Done. Add your GitHub Actions SSH key to ~/.ssh/authorized_keys,"
echo "    then set the four secrets in the repo settings:"
echo "      VPS_HOST, VPS_USER, VPS_SSH_KEY, TELEGRAM_BOT_TOKEN"

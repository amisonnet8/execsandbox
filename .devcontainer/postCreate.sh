#!/usr/bin/env bash
set -euo pipefail

sudo apt-get update
sudo apt-get install -y make wabt wget apt-transport-https gnupg lsb-release

# trivy: 依存ライブラリの脆弱性・ライセンスチェック(.claude/rules/testing.md)。
# aquasecurity公式のaptリポジトリからインストールする
# (以前使っていたdevcontainer feature `ghcr.io/dhoeric/features/trivy` は
# 個人のGitHub名前空間で公開された非公式のcommunity featureだったため
# 使わないことにした)。
wget -qO - https://aquasecurity.github.io/trivy-repo/deb/public.key | gpg --dearmor | sudo tee /usr/share/keyrings/trivy.gpg >/dev/null
echo "deb [signed-by=/usr/share/keyrings/trivy.gpg] https://aquasecurity.github.io/trivy-repo/deb $(lsb_release -sc) main" | sudo tee /etc/apt/sources.list.d/trivy.list
sudo apt-get update
sudo apt-get install -y trivy

go install golang.org/x/tools/gopls@latest
go install golang.org/x/tools/cmd/goimports@latest

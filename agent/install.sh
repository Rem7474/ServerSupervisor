#!/bin/bash
# ============================================
# ServerSupervisor Agent - Installation Script
# ============================================
# Usage (one-liner from GitHub):
#   curl -sSL https://raw.githubusercontent.com/Rem7474/ServerSupervisor/main/agent/install.sh | sudo bash -s -- --server-url http://your-server:8080 --api-key your-key
#
# Or download and run manually:
#   chmod +x install.sh
#   sudo ./install.sh --server-url http://your-server:8080 --api-key your-key

set -e

GITHUB_REPO="Rem7474/ServerSupervisor"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/serversupervisor"
AGENT_BINARY="serversupervisor-agent"
REPORT_INTERVAL=30
COLLECT_DOCKER=true
COLLECT_APT=true
SERVER_URL=""
API_KEY=""

while [[ $# -gt 0 ]]; do
  case $1 in
    --server-url) SERVER_URL="$2"; shift 2 ;;
    --api-key)    API_KEY="$2";    shift 2 ;;
    --interval)   REPORT_INTERVAL="$2"; shift 2 ;;
    --no-docker)  COLLECT_DOCKER=false; shift ;;
    --no-apt)     COLLECT_APT=false;    shift ;;
    *) echo "Unknown option: $1"; exit 1 ;;
  esac
done

if [ -z "$SERVER_URL" ] || [ -z "$API_KEY" ]; then
  echo "Usage: $0 --server-url <url> --api-key <key>"
  echo ""
  echo "Options:"
  echo "  --server-url   Server URL (required)"
  echo "  --api-key      API key from server registration (required)"
  echo "  --interval     Report interval in seconds (default: 30)"
  echo "  --no-docker    Disable Docker monitoring"
  echo "  --no-apt       Disable APT monitoring"
  exit 1
fi

if [ "$(id -u)" -ne 0 ]; then
  echo "This script must be run as root (use sudo)"
  exit 1
fi

echo "=== ServerSupervisor Agent Installer ==="
echo "Server: $SERVER_URL"
echo ""

# Detect OS and architecture
ARCH=$(uname -m)
case $ARCH in
  x86_64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  armv7l|armv7)  ARCH="armv7" ;;
  armv6l|armv6)  ARCH="armv6" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
if [ "$OS" != "linux" ]; then
  echo "Unsupported OS: $OS (only linux is supported)"
  exit 1
fi

echo "Detected: $OS/$ARCH"

# Download binary from GitHub releases
# CI release assets are named: agent-linux-amd64.gz, agent-linux-arm64.gz, agent-linux-armv7.gz, agent-linux-armv6.gz
BINARY_NAME="agent-${OS}-${ARCH}.gz"
DOWNLOAD_URL="https://github.com/${GITHUB_REPO}/releases/latest/download/${BINARY_NAME}"
TMP_GZ="/tmp/${BINARY_NAME}"

echo "Downloading agent binary..."
if command -v curl &>/dev/null; then
  curl -fsSL "$DOWNLOAD_URL" -o "$TMP_GZ"
elif command -v wget &>/dev/null; then
  wget -qO "$TMP_GZ" "$DOWNLOAD_URL"
else
  echo "Error: curl or wget is required"
  exit 1
fi

if ! command -v gunzip &>/dev/null; then
  echo "Error: gunzip is required to extract release binary"
  exit 1
fi

gunzip -c "$TMP_GZ" > "$INSTALL_DIR/$AGENT_BINARY"
rm -f "$TMP_GZ"

chmod +x "$INSTALL_DIR/$AGENT_BINARY"
echo "Binary installed: $INSTALL_DIR/$AGENT_BINARY"

# Create config directory and file
mkdir -p "$CONFIG_DIR"

cat > "$CONFIG_DIR/agent.yaml" <<EOF
server_url: "$SERVER_URL"
api_key: "$API_KEY"
report_interval: $REPORT_INTERVAL
collect_docker: $COLLECT_DOCKER
collect_apt: $COLLECT_APT
insecure_skip_verify: false
EOF

chmod 600 "$CONFIG_DIR/agent.yaml"
echo "Config written: $CONFIG_DIR/agent.yaml"

# Create systemd service
cat > /etc/systemd/system/serversupervisor-agent.service <<EOF
[Unit]
Description=ServerSupervisor Agent
After=network-online.target docker.service
Wants=network-online.target

[Service]
Type=simple
ExecStart=$INSTALL_DIR/$AGENT_BINARY --config $CONFIG_DIR/agent.yaml
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

echo "Systemd service created"

# Enable and start
systemctl daemon-reload
systemctl enable serversupervisor-agent
systemctl start serversupervisor-agent

echo ""
echo "=== Installation complete ==="
echo ""
echo "Check status:  sudo systemctl status serversupervisor-agent"
echo "View logs:     sudo journalctl -u serversupervisor-agent -f"

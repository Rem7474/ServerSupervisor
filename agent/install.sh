#!/bin/bash
# ============================================
# ServerSupervisor Agent - Installation Script
# ============================================
# Usage: curl -sSL https://your-server/install.sh | sudo bash -s -- --server-url http://your-server:8080 --api-key your-key
#
# Or download and run manually:
#   chmod +x install.sh
#   sudo ./install.sh --server-url http://your-server:8080 --api-key your-key

set -e

# Default values
SERVER_URL=""
API_KEY=""
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/serversupervisor"
AGENT_BINARY="serversupervisor-agent"
REPORT_INTERVAL=30
COLLECT_DOCKER=true
COLLECT_APT=true

# Parse arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --server-url) SERVER_URL="$2"; shift 2 ;;
    --api-key) API_KEY="$2"; shift 2 ;;
    --interval) REPORT_INTERVAL="$2"; shift 2 ;;
    --no-docker) COLLECT_DOCKER=false; shift ;;
    --no-apt) COLLECT_APT=false; shift ;;
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

echo "=== ServerSupervisor Agent Installer ==="
echo "Server: $SERVER_URL"
echo ""

# Detect architecture
ARCH=$(uname -m)
case $ARCH in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  armv7l)  ARCH="arm" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
echo "Detected: $OS/$ARCH"

# Create config directory
mkdir -p "$CONFIG_DIR"

# Create config file
cat > "$CONFIG_DIR/agent.yaml" <<EOF
server_url: "$SERVER_URL"
api_key: "$API_KEY"
report_interval: $REPORT_INTERVAL
collect_docker: $COLLECT_DOCKER
collect_apt: $COLLECT_APT
insecure_skip_verify: false
EOF

chmod 600 "$CONFIG_DIR/agent.yaml"
echo "Config written to $CONFIG_DIR/agent.yaml"

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

# Note: The binary must be compiled and placed manually
echo ""
echo "=== Installation complete ==="
echo ""
echo "Next steps:"
echo "1. Build the agent binary for $OS/$ARCH:"
echo "   cd agent && GOOS=$OS GOARCH=$ARCH go build -o $AGENT_BINARY ./cmd/agent"
echo ""
echo "2. Copy the binary to the target machine:"
echo "   scp $AGENT_BINARY user@host:$INSTALL_DIR/"
echo ""
echo "3. Start the agent:"
echo "   sudo systemctl daemon-reload"
echo "   sudo systemctl enable --now serversupervisor-agent"
echo ""
echo "4. Check status:"
echo "   sudo systemctl status serversupervisor-agent"
echo "   sudo journalctl -u serversupervisor-agent -f"

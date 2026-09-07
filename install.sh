#!/usr/bin/env bash
# Dokidoki Host Installer
# Bootstraps Dokidoki's own Docker Compose stack in the stacks directory and starts it.

set -euo pipefail

# ANSI color codes (disabled if not a tty)
if [ -t 1 ]; then
  BOLD="$(printf '\033[1m')"
  GREEN="$(printf '\033[32m')"
  YELLOW="$(printf '\033[33m')"
  RED="$(printf '\033[31m')"
  CYAN="$(printf '\033[36m')"
  DIM="$(printf '\033[2m')"
  RESET="$(printf '\033[0m')"
else
  BOLD=""
  GREEN=""
  YELLOW=""
  RED=""
  CYAN=""
  DIM=""
  RESET=""
fi

info() {
  printf "${CYAN}${BOLD}==>${RESET} %s\n" "$1"
}

success() {
  printf "${GREEN}${BOLD}✓${RESET} %s\n" "$1"
}

warn() {
  printf "${YELLOW}${BOLD}!${RESET} %s\n" "$1"
}

error() {
  printf "${RED}${BOLD}x${RESET} %s\n" "$1" >&2
  exit 1
}

# Resolve default host name from current machine
CURRENT_HOSTNAME="$(hostname 2>/dev/null || cat /etc/hostname 2>/dev/null || uname -n 2>/dev/null || echo "dokidoki-node")"

# Default configurations
STACKS_DIR="${DOKIDOKI_STACKS_DIR:-/opt/stacks}"
PORT="${DOKIDOKI_PORT:-}"
PORT_SPECIFIED=false
if [ -n "$PORT" ]; then
  PORT_SPECIFIED=true
else
  PORT="8080"
fi
NODE_NAME="${DOKIDOKI_NODE_NAME:-$CURRENT_HOSTNAME}"
PEERS="${DOKIDOKI_PEERS:-}"
CLUSTER_TOKEN="${DOKIDOKI_CLUSTER_TOKEN:-}"
ADVERTISE_ADDR="${DOKIDOKI_ADVERTISE_ADDR:-}"
ENABLE_MDNS="${DOKIDOKI_ENABLE_MDNS:-true}"
IMAGE="${DOKIDOKI_IMAGE:-ghcr.io/chickenzord/dokidoki:latest}"
DOCKER_SOCKET="${DOCKER_SOCKET:-/var/run/docker.sock}"

INTERACTIVE=true
NON_INTERACTIVE=false

# Auto-detect headless or non-interactive environments
if [ "${CI:-}" = "true" ] || [ "${DEBIAN_FRONTEND:-}" = "noninteractive" ]; then
  INTERACTIVE=false
  NON_INTERACTIVE=true
elif [ ! -t 0 ] && [ ! -c /dev/tty ]; then
  INTERACTIVE=false
  NON_INTERACTIVE=true
fi

show_help() {
  cat << EOF
Dokidoki Installer
Bootstraps Dokidoki Docker Compose stack into the host stacks directory.

USAGE:
  ./install.sh [OPTIONS]

OPTIONS:
  -i, --interactive            Interactively prompt and configure settings (default)
  -y, --yes, --non-interactive Non-interactive mode (proceed with defaults without prompting)
  -s, --stacks-dir <dir>       Directory for Compose stacks (default: /opt/stacks)
  -p, --port <port>            Port to listen on (default: 8080 or next free port)
  -n, --node-name <name>       Node name for the cluster (default: current hostname '${CURRENT_HOSTNAME}')
      --peers <urls>           Comma-separated bootstrap seed peer URLs
      --token <token>          Cluster security token for authorized peering
      --advertise-addr <urls>  Comma-separated candidate addresses to advertise (default: auto-detected host IP)
      --disable-mdns           Disable LAN mDNS discovery (default: enabled)
      --image <image>          Dokidoki container image (default: ghcr.io/chickenzord/dokidoki:latest)
      --socket <path>          Path to Docker socket (default: /var/run/docker.sock)
  -h, --help                   Show this help message

ENVIRONMENT VARIABLES:
  DOKIDOKI_STACKS_DIR, DOKIDOKI_PORT, DOKIDOKI_NODE_NAME, DOKIDOKI_PEERS,
  DOKIDOKI_CLUSTER_TOKEN, DOKIDOKI_ADVERTISE_ADDR, DOKIDOKI_ENABLE_MDNS, DOKIDOKI_IMAGE
EOF
  exit 0
}

# Parse command-line flags
while [[ $# -gt 0 ]]; do
  case "$1" in
    -i|--interactive)
      INTERACTIVE=true
      NON_INTERACTIVE=false
      shift
      ;;
    -y|--yes|--non-interactive)
      NON_INTERACTIVE=true
      INTERACTIVE=false
      shift
      ;;
    -s|--stacks-dir)
      STACKS_DIR="$2"
      shift 2
      ;;
    -p|--port)
      PORT="$2"
      PORT_SPECIFIED=true
      shift 2
      ;;
    -n|--node-name)
      NODE_NAME="$2"
      shift 2
      ;;
    --peers)
      PEERS="$2"
      shift 2
      ;;
    --token)
      CLUSTER_TOKEN="$2"
      shift 2
      ;;
    --advertise-addr)
      ADVERTISE_ADDR="$2"
      shift 2
      ;;
    --disable-mdns)
      ENABLE_MDNS="false"
      shift
      ;;
    --image)
      IMAGE="$2"
      shift 2
      ;;
    --socket)
      DOCKER_SOCKET="$2"
      shift 2
      ;;
    -h|--help)
      show_help
      ;;
    *)
      error "Unknown option: $1 (run with --help for usage)"
      ;;
  esac
done

# Helper: prompt user for input with default
prompt_input() {
  local prompt_label="$1"
  local current_val="$2"
  local input=""

  if [ -c /dev/tty ] && exec 3</dev/tty 2>/dev/null; then
    exec 3<&-
    printf "${BOLD}%s${RESET} [%s]: " "$prompt_label" "$current_val" >/dev/tty
    read -r input </dev/tty || true
  elif [ -t 0 ]; then
    printf "${BOLD}%s${RESET} [%s]: " "$prompt_label" "$current_val"
    read -r input || true
  else
    input="$current_val"
  fi

  if [ -n "$input" ]; then
    echo "$input"
  else
    echo "$current_val"
  fi
}

# Helper: prompt confirmation (Y/n)
prompt_confirm() {
  local prompt_label="$1"
  local default_choice="${2:-Y}"
  local answer=""

  if [ "$NON_INTERACTIVE" = true ]; then
    return 0
  fi

  if [ -c /dev/tty ] && exec 3</dev/tty 2>/dev/null; then
    exec 3<&-
    printf "${BOLD}%s${RESET} [%s]: " "$prompt_label" "$default_choice" >/dev/tty
    read -r answer </dev/tty || true
  elif [ -t 0 ]; then
    printf "${BOLD}%s${RESET} [%s]: " "$prompt_label" "$default_choice"
    read -r answer || true
  else
    # Headless / piped execution without controlling terminal
    return 0
  fi

  answer="${answer:-$default_choice}"
  case "$answer" in
    [yY][eE][sS]|[yY])
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

# Helper: detect host candidate IPs and default gateway interface
detect_network_interfaces() {
  DEFAULT_IFACE=""
  DEFAULT_IP=""
  AVAILABLE_IPS=()

  if command -v ip >/dev/null 2>&1; then
    # 1. Determine primary interface with default route
    DEFAULT_IFACE="$((ip -4 route show default 2>/dev/null || true) | awk '{for(i=1;i<=NF;i++) if($i=="dev") print $(i+1); exit}')"
    if [ -z "$DEFAULT_IFACE" ]; then
      DEFAULT_IFACE="$((ip -4 route get 1.1.1.1 2>/dev/null || true) | awk '{for(i=1;i<=NF;i++) if($i=="dev") print $(i+1); exit}')"
    fi
    if [ -z "$DEFAULT_IFACE" ]; then
      DEFAULT_IFACE="$((ip -4 route 2>/dev/null || true) | grep -E "proto kernel scope link src|scope link" | awk '{for(i=1;i<=NF;i++) if($i=="dev") print $(i+1); exit}')"
    fi

    # 2. Extract active global IPv4 addresses excluding loopback and virtual/docker interfaces
    while IFS= read -r line; do
      [ -z "$line" ] && continue
      local iface ip
      iface="$(echo "$line" | awk '{print $2}')"
      ip="$(echo "$line" | awk '{print $4}' | cut -d/ -f1)"

      # Filter out loopback, docker bridges, and virtual ethernet
      case "$iface" in
        lo*|docker*|br-*|veth*|virbr*|cni*|flannel*) continue ;;
      esac

      if [ -n "$ip" ]; then
        AVAILABLE_IPS+=("$iface:$ip")
        if [ "$iface" = "$DEFAULT_IFACE" ] && [ -z "$DEFAULT_IP" ]; then
          DEFAULT_IP="$ip"
        fi
      fi
    done < <(ip -o -4 addr show scope global 2>/dev/null || true)
  elif command -v ifconfig >/dev/null 2>&1; then
    local cur_iface=""
    while IFS= read -r line; do
      if [[ "$line" =~ ^([a-zA-Z0-9_-]+): ]]; then
        cur_iface="${BASH_REMATCH[1]}"
      elif [[ "$line" =~ inet[[:space:]]+([0-9]+\.[0-9]+\.[0-9]+\.[0-9]+) ]]; then
        local ip="${BASH_REMATCH[1]}"
        case "$cur_iface" in
          lo*|docker*|br-*|veth*|virbr*) continue ;;
        esac
        if [ "$ip" != "127.0.0.1" ]; then
          AVAILABLE_IPS+=("$cur_iface:$ip")
        fi
      fi
    done < <(ifconfig 2>/dev/null || true)
  fi

  # Fallback if no default IP matched the default interface
  if [ -z "$DEFAULT_IP" ] && [ ${#AVAILABLE_IPS[@]} -gt 0 ]; then
    DEFAULT_IP="${AVAILABLE_IPS[0]#*:}"
  fi
  DEFAULT_IP="${DEFAULT_IP:-127.0.0.1}"
}

# Helper: check if a TCP port is currently in use
is_port_in_use() {
  local port="$1"

  # 1. Pure bash /dev/tcp connection probe (fast, no external tools needed)
  if (exec 3<>/dev/tcp/127.0.0.1/"$port") 2>/dev/null; then
    exec 3<&- 2>/dev/null || true
    exec 3>&- 2>/dev/null || true
    return 0
  fi

  # 2. Check with lsof if available
  if command -v lsof >/dev/null 2>&1; then
    if lsof -iTCP:"$port" -sTCP:LISTEN -P -n >/dev/null 2>&1; then
      return 0
    fi
  fi

  # 3. Check with ss if available
  if command -v ss >/dev/null 2>&1; then
    if ss -tlnH "sport = :$port" 2>/dev/null | grep -q ":$port\b"; then
      return 0
    fi
  fi

  return 1
}

# Helper: find first available TCP port starting from start_port
find_free_port() {
  local start_port="${1:-8080}"
  local p="$start_port"
  while [ "$p" -le 65535 ]; do
    if ! is_port_in_use "$p"; then
      echo "$p"
      return 0
    fi
    p=$((p + 1))
  done
  echo "$start_port"
}

print_summary() {
  local masked_token="[disabled / open]"
  if [ -n "$CLUSTER_TOKEN" ]; then
    masked_token="[configured: ${#CLUSTER_TOKEN} chars]"
  fi
  local peers_display="[none]"
  if [ -n "$PEERS" ]; then
    peers_display="$PEERS"
  fi
  local mdns_display="enabled"
  if [ "$ENABLE_MDNS" = "false" ]; then
    mdns_display="disabled"
  fi
  local advertise_display="[none]"
  if [ -n "$ADVERTISE_ADDR" ]; then
    advertise_display="$ADVERTISE_ADDR"
  fi

  echo ""
  printf "${BOLD}Configuration Summary:${RESET}\n"
  printf "  ${CYAN}%-18s${RESET}: http://%s:%s\n" "Web UI URL" "${DEFAULT_IP:-localhost}" "$PORT"
  printf "  ${CYAN}%-18s${RESET}: %s\n" "Stacks Directory" "$STACKS_DIR"
  printf "  ${CYAN}%-18s${RESET}: %s\n" "Host Port" "$PORT"
  printf "  ${CYAN}%-18s${RESET}: %s\n" "Node Name" "$NODE_NAME"
  printf "  ${CYAN}%-18s${RESET}: %s\n" "Advertised Addr" "$advertise_display"
  printf "  ${CYAN}%-18s${RESET}: %s\n" "Docker Socket" "$DOCKER_SOCKET"
  printf "  ${CYAN}%-18s${RESET}: %s\n" "Container Image" "$IMAGE"
  printf "  ${CYAN}%-18s${RESET}: %s\n" "Cluster Token" "$masked_token"
  printf "  ${CYAN}%-18s${RESET}: %s\n" "Seed Peers" "$peers_display"
  printf "  ${CYAN}%-18s${RESET}: %s\n" "mDNS Discovery" "$mdns_display"
  echo ""
}

info "Checking system prerequisites..."

# Check Docker CLI
if ! command -v docker >/dev/null 2>&1; then
  error "Docker is not installed or not in PATH. Please install Docker first: https://docs.docker.com/engine/install/"
fi

# Check Docker Compose (Compose v2 plugin or standalone)
COMPOSE_CMD=""
if docker compose version >/dev/null 2>&1; then
  COMPOSE_CMD="docker compose"
elif command -v docker-compose >/dev/null 2>&1; then
  COMPOSE_CMD="docker-compose"
else
  error "Docker Compose is required but was not found. Please install the Docker Compose plugin: https://docs.docker.com/compose/install/"
fi

# Check Docker daemon connectivity
if ! docker info >/dev/null 2>&1; then
  error "Cannot connect to Docker daemon at ${DOCKER_SOCKET}. Is Docker running? Do you need sudo / docker group membership?"
fi

# Check Docker socket presence
if [ ! -S "$DOCKER_SOCKET" ]; then
  warn "Docker socket not found at $DOCKER_SOCKET. Make sure Docker is running on this host."
fi

success "Prerequisites satisfied ($COMPOSE_CMD detected)"

# Detect network interfaces
detect_network_interfaces

# Auto-detect available port starting at 8080 if not explicitly provided
if [ "$PORT_SPECIFIED" = false ]; then
  DETECTED_PORT="$(find_free_port 8080)"
  if [ "$DETECTED_PORT" != "8080" ]; then
    info "Port 8080 is in use, auto-selected available port ${DETECTED_PORT}"
  fi
  PORT="$DETECTED_PORT"
fi

# Interactive configuration prompts if enabled
if [ "$INTERACTIVE" = true ] && [ "$NON_INTERACTIVE" = false ]; then
  info "Interactive Configuration Setup:"
  STACKS_DIR="$(prompt_input "Stacks Directory" "$STACKS_DIR")"
  PORT="$(prompt_input "Port" "$PORT")"
  if is_port_in_use "$PORT"; then
    warn "Port $PORT is currently in use by another process."
  fi
  NODE_NAME="$(prompt_input "Node Name" "$NODE_NAME")"

  echo ""
  printf "${BOLD}Detected Network Interfaces:${RESET}\n"
  if [ ${#AVAILABLE_IPS[@]} -gt 0 ]; then
    for entry in "${AVAILABLE_IPS[@]}"; do
      dev="${entry%%:*}"
      ip="${entry#*:}"
      if [ "$ip" = "$DEFAULT_IP" ]; then
        printf "  - ${CYAN}%-12s${RESET} %s ${GREEN}(recommended / default route)${RESET}\n" "$dev" "$ip"
      else
        printf "  - ${CYAN}%-12s${RESET} %s\n" "$dev" "$ip"
      fi
    done
  else
    printf "  ${DIM}(No active global IPv4 interfaces detected, fallback to %s)${RESET}\n" "$DEFAULT_IP"
  fi
  echo ""

  SUGGESTED_ADVERTISE="${ADVERTISE_ADDR:-http://${DEFAULT_IP}:${PORT}}"
  ADVERTISE_ADDR="$(prompt_input "Advertised Address (reachable by peers)" "$SUGGESTED_ADVERTISE")"
  CLUSTER_TOKEN="$(prompt_input "Cluster Security Token (optional)" "$CLUSTER_TOKEN")"
  PEERS="$(prompt_input "Seed Peers (comma-separated, optional)" "$PEERS")"
else
  # In non-interactive mode, set default advertise address if not explicitly specified
  if [ -z "$ADVERTISE_ADDR" ]; then
    ADVERTISE_ADDR="http://${DEFAULT_IP}:${PORT}"
  fi
fi

# Print configuration summary before making actual changes
print_summary

# Confirm before making actual changes
if ! prompt_confirm "Proceed with installation and start Dokidoki?" "Y"; then
  info "Installation cancelled. No changes were made."
  exit 0
fi

# Setup target stack directory
DOKIDOKI_DIR="${STACKS_DIR}/dokidoki"
COMPOSE_FILE="${DOKIDOKI_DIR}/compose.yaml"

info "Setting up stack directory at ${DOKIDOKI_DIR}..."

# Create directories (with sudo fallback if needed)
if ! mkdir -p "$DOKIDOKI_DIR" 2>/dev/null; then
  warn "Permission denied writing to ${STACKS_DIR}, attempting with sudo..."
  sudo mkdir -p "$DOKIDOKI_DIR"
  sudo chown -R "$(id -u):$(id -g)" "$DOKIDOKI_DIR"
fi

success "Directory ready: ${DOKIDOKI_DIR}"

# Build environment entries for compose
ENV_BLOCK=""
ENV_BLOCK="${ENV_BLOCK}      - DOKIDOKI_PORT=${PORT}\n"
ENV_BLOCK="${ENV_BLOCK}      - DOKIDOKI_STACKS_DIR=/opt/stacks\n"
ENV_BLOCK="${ENV_BLOCK}      - DOKIDOKI_NODE_NAME=${NODE_NAME}\n"

if [ -n "$PEERS" ]; then
  ENV_BLOCK="${ENV_BLOCK}      - DOKIDOKI_PEERS=${PEERS}\n"
fi

if [ -n "$CLUSTER_TOKEN" ]; then
  ENV_BLOCK="${ENV_BLOCK}      - DOKIDOKI_CLUSTER_TOKEN=${CLUSTER_TOKEN}\n"
fi

if [ -n "$ADVERTISE_ADDR" ]; then
  ENV_BLOCK="${ENV_BLOCK}      - DOKIDOKI_ADVERTISE_ADDR=${ADVERTISE_ADDR}\n"
fi

if [ "$ENABLE_MDNS" = "false" ]; then
  ENV_BLOCK="${ENV_BLOCK}      - DOKIDOKI_ENABLE_MDNS=false\n"
fi

# Write compose.yaml
info "Generating ${COMPOSE_FILE}..."

cat << EOF > "$COMPOSE_FILE"
# Dokidoki Fleet Manager Stack
# Managed autonomously as a standard compose file.

services:
  dokidoki:
    image: ${IMAGE}
    container_name: dokidoki
    restart: unless-stopped
    network_mode: host
    volumes:
      - "${DOCKER_SOCKET}:/var/run/docker.sock"
      - "${STACKS_DIR}:/opt/stacks"
    environment:
$(printf "%b" "$ENV_BLOCK")
EOF

success "Generated compose definition"

# Deploy container stack
info "Starting Dokidoki with $COMPOSE_CMD..."

$COMPOSE_CMD -f "$COMPOSE_FILE" up -d

# Wait for container liveness
info "Waiting for Dokidoki to become healthy..."
MAX_RETRIES=15
COUNT=0
HEALTHY=false

while [ $COUNT -lt $MAX_RETRIES ]; do
  if curl -sf "http://127.0.0.1:${PORT}/api/v1/host/ping" >/dev/null 2>&1; then
    HEALTHY=true
    break
  fi
  sleep 1
  COUNT=$((COUNT + 1))
done

echo ""
if [ "$HEALTHY" = true ]; then
  success "Dokidoki is up and running!"
else
  warn "Dokidoki was started, but health check has not yet responded on port ${PORT}."
fi

echo "-------------------------------------------------------------"
printf "${BOLD}Dokidoki Stack Summary:${RESET}\n"
printf "  ${CYAN}%-18s${RESET}: http://%s:%s  (or http://localhost:%s)\n" "Web UI URL" "${DEFAULT_IP:-localhost}" "$PORT" "$PORT"
printf "  ${CYAN}%-18s${RESET}: %s\n" "Node Name" "$NODE_NAME"
if [ -n "$ADVERTISE_ADDR" ]; then
  printf "  ${CYAN}%-18s${RESET}: %s\n" "Advertised Addr" "$ADVERTISE_ADDR"
fi
printf "  ${CYAN}%-18s${RESET}: %s\n" "Stack Path" "$COMPOSE_FILE"
printf "  ${CYAN}%-18s${RESET}: %s\n" "Stacks Directory" "$STACKS_DIR"
echo "-------------------------------------------------------------"
printf "${BOLD}Useful Commands:${RESET}\n"
printf "  View logs:        ${DIM}%s -f %s logs -f${RESET}\n" "$COMPOSE_CMD" "$COMPOSE_FILE"
printf "  Restart stack:    ${DIM}%s -f %s restart${RESET}\n" "$COMPOSE_CMD" "$COMPOSE_FILE"
printf "  Stop stack:       ${DIM}%s -f %s down${RESET}\n" "$COMPOSE_CMD" "$COMPOSE_FILE"
echo "-------------------------------------------------------------"

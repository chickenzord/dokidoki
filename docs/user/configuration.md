# Configuration Guide

Dokidoki can be configured via **command-line flags** or **environment variables**.

## Precedence

Configuration settings are resolved using the following order of precedence:

1. **CLI Flags** (highest precedence, overrides environment variables)
2. **Environment Variables**
3. **Default Values** (lowest precedence)

---

## Configuration Reference

| Environment Variable | CLI Flag | Default | Description |
| :--- | :--- | :--- | :--- |
| `DOKIDOKI_PORT` | `-port` | `8080` | Port for the HTTP server (Web UI and API) |
| `DOKIDOKI_BIND` | `-bind` | `0.0.0.0` | IP address or interface to bind the HTTP server to |
| `DOKIDOKI_STACKS_DIR` | `-stacks-dir` | `/opt/stacks` | Path to directory containing Compose stack subdirectories |
| `DOKIDOKI_NODE_NAME` | `-node-name` | `$(hostname)` | Human-readable name displayed for this node in the cluster |
| `DOKIDOKI_NODE_ID` | `-node-id` | _(auto / persistent)_ | Unique Node UUID (saved to `.dokidoki/node_id` in stacks dir) |
| `DOKIDOKI_ADVERTISE_ADDR` | `-advertise-addr` | `""` | Comma-separated candidate URL(s) to advertise to cluster peers |
| `DOKIDOKI_PEERS` | `-peers` | `""` | Comma-separated list of bootstrap seed peer URLs |
| `DOKIDOKI_CLUSTER_TOKEN` | `-cluster-token` | `""` | Shared authentication secret required for cluster peering |
| `DOKIDOKI_ENABLE_MDNS` | `-enable-mdns` | `true` | Enable LAN multicast DNS (mDNS) peer auto-discovery |
| `DOKIDOKI_LOG_LEVEL` | `-log-level` | `info` | Logging verbosity: `debug`, `info`, `warn`, `error` |
| `DOKIDOKI_LOG_FORMAT` | `-log-format` | `text` | Log format: `text` (human-readable) or `json` (structured) |
| `DOCKER_HOST` | `-docker-host` | _(local socket)_ | Remote Docker daemon endpoint (e.g. `unix:///var/run/docker.sock` or `tcp://...`) |
| `DOKIDOKI_DOCKER_COMPOSE_BIN` | `-docker-compose-bin` | `docker compose` | Docker compose CLI binary or command (e.g. `docker compose` or `docker-compose`) |

---

## Detailed Settings

### Server & Network

- **`DOKIDOKI_PORT` / `-port`**
  The TCP port on which Dokidoki serves both its Web UI dashboard and REST API. When deployed via `install.sh` or Docker Compose, this is the host port mapped to the container.

- **`DOKIDOKI_BIND` / `-bind`**
  The network interface address to bind the listener. Defaults to `0.0.0.0` (all interfaces). Set to `127.0.0.1` if placing Dokidoki behind a reverse proxy (e.g., Nginx, Caddy, Traefik).

### Stacks & Docker Storage

- **`DOKIDOKI_STACKS_DIR` / `-stacks-dir`**
  The root directory on the host where Compose stack folders are stored (e.g. `/opt/stacks/web`, `/opt/stacks/database`). Dokidoki scans this directory for `compose.yaml`, `compose.yml`, or `docker-compose.yml` files.

- **`DOCKER_HOST` / `-docker-host`**
  Docker daemon connection string. By default, Dokidoki connects to the standard Docker socket at `/var/run/docker.sock`. To manage a remote engine, provide a TCP URL (e.g. `tcp://192.168.1.100:2375`).

- **`DOKIDOKI_DOCKER_COMPOSE_BIN` / `-docker-compose-bin`**
  Command or path to the Docker Compose CLI binary. Defaults to `docker compose`. Can be set to `docker-compose` or an absolute executable path. Used exclusively for Compose operations (`up`, `down`, `restart`, `pull`).

### Clustering & Node Identity

- **`DOKIDOKI_NODE_NAME` / `-node-name`**
  A friendly, human-readable name identifying this host in the cluster web UI and node lists. Defaults to the system hostname.

- **`DOKIDOKI_NODE_ID` / `-node-id`**
  A persistent UUID identifying this node across restarts. If not explicitly specified, Dokidoki automatically generates a UUID on first launch and saves it to `.dokidoki/node_id` inside `DOKIDOKI_STACKS_DIR`.

- **`DOKIDOKI_ADVERTISE_ADDR` / `-advertise-addr`**
  The address(es) broadcasted to other cluster nodes so they can connect back to this instance. When running inside Docker on bridge networks, you should specify the host's LAN or VPN IP (e.g. `http://192.168.1.50:8080`). Multiple URLs can be supplied as comma-separated values.

- **`DOKIDOKI_CLUSTER_TOKEN` / `-cluster-token`**
  Pre-shared security token for cluster peering. When set, only nodes presenting this exact token in HTTP peering handshakes (`X-Cluster-Token` header) are permitted to join the cluster. If left empty, peering is unauthenticated (recommended only on trusted, isolated local networks).

### Peer Discovery

- **`DOKIDOKI_PEERS` / `-peers`**
  A comma-separated list of known bootstrap seed peer URLs (e.g. `http://192.168.1.10:8080,http://10.0.0.5:8080`). Dokidoki will contact these peers at startup, register itself, and receive other active cluster nodes via Peer Exchange (PEX).

- **`DOKIDOKI_ENABLE_MDNS` / `-enable-mdns`**
  Enables or disables local network discovery using multicast DNS (Zeroconf / DNS-SD on service `_dokidoki._tcp`). Set to `false` if deploying across cloud VPCs or environments where multicast traffic is dropped.

### Logging

- **`DOKIDOKI_LOG_LEVEL` / `-log-level`**
  Controls log verbosity:
  - `debug`: Detailed logging including discovery events, scan cycles, and HTTP request routing.
  - `info` (default): Standard operational messages and cluster membership changes.
  - `warn`: Non-critical warnings and transient connectivity drops.
  - `error`: Critical operational failures.

- **`DOKIDOKI_LOG_FORMAT` / `-log-format`**
  - `text` (default): ANSI color-coded text for interactive terminal and Docker logs.
  - `json`: Structured JSON format suitable for log collectors (Loki, Datadog, ELK).

- **Debug Shorthands:**
  - Setting `DOKIDOKI_DEBUG=true` or `DOKIDOKI_VERBOSE=true` is equivalent to `DOKIDOKI_LOG_LEVEL=debug`.
  - Passing CLI flags `-debug`, `-verbose`, or `-v` sets the log level to `debug`.

---

## Configuration Examples

### In `compose.yaml`

```yaml
services:
  dokidoki:
    image: ghcr.io/chickenzord/dokidoki:latest
    container_name: dokidoki
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - /opt/stacks:/opt/stacks
    environment:
      - DOKIDOKI_PORT=8080
      - DOKIDOKI_STACKS_DIR=/opt/stacks
      - DOKIDOKI_NODE_NAME=node-01
      - DOKIDOKI_ADVERTISE_ADDR=http://192.168.1.50:8080
      - DOKIDOKI_CLUSTER_TOKEN=my-secure-cluster-token
      - DOKIDOKI_PEERS=http://192.168.1.10:8080
      - DOKIDOKI_LOG_LEVEL=info
```

### In an Environment File (`.env`)

```ini
DOKIDOKI_PORT=8080
DOKIDOKI_STACKS_DIR=/opt/stacks
DOKIDOKI_NODE_NAME=alpha-worker
DOKIDOKI_ADVERTISE_ADDR=http://10.0.0.12:8080
DOKIDOKI_CLUSTER_TOKEN=secret-token-123
DOKIDOKI_LOG_FORMAT=json
```

### Via CLI Flags

```bash
dokidoki \
  -port 8080 \
  -stacks-dir /opt/stacks \
  -node-name node-01 \
  -advertise-addr http://192.168.1.50:8080 \
  -peers http://192.168.1.10:8080 \
  -cluster-token my-secure-cluster-token \
  -log-level info
```

---

## Related Documentation

* [Installation Guide](install.md) - Host installer, unattended installation, and multi-host setup
* [Dokidoki System Design](../design/system_design.md) - Architecture, clustering, and discovery design

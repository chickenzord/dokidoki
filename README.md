# Dokidoki (ドキドキ)

Docker Compose manager for multiple hosts.

Dokidoki monitors Docker Compose stacks on your hosts (`/opt/stacks`), connects nodes together via local mDNS or peer exchange, and provides a web interface to inspect stacks and containers across your machines.

---

## Quick Start

Run the installer on each of your Docker hosts:

```bash
curl -sSL https://raw.githubusercontent.com/chickenzord/dokidoki/main/install.sh | bash
```

The installer automatically detects host network interfaces, suggests a reachable advertise address, and configures the Dokidoki stack.

Nodes on the same local network automatically discover each other (via mDNS and peer exchange). Once installed, open the Web UI at `http://<any-host-ip>:8080` from any node to view and manage Compose stacks and containers across all discovered nodes in your cluster.

> For more installation options (unattended mode, manual Docker Compose deployment, custom flags, or cross-network peering), see the [Installation Guide](docs/user/install.md).

---

## Configuration

Dokidoki is configured via environment variables or command-line flags (flags take precedence):

| Variable | CLI Flag | Default | Description |
| :--- | :--- | :--- | :--- |
| `DOKIDOKI_PORT` | `-port` | `8080` | HTTP port to listen on |
| `DOKIDOKI_BIND` | `-bind` | `0.0.0.0` | Network interface to bind |
| `DOKIDOKI_STACKS_DIR` | `-stacks-dir` | `/opt/stacks` | Path to host Compose stacks directory |
| `DOKIDOKI_NODE_NAME` | `-node-name` | `$(hostname)` | Node name reported in the cluster |
| `DOKIDOKI_NODE_ID` | `-node-id` | _(auto / persistent)_ | Unique Node UUID (saved to `.dokidoki/node_id`) |
| `DOKIDOKI_CLUSTER_TOKEN` | `-cluster-token` | `""` | Pre-shared key required to peer with this node |
| `DOKIDOKI_PEERS` | `-peers` | `""` | Comma-separated list of bootstrap seed peers |
| `DOKIDOKI_ADVERTISE_ADDR` | `-advertise-addr`| `""` | Comma-separated candidate addresses to advertise |
| `DOKIDOKI_ENABLE_MDNS` | `-enable-mdns` | `true` | Enable LAN multicast DNS peer discovery |
| `DOKIDOKI_LOG_LEVEL` | `-log-level` | `info` | Logging verbosity (`debug`, `info`, `warn`, `error`) |
| `DOKIDOKI_LOG_FORMAT` | `-log-format` | `text` | Logging output format (`text`, `json`) |
| `DOCKER_HOST` | `-docker-host` | Local unix socket | Remote Docker daemon endpoint (e.g. `tcp://...`) |
| `DOKIDOKI_DOCKER_COMPOSE_BIN` | `-docker-compose-bin` | `docker compose` | Docker compose CLI binary / command |

> For in-depth descriptions of all options, logging formats, and precedence rules, see the [Configuration Guide](docs/user/configuration.md).

---

## Documentation

* [Installation Guide](docs/user/install.md) - Complete installer options, unattended setup, and multi-host peering
* [Configuration Guide](docs/user/configuration.md) - Detailed runtime settings, clustering flags, and environment variables
* [Dokidoki System Design](docs/design/system_design.md) - Architecture, clustering, and discovery design

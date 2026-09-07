# Dokidoki (ドキドキ)

Docker Compose manager for multiple hosts.

Dokidoki monitors Docker Compose stacks on your hosts (`/opt/stacks`), connects nodes together via local mDNS or peer exchange, and provides a web interface to inspect stacks and containers across your machines.

---

## Quick Install

Run the host installer to set up Dokidoki as a Docker Compose stack:

```bash
curl -sSL https://raw.githubusercontent.com/chickenzord/dokidoki/main/install.sh | bash
```

### Interactive Mode

To interactively configure the node name, port, stacks directory, and cluster tokens:

```bash
curl -sSL https://raw.githubusercontent.com/chickenzord/dokidoki/main/install.sh | bash -s -- -i
```

### Installer Options

| Option | Flag | Default | Description |
| :--- | :--- | :--- | :--- |
| **Interactive** | `-i, --interactive` | `false` | Prompt and configure settings interactively |
| **Non-Interactive** | `-y, --yes, --non-interactive` | `false` | Proceed without confirmation prompt |
| **Stacks Directory** | `-s, --stacks-dir <dir>` | `/opt/stacks` | Directory where compose stacks reside |
| **Port** | `-p, --port <port>` | `8080` | Host port to expose the Web UI and API |
| **Node Name** | `-n, --node-name <name>` | `$(hostname)` | Identifier for this node in the cluster |
| **Seed Peers** | `--peers <urls>` | _(none)_ | Comma-separated bootstrap peer URLs |
| **Cluster Token** | `--token <token>` | _(open)_ | Shared authentication token for the cluster |
| **Advertise Addr** | `--advertise-addr <urls>` | _(auto)_ | Specific candidate address(es) to broadcast |
| **Disable mDNS** | `--disable-mdns` | `enabled` | Disable local network mDNS peer discovery |

---

## Docker Compose

Dokidoki can also be deployed directly using Docker Compose:

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
      # - DOKIDOKI_NODE_NAME=node-01
      # - DOKIDOKI_CLUSTER_TOKEN=secret
      # - DOKIDOKI_PEERS=http://192.168.1.10:8080
```

Start the stack:

```bash
docker compose up -d
```

Once running, access the dashboard at `http://<host-ip>:8080`.

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
| `DOCKER_HOST` | `-docker-host` | Local unix socket | Remote Docker daemon endpoint (e.g. `tcp://...`) |

---

## Design Documentation

For details on architecture, clustering, and design decisions, see:

* [Dokidoki System Design](docs/design/system_design.md)

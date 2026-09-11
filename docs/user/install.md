# Installation Guide

Dokidoki is deployed on each host as an autonomous Docker Compose stack located in `/opt/stacks/dokidoki`.

---

## Quick Install (Interactive)

On any machine with Docker installed, run the host installer:

```bash
curl -sSL https://raw.githubusercontent.com/chickenzord/dokidoki/main/install.sh | bash
```

The installer runs interactively by default:
1. Verifies system prerequisites (`docker`, `docker compose`, daemon connectivity).
2. Probes host network interfaces, recommending the default route IP as the candidate advertised address for peer clustering.
3. Prompts for optional custom configuration (port, node name, stacks directory, cluster token, seed peers).
4. Generates `/opt/stacks/dokidoki/compose.yaml` and launches the stack with `docker compose up -d`.
5. Verifies health and displays the Web UI URL and management commands.

---

## Unattended / Non-Interactive Install

For automated server provisioning, CI/CD pipelines, cloud-init scripts, or configuration managers (Ansible, Puppet), run the installer with the `-y` (`--non-interactive`) flag:

```bash
curl -sSL https://raw.githubusercontent.com/chickenzord/dokidoki/main/install.sh | bash -s -- -y
```

In non-interactive mode:
- All prompts are bypassed.
- Defaults are used unless overridden by CLI flags or environment variables.
- Host network interfaces are automatically discovered and the default route IP is selected as the advertised address (`http://<default-ip>:<port>`).
- Confirmation checks automatically proceed.

### Headless Environment Detection

The installer automatically switches to non-interactive mode when:
- `CI=true` or `DEBIAN_FRONTEND=noninteractive` is set in the environment.
- The script is executed in a headless pipe without an attached controlling terminal (`/dev/tty`).

---

## Installer Options & Flags

You can customize the installation by passing command-line arguments to `install.sh`:

```bash
curl -sSL https://raw.githubusercontent.com/chickenzord/dokidoki/main/install.sh | bash -s -- [OPTIONS]
```

### Complete CLI Flags

| Option | Flag | Default | Description |
| :--- | :--- | :--- | :--- |
| **Interactive** | `-i, --interactive` | `true` (in terminal) | Interactively prompt and configure settings |
| **Non-Interactive** | `-y, --yes, --non-interactive` | `false` | Unattended mode (proceed with defaults/flags without prompts) |
| **Stacks Directory** | `-s, --stacks-dir <dir>` | `/opt/stacks` | Directory where compose stacks reside on the host |
| **Port** | `-p, --port <port>` | `8080` | Host port to expose the Web UI and API |
| **Node Name** | `-n, --node-name <name>` | `$(hostname)` | Human-readable identifier for this node in the cluster |
| **Advertise Addr** | `--advertise-addr <urls>` | `http://<ip>:<port>` | Comma-separated candidate URL(s) reachable by other nodes |
| **Seed Peers** | `--peers <urls>` | _(none)_ | Comma-separated bootstrap peer URLs (e.g. `http://192.168.1.10:8080`) |
| **Cluster Token** | `--token <token>` | _(open)_ | Shared authentication secret required for cluster peering |
| **Disable mDNS** | `--disable-mdns` | `enabled` | Disable LAN multicast DNS peer discovery |
| **Image** | `--image <image>` | `ghcr.io/chickenzord/dokidoki:latest` | Container image to deploy |
| **Docker Socket** | `--socket <path>` | `/var/run/docker.sock` | Path to host Docker daemon socket |
| **Help** | `-h, --help` | | Show usage and available options |

### Automated Provisioning Example

```bash
curl -sSL https://raw.githubusercontent.com/chickenzord/dokidoki/main/install.sh | bash -s -- \
  -y \
  --node-name "node-prod-01" \
  --port 8080 \
  --token "super-secret-cluster-token" \
  --peers "http://192.168.1.10:8080"
```

---

## Environment Variables

All settings can alternatively be configured via environment variables:

| Environment Variable | Equivalent Flag | Description |
| :--- | :--- | :--- |
| `DOKIDOKI_STACKS_DIR` | `-s, --stacks-dir` | Directory where Compose stacks reside |
| `DOKIDOKI_PORT` | `-p, --port` | Port exposed on the host |
| `DOKIDOKI_NODE_NAME` | `-n, --node-name` | Node identifier |
| `DOKIDOKI_ADVERTISE_ADDR` | `--advertise-addr` | Advertised address for cluster communication |
| `DOKIDOKI_PEERS` | `--peers` | Comma-separated list of seed peers |
| `DOKIDOKI_CLUSTER_TOKEN` | `--token` | Shared security token for clustering |
| `DOKIDOKI_ENABLE_MDNS` | `--disable-mdns` | Set to `false` to disable mDNS |
| `DOKIDOKI_IMAGE` | `--image` | Docker image to run |
| `DOCKER_SOCKET` | `--socket` | Path to Docker socket |

Example:

```bash
DOKIDOKI_NODE_NAME="worker-01" DOKIDOKI_CLUSTER_TOKEN="secret" \
  curl -sSL https://raw.githubusercontent.com/chickenzord/dokidoki/main/install.sh | bash -s -- -y
```

> For full descriptions of all runtime settings, logging options, and configuration precedence, see the [Configuration Guide](configuration.md).

---

## Multi-Host Cluster Setup

Dokidoki is designed to manage Docker Compose across multiple machines seamlessly:

1. **Same Local Network (LAN):**
   Run the default installation on each machine. Dokidoki nodes will automatically discover each other via mDNS. No manual peer configuration required.

2. **Cross-Network / Cloud / VPN (Tailscale, WireGuard):**
   When multicast is not available across subnets, specify one or more seed peers:
   ```bash
   # On Node 2, pointing to Node 1
   curl -sSL https://raw.githubusercontent.com/chickenzord/dokidoki/main/install.sh | bash -s -- \
     --peers "http://10.0.0.1:8080" \
     --token "my-cluster-token"
   ```
   Nodes will exchange known peers via Peer Exchange (PEX) automatically.

---

## Manual Docker Compose Deployment

If you prefer not to use `install.sh`, you can deploy Dokidoki directly using Docker Compose:

Create a `compose.yaml` file (e.g. in `/opt/stacks/dokidoki/compose.yaml`):

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
      # - DOKIDOKI_ADVERTISE_ADDR=http://192.168.1.50:8080
      # - DOKIDOKI_PEERS=http://192.168.1.10:8080
```

> [!IMPORTANT]
> **Matching Host and Container Paths for Stacks Directory:**
> Because Dokidoki executes `docker compose` commands that talk to the host's Docker daemon via `/var/run/docker.sock`, any bind mounts specified in child stack compose files are evaluated relative to the **host filesystem**, not the container filesystem. Therefore, the stacks directory mount path inside the container and `DOKIDOKI_STACKS_DIR` **must match the host directory path identically** (e.g. `/home/user/stacks:/home/user/stacks` and `DOKIDOKI_STACKS_DIR=/home/user/stacks`, or `/opt/stacks:/opt/stacks`).

> **Note on Advertised Address:** When running in Docker bridge network mode (default), set `DOKIDOKI_ADVERTISE_ADDR` to your host IP (e.g. `http://192.168.1.50:8080`) so other cluster nodes can reach this instance. Alternatively, you can use `network_mode: host`.

Start the stack:

```bash
docker compose up -d
```

Once running, access the dashboard at `http://<host-ip>:8080`.

---

## Maintenance & Operations

Dokidoki's stack definition resides in `/opt/stacks/dokidoki/compose.yaml`.

- **View Logs:**
  ```bash
  docker compose -f /opt/stacks/dokidoki/compose.yaml logs -f
  ```
- **Update to Latest Version:**
  ```bash
  docker compose -f /opt/stacks/dokidoki/compose.yaml pull
  docker compose -f /opt/stacks/dokidoki/compose.yaml up -d
  ```
- **Restart Stack:**
  ```bash
  docker compose -f /opt/stacks/dokidoki/compose.yaml restart
  ```
- **Uninstall:**
  ```bash
  docker compose -f /opt/stacks/dokidoki/compose.yaml down -v
  rm -rf /opt/stacks/dokidoki
  ```

---

## Related Documentation

* [Configuration Guide](configuration.md) - Complete runtime configuration reference and settings
* [Dokidoki System Design](../design/system_design.md) - Architecture, clustering, and discovery design

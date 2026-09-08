<p align="center">
  <img src="docs/assets/dokidoki_t_128.png" alt="Dokidoki" width="96" height="96" />
</p>

<h1 align="center">Dokidoki (ドキドキ)</h1>

<p align="center">
  Docker Compose manager for multiple hosts.
</p>

<p align="center">
  <a href="https://github.com/chickenzord/dokidoki/actions/workflows/docker.yml"><img src="https://img.shields.io/github/actions/workflow/status/chickenzord/dokidoki/docker.yml?branch=main" alt="Build Status" /></a>
  <a href="https://github.com/chickenzord/dokidoki/pkgs/container/dokidoki"><img src="https://img.shields.io/badge/docker-ghcr.io%2Fchickenzord%2Fdokidoki-blue?logo=docker" alt="Docker Image" /></a>
  <a href="https://codecov.io/gh/chickenzord/dokidoki"><img src="https://codecov.io/gh/chickenzord/dokidoki/branch/main/graph/badge.svg" alt="codecov" /></a>
  <a href="LICENSE"><img src="https://img.shields.io/github/license/chickenzord/dokidoki" alt="License" /></a>
</p>

Dokidoki monitors Docker Compose stacks on your hosts (`/opt/stacks`), connects nodes together via local mDNS or peer exchange, and provides a web interface to inspect stacks and containers across your machines.

---

## Features

* **Multi-host overview**: View Compose stacks and containers across connected nodes in a single web interface.
* **Peer discovery**: Nodes on the same local network discover each other automatically via mDNS, or across networks using seed peers (peer exchange).
* **Compose actions**: Start, stop, restart, and pull stacks from the browser.
* **Container logs and terminal**: View container logs and open an interactive shell in your browser.
* **Decentralized**: No central server or database required. Any node can serve the web interface.
* **Cluster token**: Optional pre-shared token to restrict peer connections.

---

## Managing Stacks

Due to how Dokidoki runs as a Docker container, it can only manage Compose stack files within its mounted volume (default: `/opt/stacks`). However, you can easily import your already running Docker Compose stacks into Dokidoki through the web interface.

These are just regular Docker Compose files on your host filesystem. You can edit them through the Dokidoki web interface or modify them directly on the host via SSH.

---

## Quick Start

Run the installer on each of your Docker hosts:

```bash
curl -sSL https://raw.githubusercontent.com/chickenzord/dokidoki/main/install.sh | bash
```

Repeat this command on all nodes in your cluster. The installer detects the host network interface, writes `/opt/stacks/dokidoki/compose.yaml`, and starts the container.

Once started, open the web interface at `http://<host-ip>:<port>` from any node. Nodes on the same local network will automatically discover each other via mDNS.

> For manual Docker Compose deployment, unattended setup, or custom options, see the [Installation Guide](docs/user/install.md).

---

## Multi-Host Setup

* **Local Network (LAN):** Nodes discover each other automatically via mDNS.
* **Across Networks / VPN:** Connect nodes using seed peers. Nodes will exchange known peer addresses automatically.

---

## Documentation

* [Installation Guide](docs/user/install.md) - Installer options, manual Compose deployment, and multi-host peering
* [Configuration Guide](docs/user/configuration.md) - Full reference for environment variables and CLI flags
* [Dokidoki System Design](docs/design/system_design.md) - Architecture, clustering, and discovery design

---

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

# Dokidoki System Design

> **Decentralized, heartbeat-driven Docker Compose fleet manager.**

Dokidoki is a lightweight, file-first container stack manager designed for multi-host homelabs and edge setups. It eliminates the single-point-of-failure problem of centralized dashboards without introducing the heavyweight complexity of cluster orchestrators.

Hit **any** node in your fleet to see and manage **every** node.

---

## The Problem

Managing Docker Compose stacks across multiple machines (LAN servers, cloud VPS, edge devices) currently forces bad trade-offs:

1. **Centralized dashboards (Dockge, Coolify, Portainer):** One server acts as the primary master. If that machine reboots or goes offline, your entire management plane is unavailable.
2. **Rigid agent pairing:** Agents require manual token copying, local database links, and fixed host endpoints that don't adapt when switching networks.
3. **Heavy orchestrators (Docker Swarm, Nomad, Kubernetes):** Overkill for independent homelab stacks. They strip away the simplicity of plain `compose.yaml` files living directly on the host filesystem.

---

## Core Philosophy

* **File-First & Host-Autonomous:** Stacks live as standard `compose.yaml` files on each host (defaulting to `/opt/stacks/<name>/compose.yaml`). Each node is fully self-sufficient and operates even in a total network split.
* **Client-Side Discovery (PEX):** No centralized database. Connect to any node, and your client (Web UI / CLI) receives the known cluster topology, caches it locally, and can route directly to any peer.
* **Context-Aware Multi-Address Routing:** Nodes can broadcast all available candidate addresses (e.g. LAN IP, VPN/overlay IP, WAN). Clients race endpoints ("Happy Eyeballs") to pick the fastest reachable path.
* **Transport Agnostic:** Works seamlessly over standard LAN, WireGuard, Tailscale, or public reverse proxies without coupling to any specific networking vendor.

---

## How It Works

```
                        Browser / Client App
                                 │
                 1. Initial hit: http://node-a:8080
                                 │
                 2. Fetches cluster topology
                    Cached to localStorage
                                 │
         ┌───────────────────────┼───────────────────────┐
         │ (Direct LAN ~0.5ms)   │ (VPN / Fallback)      │ (Remote Cloud)
         ▼                       ▼                       ▼
┌──────────────────┐    ┌──────────────────┐    ┌──────────────────┐
│      Node A      │    │      Node B      │    │      Node C      │
│  /opt/stacks/    │    │  /opt/stacks/    │    │  /opt/stacks/    │
│  Docker Socket   │    │  Docker Socket   │    │  Docker Socket   │
└────────┬─────────┘    └────────┬─────────┘    └────────┬─────────┘
         │                       │                       │
         └──────── Peer Gossip & Heartbeat (SWIM) ───────┘
```

1. **Gossip Heartbeat:** Nodes exchange lightweight heartbeats over UDP/TCP to maintain an active registry of healthy peers and candidate endpoints.
2. **Cluster Topology Endpoint:** Any node responds to `GET /api/v1/nodes` with the list of alive peers and their candidate addresses.
3. **Client Probing:** The client probes endpoints to find the lowest-latency path for each node.
4. **Resilient Failover:** If the initial entrypoint goes down, the client falls back to the next cached node in `localStorage`.

---

## Architecture & Configuration

* **Default Stacks Directory:** `/opt/stacks` (configurable via `DOKIDOKI_STACKS_DIR`).
* **Docker Connection:**
  * **Default:** Local Unix socket (`unix:///var/run/docker.sock`).
  * **Remote:** Standard `DOCKER_HOST` support (e.g., `tcp://192.168.18.23:2375`) supported from day one for client-only / containerless testing environments.
* **REST Routing Principles:**
  * Strict resource-oriented hierarchy (`/api/v1/{collection}[/{id}[/{sub-resource}]]`).
  * Zero sibling static-vs-parameter route collisions (no static routes at the `{name}` or `{id}` level).
  * Filter variations (like grouping) handled exclusively via query parameters (`?grouped=true`).



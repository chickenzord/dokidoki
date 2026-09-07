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
         └──── HTTP Heartbeats & Peer Exchange (PEX) ────┘
               + Local Subnet Discovery via mDNS
```

1. **Dual Discovery (mDNS + PEX):**
   - **LAN (Zero-Config):** Multicast DNS (`_dokidoki._tcp`) automatically advertises and discovers peers on the local subnet.
   - **Cross-Subnet / WAN:** Configured seed peers (`PEERS`) and HTTP Peer Exchange (PEX) distribute known peer tables across nodes.
2. **HTTP Heartbeats & State Tracking:**
   - Nodes exchange periodic HTTP heartbeats (`POST /api/v1/cluster/heartbeat`) over their standard HTTP port—no UDP or complex gossip daemons required.
   - Node status transitions across `alive` → `suspect` → `offline` based on active TTL tracking.
   - Offline nodes are preserved in cluster memory and dashboard views until explicitly removed.
3. **Cluster Topology Endpoint:**
   - Every node exposes `GET /api/v1/nodes` returning alive, suspect, and offline peers along with their candidate IP addresses.
4. **Client Probing & Resilient Failover:**
   - The browser / client app fetches cluster topology, caches it in `localStorage`, and connects directly to each node.
   - If an entrypoint node goes down, the client seamlessly switches to any reachable peer.

---

## Cluster Architecture

### Node Identity
Each Dokidoki node generates a unique UUIDv4 persisted to `$STACKS_DIR/.dokidoki/node_id`. This guarantees identity stability across host reboots, IP address reallocations, and container restarts.

### Discovery Protocols
Instead of heavy UDP gossip protocols (like SWIM or Memberlist) that struggle behind reverse proxies and VPN firewalls, Dokidoki utilizes standard HTTP-native discovery:
* **mDNS Provider:** Broadcasts the instance on the LAN. Discovered peers trigger an immediate HTTP handshake.
* **Static Provider:** Connects to configured bootstrap seed nodes (`--peers`).
* **PEX Provider:** Piggybacks known peer tables onto periodic heartbeat requests, organically propagating cluster membership without centralized coordinators.

### Security
Inter-node clustering can be secured with a pre-shared cluster token (`DOKIDOKI_CLUSTER_TOKEN`). When configured, handshakes, heartbeats, and peer exchange require matching tokens via HTTP request payload or `X-Cluster-Token` headers.

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

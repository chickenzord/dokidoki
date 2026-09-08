# Dokidoki System Design

Dokidoki is a decentralized manager for Docker Compose stacks across multiple hosts. It allows you to monitor and manage Compose stacks and containers across your machines from a single web interface, without requiring a centralized server or database.

Accessing any running node allows you to inspect and manage stacks across all connected nodes in the cluster.

---

## The Problem

Managing Docker Compose stacks across multiple machines typically involves undesirable trade-offs:

1. **Centralized dashboards:** A single server acts as the primary coordinator. If that machine restarts or fails, the management interface becomes completely unavailable.
2. **Heavy orchestrators:** Systems like Kubernetes or Docker Swarm require significant operational overhead and change how stacks are written and run, moving away from simple `compose.yaml` files on host disks.
3. **Disconnected machines:** Managing independent servers manually requires connecting to each host individually via SSH or separate tool instances.

---

## Core Principles

* **File-First on Host Filesystem:** Stacks are stored as standard Compose files on each host's local filesystem. Nodes remain fully autonomous; local stacks continue running normally even during network partitions.
* **Decentralized Discovery:** Nodes discover each other via local network mDNS or HTTP peer exchange (PEX). There is no central cluster database or single point of failure.
* **Direct Multi-Host Visibility:** The web interface retrieves cluster topology from the local node and queries connected nodes to present a consolidated view of stacks and containers.
* **Standard Docker Integration:** Interacts directly with the local Docker daemon socket and Docker Compose CLI.

---

## System Architecture

```
                         Browser / Client
                                │
               1. Connect to any node: http://<node-ip>:<port>
                                │
               2. Retrieve cluster topology
                                │
          ┌─────────────────────┼─────────────────────┐
          ▼                     ▼                     ▼
┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐
│      Node A      │  │      Node B      │  │      Node C      │
│  Stacks Dir      │  │  Stacks Dir      │  │  Stacks Dir      │
│  Docker Socket   │  │  Docker Socket   │  │  Docker Socket   │
└────────┬─────────┘  └────────┬─────────┘  └────────┬─────────┘
         │                     │                     │
         └──── HTTP Heartbeats & Peer Exchange (PEX) ─┘
               + Local Subnet Discovery via mDNS
```

### Discovery & Clustering

Dokidoki uses standard HTTP and network protocols for peer discovery:

1. **mDNS Provider (LAN):** Broadcasts instances on the local network using multicast DNS (`_dokidoki._tcp`). When peers are discovered, an HTTP handshake is initiated.
2. **Static Provider (Seed Peers):** Connects to specified bootstrap seed nodes on startup, useful across routed networks or VPNs (e.g. Tailscale, WireGuard) where multicast is unavailable.
3. **Peer Exchange (PEX):** Nodes exchange their list of known active peers during periodic heartbeats, allowing newly joined nodes to discover the rest of the cluster automatically.
4. **Heartbeats & Node Lifecycle:**
   - Nodes exchange periodic HTTP heartbeats (`POST /api/v1/cluster/heartbeat`).
   - Node status transitions across `alive` → `suspect` → `offline` based on missed heartbeat thresholds.
   - Offline nodes remain visible in the topology until explicitly pruned or restarted.
5. **Cluster Authentication:** Peering handshakes and heartbeats can be restricted with a pre-shared cluster token passed via the `X-Cluster-Token` header.

### Node Identity

Each node generates a UUIDv4 on first run, persisted to `.dokidoki/node_id` inside the stacks directory. This ensures consistent node identity across restarts, IP address reassignments, and container recreation.

---

## Stack & Container Management

### Stack Categorization

Dokidoki classifies stacks and containers into three categories:

1. **Managed Stacks:** Stacks whose Compose definitions reside directly within Dokidoki's configured stacks directory. These can be edited, created, pulled, started, and stopped directly from the web interface.
2. **External Stacks:** Containers running on the Docker engine that were started by Docker Compose from files outside Dokidoki's stacks directory. Dokidoki detects their Compose project labels and allows importing them into the managed stacks directory.
3. **Standalone Containers:** Individual containers running without Compose project metadata.

### Operations & Execution

* **Compose Commands:** Lifecycle operations (`up`, `down`, `restart`, `pull`) are executed via the Docker Compose binary against the target stack directory.
* **Container Logs:** Live container output is streamed to the client using chunked HTTP streaming directly from Docker daemon log multiplexers.
* **Interactive Terminal:** Web terminal sessions connect to containers using Docker exec with PTY allocation, streamed in real time over WebSockets.

---

## API Design Principles

The REST API follows a resource-oriented structure:

* `/api/v1/nodes`: Cluster membership and topology.
* `/api/v1/cluster/*`: Inter-node communication (heartbeats, peering handshakes).
* `/api/v1/stacks`: Stack listing, creation, file editing, and Compose actions.
* `/api/v1/containers`: Container inspection, lifecycle actions, log streaming, and PTY exec.
* `/api/v1/host/*`: Node health (`/ping`) and daemon information (`/info`).

All endpoints use standard JSON payloads and responses, with WebSockets reserved for interactive terminal sessions and chunked streaming for live logs.

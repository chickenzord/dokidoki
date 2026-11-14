# Dokidoki Security, Clustering, and Authentication Architecture

## 1. Problem Statement & Threat Model

Dokidoki manages Docker Compose stacks by mounting `/var/run/docker.sock` and `/opt/stacks`. Direct access to the Dokidoki HTTP API implies full host compromise (equivalent to root access).

### Attack Vectors
1. **Unprotected API Exposure**: An unauthenticated Dokidoki instance bound to `0.0.0.0` allows any network caller to inspect OpenAPI docs, create rogue stacks, mount host root `/`, or trigger privilege escalation via Docker containers.
2. **Unauthenticated Cluster Peering & mDNS**: Nodes advertising via mDNS or static seeds without a shared secret automatically peer with rogue nodes, allowing untrusted machines to execute commands across the cluster topology.
3. **Cross-Node Proxy Hijacking**: Inter-node proxy endpoints (`/api/v1/nodes/{id}/proxy/*`) allow arbitrary remote command execution if worker nodes do not verify peer node identity.

---

## 2. Two Distinct Authentication Layers

```
[ User in Browser / CLI ]
           |
     (1) User-Facing Auth (Human-to-Machine)
           v
    [ Ingress Node A ]
           |
     (2) Inter-Node Auth (Machine-to-Machine)
           v
    [ Worker Node B ] (Executes Docker compose / pulls)
```

### Layer 1: Inter-Node Auth & Trust (Machine-to-Machine)

Guards communication between nodes in a cluster:
- Node discovery (mDNS / static seed peering)
- Heartbeats & topology exchange
- Forwarding proxied requests (`/api/v1/nodes/{id}/proxy/*`)

**Requirements**:
- **Mandatory Cluster Token for mDNS**: Disable or refuse unauthenticated mDNS discovery if `DOKIDOKI_CLUSTER_TOKEN` is unset. Prevent rogue nodes from automatically joining the fleet.
- **Node-to-Node Verification**: All cluster endpoints (`/api/v1/cluster/*`, `/api/v1/nodes/*`) and inter-node proxied API endpoints must validate `X-Cluster-Token` (or mTLS client certificates).
- **Reject Blank Tokens**: Empty tokens must never match or allow access.

---

### Layer 2: User-Facing Auth (Human-to-Machine)

Guards human interaction with the Web UI and external REST API clients:

**Requirements**:
- **API Protection**: All `/api/v1/*` routes (except health ping / login) require valid authentication.
- **Web UI Lock Screen**: Prompt unauthenticated users with an unlock screen before revealing containers, logs, or compose files.
- **Session Handling**: Browser stores the token in `localStorage` or `HttpOnly` cookie; automatically attaches `Authorization: Bearer <token>` to REST requests and SSE log streams (`?token=...`).
- **Seamless Multi-Node Browsing**: Logging into the entry node (Node A) allows viewing and controlling worker Node B without secondary login prompts; Node A verifies the user session and forwards commands to Node B over the authenticated inter-node channel.
- **Reverse-Proxy / Forward-Auth Delegation**: Support trusting `Remote-User` / `Remote-Email` headers when behind trusted proxies (Tailscale, Cloudflare Access, Authelia).

---

## 3. Secret Model Options

### Option 1: Unified Cluster Secret (`DOKIDOKI_SECRET`)
- **Concept**: A single secret string acts as both the admin login password and the cluster join token.
- **Pros**:
  - Extremely simple for homelab users.
  - Zero cognitive overhead: one password during `install.sh`, used for both the UI and joining additional nodes.
- **Cons**:
  - Anyone with UI access possesses the cluster token. Cannot support read-only UI users without leaking the cluster secret.

### Option 2: Split User Auth vs. Cluster Secret (Recommended)
- **Concept**:
  - `DOKIDOKI_AUTH_TOKEN`: Protects user endpoints (Web UI & REST API).
  - `DOKIDOKI_CLUSTER_TOKEN`: Protects cluster discovery, peer gossip, and inter-node proxying.
- **Pros**:
  - Clear separation of concerns (Least Privilege).
  - Worker nodes need only the cluster token; they never handle user credentials.
  - Enables future multi-user support, read-only UI tokens, or OIDC/SSO without altering cluster communication.
- **Cons**:
  - Two configuration parameters (can be streamlined via installer defaults).

---

## 4. Default Hardening & Safe Defaults

1. **Loopback by Default**:
   - Change default bind address from `0.0.0.0` to `127.0.0.1` unless clustering or explicit multi-homing is configured.
   - Fail or strongly warn if exposed on public interfaces without authentication.
2. **Installer Bootstrapping (`install.sh`)**:
   - Auto-generate cryptographically secure random secrets on installation (`openssl rand -hex 16`).
   - Store generated secrets in `/opt/stacks/.dokidoki/.env` with file permissions `600`.
3. **CORS Hardening**:
   - Remove wildcard CORS (`AllowedOrigins: ["*"]`); restrict to same-origin or explicit origins.

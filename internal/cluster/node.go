package cluster

import "time"

// Status represents the state of a node in the cluster.
type Status string

const (
	StatusAlive   Status = "alive"
	StatusSuspect Status = "suspect"
	StatusOffline Status = "offline"
)

// Node represents a Dokidoki cluster member.
type Node struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Addresses []string  `json:"addresses"`
	Status    Status    `json:"status"`
	Version   string    `json:"version"`
	IsSelf    bool      `json:"is_self"`
	LastSeen  time.Time `json:"last_seen,omitempty"`
}

// HandshakeRequest is sent by a node to introduce itself to a peer.
type HandshakeRequest struct {
	NodeID       string   `json:"node_id"`
	Name         string   `json:"name"`
	Addresses    []string `json:"addresses"`
	Version      string   `json:"version"`
	ClusterToken string   `json:"cluster_token,omitempty"`
}

// HandshakeResponse is returned to an incoming handshake request.
type HandshakeResponse struct {
	NodeID    string   `json:"node_id"`
	Name      string   `json:"name"`
	Addresses []string `json:"addresses"`
	Version   string   `json:"version"`
	Peers     []Node   `json:"peers"`
}

// HeartbeatMessage is exchanged periodically between peers.
type HeartbeatMessage struct {
	NodeID       string   `json:"node_id"`
	Addresses    []string `json:"addresses"`
	ClusterToken string   `json:"cluster_token,omitempty"`
	KnownPeers   []Node   `json:"known_peers"`
}

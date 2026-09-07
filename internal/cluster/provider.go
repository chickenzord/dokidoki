package cluster

import "context"

// EventType defines the kind of peer discovery event.
type EventType string

const (
	EventDiscovered EventType = "discovered"
	EventUpdated    EventType = "updated"
	EventLost       EventType = "lost"
)

// PeerEvent represents a notification from a DiscoveryProvider regarding a peer.
type PeerEvent struct {
	Type      EventType `json:"type"`
	NodeID    string    `json:"node_id"`
	Name      string    `json:"name"`
	Addresses []string  `json:"addresses"`
}

// Provider defines the interface for peer discovery mechanisms
// such as static seed peers, LAN mDNS broadcast, or peer exchange (PEX).
type Provider interface {
	Name() string
	Start(ctx context.Context, events chan<- PeerEvent) error
	Stop() error
}

// DiscoveryProvider is an alias for Provider.
type DiscoveryProvider = Provider

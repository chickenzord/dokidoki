package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/chickenzord/dokidoki/internal/logger"
	"github.com/chickenzord/dokidoki/internal/model"
)

// Cluster errors.
var (
	ErrInvalidToken     = errors.New("invalid or unauthorized cluster token")
	ErrCannotRemoveSelf = errors.New("cannot remove self node")
	ErrNodeNotFound     = errors.New("node not found")
)

// Manager coordinates cluster topology, peer discovery, heartbeats, and TTL tracking.
type Manager struct {
	self         model.Node
	mu           sync.RWMutex
	peers        map[string]*model.Node
	clusterToken string
	ttl          time.Duration
	providers    []DiscoveryProvider
	httpClient   *http.Client

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewManager creates an initialized cluster Manager.
func NewManager(self model.Node, clusterToken string, ttl time.Duration, httpClient *http.Client) *Manager {
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 5 * time.Second,
		}
	}

	self.IsSelf = true
	self.Status = model.NodeStatusAlive

	return &Manager{
		self:         self,
		peers:        make(map[string]*model.Node),
		clusterToken: clusterToken,
		ttl:          ttl,
		httpClient:   httpClient,
	}
}

// Self returns the local node metadata.
func (m *Manager) Self() model.Node {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.self
}

// GetNode retrieves a node by ID, including self or registered peers.
func (m *Manager) GetNode(id string) (*model.Node, bool) {
	if id == m.self.ID {
		cp := m.self
		return &cp, true
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	peer, ok := m.peers[id]
	if !ok {
		return nil, false
	}
	cp := *peer
	return &cp, true
}

// ListNodes returns Self and all registered peers sorted ascending by Name.
func (m *Manager) ListNodes() []model.Node {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nodes := make([]model.Node, 0, len(m.peers)+1)
	nodes = append(nodes, m.self)
	for _, p := range m.peers {
		nodes = append(nodes, *p)
	}

	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Name == nodes[j].Name {
			return nodes[i].ID < nodes[j].ID
		}
		return nodes[i].Name < nodes[j].Name
	})

	return nodes
}

// RegisterProvider registers a peer discovery provider.
func (m *Manager) RegisterProvider(p DiscoveryProvider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers = append(m.providers, p)
}

// Start starts all registered providers, the peer event loop, and the TTL eviction timer.
func (m *Manager) Start(ctx context.Context) {
	m.mu.Lock()
	m.ctx, m.cancel = context.WithCancel(ctx)
	providers := make([]DiscoveryProvider, len(m.providers))
	copy(providers, m.providers)
	m.mu.Unlock()

	events := make(chan PeerEvent, 128)

	// Start providers
	for _, p := range providers {
		prov := p
		if err := prov.Start(m.ctx, events); err != nil {
			logger.Warnf("Cluster: provider %s failed to start: %v", prov.Name(), err)
		}
	}

	// Start event loop
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		for {
			select {
			case <-m.ctx.Done():
				return
			case ev, ok := <-events:
				if !ok {
					return
				}
				m.handlePeerEvent(ev)
			}
		}
	}()

	// Start TTL eviction timer
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()

		interval := m.ttl / 2
		if interval < 50*time.Millisecond {
			interval = 50 * time.Millisecond
		} else if interval > 5*time.Second {
			interval = 5 * time.Second
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-m.ctx.Done():
				return
			case <-ticker.C:
				m.EvictExpiredPeers()
			}
		}
	}()
}

// EvictExpiredPeers checks peer LastSeen timestamps and marks expired nodes offline or suspect.
func (m *Manager) EvictExpiredPeers() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for _, peer := range m.peers {
		age := now.Sub(peer.LastSeen)
		if age > m.ttl {
			peer.Status = model.NodeStatusOffline
		} else if age > m.ttl/2 {
			peer.Status = model.NodeStatusSuspect
		}
	}
}

func (m *Manager) handlePeerEvent(ev PeerEvent) {
	if ev.NodeID == "" || ev.NodeID == m.self.ID {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	switch ev.Type {
	case EventDiscovered, EventUpdated:
		if peer, ok := m.peers[ev.NodeID]; ok {
			peer.LastSeen = time.Now()
			peer.Status = model.NodeStatusAlive
			if ev.Name != "" {
				peer.Name = ev.Name
			}
			if len(ev.Addresses) > 0 {
				peer.Addresses = ev.Addresses
			}
		} else {
			m.peers[ev.NodeID] = &model.Node{
				ID:        ev.NodeID,
				Name:      ev.Name,
				Addresses: ev.Addresses,
				Status:    model.NodeStatusAlive,
				LastSeen:  time.Now(),
			}
		}
	case EventLost:
		if peer, ok := m.peers[ev.NodeID]; ok {
			peer.Status = model.NodeStatusOffline
		}
	}
}

// Stop stops all providers and notifies active peers if possible.
func (m *Manager) Stop() error {
	m.mu.Lock()
	if m.cancel != nil {
		m.cancel()
	}
	providers := make([]DiscoveryProvider, len(m.providers))
	copy(providers, m.providers)

	var peersToNotify []model.Node
	for _, p := range m.peers {
		peersToNotify = append(peersToNotify, *p)
	}
	m.mu.Unlock()

	for _, p := range providers {
		if err := p.Stop(); err != nil {
			logger.Warnf("Cluster: error stopping provider %s: %v", p.Name(), err)
		}
	}

	m.wg.Wait()

	// Best-effort notify peers that this node is leaving
	leaveBody, _ := json.Marshal(map[string]string{"node_id": m.self.ID})
	for _, peer := range peersToNotify {
		for _, addr := range peer.Addresses {
			endpoint := strings.TrimRight(addr, "/") + "/api/v1/cluster/leave"
			reqCtx, reqCancel := context.WithTimeout(context.Background(), 1*time.Second)
			req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, endpoint, bytes.NewReader(leaveBody))
			if err == nil {
				req.Header.Set("Content-Type", "application/json")
				if m.clusterToken != "" {
					req.Header.Set("X-Cluster-Token", m.clusterToken)
				}
				resp, postErr := m.httpClient.Do(req)
				if postErr == nil {
					resp.Body.Close()
					reqCancel()
					break
				}
			}
			reqCancel()
		}
	}

	return nil
}

func (m *Manager) validateToken(token string) error {
	if m.clusterToken != "" && token != m.clusterToken {
		return ErrInvalidToken
	}
	return nil
}

// HandleHandshake processes an incoming handshake request from a peer.
// Validates ClusterToken, registers the peer, and returns Self + known peers.
func (m *Manager) HandleHandshake(req model.HandshakeRequest) (*model.HandshakeResponse, error) {
	if err := m.validateToken(req.ClusterToken); err != nil {
		return nil, err
	}

	if req.NodeID == "" {
		return nil, errors.New("missing node_id in handshake request")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if req.NodeID != m.self.ID {
		m.peers[req.NodeID] = &model.Node{
			ID:        req.NodeID,
			Name:      req.Name,
			Addresses: req.Addresses,
			Status:    model.NodeStatusAlive,
			Version:   req.Version,
			IsSelf:    false,
			LastSeen:  time.Now(),
		}
	}

	knownPeers := make([]model.Node, 0, len(m.peers)+1)
	knownPeers = append(knownPeers, m.self)
	for _, p := range m.peers {
		if p.ID != req.NodeID {
			knownPeers = append(knownPeers, *p)
		}
	}

	resp := &model.HandshakeResponse{
		NodeID:    m.self.ID,
		Name:      m.self.Name,
		Addresses: m.self.Addresses,
		Version:   m.self.Version,
		Peers:     knownPeers,
	}

	return resp, nil
}

// HandleHeartbeat processes an incoming heartbeat message.
// Validates ClusterToken, updates peer LastSeen and Addresses, and merges newly learned peers.
func (m *Manager) HandleHeartbeat(msg model.HeartbeatMessage) error {
	if err := m.validateToken(msg.ClusterToken); err != nil {
		return err
	}

	if msg.NodeID == "" {
		return errors.New("missing node_id in heartbeat message")
	}

	if msg.NodeID == m.self.ID {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if peer, ok := m.peers[msg.NodeID]; ok {
		peer.LastSeen = time.Now()
		peer.Status = model.NodeStatusAlive
		if len(msg.Addresses) > 0 {
			peer.Addresses = msg.Addresses
		}
	} else {
		m.peers[msg.NodeID] = &model.Node{
			ID:        msg.NodeID,
			Addresses: msg.Addresses,
			Status:    model.NodeStatusAlive,
			LastSeen:  time.Now(),
		}
	}

	// PEX merge for newly learned peers
	for _, kp := range msg.KnownPeers {
		if kp.ID == m.self.ID || kp.ID == msg.NodeID {
			continue
		}
		if existing, ok := m.peers[kp.ID]; ok {
			if len(existing.Addresses) == 0 && len(kp.Addresses) > 0 {
				existing.Addresses = kp.Addresses
			}
		} else {
			newNode := kp
			newNode.IsSelf = false
			if newNode.Status == "" {
				newNode.Status = model.NodeStatusAlive
			}
			if newNode.LastSeen.IsZero() {
				newNode.LastSeen = time.Now()
			}
			m.peers[kp.ID] = &newNode
		}
	}

	return nil
}

// HandleLeave marks a node as offline upon receiving a departure signal.
func (m *Manager) HandleLeave(nodeID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if peer, ok := m.peers[nodeID]; ok {
		peer.Status = model.NodeStatusOffline
	}
}

// RemoveNode manually deletes a peer from the cluster directory.
func (m *Manager) RemoveNode(nodeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if nodeID == m.self.ID {
		return ErrCannotRemoveSelf
	}

	if _, ok := m.peers[nodeID]; !ok {
		return ErrNodeNotFound
	}

	delete(m.peers, nodeID)
	return nil
}

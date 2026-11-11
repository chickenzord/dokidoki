package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// Cluster errors.
var (
	ErrInvalidToken     = errors.New("cluster: invalid or unauthorized cluster token")
	ErrCannotRemoveSelf = errors.New("cluster: cannot remove self node")
	ErrNodeNotFound     = errors.New("cluster: node not found")
)

// Manager coordinates cluster topology, peer discovery, heartbeats, and TTL tracking.
type Manager struct {
	self         Node
	mu           sync.RWMutex
	persistMu    sync.Mutex
	peers        map[string]*Node
	tombstones   map[string]time.Time
	clusterToken string
	ttl          time.Duration
	stacksDir    string
	providers    []DiscoveryProvider
	httpClient   *http.Client

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewManager creates an initialized cluster Manager.
// If stacksDir is provided, previously discovered nodes are loaded from $stacksDir/.dokidoki/nodes.json,
// and subsequent cluster changes will be persisted to that file.
func NewManager(self Node, clusterToken string, ttl time.Duration, httpClient *http.Client, stacksDir string) *Manager {
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 5 * time.Second,
		}
	}

	self.IsSelf = true
	self.Status = StatusAlive

	peers := make(map[string]*Node)
	tombstones := make(map[string]time.Time)
	if stacksDir != "" {
		if loadedTombstones, err := LoadPersistedTombstones(stacksDir); err == nil {
			tombstones = loadedTombstones
			if len(tombstones) > 0 {
				slog.Info("Cluster: loaded persisted tombstone(s)", "count", len(tombstones), "path", TombstonesFilePath(stacksDir))
			}
		} else {
			slog.Warn("Cluster: failed to load persisted tombstones", "error", err)
		}

		if loaded, err := LoadPersistedNodes(stacksDir); err == nil {
			for _, n := range loaded {
				if n.ID == "" || n.ID == self.ID {
					continue
				}
				if _, isDead := tombstones[n.ID]; isDead {
					continue
				}
				nodeCopy := n
				nodeCopy.IsSelf = false
				if nodeCopy.Status == StatusAlive {
					nodeCopy.Status = StatusSuspect
				}
				peers[nodeCopy.ID] = &nodeCopy
			}
			if len(peers) > 0 {
				slog.Info("Cluster: loaded persisted peer(s)", "count", len(peers), "path", NodesFilePath(stacksDir))
			}
		} else {
			slog.Warn("Cluster: failed to load persisted nodes", "error", err)
		}
	}

	return &Manager{
		self:         self,
		peers:        peers,
		tombstones:   tombstones,
		clusterToken: clusterToken,
		ttl:          ttl,
		stacksDir:    stacksDir,
		httpClient:   httpClient,
	}
}

// cloneNode creates a deep copy of a Node, specifically copying Addresses
// to prevent slice backing array aliasing across callers and background goroutines.
func cloneNode(n Node) Node {
	cp := n
	if n.Addresses != nil {
		cp.Addresses = append([]string(nil), n.Addresses...)
	}
	return cp
}

// Self returns the local node metadata.
func (m *Manager) Self() Node {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneNode(m.self)
}

// GetNode retrieves a node by ID, including self or registered peers.
func (m *Manager) GetNode(id string) (*Node, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if id == m.self.ID {
		cp := cloneNode(m.self)
		return &cp, true
	}

	peer, ok := m.peers[id]
	if !ok {
		return nil, false
	}
	cp := cloneNode(*peer)
	return &cp, true
}

// ListNodes returns Self and all registered peers sorted ascending by Name.
func (m *Manager) ListNodes() []Node {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nodes := make([]Node, 0, len(m.peers)+1)
	nodes = append(nodes, cloneNode(m.self))
	for _, p := range m.peers {
		nodes = append(nodes, cloneNode(*p))
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
			slog.Warn("Cluster: provider failed to start", "provider", prov.Name(), "error", err)
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
	now := time.Now()
	changed := false
	for _, peer := range m.peers {
		prevStatus := peer.Status
		age := now.Sub(peer.LastSeen)
		if age > m.ttl {
			peer.Status = StatusOffline
		} else if age > m.ttl/2 {
			peer.Status = StatusSuspect
		}
		if peer.Status != prevStatus {
			changed = true
		}
	}
	var snapshot []Node
	if changed {
		snapshot = m.peerSnapshotLocked()
	}
	m.mu.Unlock()

	if snapshot != nil {
		m.savePeers(snapshot)
	}
}

func (m *Manager) isTombstonedLocked(nodeID string) bool {
	ts, ok := m.tombstones[nodeID]
	if !ok {
		return false
	}
	if time.Since(ts) > defaultTombstoneTTL {
		delete(m.tombstones, nodeID)
		return false
	}
	return true
}

func (m *Manager) handlePeerEvent(ev PeerEvent) {
	if ev.NodeID == "" || ev.NodeID == m.self.ID {
		return
	}

	m.mu.Lock()
	if m.isTombstonedLocked(ev.NodeID) {
		m.mu.Unlock()
		return
	}

	changed := false
	switch ev.Type {
	case EventDiscovered, EventUpdated:
		var addrs []string
		if len(ev.Addresses) > 0 {
			addrs = append([]string(nil), ev.Addresses...)
		}

		if peer, ok := m.peers[ev.NodeID]; ok {
			peer.LastSeen = time.Now()
			if peer.Status != StatusAlive {
				peer.Status = StatusAlive
				changed = true
			}
			if ev.Name != "" && peer.Name != ev.Name {
				peer.Name = ev.Name
				changed = true
			}
			if len(ev.Addresses) > 0 && !equalStrings(peer.Addresses, ev.Addresses) {
				peer.Addresses = addrs
				changed = true
			}
		} else {
			m.peers[ev.NodeID] = &Node{
				ID:        ev.NodeID,
				Name:      ev.Name,
				Addresses: addrs,
				Status:    StatusAlive,
				LastSeen:  time.Now(),
			}
			changed = true
		}
	case EventLost:
		if peer, ok := m.peers[ev.NodeID]; ok {
			if peer.Status != StatusOffline {
				peer.Status = StatusOffline
				changed = true
			}
		}
	}

	var snapshot []Node
	if changed {
		snapshot = m.peerSnapshotLocked()
	}
	m.mu.Unlock()

	if snapshot != nil {
		m.savePeers(snapshot)
	}
}

// Stop stops all providers and notifies active peers if possible.
func (m *Manager) Stop(ctx context.Context) error {
	m.mu.Lock()
	if m.cancel != nil {
		m.cancel()
	}
	providers := make([]DiscoveryProvider, len(m.providers))
	copy(providers, m.providers)

	var peersToNotify []Node
	for _, p := range m.peers {
		peersToNotify = append(peersToNotify, cloneNode(*p))
	}
	snapshot := m.peerSnapshotLocked()
	m.mu.Unlock()

	if snapshot != nil {
		m.savePeers(snapshot)
	}

	for _, p := range providers {
		if err := p.Stop(); err != nil {
			slog.Warn("Cluster: error stopping provider", "provider", p.Name(), "error", err)
		}
	}

	m.wg.Wait()

	// Context with fallback to 5-10s timeout if ctx is nil or Background
	var cancel context.CancelFunc
	if ctx == nil {
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	} else if _, ok := ctx.Deadline(); !ok {
		ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
	}

	// Best-effort notify peers concurrently that this node is leaving
	leaveBody, _ := json.Marshal(map[string]string{"node_id": m.self.ID})
	var leaveWg sync.WaitGroup
	sem := make(chan struct{}, 10)

	for _, peer := range peersToNotify {
		if peer.IsSelf || peer.ID == m.self.ID || len(peer.Addresses) == 0 {
			continue
		}

		leaveWg.Add(1)
		go func(targetPeer Node) {
			defer leaveWg.Done()

			select {
			case <-ctx.Done():
				return
			case sem <- struct{}{}:
			}
			defer func() { <-sem }()

			for _, addr := range targetPeer.Addresses {
				if ctx.Err() != nil {
					return
				}
				endpoint := strings.TrimRight(addr, "/") + "/api/v1/cluster/leave"
				reqCtx, reqCancel := context.WithTimeout(ctx, 2*time.Second)
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
		}(peer)
	}

	leaveWg.Wait()
	return nil
}

func (m *Manager) validateToken(token string) error {
	if m.clusterToken != "" && token != m.clusterToken {
		return ErrInvalidToken
	}
	return nil
}

// AddPeer manually connects to a remote node at address, performs handshake,
// and registers the peer into the cluster topology.
func (m *Manager) AddPeer(ctx context.Context, address string) (*Node, error) {
	trimmed := strings.TrimSpace(address)
	if trimmed == "" {
		return nil, errors.New("address is required")
	}
	if !strings.HasPrefix(trimmed, "http://") && !strings.HasPrefix(trimmed, "https://") {
		trimmed = "http://" + trimmed
	}
	trimmed = strings.TrimRight(trimmed, "/")

	handshakeReq := HandshakeRequest{
		NodeID:       m.self.ID,
		Name:         m.self.Name,
		Addresses:    m.self.Addresses,
		Version:      m.self.Version,
		ReadOnly:     m.self.ReadOnly,
		ClusterToken: m.clusterToken,
	}

	body, err := json.Marshal(handshakeReq)
	if err != nil {
		return nil, err
	}

	target := trimmed + "/api/v1/cluster/handshake"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if m.clusterToken != "" {
		req.Header.Set("X-Cluster-Token", m.clusterToken)
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrInvalidToken
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("handshake failed with status %d", resp.StatusCode)
	}

	var hsResp HandshakeResponse
	if err := json.NewDecoder(resp.Body).Decode(&hsResp); err != nil {
		return nil, fmt.Errorf("failed to decode handshake response: %w", err)
	}

	if hsResp.NodeID == "" {
		return nil, errors.New("missing node_id in handshake response")
	}

	if hsResp.NodeID == m.self.ID {
		return nil, errors.New("cannot add self as peer")
	}

	// Remote node addresses
	addrs := hsResp.Addresses
	if len(addrs) == 0 {
		addrs = []string{trimmed}
	}

	m.mu.Lock()
	// Clear tombstone if previously tombstoned (force-join / intentional re-join)
	delete(m.tombstones, hsResp.NodeID)

	var targetNode *Node
	if peer, ok := m.peers[hsResp.NodeID]; ok {
		peer.LastSeen = time.Now()
		peer.Status = StatusAlive
		peer.Name = hsResp.Name
		peer.Addresses = append([]string(nil), addrs...)
		peer.Version = hsResp.Version
		peer.ReadOnly = hsResp.ReadOnly
		targetNode = peer
	} else {
		targetNode = &Node{
			ID:        hsResp.NodeID,
			Name:      hsResp.Name,
			Addresses: append([]string(nil), addrs...),
			Status:    StatusAlive,
			Version:   hsResp.Version,
			ReadOnly:  hsResp.ReadOnly,
			IsSelf:    false,
			LastSeen:  time.Now(),
		}
		m.peers[hsResp.NodeID] = targetNode
	}

	// Also merge any known peers returned in handshake response
	for _, peer := range hsResp.Peers {
		if peer.ID == m.self.ID || peer.ID == hsResp.NodeID || m.isTombstonedLocked(peer.ID) {
			continue
		}
		if existing, ok := m.peers[peer.ID]; ok {
			if len(existing.Addresses) == 0 && len(peer.Addresses) > 0 {
				existing.Addresses = append([]string(nil), peer.Addresses...)
			}
		} else {
			newNode := cloneNode(peer)
			newNode.IsSelf = false
			if newNode.Status == "" {
				newNode.Status = StatusAlive
			}
			if newNode.LastSeen.IsZero() {
				newNode.LastSeen = time.Now()
			}
			m.peers[peer.ID] = &newNode
		}
	}

	snapshot := m.peerSnapshotLocked()
	tombstonesSnapshot := m.tombstoneSnapshotLocked()
	resNode := cloneNode(*targetNode)
	m.mu.Unlock()

	if snapshot != nil {
		m.savePeers(snapshot)
	}
	if tombstonesSnapshot != nil {
		m.saveTombstones(tombstonesSnapshot)
	}

	return &resNode, nil
}

// HandleHandshake processes an incoming handshake request from a peer.
// Validates ClusterToken, registers the peer, and returns Self + known peers.
func (m *Manager) HandleHandshake(req HandshakeRequest) (*HandshakeResponse, error) {
	if err := m.validateToken(req.ClusterToken); err != nil {
		return nil, err
	}

	if req.NodeID == "" {
		return nil, errors.New("missing node_id in handshake request")
	}

	m.mu.Lock()
	var snapshot []Node
	if req.NodeID != m.self.ID && !m.isTombstonedLocked(req.NodeID) {
		var reqAddrs []string
		if len(req.Addresses) > 0 {
			reqAddrs = append([]string(nil), req.Addresses...)
		}
		if peer, ok := m.peers[req.NodeID]; ok {
			peer.LastSeen = time.Now()
			peer.Status = StatusAlive
			peer.Name = req.Name
			peer.Addresses = reqAddrs
			peer.Version = req.Version
			peer.ReadOnly = req.ReadOnly
		} else {
			m.peers[req.NodeID] = &Node{
				ID:        req.NodeID,
				Name:      req.Name,
				Addresses: reqAddrs,
				Status:    StatusAlive,
				Version:   req.Version,
				ReadOnly:  req.ReadOnly,
				IsSelf:    false,
				LastSeen:  time.Now(),
			}
		}
		snapshot = m.peerSnapshotLocked()
	}

	knownPeers := make([]Node, 0, len(m.peers)+1)
	knownPeers = append(knownPeers, cloneNode(m.self))
	for _, p := range m.peers {
		if p.ID != req.NodeID {
			knownPeers = append(knownPeers, cloneNode(*p))
		}
	}

	resp := &HandshakeResponse{
		NodeID:    m.self.ID,
		Name:      m.self.Name,
		Addresses: append([]string(nil), m.self.Addresses...),
		Version:   m.self.Version,
		ReadOnly:  m.self.ReadOnly,
		Peers:     knownPeers,
	}
	m.mu.Unlock()

	if snapshot != nil {
		m.savePeers(snapshot)
	}

	return resp, nil
}

// HandleHeartbeat processes an incoming heartbeat message.
// Validates ClusterToken, updates peer LastSeen and Addresses, and merges newly learned peers.
func (m *Manager) HandleHeartbeat(msg HeartbeatMessage) error {
	if err := m.validateToken(msg.ClusterToken); err != nil {
		return err
	}

	if msg.NodeID == "" {
		return errors.New("missing node_id in heartbeat message")
	}

	if msg.NodeID == m.self.ID {
		return nil
	}

	slog.Debug("Cluster: received heartbeat", "node_id", msg.NodeID, "addresses", msg.Addresses)

	m.mu.Lock()
	if m.isTombstonedLocked(msg.NodeID) {
		m.mu.Unlock()
		return nil
	}

	changed := false
	var addrs []string
	if len(msg.Addresses) > 0 {
		addrs = append([]string(nil), msg.Addresses...)
	}

	if peer, ok := m.peers[msg.NodeID]; ok {
		peer.LastSeen = time.Now()
		if peer.Status != StatusAlive {
			peer.Status = StatusAlive
			changed = true
		}
		if len(msg.Addresses) > 0 && !equalStrings(peer.Addresses, msg.Addresses) {
			peer.Addresses = addrs
			changed = true
		}
		if peer.ReadOnly != msg.ReadOnly {
			peer.ReadOnly = msg.ReadOnly
			changed = true
		}
	} else {
		m.peers[msg.NodeID] = &Node{
			ID:        msg.NodeID,
			Addresses: addrs,
			Status:    StatusAlive,
			ReadOnly:  msg.ReadOnly,
			LastSeen:  time.Now(),
		}
		changed = true
	}

	// PEX merge for newly learned peers
	for _, kp := range msg.KnownPeers {
		if kp.ID == m.self.ID || kp.ID == msg.NodeID || m.isTombstonedLocked(kp.ID) {
			continue
		}
		if existing, ok := m.peers[kp.ID]; ok {
			if len(existing.Addresses) == 0 && len(kp.Addresses) > 0 {
				existing.Addresses = append([]string(nil), kp.Addresses...)
				changed = true
			}
			if existing.ReadOnly != kp.ReadOnly {
				existing.ReadOnly = kp.ReadOnly
				changed = true
			}
		} else {
			newNode := cloneNode(kp)
			newNode.IsSelf = false
			if newNode.Status == "" {
				newNode.Status = StatusAlive
			}
			if newNode.LastSeen.IsZero() {
				newNode.LastSeen = time.Now()
			}
			m.peers[kp.ID] = &newNode
			changed = true
		}
	}

	var snapshot []Node
	if changed {
		snapshot = m.peerSnapshotLocked()
	}
	m.mu.Unlock()

	if snapshot != nil {
		m.savePeers(snapshot)
	}

	return nil
}

// HandleLeave marks a node as offline upon receiving a departure signal.
func (m *Manager) HandleLeave(nodeID string) {
	m.mu.Lock()
	var snapshot []Node
	if peer, ok := m.peers[nodeID]; ok {
		peer.Status = StatusOffline
		snapshot = m.peerSnapshotLocked()
	}
	m.mu.Unlock()

	if snapshot != nil {
		m.savePeers(snapshot)
	}
}

// RemoveNode manually deletes a peer from the cluster directory, records a tombstone,
// and optionally fans out the deletion request to remaining active peers if broadcast is true.
func (m *Manager) RemoveNode(nodeID string, broadcast ...bool) error {
	m.mu.Lock()
	if nodeID == m.self.ID {
		m.mu.Unlock()
		return ErrCannotRemoveSelf
	}

	peer, ok := m.peers[nodeID]
	if !ok {
		// Even if not currently in peers map, if it's already tombstoned, return ErrNodeNotFound
		m.mu.Unlock()
		return ErrNodeNotFound
	}

	deletedPeer := cloneNode(*peer)
	delete(m.peers, nodeID)
	if m.tombstones == nil {
		m.tombstones = make(map[string]time.Time)
	}
	m.tombstones[nodeID] = time.Now()

	snapshot := m.peerSnapshotLocked()
	tombstonesSnapshot := m.tombstoneSnapshotLocked()

	// Snapshot remaining peers for fan-out
	var peersToNotify []Node
	for _, p := range m.peers {
		if !p.IsSelf && p.ID != m.self.ID && p.Status != StatusOffline && len(p.Addresses) > 0 {
			peersToNotify = append(peersToNotify, cloneNode(*p))
		}
	}
	m.mu.Unlock()

	if snapshot != nil {
		m.savePeers(snapshot)
	}
	if tombstonesSnapshot != nil {
		m.saveTombstones(tombstonesSnapshot)
	}

	shouldBroadcast := true
	if len(broadcast) > 0 {
		shouldBroadcast = broadcast[0]
	}

	if shouldBroadcast {
		// Fan out deletion concurrently with 30s timeout per peer
		m.fanOutDelete(nodeID, peersToNotify, deletedPeer)
	}

	return nil
}

// LeaveCluster triggers self node departure from the cluster:
// notifies known peers that this node is leaving, clears all known peers,
// and clears the persisted nodes file to reset this node to standalone mode.
func (m *Manager) LeaveCluster(ctx context.Context) error {
	m.mu.Lock()
	peersToNotify := make([]Node, 0, len(m.peers))
	for _, p := range m.peers {
		if !p.IsSelf && p.ID != m.self.ID && len(p.Addresses) > 0 {
			peersToNotify = append(peersToNotify, cloneNode(*p))
		}
	}
	// Clear all known peers
	m.peers = make(map[string]*Node)
	m.mu.Unlock()

	// Best-effort notify peers that self is leaving (concurrently)
	var cancel context.CancelFunc
	if ctx == nil {
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	} else if _, ok := ctx.Deadline(); !ok {
		ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
	}

	leaveBody, _ := json.Marshal(map[string]string{"node_id": m.self.ID})
	var leaveWg sync.WaitGroup
	sem := make(chan struct{}, 10)

	for _, peer := range peersToNotify {
		leaveWg.Add(1)
		go func(targetPeer Node) {
			defer leaveWg.Done()

			select {
			case <-ctx.Done():
				return
			case sem <- struct{}{}:
			}
			defer func() { <-sem }()

			for _, addr := range targetPeer.Addresses {
				if ctx.Err() != nil {
					return
				}
				endpoint := strings.TrimRight(addr, "/") + "/api/v1/cluster/leave"
				reqCtx, reqCancel := context.WithTimeout(ctx, 30*time.Second)
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
		}(peer)
	}

	leaveWg.Wait()

	// Clear persisted peers file by saving empty list
	m.savePeers([]Node{})

	return nil
}

func (m *Manager) fanOutDelete(deletedNodeID string, peersToNotify []Node, deletedPeer Node) {
	// Attempt to notify target deleted peer as well if it had addresses
	allTargets := make([]Node, 0, len(peersToNotify)+1)
	if len(deletedPeer.Addresses) > 0 {
		allTargets = append(allTargets, deletedPeer)
	}
	allTargets = append(allTargets, peersToNotify...)

	sem := make(chan struct{}, 10)
	var wg sync.WaitGroup

	for _, p := range allTargets {
		wg.Add(1)
		go func(target Node) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			default:
				// If concurrency limit reached, still acquire slot
				sem <- struct{}{}
			}
			defer func() { <-sem }()

			for _, addr := range target.Addresses {
				endpoint := strings.TrimRight(addr, "/") + "/api/v1/nodes/" + deletedNodeID + "?broadcast=false"
				reqCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				req, err := http.NewRequestWithContext(reqCtx, http.MethodDelete, endpoint, nil)
				if err != nil {
					cancel()
					continue
				}
				if m.clusterToken != "" {
					req.Header.Set("X-Cluster-Token", m.clusterToken)
				}
				resp, err := m.httpClient.Do(req)
				if err != nil {
					cancel()
					continue
				}
				resp.Body.Close()
				cancel()
				if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotFound {
					break
				}
			}
		}(p)
	}

	// We can let fan-out run asynchronously in background or wait with short grace period
	// Run asynchronously so caller is never blocked if peers take up to 30s
	_ = wg
}

func (m *Manager) tombstoneSnapshotLocked() map[string]time.Time {
	if m.stacksDir == "" || len(m.tombstones) == 0 {
		return nil
	}
	cp := make(map[string]time.Time, len(m.tombstones))
	now := time.Now()
	for id, ts := range m.tombstones {
		if now.Sub(ts) < defaultTombstoneTTL {
			cp[id] = ts
		}
	}
	return cp
}

func (m *Manager) saveTombstones(snapshot map[string]time.Time) {
	if m.stacksDir == "" || snapshot == nil {
		return
	}
	m.persistMu.Lock()
	defer m.persistMu.Unlock()

	if err := SavePersistedTombstones(m.stacksDir, snapshot); err != nil {
		slog.Warn("Cluster: failed to persist tombstones", "error", err)
	}
}

func (m *Manager) peerSnapshotLocked() []Node {
	if m.stacksDir == "" {
		return nil
	}
	peers := make([]Node, 0, len(m.peers))
	for _, p := range m.peers {
		if p.ID == m.self.ID || p.IsSelf {
			continue
		}
		peers = append(peers, cloneNode(*p))
	}
	return peers
}

func (m *Manager) savePeers(snapshot []Node) {
	if m.stacksDir == "" || snapshot == nil {
		return
	}
	m.persistMu.Lock()
	defer m.persistMu.Unlock()

	if err := SavePersistedNodes(m.stacksDir, snapshot); err != nil {
		slog.Warn("Cluster: failed to persist discovered nodes", "error", err)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

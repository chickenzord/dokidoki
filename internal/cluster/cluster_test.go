package cluster

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/chickenzord/dokidoki/internal/model"
	"github.com/google/uuid"
)

func TestResolveNodeID(t *testing.T) {
	// 1. Configured ID takes top priority
	t.Run("ConfiguredID", func(t *testing.T) {
		id, err := ResolveNodeID("configured-node-abc", "/tmp")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id != "configured-node-abc" {
			t.Errorf("expected 'configured-node-abc', got %q", id)
		}
	})

	// 2. Existing valid UUID in file
	t.Run("ExistingFileValidUUID", func(t *testing.T) {
		tempDir := t.TempDir()
		dokidokiDir := filepath.Join(tempDir, ".dokidoki")
		if err := os.MkdirAll(dokidokiDir, 0755); err != nil {
			t.Fatal(err)
		}
		expectedID := uuid.NewString()
		if err := os.WriteFile(filepath.Join(dokidokiDir, "node_id"), []byte(expectedID+"\n"), 0644); err != nil {
			t.Fatal(err)
		}

		id, err := ResolveNodeID("", tempDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id != expectedID {
			t.Errorf("expected %q, got %q", expectedID, id)
		}
	})

	// 3. No file exists -> generate, persist, and verify subsequent reads
	t.Run("GenerateAndPersist", func(t *testing.T) {
		tempDir := t.TempDir()
		id, err := ResolveNodeID("", tempDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, parseErr := uuid.Parse(id); parseErr != nil {
			t.Fatalf("generated ID is not a valid UUID: %q (%v)", id, parseErr)
		}

		// Read file directly from disk
		data, err := os.ReadFile(filepath.Join(tempDir, ".dokidoki", "node_id"))
		if err != nil {
			t.Fatalf("failed to read persisted node_id: %v", err)
		}
		if strings.TrimSpace(string(data)) != id {
			t.Errorf("file content %q does not match generated %q", string(data), id)
		}

		// Subsequent resolve returns the exact same ID
		secondID, err := ResolveNodeID("", tempDir)
		if err != nil {
			t.Fatal(err)
		}
		if secondID != id {
			t.Errorf("second resolve got %q, want %q", secondID, id)
		}
	})

	// 4. Invalid file content -> regenerated
	t.Run("InvalidFileContentRegenerated", func(t *testing.T) {
		tempDir := t.TempDir()
		dokidokiDir := filepath.Join(tempDir, ".dokidoki")
		_ = os.MkdirAll(dokidokiDir, 0755)
		_ = os.WriteFile(filepath.Join(dokidokiDir, "node_id"), []byte("corrupt-not-uuid\n"), 0644)

		id, err := ResolveNodeID("", tempDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, parseErr := uuid.Parse(id); parseErr != nil {
			t.Fatalf("expected valid UUID after regenerating, got %q", id)
		}
	})

	// 5. Unwritable directory fallback to in-memory UUID without error
	t.Run("UnwritableDirFallback", func(t *testing.T) {
		tempDir := t.TempDir()
		// Make directory read-only
		if err := os.Chmod(tempDir, 0555); err != nil {
			t.Skip("unable to chmod read-only on this OS")
		}
		defer os.Chmod(tempDir, 0755)

		id, err := ResolveNodeID("", filepath.Join(tempDir, "cannot_create"))
		if err != nil {
			t.Fatalf("expected fallback in memory without error, got: %v", err)
		}
		if _, parseErr := uuid.Parse(id); parseErr != nil {
			t.Fatalf("expected valid UUID, got %q", id)
		}
	})
}

func TestDetectCandidateAddresses(t *testing.T) {
	t.Run("ConfiguredAddressesNormalized", func(t *testing.T) {
		configured := []string{
			"192.168.1.100:8080",
			"http://10.0.0.5:8080/",
			"https://edge.domain.internal:8443",
		}

		result := DetectCandidateAddresses("0.0.0.0", 8080, configured)
		expected := []string{
			"http://192.168.1.100:8080",
			"http://10.0.0.5:8080",
			"https://edge.domain.internal:8443",
		}

		if len(result) != len(expected) {
			t.Fatalf("expected %d addresses, got %d: %v", len(expected), len(result), result)
		}
		for i := range expected {
			if result[i] != expected[i] {
				t.Errorf("index %d: expected %q, got %q", i, expected[i], result[i])
			}
		}
	})

	t.Run("AutoDetectInterfaceAddresses", func(t *testing.T) {
		candidates := DetectCandidateAddresses("0.0.0.0", 8080, nil)
		if len(candidates) == 0 {
			t.Fatal("expected at least one candidate address")
		}

		// Verify sorted
		if !sort.StringsAreSorted(candidates) {
			t.Errorf("candidate addresses should be sorted: %v", candidates)
		}

		for _, addr := range candidates {
			if !strings.HasPrefix(addr, "http://") {
				t.Errorf("candidate %q missing http:// prefix", addr)
			}
			if !strings.HasSuffix(addr, ":8080") {
				t.Errorf("candidate %q missing :8080 port", addr)
			}
		}
	})
}

func TestManagerPeerRegistration(t *testing.T) {
	selfNode := model.Node{
		ID:        "self-node-1",
		Name:      "Node A",
		Addresses: []string{"http://10.0.0.1:8080"},
		Version:   "0.1.0",
	}

	mgr := NewManager(selfNode, "test-token", 30*time.Second, nil)

	// Verify Self
	self := mgr.Self()
	if self.ID != "self-node-1" || !self.IsSelf || self.Status != model.NodeStatusAlive {
		t.Errorf("invalid self node: %+v", self)
	}

	// Verify GetNode for self
	node, ok := mgr.GetNode("self-node-1")
	if !ok || node.ID != "self-node-1" {
		t.Fatalf("GetNode for self failed: ok=%v, node=%+v", ok, node)
	}

	// Register peer via Handshake
	hsReq := model.HandshakeRequest{
		NodeID:       "peer-node-2",
		Name:         "Node B",
		Addresses:    []string{"http://10.0.0.2:8080"},
		Version:      "0.1.0",
		ClusterToken: "test-token",
	}

	resp, err := mgr.HandleHandshake(hsReq)
	if err != nil {
		t.Fatalf("HandleHandshake failed: %v", err)
	}
	if resp.NodeID != selfNode.ID {
		t.Errorf("expected response NodeID to be self, got %q", resp.NodeID)
	}

	// Verify peer registered
	peer, ok := mgr.GetNode("peer-node-2")
	if !ok {
		t.Fatal("peer-node-2 not found after handshake")
	}
	if peer.Name != "Node B" || peer.Status != model.NodeStatusAlive || peer.IsSelf {
		t.Errorf("unexpected peer attributes: %+v", peer)
	}

	// ListNodes sorted by name
	nodes := mgr.ListNodes()
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}
	if nodes[0].Name != "Node A" || nodes[1].Name != "Node B" {
		t.Errorf("nodes not sorted by name: [%s, %s]", nodes[0].Name, nodes[1].Name)
	}

	// HandleLeave marks node offline
	mgr.HandleLeave("peer-node-2")
	peerNode, ok := mgr.GetNode("peer-node-2")
	if !ok {
		t.Errorf("peer-node-2 should still exist after HandleLeave")
	}
	if peerNode.Status != model.NodeStatusOffline {
		t.Errorf("expected offline status after HandleLeave, got %v", peerNode.Status)
	}

	// Manual removal deletes it
	if err := mgr.RemoveNode("peer-node-2"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := mgr.GetNode("peer-node-2"); ok {
		t.Errorf("peer-node-2 should be removed after RemoveNode")
	}
}

func TestManagerTTLEviction(t *testing.T) {
	selfNode := model.Node{
		ID:   "self-1",
		Name: "Self",
	}
	ttl := 100 * time.Millisecond
	mgr := NewManager(selfNode, "", ttl, nil)

	// Add peer
	peerID := "peer-expiring"
	_, err := mgr.HandleHandshake(model.HandshakeRequest{
		NodeID: peerID,
		Name:   "Peer Expiring",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Manually backdate LastSeen to simulate half-TTL (suspect)
	mgr.mu.Lock()
	mgr.peers[peerID].LastSeen = time.Now().Add(-60 * time.Millisecond)
	mgr.mu.Unlock()

	mgr.EvictExpiredPeers()
	node, ok := mgr.GetNode(peerID)
	if !ok {
		t.Fatal("node should still exist in suspect state")
	}
	if node.Status != model.NodeStatusSuspect {
		t.Errorf("expected suspect status, got %v", node.Status)
	}

	// Manually backdate LastSeen beyond TTL (offline)
	mgr.mu.Lock()
	mgr.peers[peerID].LastSeen = time.Now().Add(-150 * time.Millisecond)
	mgr.mu.Unlock()

	mgr.EvictExpiredPeers()
	node, ok = mgr.GetNode(peerID)
	if !ok {
		t.Fatalf("node should still exist in offline state")
	}
	if node.Status != model.NodeStatusOffline {
		t.Errorf("expected offline status, got %v", node.Status)
	}

	// Manual removal
	if err := mgr.RemoveNode(peerID); err != nil {
		t.Fatalf("unexpected error removing node: %v", err)
	}
	_, ok = mgr.GetNode(peerID)
	if ok {
		t.Errorf("node should be deleted after RemoveNode")
	}

	// Cannot remove self
	if err := mgr.RemoveNode(selfNode.ID); err != ErrCannotRemoveSelf {
		t.Errorf("expected ErrCannotRemoveSelf, got %v", err)
	}
}

func TestManagerTokenValidation(t *testing.T) {
	selfNode := model.Node{ID: "self", Name: "Self"}
	validToken := "cluster-secret-key"

	t.Run("TokenEnforced", func(t *testing.T) {
		mgr := NewManager(selfNode, validToken, 30*time.Second, nil)

		// 1. Handshake with valid token
		_, err := mgr.HandleHandshake(model.HandshakeRequest{
			NodeID:       "peer-1",
			Name:         "Peer 1",
			ClusterToken: validToken,
		})
		if err != nil {
			t.Errorf("expected success with valid token, got: %v", err)
		}

		// 2. Handshake with wrong token
		_, err = mgr.HandleHandshake(model.HandshakeRequest{
			NodeID:       "peer-2",
			Name:         "Peer 2",
			ClusterToken: "wrong-token",
		})
		if err != ErrInvalidToken {
			t.Errorf("expected ErrInvalidToken, got: %v", err)
		}

		// 3. Handshake with empty token
		_, err = mgr.HandleHandshake(model.HandshakeRequest{
			NodeID:       "peer-3",
			Name:         "Peer 3",
			ClusterToken: "",
		})
		if err != ErrInvalidToken {
			t.Errorf("expected ErrInvalidToken for empty token, got: %v", err)
		}

		// 4. Heartbeat with valid token
		err = mgr.HandleHeartbeat(model.HeartbeatMessage{
			NodeID:       "peer-1",
			ClusterToken: validToken,
		})
		if err != nil {
			t.Errorf("expected heartbeat success with valid token, got: %v", err)
		}

		// 5. Heartbeat with invalid token
		err = mgr.HandleHeartbeat(model.HeartbeatMessage{
			NodeID:       "peer-1",
			ClusterToken: "invalid-token",
		})
		if err != ErrInvalidToken {
			t.Errorf("expected ErrInvalidToken on heartbeat, got: %v", err)
		}
	})

	t.Run("OpenClusterNoTokenRequired", func(t *testing.T) {
		mgr := NewManager(selfNode, "", 30*time.Second, nil)

		// Handshake with empty token succeeds
		_, err := mgr.HandleHandshake(model.HandshakeRequest{
			NodeID: "peer-open",
			Name:   "Peer Open",
		})
		if err != nil {
			t.Errorf("expected success for open cluster, got: %v", err)
		}

		// Heartbeat with empty token succeeds
		err = mgr.HandleHeartbeat(model.HeartbeatMessage{
			NodeID: "peer-open",
		})
		if err != nil {
			t.Errorf("expected heartbeat success for open cluster, got: %v", err)
		}
	})
}

func TestPEXMerge(t *testing.T) {
	selfNode := model.Node{ID: "self", Name: "Self"}
	mgr := NewManager(selfNode, "", 30*time.Second, nil)

	// Heartbeat introduces sender node and peer-3 via KnownPeers
	err := mgr.HandleHeartbeat(model.HeartbeatMessage{
		NodeID:    "peer-2",
		Addresses: []string{"http://10.0.0.2:8080"},
		KnownPeers: []model.Node{
			{
				ID:        "peer-3",
				Name:      "Node 3",
				Addresses: []string{"http://10.0.0.3:8080"},
			},
		},
	})
	if err != nil {
		t.Fatalf("HandleHeartbeat failed: %v", err)
	}

	// Verify both peer-2 and peer-3 are registered
	if _, ok := mgr.GetNode("peer-2"); !ok {
		t.Errorf("peer-2 not found")
	}
	p3, ok := mgr.GetNode("peer-3")
	if !ok {
		t.Fatalf("peer-3 not found after PEX merge")
	}
	if p3.Name != "Node 3" || len(p3.Addresses) == 0 || p3.Addresses[0] != "http://10.0.0.3:8080" {
		t.Errorf("unexpected peer-3 state: %+v", p3)
	}
}

func TestStaticProvider(t *testing.T) {
	// Setup a mock peer server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/cluster/handshake" {
			http.NotFound(w, r)
			return
		}

		var req model.HandshakeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		resp := model.HandshakeResponse{
			NodeID:    "mock-remote-node",
			Name:      "Remote Node",
			Addresses: []string{"http://127.0.0.1:9999"},
			Version:   "0.1.0",
			Peers: []model.Node{
				{
					ID:        "gossip-peer",
					Name:      "Gossip Peer",
					Addresses: []string{"http://127.0.0.1:9998"},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	selfNode := model.Node{
		ID:        "self-node",
		Name:      "Local Node",
		Addresses: []string{"http://127.0.0.1:8080"},
		Version:   "0.1.0",
	}

	events := make(chan PeerEvent, 10)
	provider := NewStaticProvider([]string{mockServer.URL}, selfNode, "", mockServer.Client())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := provider.Start(ctx, events); err != nil {
		t.Fatalf("provider.Start failed: %v", err)
	}
	defer provider.Stop()

	// Expect discovered event for mock-remote-node and gossip-peer
	discovered := make(map[string]bool)
	timeout := time.After(2 * time.Second)

	for len(discovered) < 2 {
		select {
		case ev := <-events:
			if ev.Type == EventDiscovered {
				discovered[ev.NodeID] = true
			}
		case <-timeout:
			t.Fatalf("timed out waiting for peer discovery events, got %d/2: %v", len(discovered), discovered)
		}
	}

	if !discovered["mock-remote-node"] || !discovered["gossip-peer"] {
		t.Errorf("missing expected discovered nodes: %v", discovered)
	}
}

func TestPEXProvider(t *testing.T) {
	heartbeatReceived := make(chan string, 1)
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/cluster/heartbeat" {
			var msg model.HeartbeatMessage
			_ = json.NewDecoder(r.Body).Decode(&msg)
			heartbeatReceived <- msg.NodeID
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer mockServer.Close()

	selfNode := model.Node{
		ID:        "self-node",
		Name:      "Local Node",
		Addresses: []string{"http://127.0.0.1:8080"},
	}

	peersFunc := func() []model.Node {
		return []model.Node{
			selfNode,
			{
				ID:        "target-peer",
				Name:      "Target Peer",
				Addresses: []string{mockServer.URL},
			},
		}
	}

	events := make(chan PeerEvent, 10)
	provider := NewPEXProvider(selfNode, "", peersFunc, 50*time.Millisecond, mockServer.Client())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := provider.Start(ctx, events); err != nil {
		t.Fatalf("PEX Start failed: %v", err)
	}
	defer provider.Stop()

	select {
	case sender := <-heartbeatReceived:
		if sender != selfNode.ID {
			t.Errorf("expected heartbeat from %q, got %q", selfNode.ID, sender)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for heartbeat to be received")
	}

	select {
	case ev := <-events:
		if ev.Type != EventUpdated || ev.NodeID != "target-peer" {
			t.Errorf("unexpected event: %+v", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for peer update event")
	}
}

func TestManagerLifecycleAndEventConsumption(t *testing.T) {
	selfNode := model.Node{
		ID:        "self-lifecycle",
		Name:      "Self Lifecycle",
		Addresses: []string{"http://127.0.0.1:8080"},
	}

	mgr := NewManager(selfNode, "", 200*time.Millisecond, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mgr.Start(ctx)
	defer mgr.Stop()

	// Simulate discovery event
	mgr.handlePeerEvent(PeerEvent{
		Type:      EventDiscovered,
		NodeID:    "peer-event-1",
		Name:      "Discovered Peer",
		Addresses: []string{"http://127.0.0.1:9090"},
	})

	node, ok := mgr.GetNode("peer-event-1")
	if !ok || node.Name != "Discovered Peer" {
		t.Fatalf("expected peer-event-1 to be discovered: ok=%v, node=%+v", ok, node)
	}

	// Simulate peer lost event
	mgr.handlePeerEvent(PeerEvent{
		Type:   EventLost,
		NodeID: "peer-event-1",
	})

	node, ok = mgr.GetNode("peer-event-1")
	if !ok {
		t.Fatalf("expected peer-event-1 to still exist in offline status after EventLost")
	}
	if node.Status != model.NodeStatusOffline {
		t.Errorf("expected offline status after EventLost, got %v", node.Status)
	}

	// Manual removal
	if err := mgr.RemoveNode("peer-event-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := mgr.GetNode("peer-event-1"); ok {
		t.Errorf("expected peer-event-1 to be removed after RemoveNode")
	}
}

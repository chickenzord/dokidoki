package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/chickenzord/dokidoki/internal/logger"

	"github.com/grandcat/zeroconf"
)

const (
	mDNSService = "_dokidoki._tcp"
	mDNSDomain  = "local."
)

// MDNSProvider advertises and browses Dokidoki instances over LAN via mDNS / DNS-SD.
type MDNSProvider struct {
	self         Node
	port         int
	clusterToken string
	client       *http.Client

	server *zeroconf.Server
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.Mutex
}

// NewMDNSProvider creates a new MDNSProvider.
func NewMDNSProvider(self Node, port int, clusterToken string, client *http.Client) *MDNSProvider {
	if client == nil {
		client = &http.Client{
			Timeout: 5 * time.Second,
		}
	}
	return &MDNSProvider{
		self:         self,
		port:         port,
		clusterToken: clusterToken,
		client:       client,
	}
}

// Name returns the provider identifier.
func (p *MDNSProvider) Name() string {
	return "mdns"
}

// Start registers the mDNS service and begins browsing for peers.
func (p *MDNSProvider) Start(ctx context.Context, events chan<- PeerEvent) error {
	p.mu.Lock()
	p.ctx, p.cancel = context.WithCancel(ctx)
	p.mu.Unlock()

	txt := []string{
		"id=" + p.self.ID,
		"name=" + p.self.Name,
		"version=" + p.self.Version,
	}

	server, err := zeroconf.Register(
		p.self.ID,
		mDNSService,
		mDNSDomain,
		p.port,
		txt,
		nil,
	)
	if err != nil {
		logger.Warnf("Cluster: mDNS registration failed (will still attempt browsing): %v", err)
	} else {
		p.mu.Lock()
		p.server = server
		p.mu.Unlock()
	}

	resolver, err := zeroconf.NewResolver(zeroconf.SelectIPTraffic(zeroconf.IPv4))
	if err != nil {
		resolver, err = zeroconf.NewResolver()
	}
	if err != nil {
		return fmt.Errorf("failed to initialize mDNS resolver: %w", err)
	}

	entries := make(chan *zeroconf.ServiceEntry, 32)

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		if err := resolver.Browse(p.ctx, mDNSService, mDNSDomain, entries); err != nil {
			logger.Debugf("Cluster: mDNS browse ended: %v", err)
		}
	}()

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		for {
			select {
			case <-p.ctx.Done():
				return
			case entry, ok := <-entries:
				if !ok {
					return
				}
				p.handleDiscoveredEntry(entry, events)
			}
		}
	}()

	return nil
}

// Stop unregisters the mDNS service and stops browsing.
func (p *MDNSProvider) Stop() error {
	p.mu.Lock()
	if p.cancel != nil {
		p.cancel()
	}
	if p.server != nil {
		p.server.Shutdown()
		p.server = nil
	}
	p.mu.Unlock()

	p.wg.Wait()
	return nil
}

func (p *MDNSProvider) handleDiscoveredEntry(entry *zeroconf.ServiceEntry, events chan<- PeerEvent) {
	nodeID := entry.Instance
	for _, t := range entry.Text {
		if strings.HasPrefix(t, "id=") {
			nodeID = strings.TrimPrefix(t, "id=")
			break
		}
	}

	if nodeID == p.self.ID || nodeID == "" {
		return
	}

	var candidates []string
	for _, ip := range entry.AddrIPv4 {
		candidates = append(candidates, fmt.Sprintf("http://%s:%d", ip.String(), entry.Port))
	}
	if len(candidates) == 0 && entry.HostName != "" {
		host := strings.TrimRight(entry.HostName, ".")
		candidates = append(candidates, fmt.Sprintf("http://%s:%d", host, entry.Port))
	}

	// Attempt handshake
	for _, addr := range candidates {
		target := strings.TrimRight(addr, "/") + "/api/v1/cluster/handshake"
		handshakeReq := HandshakeRequest{
			NodeID:       p.self.ID,
			Name:         p.self.Name,
			Addresses:    p.self.Addresses,
			Version:      p.self.Version,
			ClusterToken: p.clusterToken,
		}

		body, err := json.Marshal(handshakeReq)
		if err != nil {
			continue
		}

		reqCtx, reqCancel := context.WithTimeout(p.ctx, 3*time.Second)
		req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, target, bytes.NewReader(body))
		if err != nil {
			reqCancel()
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		if p.clusterToken != "" {
			req.Header.Set("X-Cluster-Token", p.clusterToken)
		}

		resp, err := p.client.Do(req)
		if err != nil {
			reqCancel()
			continue
		}

		var hsResp HandshakeResponse
		decodeErr := json.NewDecoder(resp.Body).Decode(&hsResp)
		resp.Body.Close()
		reqCancel()

		if decodeErr != nil || resp.StatusCode != http.StatusOK {
			continue
		}

		select {
		case events <- PeerEvent{
			Type:      EventDiscovered,
			NodeID:    hsResp.NodeID,
			Name:      hsResp.Name,
			Addresses: hsResp.Addresses,
		}:
		case <-p.ctx.Done():
			return
		}

		break
	}
}

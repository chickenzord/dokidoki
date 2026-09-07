package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// PEXProvider periodically sends heartbeats to active peers to exchange topology tables.
type PEXProvider struct {
	self         Node
	clusterToken string
	peersFunc    func() []Node
	interval     time.Duration
	client       *http.Client

	mu       sync.Mutex
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	workerWg sync.WaitGroup
	sem      chan struct{}
}

// NewPEXProvider creates a new PEX (Peer Exchange) provider.
func NewPEXProvider(
	self Node,
	clusterToken string,
	peersFunc func() []Node,
	interval time.Duration,
	client *http.Client,
) *PEXProvider {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	if client == nil {
		client = &http.Client{
			Timeout: 5 * time.Second,
		}
	}

	return &PEXProvider{
		self:         self,
		clusterToken: clusterToken,
		peersFunc:    peersFunc,
		interval:     interval,
		client:       client,
		sem:          make(chan struct{}, 10),
	}
}

// Name returns the provider identifier.
func (p *PEXProvider) Name() string {
	return "pex"
}

// Start initiates periodic heartbeat exchanges with known peers.
func (p *PEXProvider) Start(ctx context.Context, events chan<- PeerEvent) error {
	p.mu.Lock()
	if p.cancel != nil {
		p.cancel()
	}
	p.ctx, p.cancel = context.WithCancel(ctx)
	if p.sem == nil {
		p.sem = make(chan struct{}, 10)
	}
	pCtx := p.ctx
	p.mu.Unlock()

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()

		ticker := time.NewTicker(p.interval)
		defer ticker.Stop()

		for {
			select {
			case <-pCtx.Done():
				return
			case <-ticker.C:
				p.sendHeartbeats(events)
			}
		}
	}()

	return nil
}

// Stop halts heartbeat dissemination.
func (p *PEXProvider) Stop() error {
	p.mu.Lock()
	if p.cancel != nil {
		p.cancel()
	}
	p.mu.Unlock()

	p.wg.Wait()
	p.workerWg.Wait()
	return nil
}

func (p *PEXProvider) sendHeartbeats(events chan<- PeerEvent) {
	if p.peersFunc == nil {
		return
	}

	p.mu.Lock()
	if p.ctx == nil || p.ctx.Err() != nil {
		p.mu.Unlock()
		return
	}
	p.mu.Unlock()

	knownPeers := p.peersFunc()
	msg := HeartbeatMessage{
		NodeID:       p.self.ID,
		Addresses:    append([]string(nil), p.self.Addresses...),
		ClusterToken: p.clusterToken,
		KnownPeers:   knownPeers,
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return
	}

	for _, peer := range knownPeers {
		if peer.IsSelf || peer.ID == p.self.ID {
			continue
		}

		peerNode := peer
		p.workerWg.Add(1)
		go func(targetPeer Node) {
			defer p.workerWg.Done()

			select {
			case <-p.ctx.Done():
				return
			case p.sem <- struct{}{}:
			}
			defer func() { <-p.sem }()

			for _, addr := range targetPeer.Addresses {
				target := strings.TrimRight(addr, "/") + "/api/v1/cluster/heartbeat"
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
					slog.Debug("Cluster: heartbeat failed", "peer_id", targetPeer.ID, "target", target, "error", err)
					reqCancel()
					continue
				}
				resp.Body.Close()
				reqCancel()

				if resp.StatusCode == http.StatusOK {
					slog.Debug("Cluster: heartbeat succeeded", "peer_id", targetPeer.ID, "target", target)
					select {
					case events <- PeerEvent{
						Type:      EventUpdated,
						NodeID:    targetPeer.ID,
						Name:      targetPeer.Name,
						Addresses: append([]string(nil), targetPeer.Addresses...),
					}:
					case <-p.ctx.Done():
					}
					break
				} else {
					slog.Debug("Cluster: heartbeat returned non-200 status", "peer_id", targetPeer.ID, "target", target, "status", resp.StatusCode)
				}
			}
		}(peerNode)
	}
}

package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/chickenzord/dokidoki/internal/model"
)

// PEXProvider periodically sends heartbeats to active peers to exchange topology tables.
type PEXProvider struct {
	self         model.Node
	clusterToken string
	peersFunc    func() []model.Node
	interval     time.Duration
	client       *http.Client

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewPEXProvider creates a new PEX (Peer Exchange) provider.
func NewPEXProvider(
	self model.Node,
	clusterToken string,
	peersFunc func() []model.Node,
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
	}
}

// Name returns the provider identifier.
func (p *PEXProvider) Name() string {
	return "pex"
}

// Start initiates periodic heartbeat exchanges with known peers.
func (p *PEXProvider) Start(ctx context.Context, events chan<- PeerEvent) error {
	p.ctx, p.cancel = context.WithCancel(ctx)

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()

		ticker := time.NewTicker(p.interval)
		defer ticker.Stop()

		for {
			select {
			case <-p.ctx.Done():
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
	if p.cancel != nil {
		p.cancel()
	}
	p.wg.Wait()
	return nil
}

func (p *PEXProvider) sendHeartbeats(events chan<- PeerEvent) {
	if p.peersFunc == nil {
		return
	}

	knownPeers := p.peersFunc()
	msg := model.HeartbeatMessage{
		NodeID:       p.self.ID,
		Addresses:    p.self.Addresses,
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
		go func(targetPeer model.Node) {
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
					reqCancel()
					continue
				}
				resp.Body.Close()
				reqCancel()

				if resp.StatusCode == http.StatusOK {
					select {
					case events <- PeerEvent{
						Type:      EventUpdated,
						NodeID:    targetPeer.ID,
						Name:      targetPeer.Name,
						Addresses: targetPeer.Addresses,
					}:
					case <-p.ctx.Done():
					}
					break
				}
			}
		}(peerNode)
	}
}

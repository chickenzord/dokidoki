package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/chickenzord/dokidoki/internal/logger"
	"github.com/chickenzord/dokidoki/internal/model"
)

// StaticProvider discovers peers from configured bootstrap URLs.
type StaticProvider struct {
	bootstrapURLs []string
	self          model.Node
	clusterToken  string
	client        *http.Client
	retryInterval time.Duration

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewStaticProvider creates a new StaticProvider.
func NewStaticProvider(bootstrapURLs []string, self model.Node, clusterToken string, client *http.Client) *StaticProvider {
	if client == nil {
		client = &http.Client{
			Timeout: 5 * time.Second,
		}
	}

	cleanURLs := make([]string, 0, len(bootstrapURLs))
	for _, u := range bootstrapURLs {
		trimmed := strings.TrimSpace(u)
		if trimmed == "" {
			continue
		}
		if !strings.HasPrefix(trimmed, "http://") && !strings.HasPrefix(trimmed, "https://") {
			trimmed = "http://" + trimmed
		}
		trimmed = strings.TrimRight(trimmed, "/")
		cleanURLs = append(cleanURLs, trimmed)
	}

	return &StaticProvider{
		bootstrapURLs: cleanURLs,
		self:          self,
		clusterToken:  clusterToken,
		client:        client,
		retryInterval: 30 * time.Second,
	}
}

// Name returns the provider identifier.
func (p *StaticProvider) Name() string {
	return "static"
}

// Start begins discovery by performing an initial handshake with configured bootstrap peers.
func (p *StaticProvider) Start(ctx context.Context, events chan<- PeerEvent) error {
	p.ctx, p.cancel = context.WithCancel(ctx)

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()

		// Perform initial handshake immediately
		p.probeAll(events)

		ticker := time.NewTicker(p.retryInterval)
		defer ticker.Stop()

		for {
			select {
			case <-p.ctx.Done():
				return
			case <-ticker.C:
				p.probeAll(events)
			}
		}
	}()

	return nil
}

// Stop terminates background discovery activities.
func (p *StaticProvider) Stop() error {
	if p.cancel != nil {
		p.cancel()
	}
	p.wg.Wait()
	return nil
}

func (p *StaticProvider) probeAll(events chan<- PeerEvent) {
	for _, url := range p.bootstrapURLs {
		p.probe(url, events)
	}
}

func (p *StaticProvider) probe(baseURL string, events chan<- PeerEvent) {
	target := baseURL + "/api/v1/cluster/handshake"

	handshakeReq := model.HandshakeRequest{
		NodeID:       p.self.ID,
		Name:         p.self.Name,
		Addresses:    p.self.Addresses,
		Version:      p.self.Version,
		ClusterToken: p.clusterToken,
	}

	body, err := json.Marshal(handshakeReq)
	if err != nil {
		return
	}

	reqCtx, reqCancel := context.WithTimeout(p.ctx, 5*time.Second)
	defer reqCancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if p.clusterToken != "" {
		req.Header.Set("X-Cluster-Token", p.clusterToken)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return
	}

	var hsResp model.HandshakeResponse
	if err := json.NewDecoder(resp.Body).Decode(&hsResp); err != nil {
		return
	}

	if hsResp.NodeID != "" && hsResp.NodeID != p.self.ID {
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
	}

	// Also register any known peers returned in the handshake response
	for _, peer := range hsResp.Peers {
		if peer.ID == p.self.ID || peer.ID == hsResp.NodeID {
			continue
		}
		select {
		case events <- PeerEvent{
			Type:      EventDiscovered,
			NodeID:    peer.ID,
			Name:      peer.Name,
			Addresses: peer.Addresses,
		}:
		case <-p.ctx.Done():
			return
		}
	}

	logger.Debugf("Cluster: static handshake succeeded with %s (%s)", baseURL, hsResp.NodeID)
}

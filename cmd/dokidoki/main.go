package main

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/chickenzord/dokidoki/internal/api"
	"github.com/chickenzord/dokidoki/internal/cluster"
	"github.com/chickenzord/dokidoki/internal/config"
	"github.com/chickenzord/dokidoki/internal/docker"
	"github.com/chickenzord/dokidoki/internal/logger"
	"github.com/chickenzord/dokidoki/internal/stack"
)

var (
	version   = "0.1.0"
	commit    = "unknown"
	buildTime = "unknown"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		logger.Errorf("Failed to load configuration: %v", err)
		os.Exit(1)
	}

	// Resolve node ID and candidate addresses
	nodeID, err := cluster.ResolveNodeID(cfg.NodeID, cfg.StacksDir)
	if err != nil {
		logger.Errorf("Failed to resolve node ID: %v", err)
		os.Exit(1)
	}

	candidateAddrs := cluster.DetectCandidateAddresses(cfg.Bind, cfg.Port, cfg.AdvertiseAddrs)

	// Log startup and configuration
	logger.Infof("Starting Dokidoki v%s", version)

	dockerHostVal := cfg.DockerHost
	if dockerHostVal == "" {
		dockerHostVal = "default (local daemon / environment)"
	}
	var seedPeersVal any = "[none]"
	if len(cfg.Peers) > 0 {
		seedPeersVal = strings.Join(cfg.Peers, ", ")
	}
	mDNSVal := "enabled"
	if !cfg.EnableMDNS {
		mDNSVal = "disabled"
	}

	configItems := []logger.ConfigItem{
		{Key: "Node ID", Value: nodeID},
		{Key: "Node Name", Value: cfg.NodeName},
		{Key: "Bind", Value: cfg.Addr()},
		{Key: "Stacks Dir", Value: cfg.StacksDir},
		{Key: "Docker Host", Value: dockerHostVal},
		{Key: "Cluster Token", Value: cfg.ClusterToken, Sensitive: true},
		{Key: "mDNS Status", Value: mDNSVal},
		{Key: "Advertised Addrs", Value: strings.Join(candidateAddrs, ", ")},
		{Key: "Seed Peers", Value: seedPeersVal},
	}
	logger.PrintConfig("Configuration:", configItems)

	// Initialize Docker client
	dockerCli, err := docker.New(cfg.DockerHost)
	if err != nil {
		logger.Errorf("Failed to initialize Docker client: %v", err)
		os.Exit(1)
	}
	defer dockerCli.Close()

	// Identify container self ID if running containerized
	selfContainerID := docker.IdentifySelf(context.Background(), dockerCli)
	if selfContainerID != "" {
		shortSelfID := selfContainerID
		if len(shortSelfID) > 12 {
			shortSelfID = shortSelfID[:12]
		}
		logger.Infof("Running inside Docker container: %s", shortSelfID)
	} else {
		logger.Infof("Running on host (bare-metal, not containerized)")
	}

	// Initialize Cluster Manager
	self := cluster.Node{
		ID:        nodeID,
		Name:      cfg.NodeName,
		Addresses: candidateAddrs,
		Status:    cluster.StatusAlive,
		Version:   version,
		IsSelf:    true,
	}

	clusterManager := cluster.NewManager(self, cfg.ClusterToken, 30*time.Second, nil, cfg.StacksDir)

	// Register discovery providers
	if len(cfg.Peers) > 0 {
		clusterManager.RegisterProvider(cluster.NewStaticProvider(cfg.Peers, self, cfg.ClusterToken, nil))
	}
	clusterManager.RegisterProvider(cluster.NewPEXProvider(self, cfg.ClusterToken, clusterManager.ListNodes, 0, nil))
	if cfg.EnableMDNS {
		clusterManager.RegisterProvider(cluster.NewMDNSProvider(self, cfg.Port, cfg.ClusterToken, nil))
	}

	clusterCtx, clusterCancel := context.WithCancel(context.Background())
	defer clusterCancel()
	clusterManager.Start(clusterCtx)

	// Initialize API Router and HTTP Server
	scanner := stack.NewScanner(cfg.StacksDir)
	stackSvc := stack.NewService(scanner, dockerCli, selfContainerID)
	router := api.NewRouter(cfg, dockerCli, stackSvc, clusterManager, selfContainerID)

	server := &http.Server{
		Addr:         cfg.Addr(),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown handling
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	serverErr := make(chan error, 1)
	go func() {
		logger.Infof("Listening on http://%s", cfg.Addr())
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		logger.Errorf("Server failed to start or run: %v", err)
	case sig := <-quit:
		logger.Infof("Received signal %v, initiating graceful shutdown...", sig)
	}

	// Graceful HTTP server shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Errorf("HTTP server forced to shutdown: %v", err)
	} else {
		logger.Info("HTTP server shut down cleanly")
	}

	// Stop cluster manager (which sends leave messages to peers)
	logger.Info("Stopping cluster manager...")
	if err := clusterManager.Stop(shutdownCtx); err != nil {
		logger.Warnf("Cluster manager stop error: %v", err)
	}

	logger.Info("Dokidoki exited")
}

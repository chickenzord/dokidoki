package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/chickenzord/dokidoki/internal/api"
	"github.com/chickenzord/dokidoki/internal/config"
	"github.com/chickenzord/dokidoki/internal/docker"
	"github.com/chickenzord/dokidoki/internal/stacks"
)

const banner = `
==================================================
   ___         _     _      _          _     _ 
  / _ \  ___  | | __(_)  __| |  ___   | | __(_)
 / /_)/ / _ \ | |/ /| | / _` + "`" + ` | / _ \  | |/ /| |
/ ___/ | (_) ||   < | || (_| || (_) | |   < | |
\/      \___/ |_|\_\|_| \__,_| \___/  |_|\_\|_|
   Docker Stack Management & Monitoring
   Version: 0.1.0
==================================================`

func printBanner(cfg *config.Config) {
	fmt.Println(banner)
	fmt.Printf(" Listening on: http://%s\n", cfg.Addr())
	fmt.Printf(" Stacks Dir:   %s\n", cfg.StacksDir)
	if cfg.DockerHost != "" {
		fmt.Printf(" Docker Host:  %s\n", cfg.DockerHost)
	} else {
		fmt.Println(" Docker Host:  default (local daemon / environment)")
	}
	fmt.Println(" Endpoints:")
	fmt.Println("   GET /api/v1/stacks")
	fmt.Println("   GET /api/v1/stacks/{name}")
	fmt.Println("   GET /api/v1/stacks/{name}/compose")
	fmt.Println("   GET /api/v1/stacks/{name}/containers")
	fmt.Println("   GET /api/v1/containers")
	fmt.Println("   GET /api/v1/containers/{id}")
	fmt.Println("   GET /api/v1/host")
	fmt.Println("   GET /api/v1/host/ping")
	fmt.Println("==================================================")
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	dockerCli, err := docker.NewClient(cfg.DockerHost)
	if err != nil {
		log.Fatalf("Failed to initialize Docker client: %v", err)
	}
	defer dockerCli.Close()

	scanner := stacks.NewScanner(cfg.StacksDir)
	router := api.NewRouter(cfg, dockerCli, scanner)

	server := &http.Server{
		Addr:         cfg.Addr(),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	printBanner(cfg)

	// Graceful shutdown handling
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("Starting Dokidoki server on %s", cfg.Addr())
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		log.Fatalf("Server failed to start or run: %v", err)
	case sig := <-quit:
		log.Printf("Received signal %v, initiating graceful shutdown...", sig)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("HTTP server forced to shutdown: %v", err)
	} else {
		log.Println("HTTP server shut down cleanly")
	}

	log.Println("Dokidoki exited")
}

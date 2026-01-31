package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"stronghold/internal/proxy"
)

func main() {
	// Setup logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetPrefix("[stronghold-proxy] ")

	// Load configuration from environment or config file
	config, err := proxy.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create and start the proxy server
	server, err := proxy.NewServer(config)
	if err != nil {
		log.Fatalf("Failed to create proxy server: %v", err)
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start server in a goroutine
	errChan := make(chan error, 1)
	go func() {
		log.Printf("Starting Stronghold proxy on %s", config.GetProxyAddr())
		if err := server.Start(ctx); err != nil {
			errChan <- err
		}
	}()

	// Wait for shutdown signal or error
	select {
	case sig := <-sigChan:
		log.Printf("Received signal: %v", sig)
	case err := <-errChan:
		log.Fatalf("Server error: %v", err)
	}

	// Graceful shutdown
	log.Println("Shutting down proxy...")
	if err := server.Shutdown(context.Background()); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}

	log.Println("Proxy stopped")
}

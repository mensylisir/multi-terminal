package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mensylisir/multi-terminal/gateway/internal/config"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("failed to run server: %v", err)
	}
}

func run() error {
	// Load configuration
	if err := config.Load(); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	cfg := config.Get()

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create HTTP mux
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", handleWS)

	// Create HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.ServerConfig.Port),
		Handler:      mux,
		ReadTimeout:  time.Duration(cfg.ServerConfig.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.ServerConfig.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(cfg.ServerConfig.IdleTimeout) * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("server starting on port %d", cfg.ServerConfig.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	log.Println("shutdown signal received")

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 30*time.Second)
	defer shutdownCancel()

	// Shutdown server gracefully
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown error: %w", err)
	}

	log.Println("server stopped gracefully")
	return nil
}

func handleWS(w http.ResponseWriter, r *http.Request) {
	// TODO: implement WebSocket upgrade and session handling in Task 1.2
	w.Write([]byte("TODO: implement WebSocket handler"))
}

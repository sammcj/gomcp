// server/shutdown.go
package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// ShutdownManager handles graceful shutdown
type ShutdownManager struct {
	server     *http.Server
	bridge     Bridge
	detector   LeakDetector
	waitGroup  sync.WaitGroup
	shutdownCh chan struct{}
	logger     *log.Logger
}

// Bridge interface for bridge operations
type Bridge interface {
	Close() error
}

// LeakDetector interface for leak detection
type LeakDetector interface {
	Close() error
}

// NewShutdownManager creates a new shutdown manager
func NewShutdownManager(srv *http.Server, bridge Bridge, detector LeakDetector, logger *log.Logger) *ShutdownManager {
	return &ShutdownManager{
		server:     srv,
		bridge:     bridge,
		detector:   detector,
		shutdownCh: make(chan struct{}),
		logger:     logger,
	}
}

// HandleGracefulShutdown sets up signal handling and graceful shutdown
func (sm *ShutdownManager) HandleGracefulShutdown() error {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal
	sig := <-signals
	sm.logger.Printf("Received signal: %v", sig)

	// Notify about shutdown
	close(sm.shutdownCh)

	// Create context with timeout for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Track shutdown in wait group
	sm.waitGroup.Add(1)
	go func() {
		defer sm.waitGroup.Done()
		sm.performGracefulShutdown(ctx)
	}()

	// Wait for all shutdown tasks to complete
	shutdownComplete := make(chan struct{})
	go func() {
		sm.waitGroup.Wait()
		close(shutdownComplete)
	}()

	// Wait for shutdown or timeout
	select {
	case <-shutdownComplete:
		sm.logger.Println("Graceful shutdown completed")
		return nil
	case <-ctx.Done():
		return fmt.Errorf("shutdown timed out: %v", ctx.Err())
	}
}

// performGracefulShutdown handles the actual shutdown sequence
func (sm *ShutdownManager) performGracefulShutdown(ctx context.Context) {
	var shutdownErr error

	// Stop accepting new connections
	if err := sm.server.Shutdown(ctx); err != nil {
		shutdownErr = fmt.Errorf("server shutdown error: %v", err)
		sm.logger.Printf("Error during server shutdown: %v", err)
	}

	// Close bridge connection
	if err := sm.bridge.Close(); err != nil {
		if shutdownErr != nil {
			shutdownErr = fmt.Errorf("multiple shutdown errors: %v; bridge close error: %v", shutdownErr, err)
		} else {
			shutdownErr = fmt.Errorf("bridge close error: %v", err)
		}
		sm.logger.Printf("Error closing bridge: %v", err)
	}

	// Stop leak detector
	if err := sm.detector.Close(); err != nil {
		if shutdownErr != nil {
			shutdownErr = fmt.Errorf("multiple shutdown errors: %v; leak detector close error: %v", shutdownErr, err)
		} else {
			shutdownErr = fmt.Errorf("leak detector close error: %v", err)
		}
		sm.logger.Printf("Error closing leak detector: %v", err)
	}

	if shutdownErr != nil {
		sm.logger.Printf("Final shutdown error: %v", shutdownErr)
	}
}

// IsShuttingDown returns true if shutdown has been initiated
func (sm *ShutdownManager) IsShuttingDown() bool {
	select {
	case <-sm.shutdownCh:
		return true
	default:
		return false
	}
}

// WaitForShutdown blocks until shutdown is complete
func (sm *ShutdownManager) WaitForShutdown() {
	sm.waitGroup.Wait()
}

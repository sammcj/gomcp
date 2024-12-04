// cmd/server/server.go
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/sammcj/gomcp/bridge"
	"github.com/sammcj/gomcp/config"
)

// Server represents the HTTP server for the bridge
type Server struct {
	cfg    *config.Config
	bridge *bridge.Bridge
	srv    *http.Server
}

// MessageRequest represents an incoming message request
type MessageRequest struct {
	Message string `json:"message"`
}

// MessageResponse represents the response to a message
type MessageResponse struct {
	Response string `json:"response"`
	Error    string `json:"error,omitempty"`
}

// New creates a new server instance
func New(cfg *config.Config) *Server {
	return &Server{
		cfg: cfg,
	}
}

// Start initializes and starts the server
func (s *Server) Start() error {
	// Initialize bridge
	b, err := bridge.New(s.cfg, log.Default())
	if err != nil {
			return fmt.Errorf("failed to create bridge: %w", err)
	}
	s.bridge = b

	if err := s.bridge.Initialize(); err != nil {
			return fmt.Errorf("failed to initialize bridge: %w", err)
	}

	// Set up HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/api/chat", s.handleChat)
	mux.HandleFunc("/health", s.handleHealth)

	s.srv = &http.Server{
			Addr:    fmt.Sprintf("%s:%d", s.cfg.Server.Host, s.cfg.Server.Port),
			Handler: mux,
	}

	log.Printf("Starting server on %s", s.srv.Addr)
	if err := s.srv.ListenAndServe(); err != http.ErrServerClosed {
			return fmt.Errorf("server error: %w", err)
	}

	return nil
}

// Shutdown gracefully stops the server
func (s *Server) Shutdown() error {
	if s.srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := s.srv.Shutdown(ctx); err != nil {
			return fmt.Errorf("server shutdown error: %w", err)
		}
	}

	if s.bridge != nil {
		if err := s.bridge.Close(); err != nil {
			return fmt.Errorf("bridge shutdown error: %w", err)
		}
	}

	return nil
}

// handleChat processes chat messages
func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req MessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	response, err := s.bridge.ProcessMessage(req.Message)
	resp := MessageResponse{
		Response: response,
	}
	if err != nil {
		resp.Error = err.Error()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleHealth provides a health check endpoint
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}


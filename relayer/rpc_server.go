package relayer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/mux"

	"cosmossdk.io/log"
)

// RPCServer provides HTTP API for relayer status and monitoring
type RPCServer struct {
	relayer *RelayerService
	server  *http.Server
	router  *mux.Router
	logger  log.Logger
	mu      sync.RWMutex
	running bool
}

// RPCConfig defines the RPC server configuration
type RPCConfig struct {
	// Listen address (e.g., "0.0.0.0:8080")
	ListenAddr string `json:"listen_addr"`

	// Read timeout
	ReadTimeout time.Duration `json:"read_timeout"`

	// Write timeout
	WriteTimeout time.Duration `json:"write_timeout"`

	// Enable CORS
	EnableCORS bool `json:"enable_cors"`
}

// StatusResponse represents the relayer status response
type StatusResponse struct {
	Status    *RelayerStatus `json:"status"`
	Timestamp time.Time      `json:"timestamp"`
}

// FinalityResponse represents a finality info response
type FinalityResponse struct {
	FinalityInfo *FinalityInfo `json:"finality_info,omitempty"`
	Found        bool          `json:"found"`
	Error        string        `json:"error,omitempty"`
}

// CheckpointResponse represents the checkpoint state response
type CheckpointResponse struct {
	LastFinalityHeight  uint64                         `json:"last_finality_height"`
	PendingAttestations map[string]*PendingAttestation `json:"pending_attestations"`
	Timestamp           time.Time                      `json:"timestamp"`
}

// PendingAttestationsResponse represents pending attestations count
type PendingAttestationsResponse struct {
	Count     int       `json:"count"`
	Timestamp time.Time `json:"timestamp"`
}

// HealthResponse represents health check response
type HealthResponse struct {
	Healthy   bool      `json:"healthy"`
	Version   string    `json:"version,omitempty"`
	Uptime    string    `json:"uptime,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
}

// NewRPCServer creates a new RPC server
func NewRPCServer(relayer *RelayerService, config *RPCConfig, logger log.Logger) (*RPCServer, error) {
	if relayer == nil {
		return nil, fmt.Errorf("relayer service cannot be nil")
	}

	if config.ListenAddr == "" {
		config.ListenAddr = "0.0.0.0:8080"
	}

	if config.ReadTimeout == 0 {
		config.ReadTimeout = 15 * time.Second
	}

	if config.WriteTimeout == 0 {
		config.WriteTimeout = 15 * time.Second
	}

	rpc := &RPCServer{
		relayer: relayer,
		logger:  logger.With("component", "rpc_server"),
	}

	// Setup router
	rpc.router = mux.NewRouter()
	rpc.setupRoutes()

	// Create HTTP server
	rpc.server = &http.Server{
		Addr:         config.ListenAddr,
		Handler:      rpc.router,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
	}

	return rpc, nil
}

// setupRoutes configures all API routes
func (rpc *RPCServer) setupRoutes() {
	// Health check
	rpc.router.HandleFunc("/health", rpc.handleHealth).Methods("GET")

	// Status endpoints
	rpc.router.HandleFunc("/status", rpc.handleStatus).Methods("GET")

	// Finality endpoints
	rpc.router.HandleFunc("/finality/{chain_id}/{height}", rpc.handleGetFinality).Methods("GET")

	// Checkpoint endpoints
	rpc.router.HandleFunc("/checkpoint", rpc.handleGetCheckpoint).Methods("GET")

	// Pending attestations
	rpc.router.HandleFunc("/pending", rpc.handleGetPending).Methods("GET")

	// Add CORS middleware if needed
	rpc.router.Use(rpc.corsMiddleware)
	rpc.router.Use(rpc.loggingMiddleware)
}

// Start starts the RPC server
func (rpc *RPCServer) Start() error {
	rpc.mu.Lock()
	if rpc.running {
		rpc.mu.Unlock()
		return fmt.Errorf("RPC server already running")
	}
	rpc.running = true
	rpc.mu.Unlock()

	rpc.logger.Info("Starting RPC server", "addr", rpc.server.Addr)

	go func() {
		if err := rpc.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			rpc.logger.Error("RPC server error", "error", err)
		}
	}()

	return nil
}

// Stop stops the RPC server
func (rpc *RPCServer) Stop() error {
	rpc.mu.Lock()
	defer rpc.mu.Unlock()

	if !rpc.running {
		return nil
	}

	rpc.logger.Info("Stopping RPC server")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rpc.server.Shutdown(ctx); err != nil {
		rpc.logger.Error("Error shutting down RPC server", "error", err)
		return err
	}

	rpc.running = false
	rpc.logger.Info("RPC server stopped")
	return nil
}

// IsRunning returns whether the RPC server is running
func (rpc *RPCServer) IsRunning() bool {
	rpc.mu.RLock()
	defer rpc.mu.RUnlock()
	return rpc.running
}

// Handler implementations

// handleHealth handles health check requests
func (rpc *RPCServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	status := rpc.relayer.GetStatus()

	response := HealthResponse{
		Healthy:   status.Running,
		Version:   "1.0.0",
		Timestamp: time.Now(),
	}

	rpc.writeJSON(w, http.StatusOK, response)
}

// handleStatus handles status requests
func (rpc *RPCServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	status := rpc.relayer.GetStatus()

	response := StatusResponse{
		Status:    &status, // Convert value to pointer
		Timestamp: time.Now(),
	}

	rpc.writeJSON(w, http.StatusOK, response)
}

// handleGetFinality handles finality info requests
func (rpc *RPCServer) handleGetFinality(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	chainID := vars["chain_id"]
	heightStr := vars["height"]

	height, err := strconv.ParseUint(heightStr, 10, 64)
	if err != nil {
		rpc.writeError(w, http.StatusBadRequest, "Invalid height parameter", err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	finalityInfo, err := rpc.relayer.GetFinalityInfo(ctx, chainID, height)

	response := FinalityResponse{
		FinalityInfo: finalityInfo,
		Found:        finalityInfo != nil,
	}

	if err != nil {
		response.Error = err.Error()
		rpc.writeJSON(w, http.StatusOK, response)
		return
	}

	rpc.writeJSON(w, http.StatusOK, response)
}

// handleGetCheckpoint handles checkpoint state requests
func (rpc *RPCServer) handleGetCheckpoint(w http.ResponseWriter, r *http.Request) {
	lastHeight, pendingMap := rpc.relayer.GetCheckpointState()

	response := CheckpointResponse{
		LastFinalityHeight:  lastHeight,
		PendingAttestations: pendingMap,
		Timestamp:           time.Now(),
	}

	rpc.writeJSON(w, http.StatusOK, response)
}

// handleGetPending handles pending attestations count requests
func (rpc *RPCServer) handleGetPending(w http.ResponseWriter, r *http.Request) {
	count := rpc.relayer.GetPendingAttestationsCount()

	response := PendingAttestationsResponse{
		Count:     count,
		Timestamp: time.Now(),
	}

	rpc.writeJSON(w, http.StatusOK, response)
}

// Middleware

// corsMiddleware adds CORS headers
func (rpc *RPCServer) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// loggingMiddleware logs incoming requests
func (rpc *RPCServer) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rpc.logger.Debug("Incoming request",
			"method", r.Method,
			"path", r.URL.Path,
			"remote", r.RemoteAddr,
		)

		next.ServeHTTP(w, r)

		rpc.logger.Debug("Request completed",
			"method", r.Method,
			"path", r.URL.Path,
			"duration", time.Since(start),
		)
	})
}

// Helper methods

// writeJSON writes a JSON response
func (rpc *RPCServer) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		rpc.logger.Error("Failed to encode JSON response", "error", err)
	}
}

// writeError writes an error response
func (rpc *RPCServer) writeError(w http.ResponseWriter, status int, message, details string) {
	response := ErrorResponse{
		Error:   message,
		Code:    status,
		Message: details,
	}

	rpc.writeJSON(w, status, response)
}

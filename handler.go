package websocket

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/luraproject/lura/config"
	"github.com/luraproject/lura/logging"
	"github.com/luraproject/lura/proxy"
	router "github.com/luraproject/lura/router/gin"
	"nhooyr.io/websocket"
)

const ConfigNamespace = "websocket"

// Config holds the configuration for WebSocket endpoints
type Config struct {
	ReadBufferSize   int           `json:"read_buffer_size"`
	WriteBufferSize  int           `json:"write_buffer_size"`
	HandshakeTimeout time.Duration `json:"handshake_timeout"`
	Compression      bool          `json:"compression"`
	Subprotocols     []string      `json:"subprotocols"`
	BackendScheme    string        `json:"backend_scheme"` // "ws" or "wss" to override scheme detection
	MaxMessageSize   int64         `json:"max_message_size"` // Maximum message size in bytes (0 = no limit)
}

// BackendRegistry holds the mapping of backend names to WebSocket URLs
type BackendRegistry struct {
	Backends map[string]string `json:"backends"`
}

// Global backend registry - should be initialized from configuration
var globalBackendRegistry *BackendRegistry

// HandlerFactory creates handlers for WebSocket endpoints
type HandlerFactory struct {
	logger logging.Logger
}

// Define custom context key type for Gin compatibility
type contextKey string

const ginContextKey contextKey = "gin-context"

// NewHandlerFactory returns a new WebSocket HandlerFactory
func NewHandlerFactory(logger logging.Logger) *HandlerFactory {
	return &HandlerFactory{
		logger: logger,
	}
}

// InitializeBackendRegistry initializes the global backend registry from configuration
func InitializeBackendRegistry(serviceConfig config.ServiceConfig) {
	// Look for websocket_backends configuration in the service config
	if registryConfig, ok := serviceConfig.ExtraConfig["websocket_backends"]; ok {
		if registryMap, ok := registryConfig.(map[string]interface{}); ok {
			backends := make(map[string]string)
			if backendsInterface, ok := registryMap["backends"]; ok {
				if backendsMap, ok := backendsInterface.(map[string]interface{}); ok {
					for name, url := range backendsMap {
						if urlStr, ok := url.(string); ok {
							backends[name] = urlStr
						}
					}
				}
			}
			globalBackendRegistry = &BackendRegistry{Backends: backends}
		}
	}

	// Fallback to empty registry if no configuration found
	if globalBackendRegistry == nil {
		globalBackendRegistry = &BackendRegistry{Backends: make(map[string]string)}
	}
}

// HandlerWrapper wraps the standard handler factory to support WebSocket endpoints
func (w *HandlerFactory) HandlerWrapper(standardHandlerFactory router.HandlerFactory) router.HandlerFactory {
	return func(cfg *config.EndpointConfig, p proxy.Proxy) gin.HandlerFunc {
		w.logger.Debug(fmt.Sprintf("[ENDPOINT: %s] Building the WebSocket handler", cfg.Endpoint))

		// Check if this is a WebSocket endpoint
		wsConfig, hasWebSocketConfig := parseWebSocketConfig(cfg.ExtraConfig)
		if hasWebSocketConfig {
			w.logger.Debug(fmt.Sprintf("[ENDPOINT: %s] WebSocket configuration detected: %+v", cfg.Endpoint, wsConfig))
			// For WebSocket endpoints, we need to handle upgrade requests
			return func(c *gin.Context) {
				// Log all incoming headers for debugging
				w.logger.Debug(fmt.Sprintf("[ENDPOINT: %s] Request headers: %v", cfg.Endpoint, c.Request.Header))

				// Check if this is a WebSocket upgrade request
				if !isWebSocketUpgrade(c.Request) {
					w.logger.Debug(fmt.Sprintf("[ENDPOINT: %s] Not a WebSocket upgrade request, handling as HTTP", cfg.Endpoint))
					// Not a WebSocket upgrade, handle as regular HTTP request
					standardHandler := standardHandlerFactory(cfg, p)
					standardHandler(c)
					return
				}

				// WebSocket upgrades must be GET requests, but KrakenD config might specify POST
				// This is normal - the client sends GET for upgrade, backend receives WebSocket connection

				w.logger.Debug(fmt.Sprintf("[ENDPOINT: %s] WebSocket upgrade request detected", cfg.Endpoint))

				// For WebSocket upgrades, we need to capture auth headers without running
				// the full middleware chain that interferes with WebSocket upgrade

				// Capture any existing auth headers from the original request
				authHeaders := w.extractAuthHeaders(c.Request.Header)
				w.logger.Debug(fmt.Sprintf("[ENDPOINT: %s] Extracted auth headers for WebSocket: %v", cfg.Endpoint, authHeaders))

				// If no auth headers found in request, we could potentially run a lightweight
				// auth validation here, but for now we'll proceed with what we have
				if len(authHeaders) == 0 {
					w.logger.Debug(fmt.Sprintf("[ENDPOINT: %s] No auth headers found in WebSocket request", cfg.Endpoint))
				}

				// Now handle the WebSocket upgrade and connection with auth headers
				w.handleWebSocketConnection(c, cfg, p, wsConfig, authHeaders)
			}
		}

		// Return standard handler for non-WebSocket endpoints
		return standardHandlerFactory(cfg, p)
	}
}

// isWebSocketUpgrade checks if the HTTP request is a WebSocket upgrade request
func isWebSocketUpgrade(r *http.Request) bool {
	upgrade := strings.ToLower(r.Header.Get("Upgrade"))
	connection := strings.ToLower(r.Header.Get("Connection"))
	key := r.Header.Get("Sec-WebSocket-Key")

	return upgrade == "websocket" &&
		strings.Contains(connection, "upgrade") &&
		key != ""
}

// parseWebSocketConfig extracts WebSocket configuration from endpoint extra config
func parseWebSocketConfig(extraConfig config.ExtraConfig) (Config, bool) {
	wsConfigInterface, ok := extraConfig[ConfigNamespace]
	if !ok {
		return Config{}, false
	}

	wsConfigMap, ok := wsConfigInterface.(map[string]interface{})
	if !ok {
		return Config{}, false
	}

	cfg := Config{
		ReadBufferSize:   1024, // Default values
		WriteBufferSize:  1024,
		HandshakeTimeout: 10 * time.Second,
		Compression:      false,
		Subprotocols:     []string{},
		MaxMessageSize:   1 << 20, // Default 1MB limit
	}

	if readBufferSize, ok := wsConfigMap["read_buffer_size"].(float64); ok {
		cfg.ReadBufferSize = int(readBufferSize)
	}

	if writeBufferSize, ok := wsConfigMap["write_buffer_size"].(float64); ok {
		cfg.WriteBufferSize = int(writeBufferSize)
	}

	if handshakeTimeoutStr, ok := wsConfigMap["handshake_timeout"].(string); ok {
		if duration, err := time.ParseDuration(handshakeTimeoutStr); err == nil {
			cfg.HandshakeTimeout = duration
		}
	}

	if compression, ok := wsConfigMap["compression"].(bool); ok {
		cfg.Compression = compression
	}

	if subprotocols, ok := wsConfigMap["subprotocols"].([]interface{}); ok {
		for _, sp := range subprotocols {
			if spStr, ok := sp.(string); ok {
				cfg.Subprotocols = append(cfg.Subprotocols, spStr)
			}
		}
	}

	if backendScheme, ok := wsConfigMap["backend_scheme"].(string); ok {
		cfg.BackendScheme = backendScheme
	}

	if maxMessageSize, ok := wsConfigMap["max_message_size"].(float64); ok {
		cfg.MaxMessageSize = int64(maxMessageSize)
	}

	return cfg, true
}

// handleWebSocketConnection manages the WebSocket upgrade and connection lifecycle
func (w *HandlerFactory) handleWebSocketConnection(c *gin.Context, cfg *config.EndpointConfig, p proxy.Proxy, wsConfig Config, authHeaders map[string]string) {

	// Validate backend configuration
	if len(cfg.Backend) == 0 {
		w.logger.Error("No backend configured for WebSocket endpoint")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No backend configured"})
		return
	}

	// Accept the WebSocket connection
	acceptOpts := &websocket.AcceptOptions{
		Subprotocols:       wsConfig.Subprotocols,
		CompressionMode:    websocket.CompressionNoContextTakeover,
		InsecureSkipVerify: true, // Allow cross-origin connections for development
	}

	if wsConfig.Compression {
		acceptOpts.CompressionMode = websocket.CompressionContextTakeover
	}

	conn, err := websocket.Accept(c.Writer, c.Request, acceptOpts)
	if err != nil {
		w.logger.Error("WebSocket upgrade failed:", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "WebSocket upgrade failed"})
		return
	}
	defer conn.Close(websocket.StatusInternalError, "Internal error")

	// Set read limit for client connection
	if wsConfig.MaxMessageSize > 0 {
		conn.SetReadLimit(wsConfig.MaxMessageSize)
		w.logger.Debug(fmt.Sprintf("Set client read limit to %d bytes", wsConfig.MaxMessageSize))
	}

	w.logger.Debug("WebSocket connection established for:", cfg.Endpoint)

	// Handle the WebSocket connection lifecycle with auth headers
	w.handleConnectionLifecycle(c.Request.Context(), conn, cfg, p, wsConfig, authHeaders)
}

// handleConnectionLifecycle manages the WebSocket connection lifecycle and establishes backend proxy
func (w *HandlerFactory) handleConnectionLifecycle(ctx context.Context, clientConn *websocket.Conn, cfg *config.EndpointConfig, p proxy.Proxy, wsConfig Config, authHeaders map[string]string) {
	// Create a context for this connection
	connCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Establish WebSocket connection to backend
	backendConn, err := w.connectToBackend(connCtx, cfg, wsConfig, authHeaders)
	if err != nil {
		w.logger.Error("Failed to connect to backend WebSocket:", err)
		clientConn.Close(websocket.StatusInternalError, "Backend connection failed")
		return
	}
	defer backendConn.Close(websocket.StatusNormalClosure, "Connection closed")

	w.logger.Debug("Established proxy connection between client and backend")

	// Start bidirectional proxying
	errChan := make(chan error, 2)

	// Proxy: Client -> Backend
	go func() {
		errChan <- w.proxyMessages(connCtx, clientConn, backendConn, "client->backend")
	}()

	// Proxy: Backend -> Client
	go func() {
		errChan <- w.proxyMessages(connCtx, backendConn, clientConn, "backend->client")
	}()

	// Wait for either direction to fail or context to be cancelled
	select {
	case err := <-errChan:
		if err != nil {
			w.logger.Error("WebSocket proxy error:", err)
		}
	case <-connCtx.Done():
		w.logger.Debug("WebSocket proxy context cancelled")
	}
}

// connectToBackend establishes a WebSocket connection to the backend service
func (w *HandlerFactory) connectToBackend(ctx context.Context, cfg *config.EndpointConfig, wsConfig Config, authHeaders map[string]string) (*websocket.Conn, error) {
	// Support both old and new configuration formats
	var wsURL string
	var err error

	// Try new format first (backend/backend_path in extra_config)
	if backendName, ok := cfg.ExtraConfig["backend"].(string); ok {
		if backendPath, ok := cfg.ExtraConfig["backend_path"].(string); ok {
			wsURL, err = w.deriveWebSocketURL(backendName, backendPath, wsConfig.BackendScheme)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, fmt.Errorf("no backend_path configured in endpoint")
		}
	} else {
		// Fallback to old format (backend array)
		if len(cfg.Backend) == 0 {
			return nil, fmt.Errorf("no backend configured for WebSocket endpoint")
		}

		backend := cfg.Backend[0]
		if len(backend.Host) == 0 {
			return nil, fmt.Errorf("no host configured in backend")
		}

		// Convert HTTP backend to WebSocket URL
		httpHost := backend.Host[0]
		urlPattern := backend.URLPattern

		wsURL, err = w.convertHTTPToWebSocketURL(httpHost, urlPattern, wsConfig.BackendScheme)
		if err != nil {
			return nil, err
		}
	}

	// Override scheme if specified in config
	if wsConfig.BackendScheme != "" {
		parsedURL, err := url.Parse(wsURL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse WebSocket URL: %w", err)
		}
		parsedURL.Scheme = wsConfig.BackendScheme
		wsURL = parsedURL.String()
	}

	w.logger.Debug(fmt.Sprintf("Connecting to backend WebSocket: %s", wsURL))

	// Create request headers with auth headers
	headers := make(map[string][]string)
	for key, value := range authHeaders {
		headers[key] = []string{value}
		w.logger.Debug(fmt.Sprintf("Adding auth header to backend connection: %s = %s", key, value))
	}

	// Dial the backend WebSocket
	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: headers,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to backend WebSocket %s: %w", wsURL, err)
	}

	// Set read limit for backend connection
	if wsConfig.MaxMessageSize > 0 {
		conn.SetReadLimit(wsConfig.MaxMessageSize)
		w.logger.Debug(fmt.Sprintf("Set backend read limit to %d bytes", wsConfig.MaxMessageSize))
	}

	return conn, nil
}

// deriveWebSocketURL converts backend name and path to WebSocket URL
func (w *HandlerFactory) deriveWebSocketURL(backendName, backendPath, forceScheme string) (string, error) {
	// Try to get from registry first (if configured)
	if globalBackendRegistry != nil {
		if registryURL, exists := globalBackendRegistry.Backends[backendName]; exists {
			return registryURL + backendPath, nil
		}
	}

	// Fallback: Use the same backend resolution logic as HTTP endpoints
	// This makes WebSocket work exactly like HTTP endpoints

	// For now, use a simple mapping based on backend names
	// In a real implementation, this should use the same service discovery
	// mechanism as regular HTTP backends
	defaultMappings := map[string]string{
		"albus": "localhost:3000", // Your service default
	}

	host, exists := defaultMappings[backendName]
	if !exists {
		// Default: assume localhost with common WebSocket port
		host = "localhost:8080"
		w.logger.Debug(fmt.Sprintf("Using default host %s for unknown backend %s", host, backendName))
	}

	// Determine scheme
	scheme := "ws"
	if forceScheme != "" {
		scheme = forceScheme
	}

	wsURL := fmt.Sprintf("%s://%s%s", scheme, host, backendPath)
	w.logger.Debug(fmt.Sprintf("Derived WebSocket URL: %s", wsURL))

	return wsURL, nil
}

// convertHTTPToWebSocketURL converts HTTP backend configuration to WebSocket URL
func (w *HandlerFactory) convertHTTPToWebSocketURL(httpHost, urlPattern, forceScheme string) (string, error) {
	// Parse the HTTP host URL
	parsedURL, err := url.Parse(httpHost)
	if err != nil {
		return "", fmt.Errorf("failed to parse backend host %s: %w", httpHost, err)
	}

	// Convert HTTP scheme to WebSocket scheme
	scheme := "ws"
	if parsedURL.Scheme == "https" {
		scheme = "wss"
	}

	// Override scheme if specified in config
	if forceScheme != "" {
		scheme = forceScheme
	}

	// Construct WebSocket URL
	wsURL := fmt.Sprintf("%s://%s%s", scheme, parsedURL.Host, urlPattern)
	w.logger.Debug(fmt.Sprintf("Converted HTTP URL %s%s to WebSocket URL: %s", httpHost, urlPattern, wsURL))

	return wsURL, nil
}

// getAvailableBackends returns a list of available backend names for error messages
func (w *HandlerFactory) getAvailableBackends() []string {
	if globalBackendRegistry == nil {
		return []string{}
	}

	backends := make([]string, 0, len(globalBackendRegistry.Backends))
	for name := range globalBackendRegistry.Backends {
		backends = append(backends, name)
	}
	return backends
}

// proxyMessages forwards messages between two WebSocket connections
func (w *HandlerFactory) proxyMessages(ctx context.Context, src, dest *websocket.Conn, direction string) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			messageType, message, err := src.Read(ctx)
			if err != nil {
				w.logger.Debug(fmt.Sprintf("WebSocket read error (%s): %v", direction, err))
				return err
			}

			w.logger.Debug(fmt.Sprintf("Proxying message (%s): %d bytes", direction, len(message)))

			if err := dest.Write(ctx, messageType, message); err != nil {
				w.logger.Debug(fmt.Sprintf("WebSocket write error (%s): %v", direction, err))
				return err
			}
		}
	}
}

// extractAuthHeaders extracts auth headers from the incoming request
func (w *HandlerFactory) extractAuthHeaders(headers map[string][]string) map[string]string {
	authHeaders := make(map[string]string)

	// Common auth headers that might be present or injected by krakend-auth
	authHeaderNames := []string{
		"X-User-Id",
		"X-User-Uid",
		"X-User-Email",
		"X-User-Groups",
		"X-User-Type",
	}

	// Also check for any headers with common auth prefixes
	authHeaderPrefixes := []string{
		"X-User-",
		"X-Auth-",
		"X-Group-",
	}

	for key, values := range headers {
		// Check exact matches
		for _, authHeader := range authHeaderNames {
			if strings.EqualFold(key, authHeader) && len(values) > 0 {
				authHeaders[key] = values[0]
				w.logger.Debug(fmt.Sprintf("Found auth header %s: %s", key, values[0]))
			}
		}

		// Check prefix matches
		for _, prefix := range authHeaderPrefixes {
			if strings.HasPrefix(strings.ToUpper(key), strings.ToUpper(prefix)) && len(values) > 0 {
				authHeaders[key] = values[0]
				w.logger.Debug(fmt.Sprintf("Found prefixed auth header %s: %s", key, values[0]))
			}
		}
	}

	return authHeaders
}

// New creates a new WebSocket middleware that wraps the provided handler factory
func New(handlerFactory router.HandlerFactory, logger logging.Logger) router.HandlerFactory {
	wsFactory := NewHandlerFactory(logger)
	return wsFactory.HandlerWrapper(handlerFactory)
}

// NewWithConfig creates a new WebSocket middleware with backend registry configuration
func NewWithConfig(handlerFactory router.HandlerFactory, logger logging.Logger, serviceConfig config.ServiceConfig) router.HandlerFactory {
	// Initialize backend registry from service configuration
	InitializeBackendRegistry(serviceConfig)

	wsFactory := NewHandlerFactory(logger)
	return wsFactory.HandlerWrapper(handlerFactory)
}

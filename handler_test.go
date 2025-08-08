package websocket

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/luraproject/lura/config"
	"github.com/luraproject/lura/logging"
)

func TestParseWebSocketConfig(t *testing.T) {
	tests := []struct {
		name      string
		input     config.ExtraConfig
		expected  Config
		hasConfig bool
	}{
		{
			name: "valid websocket config",
			input: config.ExtraConfig{
				ConfigNamespace: map[string]interface{}{
					"read_buffer_size":  1024.0,
					"write_buffer_size": 2048.0,
					"handshake_timeout": "30s",
					"compression":       true,
					"subprotocols":      []interface{}{"chat", "v1"},
				},
			},
			expected: Config{
				ReadBufferSize:   1024,
				WriteBufferSize:  2048,
				HandshakeTimeout: 30 * time.Second,
				Compression:      true,
				Subprotocols:     []string{"chat", "v1"},
			},
			hasConfig: true,
		},
		{
			name:      "no websocket config",
			input:     config.ExtraConfig{},
			expected:  Config{},
			hasConfig: false,
		},
		{
			name: "partial websocket config with defaults",
			input: config.ExtraConfig{
				ConfigNamespace: map[string]interface{}{
					"compression": true,
				},
			},
			expected: Config{
				ReadBufferSize:   1024,                // default
				WriteBufferSize:  1024,                // default
				HandshakeTimeout: 10 * time.Second,    // default
				Compression:      true,
				Subprotocols:     []string{},
			},
			hasConfig: true,
		},
		{
			name: "invalid timeout format uses default",
			input: config.ExtraConfig{
				ConfigNamespace: map[string]interface{}{
					"handshake_timeout": "invalid",
					"compression":       false,
				},
			},
			expected: Config{
				ReadBufferSize:   1024,                // default
				WriteBufferSize:  1024,                // default
				HandshakeTimeout: 10 * time.Second,    // default due to parse error
				Compression:      false,
				Subprotocols:     []string{},
				BackendScheme:    "",                  // default
			},
			hasConfig: true,
		},
		{
			name: "backend scheme configuration",
			input: config.ExtraConfig{
				ConfigNamespace: map[string]interface{}{
					"backend_scheme": "wss",
					"compression":    true,
				},
			},
			expected: Config{
				ReadBufferSize:   1024,                // default
				WriteBufferSize:  1024,                // default
				HandshakeTimeout: 10 * time.Second,    // default
				Compression:      true,
				Subprotocols:     []string{},
				BackendScheme:    "wss",
			},
			hasConfig: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, hasConfig := parseWebSocketConfig(tt.input)

			if hasConfig != tt.hasConfig {
				t.Errorf("parseWebSocketConfig() hasConfig = %v, want %v", hasConfig, tt.hasConfig)
				return
			}

			if !hasConfig {
				return // No config to compare
			}

			if cfg.ReadBufferSize != tt.expected.ReadBufferSize {
				t.Errorf("ReadBufferSize = %v, want %v", cfg.ReadBufferSize, tt.expected.ReadBufferSize)
			}

			if cfg.WriteBufferSize != tt.expected.WriteBufferSize {
				t.Errorf("WriteBufferSize = %v, want %v", cfg.WriteBufferSize, tt.expected.WriteBufferSize)
			}

			if cfg.HandshakeTimeout != tt.expected.HandshakeTimeout {
				t.Errorf("HandshakeTimeout = %v, want %v", cfg.HandshakeTimeout, tt.expected.HandshakeTimeout)
			}

			if cfg.Compression != tt.expected.Compression {
				t.Errorf("Compression = %v, want %v", cfg.Compression, tt.expected.Compression)
			}

			if len(cfg.Subprotocols) != len(tt.expected.Subprotocols) {
				t.Errorf("Subprotocols length = %v, want %v", len(cfg.Subprotocols), len(tt.expected.Subprotocols))
				return
			}

			for i, sp := range cfg.Subprotocols {
				if sp != tt.expected.Subprotocols[i] {
					t.Errorf("Subprotocols[%d] = %v, want %v", i, sp, tt.expected.Subprotocols[i])
				}
			}

			if cfg.BackendScheme != tt.expected.BackendScheme {
				t.Errorf("BackendScheme = %v, want %v", cfg.BackendScheme, tt.expected.BackendScheme)
			}
		})
	}
}

func TestNewHandlerFactory(t *testing.T) {
	logger := logging.NoOp

	factory := NewHandlerFactory(logger)

	if factory == nil {
		t.Errorf("NewHandlerFactory() returned nil")
	}

	if factory.logger != logger {
		t.Errorf("logger not set correctly")
	}
}

func TestConnectToBackend(t *testing.T) {
	logger := logging.NoOp
	factory := NewHandlerFactory(logger)

	// Test with valid config
	endpointConfig := &config.EndpointConfig{
		ExtraConfig: config.ExtraConfig{
			"backend":      "albus",
			"backend_path": "/api/v1/test/",
		},
	}

	wsConfig := Config{
		BackendScheme: "ws",
	}

	authHeaders := map[string]string{
		"X-User-Id": "test-user",
	}

	// This test will fail because we can't actually connect to a backend
	// but we can test the URL construction logic
	_, err := factory.connectToBackend(context.Background(), endpointConfig, wsConfig, authHeaders)
	if err == nil {
		t.Errorf("connectToBackend() should fail when backend is not available")
	}

	// Check that error contains expected WebSocket URL
	expectedURL := "ws://localhost:8080/api/v1/test/"
	if !strings.Contains(err.Error(), expectedURL) {
		t.Errorf("connectToBackend() error should contain URL %s, got: %v", expectedURL, err)
	}
}

func TestConnectToBackendWithInvalidConfig(t *testing.T) {
	logger := logging.NoOp
	factory := NewHandlerFactory(logger)

	// Test with missing backend config
	endpointConfig := &config.EndpointConfig{
		ExtraConfig: config.ExtraConfig{},
	}

	wsConfig := Config{}
	authHeaders := map[string]string{}

	_, err := factory.connectToBackend(context.Background(), endpointConfig, wsConfig, authHeaders)
	if err == nil {
		t.Errorf("connectToBackend() expected error for missing backend config, got nil")
	}

	expectedError := "no backend name configured"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("connectToBackend() error should contain '%s', got: %v", expectedError, err)
	}
}

func TestIsWebSocketUpgrade(t *testing.T) {
	tests := []struct {
		name     string
		headers  map[string]string
		expected bool
	}{
		{
			name: "valid websocket upgrade",
			headers: map[string]string{
				"Upgrade":              "websocket",
				"Connection":           "Upgrade",
				"Sec-WebSocket-Key":    "dGhlIHNhbXBsZSBub25jZQ==",
			},
			expected: true,
		},
		{
			name: "missing upgrade header",
			headers: map[string]string{
				"Connection":        "Upgrade",
				"Sec-WebSocket-Key": "dGhlIHNhbXBsZSBub25jZQ==",
			},
			expected: false,
		},
		{
			name: "wrong upgrade header",
			headers: map[string]string{
				"Upgrade":           "http2",
				"Connection":        "Upgrade",
				"Sec-WebSocket-Key": "dGhlIHNhbXBsZSBub25jZQ==",
			},
			expected: false,
		},
		{
			name: "case insensitive headers",
			headers: map[string]string{
				"upgrade":           "WebSocket",
				"connection":        "upgrade",
				"sec-websocket-key": "dGhlIHNhbXBsZSBub25jZQ==",
			},
			expected: true,
		},
		{
			name: "connection header with multiple values",
			headers: map[string]string{
				"Upgrade":              "websocket",
				"Connection":           "keep-alive, Upgrade",
				"Sec-WebSocket-Key":    "dGhlIHNhbXBsZSBub25jZQ==",
			},
			expected: true,
		},
		{
			name: "missing connection header",
			headers: map[string]string{
				"Upgrade":           "websocket",
				"Sec-WebSocket-Key": "dGhlIHNhbXBsZSBub25jZQ==",
			},
			expected: false,
		},
		{
			name: "missing websocket key",
			headers: map[string]string{
				"Upgrade":    "websocket",
				"Connection": "Upgrade",
			},
			expected: false,
		},
		{
			name:     "no headers",
			headers:  map[string]string{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock HTTP request
			req := &http.Request{
				Header: make(map[string][]string),
			}

			// Set headers
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			result := isWebSocketUpgrade(req)
			if result != tt.expected {
				t.Errorf("isWebSocketUpgrade() = %v, want %v", result, tt.expected)
			}
		})
	}
}
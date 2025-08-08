# KrakenD WebSocket Middleware

A WebSocket middleware for KrakenD that enables WebSocket support while maintaining compatibility with KrakenD's existing middleware chain (authentication, rate limiting, logging, etc.).

This middleware acts as a **WebSocket proxy**, establishing WebSocket connections to your backend services while preserving KrakenD's authentication and middleware chain.

## Features

- **Seamless Integration**: Plugs into KrakenD's existing middleware chain
- **Selective WebSocket Support**: Only endpoints with WebSocket configuration become WebSocket-enabled
- **Authentication Header Forwarding**: Automatically extracts and forwards auth headers (X-User-Id, X-Auth-Token, etc.) from WebSocket upgrade requests
- **Dual Protocol Support**: Handles both regular HTTP requests and WebSocket upgrade requests on the same endpoint  
- **WebSocket Proxy**: Establishes direct WebSocket connections to backend services and proxies messages bidirectionally
- **Configurable**: Buffer sizes, timeouts, compression, and subprotocols
- **Production Ready**: Comprehensive error handling and logging with detailed debug information

## Installation

```bash
go get github.com/unacademy/krakend-websocket
```

## Usage

### 1. Integrate into KrakenD Handler Chain

```go
import (
    websocket "github.com/unacademy/krakend-websocket"
    "github.com/luraproject/lura/logging"
    router "github.com/luraproject/lura/router/gin"
)

// Wrap your existing handler factory with WebSocket support
func setupWebSocketMiddleware(existingHandlerFactory router.HandlerFactory, logger logging.Logger) router.HandlerFactory {
    // Add WebSocket middleware - it will only activate for endpoints with websocket config
    return websocket.New(existingHandlerFactory, logger)
}
```

### 2. Configure WebSocket Endpoints

Add WebSocket configuration to your KrakenD endpoint configuration:

```json
{
  "endpoints": [
    {
      "endpoint": "/api/v1/albus/chat/",
      "methods": ["GET"],
      "extra_config": {
        "websocket": {
          "read_buffer_size": 1024,
          "write_buffer_size": 1024,
          "handshake_timeout": "30s",
          "compression": true,
          "subprotocols": ["chat.v1", "chat"],
          "backend_scheme": "wss"
        }
      },
      "auth": true,
      "backend": "albus",
      "backend_path": "/api/v1/chat/",
      "auth_config": {
        "add_groups": true,
        "should_abort": true
      }
    }
  ]
}
```

## Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `read_buffer_size` | int | 1024 | Size of the read buffer in bytes |
| `write_buffer_size` | int | 1024 | Size of the write buffer in bytes |
| `handshake_timeout` | string | "10s" | WebSocket handshake timeout (Go duration format) |
| `compression` | bool | false | Enable WebSocket compression |
| `subprotocols` | []string | [] | Supported WebSocket subprotocols |
| `backend_scheme` | string | "" | Force WebSocket scheme ("ws" or "wss"). Auto-detected if not specified |

**Note**: You must also configure your backend names to WebSocket URLs in the middleware. Currently hardcoded but should be made configurable.

## How It Works

### 1. Request Flow
- Regular HTTP requests → Standard KrakenD processing
- WebSocket upgrade requests → WebSocket middleware processing

### 2. WebSocket Proxy Processing  
1. **Request Detection**: Distinguishes between regular HTTP and WebSocket upgrade requests
2. **Endpoint Detection**: Checks for `websocket` configuration in `extra_config`
3. **Auth Header Extraction**: Extracts authentication headers from the upgrade request
4. **Client WebSocket Upgrade**: Upgrades HTTP connection to WebSocket with compression support
5. **Backend WebSocket Connection**: Establishes WebSocket connection to backend service with auth headers
6. **Bidirectional Proxying**: Forwards all messages between client and backend WebSocket connections

### 3. WebSocket Proxy Flow
The middleware creates a transparent WebSocket tunnel between client and backend, preserving the original WebSocket protocol.

**Client ↔ KrakenD ↔ Backend Flow:**
```
Client WebSocket ↔ KrakenD (Auth + Proxy) ↔ Backend WebSocket
```

**Authentication Header Forwarding:**
During the initial WebSocket handshake, the middleware extracts and forwards authentication headers to the backend connection:
- `Authorization`
- `X-User-Id`, `X-User-Uid`, `X-User-Name`, `X-User-Email`  
- `X-Auth-Token`, `X-Auth-User`
- `X-Group-Id`, `X-Group-Name`
- Any headers with `X-User-`, `X-Auth-`, or `X-Group-` prefixes

**Message Proxying:**
All WebSocket messages (text, binary, ping, pong) are forwarded bidirectionally without modification.

## Example: Chat Application

### Backend WebSocket Server
```go
// Backend runs a native WebSocket server
func handleWebSocket(w http.ResponseWriter, r *http.Request) {
    // Auth headers are available in the upgrade request
    userID := r.Header.Get("X-User-Id")
    authToken := r.Header.Get("X-Auth-Token")
    
    // Upgrade to WebSocket
    upgrader := websocket.Upgrader{
        CheckOrigin: func(r *http.Request) bool { return true },
    }
    
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Printf("WebSocket upgrade failed: %v", err)
        return
    }
    defer conn.Close()
    
    log.Printf("WebSocket connection established for user: %s", userID)
    
    // Handle WebSocket messages directly
    for {
        messageType, message, err := conn.ReadMessage()
        if err != nil {
            log.Printf("Read error: %v", err)
            break
        }
        
        // Process message and send response
        response := fmt.Sprintf("Echo from %s: %s", userID, string(message))
        err = conn.WriteMessage(messageType, []byte(response))
        if err != nil {
            log.Printf("Write error: %v", err)
            break
        }
    }
}

// Register handler
http.HandleFunc("/api/v1/chat/", handleWebSocket)
```

### WebSocket Client
```javascript
// Connect with subprotocol and auth headers in the initial request
const ws = new WebSocket('ws://localhost:8080/api/v1/websocket/chat/', ['chat.v1']);

ws.onopen = () => {
    console.log('WebSocket connected');
    ws.send(JSON.stringify({
        type: 'chat',
        content: 'Hello WebSocket!',
        timestamp: Date.now()
    }));
};

ws.onmessage = (event) => {
    const response = JSON.parse(event.data);
    console.log('Received:', response);
};

ws.onerror = (error) => {
    console.error('WebSocket error:', error);
};
```

**Note**: Authentication headers must be included in the initial HTTP upgrade request. The KrakenD auth middleware will inject these headers which are then forwarded to your backend WebSocket service.

## Authentication & Authorization

The WebSocket middleware automatically extracts and forwards authentication headers from the upgrade request to backend services. This includes:

1. **Header Extraction**: Captures auth headers from the WebSocket upgrade request
2. **Header Forwarding**: Includes these headers in all backend proxy requests
3. **Seamless Integration**: Works with existing KrakenD auth middleware

```json
{
  "endpoint": "/api/v1/websocket/private/",
  "extra_config": {
    "websocket": {
      "compression": true,
      "subprotocols": ["auth.v1"]
    }
  }
}
```

**Important**: Authentication occurs during the initial WebSocket handshake. The auth headers are then forwarded to your backend WebSocket service, allowing it to authenticate the connection.

## Error Handling

The middleware provides comprehensive error handling:

- **Upgrade Failures**: Invalid WebSocket upgrade requests return HTTP error responses
- **Authentication Failures**: Unauthorized requests are rejected before WebSocket upgrade
- **Backend Errors**: Backend failures are sent as error messages over WebSocket
- **Connection Errors**: Connection issues are logged and connections are gracefully closed

## Development & Testing

### Running Tests

```bash
go test -v ./...
```

### Project Structure

```
krakend-websocket/
├── LICENSE               # MIT License
├── README.md            # This file
├── go.mod              # Go module with Unacademy KrakenD fork
├── handler.go          # Main WebSocket middleware implementation
└── handler_test.go     # Comprehensive test suite
```

## Compatibility

- **Go Version**: 1.17+
- **KrakenD Version**: Uses Unacademy's KrakenD fork (github.com/Unacademy/krakend v1.4.1)
- **WebSocket Library**: nhooyr.io/websocket v1.8.6
- **HTTP Framework**: Gin v1.7.7

## Dependencies

The middleware uses the following key dependencies:
- **Lura Framework**: Core KrakenD functionality via Unacademy fork
- **Gin**: HTTP router and middleware support  
- **nhooyr WebSocket**: Modern, fast WebSocket implementation
- **Standard Library**: Context, JSON, HTTP utilities

## License

MIT License - Copyright (c) 2025 Unacademy

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for your changes
4. Ensure all tests pass
5. Submit a pull request

## Support

For issues and questions:
1. Check existing GitHub issues
2. Create a new issue with detailed description
3. Include configuration examples and logs
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

Add WebSocket configuration to your KrakenD endpoint configuration. The configuration follows the standard KrakenD v2 format:

```json
{
  "version": 2,
  "name": "KrakenD Gateway with WebSocket Support",
  "port": 8080,
  "endpoints": [
    {
      "endpoint": "/audio/converse/",
      "method": "GET",
      "extra_config": {
        "websocket": {
          "read_buffer_size": 1024,
          "write_buffer_size": 1024,
          "handshake_timeout": "10s",
          "compression": false
        }
      },
      "backend": [
        {
          "url_pattern": "/audio/converse/",
          "method": "GET",
          "host": ["http://localhost:8000"]
        }
      ]
    },
    {
      "endpoint": "/api/v1/chat/",
      "method": "GET",
      "extra_config": {
        "websocket": {
          "read_buffer_size": 2048,
          "write_buffer_size": 2048,
          "handshake_timeout": "30s",
          "compression": true,
          "subprotocols": ["chat.v1", "chat"]
        }
      },
      "backend": [
        {
          "url_pattern": "/api/v1/chat/",
          "method": "GET",
          "host": ["http://localhost:3000"]
        }
      ]
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

**Important Notes**: 
- Use `method: "GET"` for WebSocket endpoints (required for WebSocket upgrade)
- Backend `host` should use HTTP scheme (`http://` or `https://`) - the middleware automatically converts to WebSocket URLs
- WebSocket endpoints work alongside regular HTTP endpoints in the same configuration

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
- `X-User-Id`, `X-User-Uid`, `X-User-Email`
- `X-User-Groups`, `X-User-Type`
- Any headers with `X-User-`, `X-Auth-`, or `X-Group-` prefixes

**Message Proxying:**
All WebSocket messages (text, binary, ping, pong) are forwarded bidirectionally without modification.

## Backend Integration

Your backend WebSocket server will receive the forwarded authentication headers from KrakenD during the WebSocket upgrade request. The headers (`X-User-Id`, `X-User-Uid`, `X-User-Email`, etc.) are available in the standard HTTP request headers and can be used for authentication and authorization in your WebSocket handlers.

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
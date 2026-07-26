# WebSocket API

Real-time log streaming via WebSocket connections for BareD jobs.

## Table of Contents

- [Overview](#overview)
- [Connection](#connection)
- [Authentication](#authentication)
- [Message Format](#message-format)
- [Usage Examples](#usage-examples)
  - [Command Line (websocat)](#command-line-websocat)
  - [JavaScript/TypeScript](#javascripttypescript)
  - [Python](#python)
  - [Go](#go)
- [Connection Lifecycle](#connection-lifecycle)
- [Error Handling](#error-handling)
- [Best Practices](#best-practices)

---

## Overview

BareD provides real-time job log streaming via **WebSocket protocol (RFC 6455)**. This allows clients to receive log messages as they're generated during backup and restore operations.

**Important Notes**:
- ✅ **WebSocket is the ONLY supported protocol** for real-time streaming
- ❌ **Server-Sent Events (SSE) is NOT supported**
- 🔒 Authentication required — session cookie (browsers) or HTTP Basic Auth (other clients)
- 🌐 The handshake `Origin` must be same-origin or allowlisted via `--http-allowed-origin`
- ⏱️ The stream closes when its session is logged out or expires
- 📡 Automatic connection upgrade from HTTP to WebSocket
- 🔄 Supports reconnection (implement client-side)

**Endpoint**: `ws://localhost:8080/api/jobs/{job-id}/logs/stream`

---

## Connection

### Endpoint Format

```
ws://[host]:[port]/api/jobs/{job-id}/logs/stream
```

or with TLS:

```
wss://[host]:[port]/api/jobs/{job-id}/logs/stream
```

### Path Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `job-id` | string | UUID of the job to stream logs from |

### Requirements

1. Valid job ID (job must exist)
2. Credentials — a session cookie (browsers) or HTTP Basic Auth (other clients)
3. WebSocket client supporting RFC 6455

---

## Authentication

The handshake is a normal HTTP request, and it is authenticated the same way
every other endpoint is.

### From a browser: the session cookie

Browsers **cannot set headers on a WebSocket handshake**, so the dashboard relies
on the session cookie from `POST /api/login`, which the browser attaches
automatically to a same-origin handshake. No client-side work is needed.

### From a CLI or backend client: Basic auth

```bash
# Encode credentials
echo -n "admin:password" | base64
# Output: YWRtaW46cGFzc3dvcmQ=

# Connect with authorization header
websocat -H "Authorization: Basic YWRtaW46cGFzc3dvcmQ=" \
  ws://localhost:8080/api/jobs/job-123/logs/stream
```

### Origin checking

If the handshake carries an `Origin` header, it must be same-origin or listed in
`--http-allowed-origin`; otherwise the upgrade is refused. Clients that send no
`Origin` (CLI tools) are unaffected. This blocks cross-site WebSocket hijacking,
where a hostile page opens a stream using the cookie the browser attaches for it.

### Session lifetime

A cookie-authenticated stream is closed by the server when its session is logged
out or reaches `--http-session-ttl`. Clients should treat an unexpected close
with code `1008` (policy violation) as "log in again", not as a transient error
to reconnect through.

### Failed Authentication

If authentication fails, the connection will be rejected with HTTP `401 Unauthorized` status before the WebSocket upgrade.

---

## Message Format

### Log Entry Message

Each log message is a JSON object:

```json
{
  "timestamp": "2025-12-15T02:00:01.123Z",
  "level": "info",
  "message": "Starting backup for target: mysql-prod",
  "stage": "VALIDATING"
}
```

### Fields

| Field | Type | Description |
|-------|------|-------------|
| `timestamp` | string | ISO 8601 timestamp with milliseconds |
| `level` | string | Log level: `debug`, `info`, `warn`, `error` |
| `message` | string | Log message text |
| `stage` | string | Current operation stage (optional) |

### Stage Values

Common stage values during backup operations:

- `VALIDATING` - Configuration and connectivity validation
- `DUMPING` - Database dump in progress
- `COMPRESSING` - Compression in progress
- `ENCRYPTING` - Encryption in progress (if enabled)
- `UPLOADING` - Upload to storage backend
- `CLEANUP` - Temporary file cleanup

### Connection Lifecycle Messages

The server may send special messages:

**Connection Accepted**:
```json
{
  "timestamp": "2025-12-15T02:00:00.000Z",
  "level": "info",
  "message": "Connected to log stream for job: job-123"
}
```

**Job Completed**:
```json
{
  "timestamp": "2025-12-15T02:02:30.000Z",
  "level": "info",
  "message": "Job completed successfully"
}
```

After job completion, the connection will be closed by the server.

---

## Usage Examples

### Command Line (websocat)

Install websocat: https://github.com/vi/websocat

```bash
# Basic usage
websocat -H "Authorization: Basic $(echo -n admin:password | base64)" \
  ws://localhost:8080/api/jobs/job-123/logs/stream

# With pretty-printed JSON (using jq)
websocat -H "Authorization: Basic $(echo -n admin:password | base64)" \
  ws://localhost:8080/api/jobs/job-123/logs/stream | jq -r '.message'

# Follow logs until job completes
websocat -H "Authorization: Basic $(echo -n admin:password | base64)" \
  ws://localhost:8080/api/jobs/job-123/logs/stream | \
  while read line; do
    echo "$line" | jq -r '"\(.timestamp) [\(.level)] \(.message)"'
  done
```

---

### JavaScript/TypeScript

#### Browser

```javascript
// Get job ID from somewhere
const jobId = 'job-123'

// No credentials here: the httpOnly session cookie set by POST /api/login rides
// along on the handshake automatically.

// Create WebSocket URL
const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
const host = window.location.host
const wsUrl = `${protocol}//${host}/api/jobs/${jobId}/logs/stream`

// Create WebSocket connection
const ws = new WebSocket(wsUrl)

// Handle connection open
ws.onopen = () => {
  console.log('Connected to log stream')
}

// Handle incoming messages
ws.onmessage = (event) => {
  try {
    const logEntry = JSON.parse(event.data)
    console.log(`[${logEntry.level}] ${logEntry.message}`)

    // Update UI with log entry
    displayLog(logEntry)
  } catch (err) {
    console.error('Failed to parse log message:', err)
  }
}

// Handle errors
ws.onerror = (error) => {
  console.error('WebSocket error:', error)
}

// Handle connection close
ws.onclose = (event) => {
  console.log('Connection closed:', event.code, event.reason)

  // Implement reconnection logic if needed
  if (event.code !== 1000) { // Not a normal closure
    setTimeout(() => reconnect(), 5000)
  }
}

// Close connection when done
// ws.close()
```

#### Node.js

```javascript
const WebSocket = require('ws')

const jobId = 'job-123'
const username = 'admin'
const password = 'password'

const auth = Buffer.from(`${username}:${password}`).toString('base64')
const wsUrl = `ws://localhost:8080/api/jobs/${jobId}/logs/stream`

const ws = new WebSocket(wsUrl, {
  headers: {
    'Authorization': `Basic ${auth}`
  }
})

ws.on('open', () => {
  console.log('Connected to log stream')
})

ws.on('message', (data) => {
  const logEntry = JSON.parse(data.toString())
  console.log(`[${logEntry.timestamp}] [${logEntry.level}] ${logEntry.message}`)
})

ws.on('error', (error) => {
  console.error('WebSocket error:', error)
})

ws.on('close', (code, reason) => {
  console.log(`Connection closed: ${code} - ${reason}`)
})
```

---

### Python

```python
import asyncio
import json
import base64
import websockets

async def stream_logs(job_id: str, username: str, password: str):
    # Create authorization header
    credentials = f"{username}:{password}"
    auth = base64.b64encode(credentials.encode()).decode()
    headers = {
        "Authorization": f"Basic {auth}"
    }

    # Connect to WebSocket
    uri = f"ws://localhost:8080/api/jobs/{job_id}/logs/stream"

    try:
        async with websockets.connect(uri, extra_headers=headers) as websocket:
            print(f"Connected to log stream for job: {job_id}")

            # Receive messages
            async for message in websocket:
                log_entry = json.loads(message)
                timestamp = log_entry['timestamp']
                level = log_entry['level']
                msg = log_entry['message']

                print(f"[{timestamp}] [{level.upper()}] {msg}")

    except websockets.exceptions.ConnectionClosed:
        print("Connection closed")
    except Exception as e:
        print(f"Error: {e}")

# Run the stream
if __name__ == "__main__":
    job_id = "job-123"
    username = "admin"
    password = "password"

    asyncio.run(stream_logs(job_id, username, password))
```

**Install dependencies**:

```bash
pip install websockets
```

---

### Go

```go
package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Message   string `json:"message"`
	Stage     string `json:"stage,omitempty"`
}

func streamLogs(jobID, username, password string) error {
	// Create WebSocket URL
	wsURL := fmt.Sprintf("ws://localhost:8080/api/jobs/%s/logs/stream", jobID)

	// Create authorization header
	auth := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	headers := http.Header{
		"Authorization": []string{"Basic " + auth},
	}

	// Connect to WebSocket
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, headers)
	if err != nil {
		if resp != nil {
			return fmt.Errorf("connection failed: %v (status: %d)", err, resp.StatusCode)
		}
		return fmt.Errorf("connection failed: %v", err)
	}
	defer conn.Close()

	log.Printf("Connected to log stream for job: %s", jobID)

	// Read messages
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				log.Println("Connection closed normally")
				return nil
			}
			return fmt.Errorf("read error: %v", err)
		}

		// Parse log entry
		var logEntry LogEntry
		if err := json.Unmarshal(message, &logEntry); err != nil {
			log.Printf("Failed to parse message: %v", err)
			continue
		}

		// Display log
		fmt.Printf("[%s] [%s] %s\n",
			logEntry.Timestamp,
			logEntry.Level,
			logEntry.Message,
		)
	}
}

func main() {
	jobID := "job-123"
	username := "admin"
	password := "password"

	if err := streamLogs(jobID, username, password); err != nil {
		log.Fatal(err)
	}
}
```

**Install dependencies**:

```bash
go get github.com/gorilla/websocket
```

---

## Connection Lifecycle

### 1. Connection Establishment

```
Client                          Server
  |                               |
  |--- HTTP GET (Upgrade) ------->|
  |    Authorization: Basic ...   |
  |                               |
  |<-- 101 Switching Protocols ---|
  |    Upgrade: websocket         |
  |    Connection: Upgrade        |
  |                               |
  |===== WebSocket Connected =====|
```

### 2. Message Streaming

```
Client                          Server
  |                               |
  |<----- Log Message 1 ----------|
  |<----- Log Message 2 ----------|
  |<----- Log Message 3 ----------|
  |         ...                   |
  |<----- Log Message N ----------|
  |                               |
```

### 3. Connection Termination

**Normal Completion** (job finished):

```
Client                          Server
  |                               |
  |<-- Close (1000) --------------|
  |    "Job completed"            |
  |                               |
  |--- Close ACK ---------------->|
  |                               |
```

**Client Disconnect**:

```
Client                          Server
  |                               |
  |--- Close (1000) ------------->|
  |                               |
  |<-- Close ACK -----------------|
  |                               |
```

---

## Error Handling

### Authentication Errors

**401 Unauthorized**: Invalid credentials

```
HTTP/1.1 401 Unauthorized
WWW-Authenticate: Basic realm="BareD API"
```

### Job Not Found

**404 Not Found**: Job ID doesn't exist

```json
{
  "error": "Job not found"
}
```

Connection is rejected before WebSocket upgrade.

### Connection Errors

**WebSocket Close Codes**:

| Code | Name | Description |
|------|------|-------------|
| 1000 | Normal Closure | Job completed or client disconnect |
| 1001 | Going Away | Server shutting down |
| 1006 | Abnormal Closure | Connection lost (network issue) |
| 1008 | Policy Violation | Authentication failed |

### Handling Disconnections

Implement exponential backoff for reconnection:

```javascript
let reconnectDelay = 1000 // Start with 1 second
const maxDelay = 30000 // Max 30 seconds

function reconnect() {
  setTimeout(() => {
    console.log(`Reconnecting in ${reconnectDelay}ms...`)
    connect()

    // Exponential backoff
    reconnectDelay = Math.min(reconnectDelay * 2, maxDelay)
  }, reconnectDelay)
}

ws.onclose = (event) => {
  if (event.code !== 1000) { // Not normal closure
    reconnect()
  }
}
```

---

## Best Practices

### 1. Authentication

- Store credentials securely (never in source code)
- Use environment variables or secure vaults
- Implement credential rotation

### 2. Connection Management

- Implement reconnection with exponential backoff
- Close connections when no longer needed
- Handle network failures gracefully
- Set reasonable timeouts

### 3. Message Processing

- Parse JSON safely (handle errors)
- Validate message structure before use
- Don't block on message processing
- Use buffering for high-volume logs

### 4. Error Handling

- Log connection errors
- Notify users of disconnections
- Implement retry logic
- Handle authentication failures

### 5. Performance

- Use a single connection per job
- Close connections for completed jobs
- Implement backpressure if needed
- Monitor connection count

### 6. Security

- Always use WSS (TLS) in production
- Validate SSL certificates
- Use strong authentication credentials
- Implement rate limiting (server-side)
- Restrict access by IP (firewall)

### 7. Browser Considerations

- Browser WebSocket API doesn't support custom headers during connection
- Consider using query parameters or first-message auth as alternative
- Handle page refresh/navigation to close connections
- Implement visibility API to pause/resume when tab inactive

---

## Comparison: WebSocket vs REST

### When to Use WebSocket (`/api/jobs/{id}/logs/stream`)

✅ Real-time log streaming for active jobs
✅ Live progress monitoring
✅ Interactive applications
✅ Low-latency requirements

### When to Use REST (`/api/jobs/{id}/logs`)

✅ Historical log retrieval
✅ Completed job logs
✅ Paginated log access
✅ Simple integrations
✅ Caching requirements

---

## Troubleshooting

### Connection Refused

**Problem**: Cannot connect to WebSocket endpoint

**Solutions**:
1. Verify HTTP server is running: `curl http://localhost:8080/api/health`
2. Check firewall rules
3. Verify correct host and port
4. Check server logs for errors

### Authentication Failures

**Problem**: 401 Unauthorized or connection rejected

**Solutions**:
1. Verify credentials are correct
2. Check Authorization header format
3. Test credentials with REST API first: `curl -u user:pass http://localhost:8080/api/jobs`
4. Check for special characters in password (URL encode if needed)

### No Messages Received

**Problem**: Connected but no log messages

**Solutions**:
1. Verify job is running: `GET /api/jobs/{id}`
2. Check if job has generated logs
3. Verify job ID is correct
4. Check for JSON parsing errors in client

### Connection Drops

**Problem**: Frequent disconnections

**Solutions**:
1. Check network stability
2. Implement reconnection logic
3. Check server logs for errors
4. Verify reverse proxy configuration (if used)
5. Increase WebSocket timeout in reverse proxy

### Reverse Proxy Issues

If using nginx, ensure WebSocket support:

```nginx
location /api/jobs/ {
    proxy_pass http://localhost:8080;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
}
```

---

## Further Reading

- **[REST API Endpoints](endpoints.md)** - HTTP API reference
- **[API Overview](README.md)** - Quick start guide
- **[RFC 6455](https://tools.ietf.org/html/rfc6455)** - WebSocket Protocol Specification

---

[← Back to API Documentation](README.md)

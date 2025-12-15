# API Reference

Documentation for BareD's HTTP REST API and WebSocket interfaces.

## Contents

### [REST API Endpoints](endpoints.md)

Complete reference for all HTTP API endpoints: health checks, backup triggering, job management, and dashboard queries.

### [WebSocket API](websocket.md)

Real-time log streaming via WebSocket connections, authentication, and message formats.

## Quick Start

### Enable HTTP API

Add to your `bared.yml`:

```yaml
http:
  enabled: true
  address: ":8080"
  auth:
    username: "admin"
    password: "your-secure-password"
```

Or use CLI flags:

```bash
brd daemon --config bared.yml \
  --http :8080 \
  --http-user admin \
  --http-pass secure-password
```

### Basic Authentication

All API endpoints (except `/api/health`) require HTTP Basic Authentication:

```bash
curl -u admin:password http://localhost:8080/api/dashboard
```

### Common Endpoints

**Health Check** (no auth required):

```bash
curl http://localhost:8080/api/health
```

**Dashboard Stats**:

```bash
curl -u admin:password http://localhost:8080/api/dashboard
```

**List Jobs**:

```bash
curl -u admin:password http://localhost:8080/api/jobs
```

**Trigger Backup**:

```bash
curl -u admin:password -X POST \
  http://localhost:8080/api/jobs/backup \
  -H "Content-Type: application/json" \
  -d '{"target": "mydb"}'
```

**Stream Logs** (Real-time via WebSocket):

```bash
# Using websocat (WebSocket client)
websocat -H "Authorization: Basic $(echo -n admin:password | base64)" \
  ws://localhost:8080/api/jobs/{job-id}/logs/stream

# Or using any WebSocket client library
# Protocol: WebSocket (RFC 6455)
# Auth: HTTP Basic Authentication
# Format: JSON messages streamed in real-time
```

## API Overview

### REST Endpoints

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/api/health` | GET | Health check (no auth) |
| `/api/dashboard` | GET | Dashboard statistics |
| `/api/jobs` | GET | List all jobs |
| `/api/jobs/{id}` | GET | Get job details |
| `/api/jobs/{id}/logs` | GET | Get job logs |
| `/api/jobs/backup` | POST | Trigger manual backup |
| `/api/jobs/{id}` | DELETE | Cancel running job |

**See**: [REST API Endpoints](endpoints.md) for complete documentation

### WebSocket Endpoints

| Endpoint | Purpose | Protocol |
|----------|---------|----------|
| `/api/jobs/{id}/logs/stream` | Real-time log streaming | WebSocket (not SSE) |

**Note**: Log streaming uses WebSocket protocol (RFC 6455), not Server-Sent Events (SSE).

**See**: [WebSocket API](websocket.md) for complete documentation

## Use Cases

### Monitoring Integration

Integrate BareD with monitoring systems:

```bash
# Check health
status=$(curl -s http://localhost:8080/api/health | jq -r .status)

# Get dashboard stats
stats=$(curl -s -u admin:password http://localhost:8080/api/dashboard)

# Check for failed jobs
failed=$(echo $stats | jq '.jobs[] | select(.status=="failed")')
```

### Custom Dashboards

Build custom dashboards using the API:

```javascript
// Fetch dashboard data
const response = await fetch('http://localhost:8080/api/dashboard', {
  headers: {
    'Authorization': 'Basic ' + btoa('admin:password')
  }
});
const dashboard = await response.json();

// Display stats
console.log(`Targets: ${dashboard.targets_count}`);
console.log(`Active Jobs: ${dashboard.active_jobs}`);
console.log(`Total Jobs: ${dashboard.total_jobs}`);
```

### Automated Backup Triggers

Trigger backups from external systems:

```python
import requests

# Trigger backup
response = requests.post(
    'http://localhost:8080/api/jobs/backup',
    json={'target': 'production-db'},
    auth=('admin', 'password')
)

job = response.json()
print(f"Backup started: {job['id']}")
```

### Log Aggregation

Collect logs for external analysis:

```bash
# Get logs for all failed jobs
curl -u admin:password http://localhost:8080/api/jobs?status=failed | \
  jq -r '.[] | .id' | \
  while read job_id; do
    curl -u admin:password \
      http://localhost:8080/api/jobs/$job_id/logs >> failed_logs.txt
  done
```

## Security

### Authentication

- All endpoints (except `/api/health`) require Basic Auth
- Credentials are set via config or CLI flags
- Use HTTPS in production
- Use strong passwords (32+ characters)

### Best Practices

- ✅ Always use HTTPS in production
- ✅ Change default passwords
- ✅ Restrict API access by IP (firewall)
- ✅ Use reverse proxy (nginx, Caddy)
- ✅ Monitor API access logs
- ✅ Rotate credentials periodically
- ✅ Use environment variables for passwords

### Reverse Proxy Example (nginx)

```nginx
server {
    listen 443 ssl http2;
    server_name backups.example.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;

        # WebSocket support
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

## Rate Limiting

The API does not implement rate limiting by default. If needed, implement at reverse proxy level or application level.

## Error Handling

All API errors return JSON:

```json
{
  "error": "target not found",
  "details": "no target named 'nonexistent' in configuration"
}
```

HTTP status codes:

- `200` - Success
- `400` - Bad request
- `401` - Unauthorized
- `404` - Not found
- `500` - Internal server error

## Client Libraries

Currently, no official client libraries exist. The API is simple REST+JSON and works with standard HTTP clients in any language.

**Examples**:

- **curl**: See examples above
- **JavaScript/TypeScript**: Use `fetch` or `axios`
- **Python**: Use `requests`
- **Go**: Use standard `net/http`

## API Versioning

The API is currently unversioned. Future versions may introduce `/api/v2/` endpoints while maintaining backwards compatibility.

## Further Reading

- **[REST API Endpoints](endpoints.md)** - Complete endpoint reference
- **[WebSocket API](websocket.md)** - Real-time streaming guide
- **[Web Interface Guide](../user-guide/web-interface.md)** - Using the web UI

---

[← Back to Documentation](../README.md)

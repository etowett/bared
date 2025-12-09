# BareD Web UI

Modern React-based web interface for BareD backup management.

## Technology Stack

- **React 18** - UI framework
- **TypeScript** - Type safety
- **TanStack Query** - Server state management
- **Zustand** - Client state management
- **Vite** - Build tool and dev server

## Development Setup

### Prerequisites

- Node.js 18+ and npm
- Go 1.25+ (for backend)

### Install Dependencies

```bash
npm install
```

### Development Mode

Start the Vite dev server (runs on <http://localhost:5173>):

```bash
npm run dev
```

The dev server will proxy API requests to the Go backend at `http://localhost:8080`.

**Important**: Start the Go backend first:

```bash
# From project root
go run cmd/brd/main.go daemon --config examples/config.example.yml --http :8080 --http-user admin --http-pass changeme
```

### Production Build

Build the optimized production bundle:

```bash
npm run build
```

Output will be in the `dist/` directory, which gets embedded into the Go binary.

### Preview Production Build

Test the production build locally:

```bash
npm run preview
```

## Project Structure

```tree
web/
├── src/
│   ├── api/
│   │   └── client.ts          # API client with auth
│   ├── components/
│   │   ├── Dashboard.tsx      # Main dashboard layout
│   │   ├── TargetList.tsx     # List of backup targets
│   │   ├── JobList.tsx        # Job history table
│   │   ├── JobDetail.tsx      # Job details modal
│   │   ├── JobProgress.tsx    # Progress bar component
│   │   └── Login.tsx          # Login form
│   ├── hooks/
│   │   ├── useJobs.ts         # Jobs API hooks
│   │   ├── useTargets.ts      # Targets API hooks
│   │   ├── useDashboard.ts    # Dashboard API hook
│   │   └── useWebSocket.ts    # WebSocket connection
│   ├── styles/
│   │   └── index.css          # Global styles
│   ├── types/
│   │   └── index.ts           # TypeScript interfaces
│   ├── App.tsx                # Root component
│   └── main.tsx               # Entry point
├── index.html                  # HTML template
├── vite.config.ts             # Vite configuration
├── tsconfig.json              # TypeScript config
└── package.json               # Dependencies
```

## Features

### Authentication

- HTTP Basic Auth stored in sessionStorage
- Automatic redirect to login on 401
- Logout clears credentials

### Dashboard

- Real-time stats (targets, active jobs, storage)
- Auto-refresh every 5 seconds
- Target cards with backup status
- Manual backup triggers

### Job Management

- Real-time job list with status filters
- Progress tracking with percentage and ETA
- WebSocket log streaming
- Job cancellation
- Detailed job view with full logs

### Progress Tracking

- Stage-based progress (dumping, compressing, uploading)
- ETA calculation
- Bytes processed/total
- Real-time updates

### Log Streaming

- WebSocket connection with auto-reconnect
- Color-coded log levels (error, warn, info, debug)
- Auto-scroll to new logs
- Connection status indicator

## API Integration

The frontend communicates with the Go backend via:

1. **REST API** - Standard CRUD operations
   - `GET /api/dashboard` - Dashboard summary
   - `GET /api/targets` - List targets
   - `GET /api/jobs` - List jobs
   - `POST /api/jobs/backup` - Trigger backup
   - `DELETE /api/jobs/:id` - Cancel job

2. **WebSocket** - Real-time log streaming
   - `WS /api/jobs/:id/logs/stream` - Stream logs for a job

## Configuration

### API Proxy (Development)

Vite dev server proxies `/api` requests to the backend:

```ts
// vite.config.ts
export default defineConfig({
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
})
```

### Auto-Refresh Intervals

- Dashboard: 5 seconds
- Job list: 3 seconds
- Individual job: 2 seconds
- Job logs: 5 seconds

Adjust in `src/hooks/*.ts` files.

## Building for Production

The frontend build is embedded into the Go binary:

1. **Build frontend**: `npm run build`
2. **Build Go binary**: `go build -o brd cmd/brd/main.go`
3. **Deploy single binary**: Contains both backend and frontend

The Go binary serves the React SPA for all non-API routes via `embed.FS`.

## Docker Build

The Dockerfile uses multi-stage builds:

1. **Stage 1**: Build React frontend with Node
2. **Stage 2**: Build Go binary with embedded frontend
3. **Stage 3**: Runtime image with minimal dependencies

```dockerfile
FROM node:20-alpine AS frontend-builder
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.25-alpine AS backend-builder
WORKDIR /app
COPY --from=frontend-builder /app/web/dist ./internal/web/dist
RUN go build -o brd ./cmd/brd

FROM alpine:latest
COPY --from=backend-builder /app/brd /usr/local/bin/brd
```

## API Authentication

Default credentials in docker-compose:

- Username: `admin`
- Password: `changeme`

**IMPORTANT**: Change these in production!

```bash
docker-compose up -d
# Access web UI at http://localhost:8080
```

## Troubleshooting

### CORS Issues

If you see CORS errors in development, ensure:

1. Go backend is running with `--http` flag
2. Vite proxy is configured correctly
3. Using `http://localhost:5173` (Vite dev server)

### WebSocket Connection Fails

- Check backend is reachable
- Verify authentication is valid
- Look for error messages in browser console

### Build Fails

- Clear node_modules: `rm -rf node_modules && npm install`
- Clear Vite cache: `rm -rf node_modules/.vite`
- Check Node version: `node -v` (should be 18+)

## License

Same as parent project.

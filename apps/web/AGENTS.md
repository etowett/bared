# Web Frontend — Agent Guide

> Scope: the React 19 + TypeScript + Vite dashboard in `apps/web/` (TanStack Router/Query, Zustand, Tailwind CSS v4 + Radix UI, Vitest). Part of the BareD AGENTS.md tree — see the root [`AGENTS.md`](../../AGENTS.md) for project-wide workflow and conventions. **The innermost guide wins** when instructions conflict.

> **The package manager is [Bun](https://bun.sh), not npm.** The pinned version lives in
> `apps/web/.bun-version` (currently `1.3.14`) and is mirrored by `"packageManager"` in `package.json`.
> The lockfile is the text-format `bun.lock` and it is committed — there is no `package-lock.json`
> and no `.nvmrc`. Install with `bun install` (CI and Docker use `bun install --frozen-lockfile`)
> and run scripts with `bun run <script>`.

Full-stack recipes — adding a config field end-to-end, or adding an API endpoint end-to-end — live in the backend guide [`../internal/AGENTS.md`](../api/internal/AGENTS.md). This guide covers the frontend in isolation.

## Architecture (event-driven)

```
┌─────────────┐
│   App.tsx   │ ← Root: Theme + Auth + QueryClient
└──────┬──────┘
       │
       ▼
┌──────────────┐
│ Dashboard    │ ← Overview, stats
└──────────────┘
       │
       ├─▶ JobList ────────┐
       │                   │
       ├─▶ RestoreForm     ├─▶ useJobs (React Query)
       │                   │
       └─▶ JobDetail ──────┴─▶ useWebSocket (real-time)
                           │
                           └─▶ API Client (axios)
```

**State Management**:

- **Server State**: React Query (`@tanstack/react-query`)
- **Global UI State**: Zustand (`useAuthStore`, `useJobStore`)
- **Theme State**: React Context (`ThemeContext`)
- **Real-time Updates**: WebSocket custom hook

## Project Structure

```
apps/web/
├── src/
│   ├── api/                  # API client and types
│   │   └── client.ts         # Axios instance, auth, endpoints
│   ├── components/           # React components
│   │   ├── Dashboard.tsx     # Main dashboard view
│   │   ├── JobList.tsx       # Job listing with filters
│   │   ├── JobDetail.tsx     # Job details with real-time logs
│   │   ├── RestoreForm.tsx   # Backup restore interface
│   │   ├── TargetList.tsx    # Target configuration display
│   │   ├── Login.tsx         # Authentication form
│   │   └── ui/               # Reusable UI components (Radix)
│   ├── hooks/                # Custom React hooks
│   │   ├── useJobs.ts        # Job fetching with React Query
│   │   ├── useWebSocket.ts   # WebSocket connection
│   │   ├── useAuth.ts        # Authentication state
│   │   └── useDashboard.ts   # Dashboard data
│   ├── contexts/             # React contexts
│   │   └── ThemeContext.tsx  # Dark mode management
│   ├── types/                # TypeScript type definitions
│   │   └── index.ts          # Shared types (Job, Target, etc.)
│   ├── lib/                  # Utility libraries
│   │   └── utils.ts          # Helper functions
│   ├── styles/               # Global CSS
│   │   └── globals.css       # Tailwind v4 CSS-first config + theme vars
│   ├── App.tsx               # Root component
│   ├── main.tsx              # Entry point
│   └── vite-env.d.ts         # Vite type declarations
├── public/                   # Static assets
├── dist/                     # Build output (copied to apps/api/internal/web/dist/)
├── .bun-version              # Pinned Bun version (1.3.14)
├── bun.lock                  # Committed Bun lockfile (text format)
├── package.json              # Dependencies + "packageManager": "bun@1.3.14"
├── tsconfig.json             # TypeScript configuration
├── vite.config.ts            # Vite configuration
└── postcss.config.js         # PostCSS configuration (@tailwindcss/postcss)
```

There is no `tailwind.config.js` — Tailwind v4 is configured in CSS. See [Styling](#styling).

The live tree also contains `src/routes/` (TanStack Router route modules), `src/test/` (test setup/helpers), and `src/utils/` alongside the directories above.

## State Management

**Three-tier state architecture**:

1. **Server State** (React Query):
   - Job lists, dashboard data, targets
   - Automatic caching, refetching, invalidation
   - Loading and error states handled automatically

2. **Real-time State** (WebSocket):
   - Job logs, progress updates
   - Direct WebSocket connection, no caching
   - Falls back to polling if WebSocket fails

3. **Client State** (Zustand):
   - Authentication status (`src/stores/auth.ts`) — status + username only.
     **Never a credential:** the session lives in an `httpOnly` cookie the
     server sets, which JavaScript cannot read. The store caches the answer to
     `GET /api/me` so the route guard doesn't round-trip per navigation.
   - UI preferences (theme, filters)
   - Global app state

**Example - Using React Query**:

```typescript
// hooks/useJobs.ts
import { useQuery } from '@tanstack/react-query';
import { apiClient } from '../api/client';
import type { Job } from '../types';

export function useJobs(filters?: { status?: string; target?: string }) {
  return useQuery({
    queryKey: ['jobs', filters],
    queryFn: async () => {
      const { data } = await apiClient.get<Job[]>('/api/jobs', {
        params: filters,
      });
      return data;
    },
    refetchInterval: 5000, // Poll every 5 seconds
    staleTime: 3000,
  });
}
```

**Example - Using Zustand** (the real auth store, `src/stores/auth.ts`):

```typescript
import { create } from 'zustand';
import { fetchCurrentUser, login, logout } from '@/api/client';

interface AuthState {
  status: 'unknown' | 'authenticated' | 'anonymous';
  username: string | null;
  check: () => Promise<boolean>;
  signIn: (username: string, password: string) => Promise<void>;
  signOut: () => Promise<void>;
}
```

**Do not add `persist` to this store, and do not put a token in it.** The
session is an `httpOnly`, `SameSite=Strict` cookie issued by `POST /api/login`;
persisting anything credential-shaped to `localStorage`/`sessionStorage` would
reintroduce the XSS-exfiltration hole this design removed. Whether a session is
still live is a *server* question — `check()` asks `GET /api/me`.

### Auth rules

- `fetch` calls go through `apiFetch` in `src/api/client.ts`, which sets
  `credentials: 'same-origin'`. Never add an `Authorization` header.
- A 401 throws `AuthError` and fires the handler registered with
  `onAuthFailure` (wired to the router in `App.tsx`). Never navigate with
  `window.location` — it reloads the whole bundle.
- The WebSocket handshake needs no auth code: the cookie rides along.

## WebSocket Integration

**Pattern**: Custom hook manages WebSocket connection lifecycle and message subscriptions.

```typescript
// hooks/useWebSocket.ts
import { useEffect, useState, useCallback, useRef } from 'react';

interface LogMessage {
  job_id: string;
  timestamp: string;
  level: string;
  message: string;
}

interface UseWebSocketOptions {
  jobId?: string;
  onMessage?: (message: LogMessage) => void;
  enabled?: boolean;
}

export function useWebSocket({ jobId, onMessage, enabled = true }: UseWebSocketOptions) {
  const [connected, setConnected] = useState(false);
  const [logs, setLogs] = useState<LogMessage[]>([]);
  const wsRef = useRef<WebSocket | null>(null);

  const connect = useCallback(() => {
    if (!enabled) return;

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/api/ws`;

    const ws = new WebSocket(wsUrl);

    ws.onopen = () => {
      console.log('WebSocket connected');
      setConnected(true);
    };

    ws.onmessage = (event) => {
      const message = JSON.parse(event.data) as { type: string; payload: LogMessage };

      if (message.type === 'log') {
        const log = message.payload;

        // Filter by job ID if specified
        if (!jobId || log.job_id === jobId) {
          setLogs((prev) => [...prev, log]);
          onMessage?.(log);
        }
      }
    };

    ws.onerror = (error) => {
      console.error('WebSocket error:', error);
    };

    ws.onclose = () => {
      console.log('WebSocket disconnected');
      setConnected(false);

      // Reconnect after 3 seconds
      setTimeout(() => {
        connect();
      }, 3000);
    };

    wsRef.current = ws;
  }, [jobId, onMessage, enabled]);

  useEffect(() => {
    connect();

    return () => {
      wsRef.current?.close();
    };
  }, [connect]);

  const clearLogs = useCallback(() => {
    setLogs([]);
  }, []);

  return { connected, logs, clearLogs };
}
```

**Usage in component**:

```typescript
function JobDetail({ jobId }: { jobId: string }) {
  const { logs, connected } = useWebSocket({ jobId });

  return (
    <div>
      <div>Status: {connected ? 'Connected' : 'Disconnected'}</div>
      <div className="logs">
        {logs.map((log, i) => (
          <div key={i} className={`log-${log.level}`}>
            {log.timestamp} [{log.level}] {log.message}
          </div>
        ))}
      </div>
    </div>
  );
}
```

## API Client Pattern

**Centralized axios instance** with authentication interceptor:

```typescript
// api/client.ts
import axios from 'axios';
import { useAuthStore } from '../stores/authStore';

export const apiClient = axios.create({
  baseURL: import.meta.env.VITE_API_URL || '',
  headers: {
    'Content-Type': 'application/json',
  },
});

// Add auth token to requests
apiClient.interceptors.request.use((config) => {
  const token = useAuthStore.getState().token;
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

// Handle auth errors
apiClient.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      useAuthStore.getState().clearAuth();
      window.location.href = '/login';
    }
    return Promise.reject(error);
  }
);

// Type-safe API methods
export const api = {
  dashboard: {
    get: () => apiClient.get('/api/dashboard'),
  },
  jobs: {
    list: (params?: { status?: string; target?: string }) =>
      apiClient.get('/api/jobs', { params }),
    get: (id: string) => apiClient.get(`/api/jobs/${id}`),
    triggerBackup: (data: { target: string }) =>
      apiClient.post('/api/jobs/backup', data),
    triggerRestore: (data: { target: string; backup: string }) =>
      apiClient.post('/api/jobs/restore', data),
    cancel: (id: string) => apiClient.delete(`/api/jobs/${id}`),
    logs: (id: string) => apiClient.get(`/api/jobs/${id}/logs`),
  },
  targets: {
    list: () => apiClient.get('/api/targets'),
  },
  restoreTargets: {
    list: () => apiClient.get('/api/restore-targets'),
  },
};
```

## Component Patterns

**Pattern 1: Data-fetching component with React Query**:

```typescript
// components/JobList.tsx
import { useJobs } from '../hooks/useJobs';
import { Card } from './ui/card';
import { Loader } from './ui/loader';

export function JobList() {
  const { data: jobs, isLoading, error, refetch } = useJobs();

  if (isLoading) return <Loader />;
  if (error) return <div>Error: {error.message}</div>;

  return (
    <div className="space-y-4">
      <div className="flex justify-between items-center">
        <h2 className="text-2xl font-bold">Jobs</h2>
        <button onClick={() => refetch()}>Refresh</button>
      </div>

      {jobs?.map((job) => (
        <Card key={job.id}>
          <div className="flex justify-between">
            <div>
              <h3>{job.target}</h3>
              <p className="text-sm text-gray-500">{job.type}</p>
            </div>
            <div>
              <span className={`badge badge-${job.status}`}>
                {job.status}
              </span>
            </div>
          </div>
        </Card>
      ))}
    </div>
  );
}
```

**Pattern 2: Form component with mutations**:

```typescript
// components/RestoreForm.tsx
import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '../api/client';
import { useToast } from '../hooks/useToast';

export function RestoreForm({ target }: { target: string }) {
  const [backup, setBackup] = useState('latest');
  const { toast } = useToast();
  const queryClient = useQueryClient();

  const restoreMutation = useMutation({
    mutationFn: (data: { target: string; backup: string }) =>
      api.jobs.triggerRestore(data),
    onSuccess: () => {
      toast({ title: 'Restore job started', variant: 'success' });
      queryClient.invalidateQueries({ queryKey: ['jobs'] });
    },
    onError: (error) => {
      toast({ title: 'Failed to start restore', description: error.message, variant: 'error' });
    },
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    restoreMutation.mutate({ target, backup });
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <div>
        <label>Backup</label>
        <input
          type="text"
          value={backup}
          onChange={(e) => setBackup(e.target.value)}
          placeholder="latest or path/to/backup.tar.gz"
        />
      </div>

      <button
        type="submit"
        disabled={restoreMutation.isPending}
      >
        {restoreMutation.isPending ? 'Starting...' : 'Start Restore'}
      </button>
    </form>
  );
}
```

## Styling

**Tailwind CSS v4 is configured CSS-first.** There is no `tailwind.config.js`; everything lives in
[`src/styles/globals.css`](./src/styles/globals.css):

- `@import 'tailwindcss'` replaces the old `@tailwind base/components/utilities` triple.
- `@plugin 'tailwindcss-animate'` loads the animation utilities (`animate-in`, `fade-in-0`,
  `zoom-in-95`, `slide-in-from-*`) that Radix components rely on.
- `@custom-variant dark (&:is(.dark *))` implements the class-based dark mode that `darkMode: ['class']`
  used to provide.
- `@theme { --color-*, --radius-*, --font-* }` defines the design tokens. The shadcn colours still
  indirect through the `:root` / `.dark` HSL custom properties (e.g. `--color-border: hsl(var(--border))`),
  so adding a colour means adding both the raw HSL var and the `--color-*` token.
- Project-specific one-off classes use `@utility` (not `@layer utilities`).
- `components.json` has `"tailwind": { "config": "" }` because there is no JS config for shadcn to find.

Utility names follow v4: `outline-hidden` (not `outline-none`), `shrink-0` (not `flex-shrink-0`),
`shadow-xs` (v3's `shadow-sm`), `wrap-break-word` (not `break-words`), `data-placeholder:`/`data-disabled:`
(not `data-[placeholder]:`), and `h-(--var)` for CSS-variable values.

**Styling with Tailwind + Radix UI** — use Radix UI primitives for accessible components, styled with Tailwind:

```typescript
// components/ui/button.tsx
import * as React from 'react';
import { Slot } from '@radix-ui/react-slot';
import { cva, type VariantProps } from 'class-variance-authority';
import { cn } from '../../lib/utils';

const buttonVariants = cva(
  'inline-flex items-center justify-center rounded-md text-sm font-medium transition-colors focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50',
  {
    variants: {
      variant: {
        default: 'bg-primary text-primary-foreground hover:bg-primary/90',
        destructive: 'bg-destructive text-destructive-foreground hover:bg-destructive/90',
        outline: 'border border-input bg-background hover:bg-accent hover:text-accent-foreground',
        ghost: 'hover:bg-accent hover:text-accent-foreground',
        link: 'text-primary underline-offset-4 hover:underline',
      },
      size: {
        default: 'h-10 px-4 py-2',
        sm: 'h-9 rounded-md px-3',
        lg: 'h-11 rounded-md px-8',
        icon: 'h-10 w-10',
      },
    },
    defaultVariants: {
      variant: 'default',
      size: 'default',
    },
  }
);

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {
  asChild?: boolean;
}

const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, asChild = false, ...props }, ref) => {
    const Comp = asChild ? Slot : 'button';
    return (
      <Comp
        className={cn(buttonVariants({ variant, size, className }))}
        ref={ref}
        {...props}
      />
    );
  }
);
Button.displayName = 'Button';

export { Button, buttonVariants };
```

## Build & Dev Commands

Run scripts with `bun run <script>` from inside `apps/web/`. The `scripts` block in `package.json` exposes:

| Script | Command | Purpose |
| --- | --- | --- |
| `dev` | `vite` | Start dev server on http://localhost:5173 |
| `build` | `tsc && vite build` | Type-check then production build into `apps/web/dist/` |
| `preview` | `vite preview` | Preview the production build locally |
| `lint` | `eslint . --ext ts,tsx --report-unused-disable-directives --max-warnings 0` | Lint (zero warnings allowed) |
| `lint:fix` | `eslint . --ext ts,tsx --fix` | Lint and auto-fix |
| `type-check` | `tsc --noEmit` | TypeScript type-check only |
| `format` | `prettier --write "src/**/*.{ts,tsx,css}"` | Format sources |
| `format:check` | `prettier --check "src/**/*.{ts,tsx,css}"` | Verify formatting |
| `validate` | `bun run type-check && bun run lint && bun run format:check && bun run test:run` | Full gate: type-check + lint + format check + tests |
| `test` | `vitest` | Run Vitest in watch mode |
| `test:ui` | `vitest --ui` | Vitest with the UI runner |
| `test:run` | `vitest run` | Run the test suite once |
| `test:coverage` | `vitest run --coverage` | Run tests with coverage |

Vitest runs on Node under Bun's script runner — `bun run test:run` invokes `vitest`, it does not use
`bun test`. Don't swap it for `bun test`; the suite depends on Vitest's jsdom environment and mocking.

**Makefile wrappers** (run from the repo root) cover the common flows:

- `make web-install` — install frontend dependencies
- `make web-dev` — start the dev server
- `make web-build` — production build
- `make web-lint` — lint
- `make web-validate` — full validate gate (type-check + lint + format check + tests)
- `make web-format` — format sources

**Development**:

```bash
cd apps/web
bun install
bun run dev          # Starts dev server on http://localhost:5173
```

**Vite proxy configuration** (in `vite.config.ts`):

```typescript
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/api/ws': {
        target: 'ws://localhost:8080',
        ws: true,
      },
    },
  },
});
```

**Production build**:

```bash
bun run build        # Outputs to apps/web/dist/
```

**Integration with Go**:

```bash
make build-with-web  # Builds frontend, then embeds in Go binary
```

The frontend is embedded in the Go binary via `go:embed` directive in `apps/api/internal/web/handler.go`:

```go
//go:embed dist/*
var distFS embed.FS

func Handler() http.Handler {
    sub, _ := fs.Sub(distFS, "dist")
    return http.FileServer(http.FS(sub))
}
```

## Testing

**Component test with React Testing Library**:

```typescript
// components/JobList.test.tsx
import { render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { JobList } from './JobList';
import { api } from '../api/client';
import { vi } from 'vitest';

vi.mock('../api/client');

describe('JobList', () => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });

  const wrapper = ({ children }) => (
    <QueryClientProvider client={queryClient}>
      {children}
    </QueryClientProvider>
  );

  it('renders loading state', () => {
    render(<JobList />, { wrapper });
    expect(screen.getByText(/loading/i)).toBeInTheDocument();
  });

  it('renders jobs list', async () => {
    const mockJobs = [
      { id: '1', target: 'mysql_prod', status: 'completed', type: 'backup' },
      { id: '2', target: 'postgres_dev', status: 'running', type: 'backup' },
    ];

    vi.mocked(api.jobs.list).mockResolvedValue({ data: mockJobs });

    render(<JobList />, { wrapper });

    await waitFor(() => {
      expect(screen.getByText('mysql_prod')).toBeInTheDocument();
      expect(screen.getByText('postgres_dev')).toBeInTheDocument();
    });
  });

  it('handles error state', async () => {
    vi.mocked(api.jobs.list).mockRejectedValue(new Error('API error'));

    render(<JobList />, { wrapper });

    await waitFor(() => {
      expect(screen.getByText(/error/i)).toBeInTheDocument();
    });
  });
});
```

**Hook testing**:

```typescript
// hooks/useWebSocket.test.ts
import { renderHook, waitFor } from '@testing-library/react';
import { useWebSocket } from './useWebSocket';
import WS from 'vitest-websocket-mock';

describe('useWebSocket', () => {
  let server: WS;

  beforeEach(() => {
    server = new WS('ws://localhost/api/ws');
  });

  afterEach(() => {
    WS.clean();
  });

  it('connects to WebSocket', async () => {
    const { result } = renderHook(() => useWebSocket({ enabled: true }));

    await waitFor(() => {
      expect(result.current.connected).toBe(true);
    });
  });

  it('receives log messages', async () => {
    const { result } = renderHook(() =>
      useWebSocket({ jobId: 'job-1', enabled: true })
    );

    await server.connected;

    server.send(JSON.stringify({
      type: 'log',
      payload: {
        job_id: 'job-1',
        timestamp: '2024-01-01T00:00:00Z',
        level: 'info',
        message: 'Test log',
      },
    }));

    await waitFor(() => {
      expect(result.current.logs).toHaveLength(1);
      expect(result.current.logs[0].message).toBe('Test log');
    });
  });
});
```

See [`TESTING.md`](./TESTING.md) for the full frontend testing reference.

## Conventions

**Component naming**:

```
- PascalCase for components: `JobList.tsx`, `RestoreForm.tsx`
- camelCase for hooks: `useJobs.ts`, `useWebSocket.ts`
- kebab-case for CSS files: `job-list.css`
```

**Route modules** (`src/routes/**`) must export **both** the `Route` and the page component:

```typescript
export const Route = createFileRoute('/jobs')({ component: JobsPage })

export function JobsPage() { … }   // ← must be exported
```

`eslint-plugin-react-refresh` allows `Route` via `allowExportNames: ['Route']` (TanStack Router owns
its HMR), but it flags a route module whose page component is only local — fast refresh cannot swap a
component that the module does not export. Helper components in the same file may stay local.

**Routes are code-split by default.** Only `index.tsx` (dashboard), `login.tsx` and `__root.tsx` are
eager — they are the entry points every session hits. Every other route lives in a `*.lazy.tsx`
module using `createLazyFileRoute`, so its chunk is fetched on first navigation instead of shipping
in the initial bundle:

```typescript
// src/routes/config/storages.lazy.tsx
export const Route = createLazyFileRoute('/config/storages')({ component: StoragesPage })

export function StoragesPage() { … }   // ← still must be exported
```

**Add a new route as `*.lazy.tsx`** unless it is a new entry point. The route tree is regenerated by
the Vite plugin — never hand-edit `routeTree.gen.ts`.

**A flat `foo.lazy.tsx` next to a `foo/` directory becomes a layout parent, not a page.** File-based
routing nests `routes/foo/bar.lazy.tsx` *under* `routes/foo.lazy.tsx`, so `/foo/bar` renders the
parent and the child only appears where the parent puts an `<Outlet />`. A parent that renders page
content and no outlet silently swallows every child — the child matches, its chunk loads, and
nothing of it is displayed, with no error and no console warning (this was #103). So: put the page at
`routes/foo/index.lazy.tsx` (path string `'/foo/'`) and its siblings alongside it, which is what
`config/` and `jobs/` do. Only keep a flat `foo.lazy.tsx` when you genuinely want shared chrome, and
then it must render `<Outlet />`. `src/routes/routes.test.tsx` resolves every route through the real
router and asserts each renders its own `<h2>` — extend it when you add a route.

`LazyRouteOptions` only accepts the render-time options (`component`, `errorComponent`,
`pendingComponent`, `notFoundComponent`). Anything the router needs *before* the component resolves —
`loader`, `beforeLoad`, `validateSearch` — must stay in an eager sibling `*.tsx` that declares the
route with `createFileRoute`. `src/routes/jobs/index.tsx` + `index.lazy.tsx` is the worked example:
the eager file owns `validateSearch` and exports the `JobsSearch` type, the lazy file owns the page.

The router in `App.tsx` sets `defaultPendingComponent` so a slow chunk fetch shows a fallback rather
than a blank outlet.

**Type definitions**:

```typescript
// Use interfaces for object shapes
interface Job {
  id: string;
  target: string;
  type: 'backup' | 'restore';
  status: 'pending' | 'running' | 'completed' | 'failed';
  created_at: string;
  updated_at: string;
}

// Use type aliases for unions and utilities
type JobStatus = 'pending' | 'running' | 'completed' | 'failed';
type JobType = 'backup' | 'restore';

// Partial utility for optional updates
type JobUpdate = Partial<Job>;
```

**Async/await patterns**:

```typescript
// Always handle errors
async function triggerBackup(target: string) {
  try {
    const response = await api.jobs.triggerBackup({ target });
    toast.success('Backup started');
    return response.data;
  } catch (error) {
    if (axios.isAxiosError(error)) {
      toast.error(error.response?.data?.error || 'Failed to start backup');
    }
    throw error;
  }
}

// Use React Query for automatic error handling
const mutation = useMutation({
  mutationFn: api.jobs.triggerBackup,
  onSuccess: () => {
    toast.success('Backup started');
  },
  onError: (error) => {
    toast.error(error.message);
  },
});
```

**Component organization**:

```typescript
// 1. Imports
import React from 'react';
import { useJobs } from '../hooks/useJobs';

// 2. Types
interface Props {
  target: string;
}

// 3. Component
export function JobList({ target }: Props) {
  // 4. Hooks
  const { data, isLoading } = useJobs({ target });
  const [filter, setFilter] = React.useState('all');

  // 5. Derived state
  const filteredJobs = React.useMemo(
    () => data?.filter(job => filter === 'all' || job.status === filter),
    [data, filter]
  );

  // 6. Event handlers
  const handleFilterChange = (value: string) => {
    setFilter(value);
  };

  // 7. Render
  return (
    <div>
      {/* JSX */}
    </div>
  );
}
```

## See also

- [`../AGENTS.md`](../../AGENTS.md) — root agent guide (project-wide workflow and conventions)
- [`../internal/AGENTS.md`](../api/internal/AGENTS.md) — backend (Go) guide, including full-stack recipes for adding a config field or API endpoint end-to-end
- [`./README.md`](./README.md) — frontend README
- [`./TESTING.md`](./TESTING.md) — frontend testing guide

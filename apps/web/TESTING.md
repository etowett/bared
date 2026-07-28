# Web Frontend Testing & Validation Guide

## Overview

The BareD web frontend includes comprehensive testing, linting, and validation tools to ensure code quality.

The toolchain is **Bun** (version pinned in [`.bun-version`](./.bun-version)) plus **Vitest 4**. The
lockfile is `bun.lock`; there is no `package-lock.json` and no `.nvmrc`. `bun run test:run` invokes
Vitest — do not substitute `bun test`, which is a different runner and does not use this project's
jsdom setup.

## Quick Start

```bash
# Install dependencies
bun install

# Build for production
bun run build

# Run development server
bun run dev

# Validate everything
bun run validate
```

## Available Commands

### Build Commands

```bash
# Development server with hot reload
bun run dev

# Production build (TypeScript check + Vite build)
bun run build

# Preview production build locally
bun run preview
```

### Linting & Formatting

```bash
# Run ESLint
bun run lint

# Run ESLint with auto-fix
bun run lint:fix

# Format code with Prettier
bun run format

# Check formatting without modifying files
bun run format:check
```

### Type Checking

```bash
# Run TypeScript type checker (no emit)
bun run type-check
```

### Complete Validation

```bash
# Run all checks: type-check + lint + format:check + tests
bun run validate
```

## Makefile Integration

The web commands are integrated into the main project Makefile:

```bash
# From project root

# Install web dependencies
make web-install

# Build web frontend
make web-build

# Start development server
make web-dev

# Lint web code
make web-lint

# Format web code
make web-format

# Validate web frontend
make web-validate

# Clean web artifacts
make web-clean

# Build Go binary with embedded web UI
make build-with-web

# Validate both Go and Web
make validate-all
```

## Test Conventions

### Time-dependent tests: pass the clock in, or pin it

**Never hardcode an absolute instant and compare it against the real clock.** A test written that
way passes until the hardcoded moment arrives, then fails on every branch at once — with no code
change to blame it on. That is exactly what happened in
[#145](https://github.com/etowett/bared/issues/145): `utils/cron.test.ts` asserted on
`new Date('2026-07-28T02:00:00Z')`, `formatNextRun` measures against the real `new Date()` and
short-circuits to `'Overdue'` for any past instant, and at 02:00 UTC on 2026-07-28 the frontend gate
went red across the whole repo.

Two ways to stay out of that trap, in order of preference:

**1. Inject the clock (preferred).** Take `now` as a defaulted parameter so the test controls it and
production code stays unchanged. `utils/age.ts` is the pattern to copy:

```ts
// src/utils/age.ts
export function formatAge(timestamp?: string | null, now: Date = new Date()): string | null
```

```ts
// src/utils/age.test.ts — the test owns the clock, so nothing can rot.
const now = new Date('2026-07-28T12:00:00Z')
const ago = (ms: number) => new Date(now.getTime() - ms).toISOString()

expect(formatAge(ago(2 * 3600_000), now)).toBe('2 hours ago')
```

An injectable `now` also makes the boundaries (just now / minutes / hours / days) directly testable
instead of approximable.

**2. Pin the clock with fake timers.** When the function reads the clock internally and changing its
signature isn't worth it, pin the system time — and always restore it in a `finally` so a failing
assertion cannot leak fake timers into the rest of the file:

```ts
import { describe, expect, it, vi } from 'vitest'

it('renders the absolute instant in the viewer zone', () => {
  vi.useFakeTimers()
  vi.setSystemTime(new Date('2026-07-28T01:00:00Z'))

  try {
    expect(formatNextRun('2026-07-28T02:00:00Z')).toContain('5:00 AM')
  } finally {
    vi.useRealTimers()
  }
})
```

**Relative offsets are fine.** Building instants as offsets from `Date.now()` never rots, because
the relationship under test is the offset, not the absolute date —
`components/JobProgress.test.tsx` does this:

```ts
// src/components/JobProgress.test.tsx
eta: new Date(Date.now() + 60000).toISOString(), // 1 minute from now
```

**Rule of thumb:** a date literal in a test is only safe if nothing in the code path calls
`new Date()` / `Date.now()` on its own. Pure parsing and formatting assertions (`serverZoneLabel`,
`describeSchedule`) can use literals freely — they never consult the clock. The moment a comparison
against "now" enters the path, inject or pin.

## Code Quality Tools

### ESLint

- **Config**: `eslint.config.js`
- **Rules**:
  - TypeScript strict mode
  - React Hooks rules
  - React Refresh compatibility
  - No unused variables (except `_` prefixed)
  - Warn on `any` types

### Prettier

- **Config**: `.prettierrc`
- **Settings**:
  - No semicolons
  - Single quotes
  - 2-space indentation
  - 100 character line width
  - ES5 trailing commas

### TypeScript

- **Config**: `tsconfig.json`
- **Settings**:
  - Strict mode enabled
  - No unused locals/parameters
  - No fallthrough cases in switch
  - JSX: react-jsx

## CI/CD Integration

### Pre-commit Hook Example

```bash
#!/bin/bash
# .git/hooks/pre-commit

cd apps/web
bun run validate || exit 1
```

### GitHub Actions Example

```yaml
name: Frontend CI

on: [push, pull_request]

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6

      - name: Setup Bun
        uses: oven-sh/setup-bun@v2
        with:
          bun-version-file: apps/web/.bun-version

      - name: Install dependencies
        run: cd apps/web && bun install --frozen-lockfile

      - name: Run validation
        run: cd apps/web && bun run validate

      - name: Build
        run: cd apps/web && bun run build
```

## Testing the Build

### 1. Clean Build Test

```bash
# Remove all artifacts
make web-clean

# Fresh install and build
make web-build

# Verify dist/ was created
ls -lh apps/web/dist/
```

### 2. Development Server Test

```bash
# Start dev server
bun run dev

# Open browser to http://localhost:5173
# Verify hot reload works by editing a component
```

### 3. Production Build Test

```bash
# Build for production
bun run build

# Preview production build
bun run preview

# Open browser to http://localhost:4173
```

### 4. Integration Test with Go Backend

```bash
# Terminal 1: Start Go backend.
# --http-allowed-origin lets the Vite dev server (a different origin) log in and
# open the log-stream WebSocket.
cd ..
make run-daemon --http :8080 --http-user admin --http-pass changeme \
  --http-allowed-origin http://localhost:5173

# Terminal 2: Start React dev server (proxies to Go)
cd apps/web
bun run dev

# Browser: http://localhost:5173
# Should proxy API requests to http://localhost:8080
```

### 5. Embedded Build Test

```bash
# Build web frontend
make web-build

# Build Go binary with embedded web
make build

# Run binary
./bin/brd daemon --config examples/config.example.yml \
  --http :8080 --http-user admin --http-pass changeme

# Browser: http://localhost:8080
# Should serve React app from embedded files
```

## Common Issues & Solutions

### Issue: `bun install` fails

**Solution**:

```bash
# Clear Bun's global cache
bun pm cache rm

# Remove installed dependencies
rm -rf node_modules

# Reinstall
bun install
```

If the lockfile is the problem, `bun install` (without `--frozen-lockfile`) will update `bun.lock` —
commit the result. Never delete `bun.lock`.

### Issue: TypeScript errors

**Solution**:

```bash
# Run type checker with verbose output
bun run type-check

# Check tsconfig.json is correct
cat tsconfig.json
```

### Issue: ESLint fails

**Solution**:

```bash
# Auto-fix what can be fixed
bun run lint:fix

# Check for remaining issues
bun run lint
```

### Issue: Build succeeds but page is blank

**Check**:

1. Browser console for errors
2. Network tab for 404s
3. Vite config proxy settings
4. API endpoint availability

**Solution**:

```bash
# Verify API is running
curl http://localhost:8080/api/health

# Check dev server proxy
# vite.config.ts should have:
# proxy: { '/api': 'http://localhost:8080' }
```

### Issue: Hot reload not working

**Solution**:

1. Check dev server is running
2. Verify file is in `src/` directory
3. Restart dev server
4. Clear browser cache

## File Structure

```
apps/web/
├── dist/                  # Production build (gitignored)
├── node_modules/          # Dependencies (gitignored)
├── src/
│   ├── api/              # API client
│   ├── components/       # React components
│   ├── hooks/            # Custom React hooks
│   ├── styles/           # CSS files
│   ├── types/            # TypeScript types
│   ├── App.tsx           # Root component
│   └── main.tsx          # Entry point
├── eslint.config.js      # ESLint flat config
├── .prettierrc           # Prettier config
├── .prettierignore       # Prettier ignore rules
├── .gitignore            # Git ignore rules
├── .bun-version          # Pinned Bun version
├── bun.lock              # Committed Bun lockfile
├── index.html            # HTML template
├── package.json          # Dependencies and scripts
├── postcss.config.js     # PostCSS (@tailwindcss/postcss)
├── tsconfig.json         # TypeScript config
├── vite.config.ts        # Vite bundler config
├── vitest.config.ts      # Vitest config
├── README.md             # Frontend docs
└── TESTING.md            # This file
```

## Performance Metrics

### Build Performance

```bash
# Measure build time
time bun run build

# Expected: < 2 seconds for production build
```

### Bundle Size

```bash
# After build, check sizes
ls -lh dist/assets/

# Typical sizes (Tailwind v4 / Vite 8):
# - HTML: < 1 KB
# - CSS: ~55 KB (gzip: ~10 KB)
# - JS:  ~627 KB (gzip: ~182 KB)
```

### Lighthouse Score Goals

- Performance: > 90
- Accessibility: > 90
- Best Practices: > 90
- SEO: > 80

## Best Practices

1. **Always validate before committing**:

   ```bash
   bun run validate
   ```

2. **Use ESLint auto-fix for simple issues**:

   ```bash
   bun run lint:fix
   ```

3. **Format code automatically**:

   ```bash
   bun run format
   ```

4. **Check types frequently**:

   ```bash
   bun run type-check
   ```

5. **Test in production mode before deploying**:

   ```bash
   bun run build && bun run preview
   ```

## Troubleshooting Checklist

- [ ] `bun --version` matches `.bun-version`
- [ ] bun install completed successfully
- [ ] No TypeScript errors (`bun run type-check`)
- [ ] No ESLint errors (`bun run lint`)
- [ ] Code is formatted (`bun run format:check`)
- [ ] Build succeeds (`bun run build`)
- [ ] dist/ directory exists
- [ ] Browser console has no errors
- [ ] API endpoints return data

## Resources

- [Bun Documentation](https://bun.sh/docs)
- [Vite Documentation](https://vite.dev/)
- [Vitest Documentation](https://vitest.dev/)
- [React Documentation](https://react.dev/)
- [TypeScript Documentation](https://www.typescriptlang.org/)
- [TanStack Query Documentation](https://tanstack.com/query)
- [Tailwind CSS Documentation](https://tailwindcss.com/docs)
- [ESLint Documentation](https://eslint.org/)
- [Prettier Documentation](https://prettier.io/)

## Support

For issues or questions:

- Check existing GitHub issues
- Create a new issue with:
  - Steps to reproduce
  - Error messages
  - Browser and Bun versions
  - Build logs

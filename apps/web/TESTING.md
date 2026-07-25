# Web Frontend Testing & Validation Guide

## Overview

The BareD web frontend includes comprehensive testing, linting, and validation tools to ensure code quality.

## Quick Start

```bash
# Install dependencies
npm install

# Build for production
npm run build

# Run development server
npm run dev

# Validate everything
npm run validate
```

## Available Commands

### Build Commands

```bash
# Development server with hot reload
npm run dev

# Production build (TypeScript check + Vite build)
npm run build

# Preview production build locally
npm run preview
```

### Linting & Formatting

```bash
# Run ESLint
npm run lint

# Run ESLint with auto-fix
npm run lint:fix

# Format code with Prettier
npm run format

# Check formatting without modifying files
npm run format:check
```

### Type Checking

```bash
# Run TypeScript type checker (no emit)
npm run type-check
```

### Complete Validation

```bash
# Run all checks: type-check + lint + format:check
npm run validate
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

cd web
npm run validate || exit 1
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

      - name: Setup Node.js
        uses: actions/setup-node@v6
        with:
          node-version-file: apps/web/.nvmrc

      - name: Install dependencies
        run: cd apps/web && npm ci

      - name: Run validation
        run: cd apps/web && npm run validate

      - name: Build
        run: cd apps/web && npm run build
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
npm run dev

# Open browser to http://localhost:5173
# Verify hot reload works by editing a component
```

### 3. Production Build Test

```bash
# Build for production
npm run build

# Preview production build
npm run preview

# Open browser to http://localhost:4173
```

### 4. Integration Test with Go Backend

```bash
# Terminal 1: Start Go backend
cd ..
make run-daemon --http :8080 --http-user admin --http-pass changeme

# Terminal 2: Start React dev server (proxies to Go)
cd web
npm run dev

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

### Issue: `npm install` fails

**Solution**:

```bash
# Clear npm cache
npm cache clean --force

# Remove node_modules
rm -rf node_modules package-lock.json

# Reinstall
npm install
```

### Issue: TypeScript errors

**Solution**:

```bash
# Run type checker with verbose output
npm run type-check

# Check tsconfig.json is correct
cat tsconfig.json
```

### Issue: ESLint fails

**Solution**:

```bash
# Auto-fix what can be fixed
npm run lint:fix

# Check for remaining issues
npm run lint
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
├── .eslintrc.js          # ESLint config
├── .prettierrc           # Prettier config
├── .prettierignore       # Prettier ignore rules
├── .gitignore            # Git ignore rules
├── index.html            # HTML template
├── package.json          # Dependencies and scripts
├── tsconfig.json         # TypeScript config
├── vite.config.ts        # Vite bundler config
├── README.md             # Frontend docs
└── TESTING.md            # This file
```

## Performance Metrics

### Build Performance

```bash
# Measure build time
time npm run build

# Expected: < 2 seconds for production build
```

### Bundle Size

```bash
# After build, check sizes
ls -lh dist/assets/

# Typical sizes:
# - HTML: < 1 KB
# - CSS: ~9 KB (gzip: ~2 KB)
# - JS: ~200 KB (gzip: ~63 KB)
```

### Lighthouse Score Goals

- Performance: > 90
- Accessibility: > 90
- Best Practices: > 90
- SEO: > 80

## Best Practices

1. **Always validate before committing**:

   ```bash
   npm run validate
   ```

2. **Use ESLint auto-fix for simple issues**:

   ```bash
   npm run lint:fix
   ```

3. **Format code automatically**:

   ```bash
   npm run format
   ```

4. **Check types frequently**:

   ```bash
   npm run type-check
   ```

5. **Test in production mode before deploying**:

   ```bash
   npm run build && npm run preview
   ```

## Troubleshooting Checklist

- [ ] Node.js version >= 18
- [ ] npm install completed successfully
- [ ] No TypeScript errors (`npm run type-check`)
- [ ] No ESLint errors (`npm run lint`)
- [ ] Code is formatted (`npm run format:check`)
- [ ] Build succeeds (`npm run build`)
- [ ] dist/ directory exists
- [ ] Browser console has no errors
- [ ] API endpoints return data

## Resources

- [Vite Documentation](https://vitejs.dev/)
- [React Documentation](https://react.dev/)
- [TypeScript Documentation](https://www.typescriptlang.org/)
- [TanStack Query Documentation](https://tanstack.com/query)
- [ESLint Documentation](https://eslint.org/)
- [Prettier Documentation](https://prettier.io/)

## Support

For issues or questions:

- Check existing GitHub issues
- Create a new issue with:
  - Steps to reproduce
  - Error messages
  - Browser/Node versions
  - Build logs

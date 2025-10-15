# Cracked Dev-o-Meter Architecture

## Overview

The Cracked Dev-o-Meter is a full-stack application that combines a SolidJS frontend with a Go backend, deployed as a single binary with embedded assets.

## Architecture Decisions

### Single Binary Deployment

The application uses Go's `embed` package to compile the frontend assets directly into the backend binary. This provides several benefits:

- **Simplified Deployment**: Single binary to deploy, no separate static file serving needed
- **Consistency**: Frontend and backend versions always match
- **Performance**: Fast asset serving directly from memory
- **Security**: Assets cannot be modified post-deployment

### Security Implementation

#### Content Security Policy (CSP)

- **Per-Request Nonces**: Cryptographically secure nonces generated for each request
- **Template Processing**: HTML templates dynamically inject nonces into script and style tags
- **Strict Policy**: Only self-hosted scripts and styles allowed, with nonce verification

#### Security Headers

- `X-Frame-Options: DENY` - Prevents clickjacking
- `X-Content-Type-Options: nosniff` - Prevents MIME sniffing
- `Referrer-Policy: strict-origin-when-cross-origin` - Controls referrer information
- `Permissions-Policy` - Restricts feature access
- `HSTS` (production only) - Enforces HTTPS

### API Routing

All backend API endpoints are under the `/api` prefix:

- `/api/health` - Health check endpoint
- `/api/analyze` - Main analysis endpoint
- `/api/leaderboard/*` - Leaderboard endpoints
- `/api/user/stats` - User statistics
- etc.

Root-level routes:

- `/swagger/*` - API documentation
- `/webhook/*` - External webhooks (e.g., Stripe)
- `/*` - Frontend SPA (fallback route)

### Frontend Development

During development, Vite dev server runs on port 3000 and proxies API requests to the backend on port 8080:

```typescript
proxy: {
    '/api': 'http://localhost:8080',
    '/swagger': 'http://localhost:8080'
}
```

This allows for hot module replacement while maintaining API connectivity.

### Build Process

1. **Frontend Build**: Vite compiles TypeScript/SolidJS to optimized JavaScript
2. **Asset Copying**: Build artifacts copied to `backend/internal/frontend/dist/`
3. **Backend Build**: Go embeds frontend assets and compiles to single binary

Use the provided `build.sh` script for automated builds:

```bash
./build.sh
```

### Docker Build

Multi-stage Docker build process:

1. **Frontend Stage**: Builds frontend with Node.js and pnpm
2. **Backend Stage**: Builds Go binary with embedded frontend
3. **Runtime Stage**: Minimal Alpine image with just the binary

```bash
docker build -t cracked-dev-o-meter .
docker run -p 8080:8080 cracked-dev-o-meter
```

## Directory Structure

```
.
├── frontend/                 # SolidJS frontend
│   ├── src/
│   │   ├── api.ts           # API client (uses /api prefix)
│   │   ├── App.tsx          # Main component
│   │   └── components/      # UI components
│   ├── dist/                # Build output (gitignored)
│   └── vite.config.ts       # Vite configuration
│
├── backend/
│   ├── cmd/server/
│   │   └── main.go          # Main server with API routes
│   ├── internal/
│   │   ├── frontend/        # Frontend embedding
│   │   │   ├── embed.go     # Go embed directive
│   │   │   ├── template.go  # HTML template processing
│   │   │   ├── handler.go   # SPA handler
│   │   │   └── dist/        # Embedded assets (gitignored)
│   │   ├── security/
│   │   │   ├── csp.go       # CSP nonce generation
│   │   │   └── headers.go   # Security headers
│   │   ├── monitoring/      # Observability
│   │   ├── ratelimit/       # Rate limiting
│   │   ├── resilience/      # Circuit breakers, retries
│   │   └── ...              # Other packages
│   └── go.mod
│
├── build.sh                 # Automated build script
├── Dockerfile               # Multi-stage Docker build
└── .dockerignore           # Docker build optimization
```

## Production Deployment

### Environment Variables

Required:

- `PORT` - Server port (default: 8080)
- `GITHUB_TOKEN` - GitHub API token
- `X_BEARER_TOKEN` - X (Twitter) API token
- `JWT_SECRET` - JWT signing secret
- `STRIPE_SECRET_KEY` - Stripe API key

Security:

- `ENABLE_HSTS=true` - Enable HSTS in production with HTTPS
- `ENABLE_CSP_REPORT=true` - Enable CSP violation reporting
- `CSP_REPORT_URI` - URI for CSP violation reports

Optional:

- `DATA_DIR` - Database directory (default: ./data)
- `REDIS_URL` - Redis connection string
- `SLACK_WEBHOOK_URL` - Slack notifications

### Running the Binary

```bash
# Set environment variables
export GITHUB_TOKEN=your_token
export X_BEARER_TOKEN=your_token
export JWT_SECRET=your_secret

# Run the server
./bin/cracked-dev-o-meter
```

The server serves both the frontend (on all routes) and the API (on `/api/*`).

### Health Checks

- `/api/health` - Basic health status
- `/api/health/services` - Detailed service health with circuit breakers

### Monitoring

- `/api/metrics` - Application metrics
- `/api/pools/*` - Connection pool statistics
- `/api/memory` - Memory usage statistics
- `/api/debug/pprof/*` - Go profiling (if `ENABLE_PROFILING=true`)

## Security Best Practices

1. **Always use HTTPS in production** - Set `ENABLE_HSTS=true`
2. **Set strong JWT secrets** - Use at least 32 random bytes
3. **Configure CSP reporting** - Monitor CSP violations
4. **Enable rate limiting** - Configure Redis for distributed rate limiting
5. **Monitor security logs** - Check for suspicious activity patterns
6. **Keep dependencies updated** - Regularly update Go modules and npm packages

## Development Workflow

1. Start backend: `cd backend && go run ./cmd/server`
2. Start frontend dev server: `cd frontend && pnpm dev`
3. Access app at `http://localhost:3000`
4. API requests automatically proxied to backend

For production builds:

```bash
./build.sh
./bin/cracked-dev-o-meter
```

## Testing

Backend tests:

```bash
cd backend
go test ./...
```

Frontend tests:

```bash
cd frontend
pnpm test
```

Integration tests:

```bash
cd backend
go test ./cmd/server/...
```

## Performance Considerations

- **Asset Caching**: Static assets served with `Cache-Control: public, max-age=31536000, immutable`
- **Compression**: Gzip compression enabled for responses
- **Connection Pooling**: HTTP clients use connection pools for external APIs
- **Rate Limiting**: Distributed rate limiting via Redis
- **Circuit Breakers**: Automatic failure detection and recovery

## Future Enhancements

- OpenAPI TypeScript client generation
- Enhanced CSP reporting and analytics
- Automated security scanning
- Performance monitoring and tracing
- A/B testing framework

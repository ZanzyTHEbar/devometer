<!-- 3cf155e4-fb56-4368-8443-7e626bcd1966 c61ef080-9cc6-4c50-a657-dab1e7304b97 -->
# Embed Frontend Assets with Security Headers and CSP Nonces

## 1. Restructure API Routes with /api Prefix

**Objective**: Move all existing API endpoints under `/api/*` prefix to clearly separate API from frontend routes.

**Files to modify**:

- `backend/cmd/server/main.go` - Update all route registrations

**Changes**:

- Create API route group: `api := r.Group("/api")`
- Move all existing routes under this group:
  - `/analyze` → `/api/analyze`
  - `/health` → `/api/health`
  - `/metrics` → `/api/metrics`
  - `/user/stats` → `/api/user/stats`
  - `/payment/*` → `/api/payment/*`
  - `/leaderboard/*` → `/api/leaderboard/*`
  - `/privacy/*` → `/api/privacy/*`
  - `/pools/*` → `/api/pools/*`
  - `/memory` → `/api/memory`
  - `/alerts` → `/api/alerts`
  - `/cache/stats` → `/api/cache/stats`
  - `/debug/*` → `/api/debug/*` (if profiling enabled)
  - All rate-limit admin endpoints → `/api/admin/rate-limits/*`

**Preserve**:

- Keep `/swagger/*` at root level for API documentation
- Keep `/webhook/stripe` at root level (external webhook)

## 2. Create Frontend Embedding Infrastructure

**Create new file**: `backend/internal/frontend/embed.go`

**Implementation**:

```go
package frontend

import (
    "embed"
    "io/fs"
)

//go:embed dist
var distFS embed.FS

// GetDistFS returns the embedded frontend distribution filesystem
func GetDistFS() (fs.FS, error) {
    return fs.Sub(distFS, "dist")
}
```

**Setup**:

- Copy frontend `dist/` directory to `backend/internal/frontend/dist/` before embedding
- Ensure `dist/` contains index.html and assets/ subdirectory

## 3. Implement CSP Nonce Generation and Middleware

**Create new file**: `backend/internal/security/csp.go`

**Implementation**:

- Generate cryptographically secure random nonce per-request (32 bytes, base64-encoded)
- Store nonce in Gin context for template access
- Build CSP header with nonce for script-src and style-src
- Implement strict CSP policy:
  - `default-src 'self'`
  - `script-src 'self' 'nonce-{NONCE}'`
  - `style-src 'self' 'nonce-{NONCE}' 'unsafe-inline'` (Tailwind requires unsafe-inline)
  - `img-src 'self' data: https:`
  - `font-src 'self' data:`
  - `connect-src 'self'`
  - `frame-ancestors 'none'`
  - `base-uri 'self'`
  - `form-action 'self'`

**Functions**:

```go
func GenerateNonce() string
func CSPMiddleware() gin.HandlerFunc
func GetNonce(c *gin.Context) string
```

## 4. Implement HTML Template Processing with Nonce Injection

**Create new file**: `backend/internal/frontend/template.go`

**Implementation**:

- Parse embedded index.html at startup
- Create template processor that injects nonce into script and style tags
- Replace `<script` tags with `<script nonce="{{.Nonce}}"`
- Replace `<link rel="stylesheet"` with `<link rel="stylesheet" nonce="{{.Nonce}}"`
- Cache processed template for performance
- Render template with per-request nonce value

**Functions**:

```go
func ProcessIndexHTML(htmlContent []byte) (*template.Template, error)
func RenderIndex(c *gin.Context, tmpl *template.Template, nonce string) error
```

## 5. Add Comprehensive Security Headers Middleware

**Create new file**: `backend/internal/security/headers.go`

**Implementation**:

```go
func SecurityHeadersMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // CSP is set by CSPMiddleware with nonce
        c.Header("X-Frame-Options", "DENY")
        c.Header("X-Content-Type-Options", "nosniff")
        c.Header("X-XSS-Protection", "1; mode=block")
        c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
        c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
        
        // HSTS (only in production with HTTPS)
        if os.Getenv("ENABLE_HSTS") == "true" {
            c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
        }
        
        c.Next()
    }
}
```

## 6. Implement SPA Routing with Asset Serving

**Create new file**: `backend/internal/frontend/handler.go`

**Implementation**:

```go
func NewSPAHandler(distFS fs.FS, indexTemplate *template.Template) gin.HandlerFunc {
    fileServer := http.FileServer(http.FS(distFS))
    
    return func(c *gin.Context) {
        path := c.Request.URL.Path
        
        // Serve static assets directly
        if strings.HasPrefix(path, "/assets/") {
            c.Header("Cache-Control", "public, max-age=31536000, immutable")
            fileServer.ServeHTTP(c.Writer, c.Request)
            return
        }
        
        // Check if file exists
        if _, err := fs.Stat(distFS, strings.TrimPrefix(path, "/")); err == nil {
            fileServer.ServeHTTP(c.Writer, c.Request)
            return
        }
        
        // Fallback to index.html for SPA routing
        nonce := GetNonce(c)
        RenderIndex(c, indexTemplate, nonce)
    }
}
```

## 7. Update OpenAPI Specification

**File to update**: `docs/swagger.yaml`

**Changes**:

- Update basePath to `/api`
- Document all existing endpoints under `/api` prefix
- Add proper schemas for request/response types
- Add authentication schemes for protected endpoints
- Document CSP and security headers in responses

**Auto-generate annotations**:

- Add Swagger annotations to route handlers in `main.go` using `swaggo/swag`
- Run `swag init` to regenerate swagger docs

## 8. Generate TypeScript OpenAPI Client

**Setup**:

- Install `openapi-typescript-codegen` in frontend
- Add script to `frontend/package.json`:
  ```json
  "generate:api": "openapi-typescript-codegen --input ../docs/swagger.yaml --output ./src/generated-api --client fetch"
  ```


**Create new file**: `frontend/src/api-client.ts`

```typescript
import { OpenAPI } from './generated-api'

// Configure base URL to be relative (same origin)
OpenAPI.BASE = '/api'

export * from './generated-api'
```

**Update**: `frontend/src/api.ts`

- Replace manual fetch calls with generated client
- Keep custom error handling and type guards
- Remove hardcoded `http://localhost:8080` URLs

## 9. Integrate Everything in main.go

**File to update**: `backend/cmd/server/main.go`

**Changes**:

```go
// Import frontend and security packages
import (
    "github.com/ZanzyTHEbar/cracked-dev-o-meter/internal/frontend"
    // ... other imports
)

func main() {
    // ... existing setup ...
    
    // Load embedded frontend
    distFS, err := frontend.GetDistFS()
    if err != nil {
        log.Fatal("Failed to load embedded frontend:", err)
    }
    
    // Process index.html template
    indexTemplate, err := frontend.LoadIndexTemplate(distFS)
    if err != nil {
        log.Fatal("Failed to load index template:", err)
    }
    
    // Add security headers middleware (before all other middleware)
    r.Use(security.SecurityHeadersMiddleware())
    r.Use(security.CSPMiddleware())
    
    // ... existing middleware ...
    
    // Create API route group
    api := r.Group("/api")
    {
        // Move all existing routes here
        api.POST("/analyze", analyzeHandler)
        api.GET("/health", healthHandler)
        // ... all other routes ...
    }
    
    // Keep swagger and webhooks at root
    r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
    r.POST("/webhook/stripe", stripeWebhookHandler)
    
    // Serve frontend for all other routes (must be last)
    r.NoRoute(frontend.NewSPAHandler(distFS, indexTemplate))
    
    // ... rest of server setup ...
}
```

## 10. Update Build Process and Deployment

**Create new file**: `build.sh`

```bash
#!/bin/bash
set -e

echo "Building frontend..."
cd frontend
pnpm install
pnpm build

echo "Copying frontend dist to backend..."
rm -rf ../backend/internal/frontend/dist
mkdir -p ../backend/internal/frontend
cp -r dist ../backend/internal/frontend/

echo "Building backend with embedded frontend..."
cd ../backend
go build -o ../bin/cracked-dev-o-meter ./cmd/server

echo "Build complete!"
```

**Update**: `backend/Dockerfile`

- Add multi-stage build: first stage builds frontend, second builds backend
- Copy frontend dist into backend before Go build
- Ensure single binary contains both frontend and backend

**Update**: `.gitignore`

- Add `backend/internal/frontend/dist/` to gitignore

## 11. Environment Configuration

**Update**: `env.example`

```bash
# Security
ENABLE_HSTS=false  # Set to true in production with HTTPS
ENABLE_CSP_REPORT=false  # Enable CSP violation reporting
CSP_REPORT_URI=  # URI for CSP violation reports

# Frontend
FRONTEND_URL=http://localhost:8080  # Used for CORS in dev
```

## 12. Update Frontend Vite Configuration

**Update**: `frontend/vite.config.ts`

```typescript
export default defineConfig({
    plugins: [solidPlugin(), tailwindcss()],
    server: {
        port: 3000,
        proxy: {
            '/api': 'http://localhost:8080',
            '/swagger': 'http://localhost:8080'
        }
    },
    build: {
        target: "esnext",
        sourcemap: true,
        outDir: "dist",
        // Ensure hashed filenames for cache busting
        rollupOptions: {
            output: {
                entryFileNames: 'assets/[name]-[hash].js',
                chunkFileNames: 'assets/[name]-[hash].js',
                assetFileNames: 'assets/[name]-[hash].[ext]'
            }
        }
    }
})
```

## Summary

This plan implements:

1. Clean API/frontend separation with `/api` prefix
2. Embedded frontend assets in Go binary using `embed` package
3. Per-request CSP nonces with HTML template processing
4. Comprehensive security headers (X-Frame-Options, HSTS, etc.)
5. SPA routing with fallback to index.html
6. OpenAPI-generated TypeScript client
7. Proper cache headers for static assets
8. Single deployable binary with frontend embedded

The result is a production-ready, secure, single-binary deployment with proper CSP protection and modern security best practices.

### To-dos

- [ ] Restructure all API routes under /api prefix in main.go
- [ ] Create frontend embedding infrastructure with embed.FS
- [ ] Implement CSP nonce generation and middleware
- [ ] Create HTML template processor with nonce injection
- [ ] Add comprehensive security headers middleware
- [ ] Implement SPA routing handler with asset serving
- [ ] Update OpenAPI specification with /api prefix and schemas
- [ ] Generate and integrate TypeScript OpenAPI client
- [ ] Integrate all components in main.go
- [ ] Update build process and Dockerfile for embedded frontend
- [ ] Add security-related environment configuration
- [ ] Update Vite configuration for proxy and build optimization
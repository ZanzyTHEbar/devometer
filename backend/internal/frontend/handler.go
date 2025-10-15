package frontend

import (
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"

	"github.com/ZanzyTHEbar/cracked-dev-o-meter/internal/security"
	"github.com/gin-gonic/gin"
)

// NewSPAHandler creates a handler for serving the SPA with proper routing fallback
func NewSPAHandler(distFS fs.FS, indexTemplate *template.Template) gin.HandlerFunc {
	fileServer := http.FileServer(http.FS(distFS))

	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// Serve static assets directly with aggressive caching
		if strings.HasPrefix(path, "/assets/") {
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}

		// Check if the requested file exists in the filesystem
		cleanPath := strings.TrimPrefix(path, "/")
		if cleanPath == "" {
			cleanPath = "."
		}

		if _, err := fs.Stat(distFS, cleanPath); err == nil {
			// File exists, serve it
			c.Header("Cache-Control", "public, max-age=3600")
			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}

		// Fallback to index.html for SPA routing (client-side routing)
		nonce := security.GetNonce(c)
		if nonce == "" {
			slog.Warn("CSP nonce not found in context, generating new one")
			var err error
			nonce, err = security.GenerateNonce()
			if err != nil {
				slog.Error("Failed to generate nonce", "error", err)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
				return
			}
		}

		if err := RenderIndex(c, indexTemplate, nonce); err != nil {
			slog.Error("Failed to render index.html", "error", err, "path", path)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to render page"})
			return
		}
	}
}

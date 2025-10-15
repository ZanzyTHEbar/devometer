package frontend

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
)

var (
	scriptTagRegex = regexp.MustCompile(`<script([^>]*)>`)
	styleTagRegex  = regexp.MustCompile(`<link([^>]*rel=["']stylesheet["'][^>]*)>`)
)

// LoadIndexTemplate loads and processes the index.html template from the embedded filesystem
func LoadIndexTemplate(distFS fs.FS) (*template.Template, error) {
	// Read index.html
	indexFile, err := distFS.Open("index.html")
	if err != nil {
		return nil, fmt.Errorf("failed to open index.html: %w", err)
	}
	defer indexFile.Close()

	htmlContent, err := io.ReadAll(indexFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read index.html: %w", err)
	}

	// Process the HTML to inject nonce placeholders
	processedHTML := processHTMLForNonce(string(htmlContent))

	// Parse as template
	tmpl, err := template.New("index").Parse(processedHTML)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	return tmpl, nil
}

// processHTMLForNonce modifies HTML to include nonce template placeholders
func processHTMLForNonce(html string) string {
	// Add nonce to script tags
	html = scriptTagRegex.ReplaceAllString(html, `<script nonce="{{.Nonce}}"$1>`)

	// Add nonce to stylesheet link tags
	html = styleTagRegex.ReplaceAllString(html, `<link nonce="{{.Nonce}}"$1>`)

	return html
}

// RenderIndex renders the index.html template with the provided nonce
func RenderIndex(c *gin.Context, tmpl *template.Template, nonce string) error {
	var buf bytes.Buffer

	data := map[string]interface{}{
		"Nonce": nonce,
	}

	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")

	c.Data(http.StatusOK, "text/html; charset=utf-8", buf.Bytes())
	return nil
}

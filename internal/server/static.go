package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// StaticServer serves the compiled React 19 web studio single-page application.
type StaticServer struct {
	distDir string
	handler http.Handler
}

// NewStaticServer creates a static asset file server.
func NewStaticServer(distDir string) *StaticServer {
	return &StaticServer{
		distDir: distDir,
		handler: http.FileServer(http.Dir(distDir)),
	}
}

// ServeHTTP serves static files, falling back to index.html for client-side routing.
func (s *StaticServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// API and WebSocket requests must not be handled by static server
	if strings.HasPrefix(r.URL.Path, "/api") {
		http.NotFound(w, r)
		return
	}

	indexPath := filepath.Join(s.distDir, "index.html")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>SandForge Studio</title><meta charset="utf-8"/></head>
<body style="font-family: sans-serif; background: #0f172a; color: #f8fafc; padding: 2rem;">
  <h2>SandForge Execution Sandbox & Security Guard</h2>
  <p>Web studio building or static directory not loaded. Connect via MCP or check <code>/api/status</code>.</p>
</body>
</html>`))
		return
	}

	targetPath := filepath.Join(s.distDir, filepath.Clean(r.URL.Path))
	if info, err := os.Stat(targetPath); err == nil && !info.IsDir() {
		s.handler.ServeHTTP(w, r)
		return
	}

	http.ServeFile(w, r, indexPath)
}

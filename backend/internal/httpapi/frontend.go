package httpapi

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

var requiredFrontendFiles = []string{
	"index.html",
	"login.html",
	"dashboard.html",
	"settings.html",
}

func NewFrontendHandler(api http.Handler, root string) (http.Handler, error) {
	root = filepath.Clean(root)
	for _, name := range requiredFrontendFiles {
		info, err := os.Stat(filepath.Join(root, name))
		if err != nil {
			return nil, fmt.Errorf("frontend file %s: %w", name, err)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("frontend file %s is not a regular file", name)
		}
	}

	files := http.FileServer(http.Dir(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" || strings.HasPrefix(r.URL.Path, "/api/") {
			api.ServeHTTP(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		files.ServeHTTP(w, r)
	}), nil
}

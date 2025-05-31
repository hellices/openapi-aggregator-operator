// Package swagger provides a Swagger UI server for displaying OpenAPI specifications
package swagger

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	observabilityv1alpha1 "github.com/hellices/openapi-aggregator-operator/api/v1alpha1"
)

//go:embed swagger-ui/*
var swaggerUI embed.FS

// APIMetadata represents metadata about an OpenAPI specification
type APIMetadata struct {
	Name         string `json:"name"`         // API name
	URL          string `json:"url"`          // URL to fetch the OpenAPI spec
	Title        string `json:"title"`        // Display title
	Version      string `json:"version"`      // API version
	Description  string `json:"description"`  // API description
	ResourceType string `json:"resourceType"` // Type of resource (e.g., Service, Deployment)
	ResourceName string `json:"resourceName"` // Name of the Kubernetes resource
	Namespace    string `json:"namespace"`    // Kubernetes namespace
	LastUpdated  string `json:"lastUpdated"`  // Last update timestamp
}

// Server serves the Swagger UI and aggregated OpenAPI specs
type Server struct {
	specs    map[string]APIMetadata // Map of API name to metadata
	specsMux sync.RWMutex           // Mutex for thread-safe access to specs
}

// NewServer creates a new Swagger UI server
func NewServer() *Server {
	return &Server{
		specs: make(map[string]APIMetadata),
	}
}

// UpdateSpecs updates the stored OpenAPI specs based on the current status
func (s *Server) UpdateSpecs(apis []observabilityv1alpha1.APIInfo) {
	s.specsMux.Lock()
	defer s.specsMux.Unlock()

	newSpecs := make(map[string]APIMetadata)
	for _, api := range apis {
		// Skip APIs with errors
		if api.Error != "" {
			continue
		}

		metadata := APIMetadata{
			Name:         api.Name,
			URL:          api.URL,
			Title:        api.Name,
			Description:  fmt.Sprintf("API from %s/%s", api.Namespace, api.ResourceName),
			ResourceType: api.ResourceType,
			ResourceName: api.ResourceName,
			Namespace:    api.Namespace,
			LastUpdated:  api.LastUpdated,
		}

		newSpecs[api.Name] = metadata
	}
	s.specs = newSpecs
}

// serveIndex serves the Swagger UI index page
func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request) {
	indexContent, err := swaggerUI.ReadFile("swagger-ui/index.html")
	if err != nil {
		http.Error(w, "Failed to read index.html", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	if _, err := w.Write(indexContent); err != nil {
		http.Error(w, "Failed to write response", http.StatusInternalServerError)
	}
}

// serveSpecs serves the list of available API specs
func (s *Server) serveSpecs(w http.ResponseWriter, r *http.Request) {
	s.specsMux.RLock()
	defer s.specsMux.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(s.specs); err != nil {
		http.Error(w, "Failed to encode specs", http.StatusInternalServerError)
	}
}

// serveIndividualSpec serves individual OpenAPI spec by fetching it in real-time
func (s *Server) serveIndividualSpec(w http.ResponseWriter, r *http.Request) {
	apiName := strings.TrimPrefix(r.URL.Path, "/api/")

	s.specsMux.RLock()
	metadata, exists := s.specs[apiName]
	s.specsMux.RUnlock()

	if !exists {
		http.Error(w, "API not found", http.StatusNotFound)
		return
	}

	// Fetch the spec in real-time
	resp, err := http.Get(metadata.URL)
	// for test
	// fmt.Printf("Metadata for %s: %+v, exists: %v\n", apiName, metadata, exists)
	// resp, err := http.Get("https://petstore.swagger.io/v2/swagger.json")
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch spec: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, fmt.Sprintf("Failed to fetch spec, status: %d", resp.StatusCode), resp.StatusCode)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := io.Copy(w, resp.Body); err != nil {
		http.Error(w, "Failed to copy response", http.StatusInternalServerError)
	}
}

// serveStaticFiles serves embedded static files
func (s *Server) serveStaticFiles(w http.ResponseWriter, r *http.Request) {
	// First try assets subdirectory for static files
	content, err := swaggerUI.ReadFile("swagger-ui/assets" + r.URL.Path)
	if err != nil {
		// If not found in assets, try the root swagger-ui directory
		content, err = swaggerUI.ReadFile("swagger-ui" + r.URL.Path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
	}

	// Set appropriate content type based on file extension
	contentType := s.getContentType(r.URL.Path)
	w.Header().Set("Content-Type", contentType)

	if _, err := w.Write(content); err != nil {
		http.Error(w, "Failed to write response", http.StatusInternalServerError)
	}
}

// getContentType determines the content type based on file extension
func (s *Server) getContentType(path string) string {
	switch {
	case strings.HasSuffix(path, ".css"):
		return "text/css"
	case strings.HasSuffix(path, ".js"):
		return "application/javascript"
	case strings.HasSuffix(path, ".png"):
		return "image/png"
	case strings.HasSuffix(path, ".html"):
		return "text/html"
	default:
		return "application/octet-stream"
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Set common headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Handle OPTIONS requests for CORS
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Route to appropriate handler
	switch {
	case r.URL.Path == "/" || r.URL.Path == "/index.html":
		s.serveIndex(w, r)
	case r.URL.Path == "/swagger-specs":
		s.serveSpecs(w, r)
	case strings.HasPrefix(r.URL.Path, "/swagger-specs/"):
		s.serveIndividualSpec(w, r)
	default:
		s.serveStaticFiles(w, r)
	}
}

// Start starts the Swagger UI server
func (s *Server) Start(port int) error {
	// Use the embedded file system instead of serving from disk
	mux := http.NewServeMux()
	mux.HandleFunc("/swagger-specs", s.serveSpecs)
	mux.HandleFunc("/api/", s.serveIndividualSpec)
	mux.HandleFunc("/", s.ServeHTTP)

	srv := &http.Server{
		Addr:      fmt.Sprintf(":%d", port),
		Handler:   mux,
		TLSConfig: nil, // Disable TLS
	}

	fmt.Printf("Starting server on port %d (HTTP)\n", port)
	return srv.ListenAndServe()
}

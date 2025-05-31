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

type APIMetadata struct {
	Name         string `json:"name"`
	URL          string `json:"url"`
	Title        string `json:"title"`
	Version      string `json:"version"`
	Description  string `json:"description"`
	ResourceType string `json:"resourceType"`
	ResourceName string `json:"resourceName"`
	Namespace    string `json:"namespace"`
	LastUpdated  string `json:"lastUpdated"`
}

// Server serves the Swagger UI and aggregated OpenAPI specs
type Server struct {
	specs    map[string]APIMetadata
	specsMux sync.RWMutex
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
	fmt.Printf("Total APIs processed: %d\n", len(newSpecs))
	s.specs = newSpecs
}

// fetchSpec fetches the OpenAPI spec from a service URL
func (s *Server) fetchSpec(url string) (map[string]interface{}, error) {
	fmt.Printf("Fetching OpenAPI spec from URL: %s\n", url)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch spec: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Printf("Error closing response body: %v\n", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("service returned status code: %d", resp.StatusCode)
	}

	var spec map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&spec); err != nil {
		return nil, fmt.Errorf("failed to decode spec: %v", err)
	}

	fmt.Printf("Successfully fetched and decoded OpenAPI spec from %s\n", url)
	return spec, nil
}

// serveIndex serves the Swagger UI index page
func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request) {
	indexContent, err := swaggerUI.ReadFile("swagger-ui/index.html")
	if err != nil {
		fmt.Printf("Failed to read index.html: %v\n", err)
		http.Error(w, "Failed to read index.html", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	if _, err := w.Write(indexContent); err != nil {
		fmt.Printf("Error writing index content: %v\n", err)
	}
}

// serveSpecs serves the list of available API specs
func (s *Server) serveSpecs(w http.ResponseWriter, r *http.Request) {
	s.specsMux.RLock()
	defer s.specsMux.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.specs)
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
	io.Copy(w, resp.Body)
}

// serveStaticFiles serves embedded static files
func (s *Server) serveStaticFiles(w http.ResponseWriter, r *http.Request) {
	// For other paths, try to serve from embedded files
	// First try assets subdirectory for static files
	var content []byte
	var err error

	content, err = swaggerUI.ReadFile("swagger-ui/assets" + r.URL.Path)
	if err != nil {
		// If not found in assets, try the root swagger-ui directory
		content, err = swaggerUI.ReadFile("swagger-ui" + r.URL.Path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
	}

	// Set content type based on file extension
	switch {
	case strings.HasSuffix(r.URL.Path, ".css"):
		w.Header().Set("Content-Type", "text/css")
	case strings.HasSuffix(r.URL.Path, ".js"):
		w.Header().Set("Content-Type", "application/javascript")
	case strings.HasSuffix(r.URL.Path, ".png"):
		w.Header().Set("Content-Type", "image/png")
	case strings.HasSuffix(r.URL.Path, ".html"):
		w.Header().Set("Content-Type", "text/html")
	}

	if _, err := w.Write(content); err != nil {
		fmt.Printf("Error writing content: %v\n", err)
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

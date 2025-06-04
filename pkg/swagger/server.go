// Package swagger provides a Swagger UI server for displaying OpenAPI specifications
package swagger

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	observabilityv1alpha1 "github.com/hellices/openapi-aggregator-operator/api/v1alpha1"
)

//go:embed swagger-ui/*
var swaggerUI embed.FS

// APIMetadata represents metadata about an OpenAPI specification
type APIMetadata struct {
	Name           string   `json:"name"`           // API name
	URL            string   `json:"url"`            // URL to fetch the OpenAPI spec
	Title          string   `json:"title"`          // Display title
	Version        string   `json:"version"`        // API version
	Description    string   `json:"description"`    // API description
	ResourceType   string   `json:"resourceType"`   // Type of resource (e.g., Service, Deployment)
	ResourceName   string   `json:"resourceName"`   // Name of the Kubernetes resource
	Namespace      string   `json:"namespace"`      // Kubernetes namespace
	LastUpdated    string   `json:"lastUpdated"`    // Last update timestamp
	AllowedMethods []string `json:"allowedMethods"` // Allowed HTTP methods for Swagger UI
}

// Server serves the Swagger UI and aggregated OpenAPI specs
type Server struct {
	specs    map[string]APIMetadata // Map of API name to metadata
	specsMux sync.RWMutex           // Mutex for thread-safe access to specs
	basePath string                 // Base path for the server (for Ingress/Route support)
}

// NewServer creates a new Swagger UI server
func NewServer() *Server {
	return &Server{
		specs:    make(map[string]APIMetadata),
		basePath: os.Getenv("SWAGGER_BASE_PATH"),
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
			Name:           api.Name,
			URL:            api.URL,
			Title:          api.Name,
			Description:    fmt.Sprintf("API from %s/%s", api.Namespace, api.ResourceName),
			ResourceType:   api.ResourceType,
			ResourceName:   api.ResourceName,
			Namespace:      api.Namespace,
			LastUpdated:    api.LastUpdated,
			AllowedMethods: api.AllowedMethods,
		}

		newSpecs[api.Name] = metadata
	}
	s.specs = newSpecs
}

// stripBasePath removes the base path prefix from the request path
func (s *Server) stripBasePath(path string) string {
	if s.basePath != "" && strings.HasPrefix(path, s.basePath) {
		return strings.TrimPrefix(path, s.basePath)
	}
	return path
}

// serveIndex serves the Swagger UI index page
func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request) {
	indexContent, err := swaggerUI.ReadFile("swagger-ui/index.html")
	if err != nil {
		http.Error(w, "Failed to read index.html", http.StatusInternalServerError)
		return
	}

	// Add base path meta tag
	htmlContent := string(indexContent)
	metaTag := fmt.Sprintf(`<meta name="base-path" content="%s">`, s.basePath)
	htmlContent = strings.Replace(htmlContent, "</head>", metaTag+"</head>", 1)

	w.Header().Set("Content-Type", "text/html")
	if _, err := w.Write([]byte(htmlContent)); err != nil {
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
	urlStr := metadata.URL
	if os.Getenv("DEV_MODE") == "true" {
		// In development mode, rewrite any cluster URLs to localhost:8080
		if strings.Contains(urlStr, ".svc.cluster.local:8080") {
			urlStr = "http://localhost:8080" + strings.Split(urlStr, ".svc.cluster.local:8080")[1]
		}
	}
	resp, err := http.Get(urlStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch spec: %v", err), http.StatusInternalServerError)
		return
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, fmt.Sprintf("Failed to fetch spec, status: %d", resp.StatusCode), resp.StatusCode)
		return
	}

	// Parse metadata URL to get the server URL
	metadataURL, err := url.Parse(metadata.URL)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse metadata URL: %v", err), http.StatusInternalServerError)
		return
	}

	// Read and parse the OpenAPI/Swagger spec
	var spec map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&spec); err != nil {
		http.Error(w, fmt.Sprintf("Failed to decode OpenAPI spec: %v", err), http.StatusInternalServerError)
		return
	}

	// Update spec based on OpenAPI/Swagger version
	s.updateSpecServerInfo(spec, metadataURL)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(spec); err != nil {
		http.Error(w, "Failed to encode modified spec", http.StatusInternalServerError)
	}
}

// serveStaticFiles serves embedded static files
func (s *Server) serveStaticFiles(w http.ResponseWriter, r *http.Request) {
	path := s.stripBasePath(r.URL.Path)

	// First try assets subdirectory for static files
	content, err := swaggerUI.ReadFile("swagger-ui/assets" + path)
	if err != nil {
		// If not found in assets, try the root swagger-ui directory
		content, err = swaggerUI.ReadFile("swagger-ui" + path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
	}

	// Set appropriate content type based on file extension
	contentType := s.getContentType(path)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=3600") // Add caching for static files

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
	// Set common headers with more permissive CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Access-Control-Expose-Headers", "Content-Length")

	// Handle OPTIONS requests for CORS
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Strip base path if configured
	path := s.stripBasePath(r.URL.Path)

	// Route to appropriate handler
	switch {
	case path == "/" || path == "/index.html":
		s.serveIndex(w, r)
	case path == "/swagger-specs":
		s.serveSpecs(w, r)
	case strings.HasPrefix(path, "/api/"):
		s.serveIndividualSpec(w, r)
	case strings.HasPrefix(path, "/proxy/"):
		s.proxyRequest(w, r)
	default:
		s.serveStaticFiles(w, r)
	}
}

// proxyRequest handles proxy requests by forwarding them to the target URL
func (s *Server) proxyRequest(w http.ResponseWriter, r *http.Request) {
	var proxyURL string
	var reqBody io.Reader

	proxyURL = r.URL.Query().Get("proxyUrl")
	if proxyURL == "" {
		http.Error(w, "proxyUrl query parameter is required for GET/HEAD requests", http.StatusBadRequest)
		return
	}
	reqBody = nil

	// Get the path after /proxy/ and combine with proxyURL if needed
	originalPath := strings.TrimPrefix(r.URL.Path, "/proxy/")
	targetURL := proxyURL
	// Fetch the spec in real-time
	if os.Getenv("DEV_MODE") == "true" {
		// In development mode, rewrite any cluster URLs to localhost:8080
		if strings.Contains(targetURL, ".svc.cluster.local:8080") {
			targetURL = "http://localhost:8080" + strings.Split(targetURL, ".svc.cluster.local:8080")[1]
		}
	}

	if originalPath != "" {
		targetURL = fmt.Sprintf("%s/%s", proxyURL, originalPath)
	}

	// Create new request with the same method and modified body
	proxyReq, err := http.NewRequest(r.Method, targetURL, reqBody)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create proxy request: %v", err), http.StatusInternalServerError)
		return
	}

	// Copy headers from original request
	for key, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(key, value)
		}
	}

	// Forward the request
	client := &http.Client{}
	resp, err := client.Do(proxyReq)
	if err != nil {
		fmt.Printf("Error forwarding request: %v\n", err)
		http.Error(w, fmt.Sprintf("Failed to forward request: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Set response status code
	w.WriteHeader(resp.StatusCode)

	// Copy response body
	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Printf("Error copying response body: %v", err)
	}
}

// makeServerURL creates a server URL by combining metadata URL with an optional path from existing URL
func (s *Server) makeServerURL(metadataURL *url.URL, existingURL string) string {
	if existingURL == "" {
		return fmt.Sprintf("%s://%s", metadataURL.Scheme, metadataURL.Host)
	}

	if parsedURL, err := url.Parse(existingURL); err == nil && parsedURL.Path != "" {
		return fmt.Sprintf("%s://%s%s", metadataURL.Scheme, metadataURL.Host, parsedURL.Path)
	}

	return fmt.Sprintf("%s://%s", metadataURL.Scheme, metadataURL.Host)
}

// updateSpecServerInfo updates the server information in the OpenAPI spec based on its version
func (s *Server) updateSpecServerInfo(spec map[string]interface{}, metadataURL *url.URL) {
	openAPIVersion, _ := spec["openapi"].(string)
	swaggerVersion, _ := spec["swagger"].(string)

	// OpenAPI 3.x
	if openAPIVersion != "" && strings.HasPrefix(openAPIVersion, "3.") {
		existingServers, _ := spec["servers"].([]interface{})
		newServers := make([]interface{}, 0)

		// If there are existing servers, get the URI part from the first server
		if len(existingServers) > 0 {
			if firstServer, ok := existingServers[0].(map[string]interface{}); ok {
				if serverURL, ok := firstServer["url"].(string); ok {
					newServers = append(newServers, map[string]interface{}{
						"url": s.makeServerURL(metadataURL, serverURL),
					})
				}
			}
		}

		// If we couldn't get URI from existing servers, add just the host
		if len(newServers) == 0 {
			newServers = append(newServers, map[string]interface{}{
				"url": s.makeServerURL(metadataURL, ""),
			})
		}

		// Append existing servers
		newServers = append(newServers, existingServers...)
		spec["servers"] = newServers

	} else if swaggerVersion == "2.0" { // Swagger/OpenAPI 2.0
		spec["host"] = metadataURL.Host

	} else { // Swagger 1.2 or undefined
		if basePath, ok := spec["basePath"].(string); ok {
			spec["basePath"] = s.makeServerURL(metadataURL, basePath)
		}
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

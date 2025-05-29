package swagger

import (
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	observabilityv1alpha1 "github.com/yourname/openapi-aggregator-operator/api/v1alpha1"
)

//go:embed swagger-ui/*
var swaggerUI embed.FS

// Server serves the Swagger UI and aggregated OpenAPI specs
type Server struct {
	specs    map[string]map[string]interface{}
	specsMux sync.RWMutex
}

// NewServer creates a new Swagger UI server
func NewServer() *Server {
	return &Server{
		specs: make(map[string]map[string]interface{}),
	}
}

// UpdateSpecs updates the stored OpenAPI specs based on the current status
func (s *Server) UpdateSpecs(apis []observabilityv1alpha1.APIInfo) {
	s.specsMux.Lock()
	defer s.specsMux.Unlock()

	newSpecs := make(map[string]map[string]interface{})

	// For testing, always provide at least one dummy spec
	newSpecs["Test API"] = map[string]interface{}{
		"openapi": "3.0.0",
		"info": map[string]interface{}{
			"title":       "Test API",
			"description": "This is a test API specification",
			"version":     "1.0.0",
		},
		"servers": []interface{}{
			map[string]interface{}{
				"url":         "/api",
				"description": "Test server",
			},
		},
		"paths": map[string]interface{}{
			"/test": map[string]interface{}{
				"get": map[string]interface{}{
					"summary":     "Test endpoint",
					"description": "This is a test endpoint",
					"operationId": "getTest",
					"tags":        []string{"test"},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Successful response",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"message": map[string]interface{}{
												"type":    "string",
												"example": "Hello from Test API",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Fetch and store new specs
	for _, api := range apis {
		if api.Error != "" {
			continue
		}

		// Save API URL to be fetched on demand
		newSpecs[api.Name] = map[string]interface{}{
			"url": api.URL,
			"info": map[string]interface{}{
				"title":       api.Name,
				"description": fmt.Sprintf("API from %s/%s", api.Namespace, api.ResourceName),
				"version":     "1.0.0",
			},
		}
	}

	// Update specs only if we successfully fetched at least one
	if len(newSpecs) > 0 {
		s.specs = newSpecs
	}
}

// fetchSpec fetches the OpenAPI spec from a service URL
func (s *Server) fetchSpec(url string) (map[string]interface{}, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch spec: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("service returned status code: %d", resp.StatusCode)
	}

	var spec map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&spec); err != nil {
		return nil, fmt.Errorf("failed to decode spec: %v", err)
	}

	return spec, nil
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

	// Serve Swagger UI index page directly for root path
	if r.URL.Path == "/" || r.URL.Path == "/index.html" {
		indexContent, err := swaggerUI.ReadFile("swagger-ui/index.html")
		if err != nil {
			fmt.Printf("Failed to read index.html: %v\n", err)
			http.Error(w, "Failed to read index.html", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write(indexContent)
		return
	}

	// Handle API specs listing
	if r.URL.Path == "/swagger-specs" {
		s.specsMux.RLock()
		defer s.specsMux.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		if len(s.specs) == 0 {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "No API specifications available"})
			return
		}

		response := make(map[string]interface{})
		for name, info := range s.specs {
			// Only include metadata in the listing
			response[name] = map[string]interface{}{
				"info": info["info"],
				"url":  info["url"],
			}
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			fmt.Printf("Failed to encode specs: %v\n", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Handle individual spec endpoint
	if strings.HasPrefix(r.URL.Path, "/swagger-specs/") {
		s.specsMux.RLock()
		name := strings.TrimPrefix(r.URL.Path, "/swagger-specs/")
		info, exists := s.specs[name]
		s.specsMux.RUnlock()

		if !exists {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "API specification not found"})
			return
		}

		url, ok := info["url"].(string)
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid URL in API specification"})
			return
		}

		spec, err := s.fetchSpec(url)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(spec); err != nil {
			fmt.Printf("Failed to encode spec: %v\n", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// For other paths, try to serve from embedded files
	content, err := swaggerUI.ReadFile("swagger-ui" + r.URL.Path)
	if err != nil {
		http.NotFound(w, r)
		return
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

	w.Write(content)
}

// Start starts the Swagger UI server
func (s *Server) Start(port int) error {
	return http.ListenAndServe(fmt.Sprintf(":%d", port), s)
}

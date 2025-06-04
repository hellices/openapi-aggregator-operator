package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/hellices/openapi-aggregator-operator/api/v1alpha1"
	"github.com/hellices/openapi-aggregator-operator/pkg/swagger"
)

func main() {
	server := swagger.NewServer()

	// Get the specs directory from environment variable or use default
	specsDir := os.Getenv("SPECS_DIR")
	if specsDir == "" {
		specsDir = "/specs"
	}

	// Start watching the specs directory
	go watchSpecsDirectory(specsDir, server)

	// Get port from environment variable or use default
	port := 9090
	if portStr := os.Getenv("PORT"); portStr != "" {
		if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
			log.Printf("Invalid port number: %s, using default: %d", portStr, port)
		}
	}

	log.Printf("Starting Swagger UI server on port %d", port)
	if err := server.Start(port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func watchSpecsDirectory(dir string, server *swagger.Server) {
	for {
		specs := loadSpecs(dir)
		server.UpdateSpecs(specs)
		time.Sleep(10 * time.Second)
	}
}

func loadSpecs(dir string) []v1alpha1.APIInfo {
	var specs []v1alpha1.APIInfo

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		log.Printf("Failed to read specs directory: %v", err)
		return specs
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		data, err := ioutil.ReadFile(filepath.Join(dir, file.Name()))
		if err != nil {
			log.Printf("Failed to read file %s: %v", file.Name(), err)
			continue
		}

		var apiInfo v1alpha1.APIInfo
		if err := json.Unmarshal(data, &apiInfo); err != nil {
			log.Printf("Failed to unmarshal API info from %s: %v", file.Name(), err)
			continue
		}

		specs = append(specs, apiInfo)
	}

	return specs
}

package swagger

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	observabilityv1alpha1 "github.com/yourname/openapi-aggregator-operator/api/v1alpha1"
)

func TestUpdateSpecs(t *testing.T) {
	server := NewServer()

	testCases := []struct {
		name     string
		apis     []observabilityv1alpha1.APIInfo
		expected int
	}{
		{
			name:     "empty apis",
			apis:     []observabilityv1alpha1.APIInfo{},
			expected: 0,
		},
		{
			name: "single api with error",
			apis: []observabilityv1alpha1.APIInfo{
				{
					Name:  "error-api",
					Error: "failed to fetch",
				},
			},
			expected: 0,
		},
		{
			name: "multiple valid apis",
			apis: []observabilityv1alpha1.APIInfo{
				{
					Name:         "api1",
					URL:          "http://example.com/api1",
					Namespace:    "default",
					ResourceName: "api1",
					LastUpdated:  time.Now().Format(time.RFC3339),
				},
				{
					Name:         "api2",
					URL:          "http://example.com/api2",
					Namespace:    "default",
					ResourceName: "api2",
					LastUpdated:  time.Now().Format(time.RFC3339),
				},
			},
			expected: 2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server.UpdateSpecs(tc.apis)
			assert.Equal(t, tc.expected, len(server.specs))
		})
	}
}

func TestServeHTTP(t *testing.T) {
	server := NewServer()

	// Test API specs endpoint
	t.Run("swagger-specs endpoint", func(t *testing.T) {
		// Setup test data
		server.UpdateSpecs([]observabilityv1alpha1.APIInfo{
			{
				Name:         "test-api",
				URL:          "http://example.com/test",
				Namespace:    "default",
				ResourceName: "test-api",
				LastUpdated:  time.Now().Format(time.RFC3339),
			},
		})

		req := httptest.NewRequest(http.MethodGet, "/swagger-specs", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var specs map[string]interface{}
		err := json.NewDecoder(w.Body).Decode(&specs)
		assert.NoError(t, err)
		assert.Contains(t, specs, "test-api")
	})

	// Test index.html
	t.Run("index.html", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "text/html", w.Header().Get("Content-Type"))
		assert.Contains(t, w.Body.String(), "<!DOCTYPE html>")
	})

	// Test CORS
	t.Run("CORS headers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "GET, OPTIONS", w.Header().Get("Access-Control-Allow-Methods"))
		assert.Equal(t, "Content-Type", w.Header().Get("Access-Control-Allow-Headers"))
	})

	// Test static assets
	t.Run("static assets", func(t *testing.T) {
		paths := []struct {
			path        string
			contentType string
		}{
			{"/swagger-ui.css", "text/css"},
			{"/swagger-ui-bundle.js", "application/javascript"},
			{"/favicon-32x32.png", "image/png"},
		}

		for _, p := range paths {
			req := httptest.NewRequest(http.MethodGet, p.path, nil)
			w := httptest.NewRecorder()

			server.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code, "Path: %s", p.path)
			assert.Equal(t, p.contentType, w.Header().Get("Content-Type"), "Path: %s", p.path)
		}
	})

	// Test not found
	t.Run("not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestConcurrency(t *testing.T) {
	server := NewServer()
	done := make(chan bool)

	// Concurrent updates
	go func() {
		for i := 0; i < 100; i++ {
			server.UpdateSpecs([]observabilityv1alpha1.APIInfo{
				{
					Name:         "api1",
					URL:          "http://example.com/api1",
					Namespace:    "default",
					ResourceName: "api1",
					ResourceType: "Deployment",
					LastUpdated:  time.Now().Format(time.RFC3339),
				},
			})
		}
		done <- true
	}()

	// Concurrent reads
	go func() {
		for i := 0; i < 100; i++ {
			req := httptest.NewRequest(http.MethodGet, "/swagger-specs", nil)
			w := httptest.NewRecorder()
			server.ServeHTTP(w, req)
		}
		done <- true
	}()

	// Wait for both goroutines
	for i := 0; i < 2; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("Test timed out")
		}
	}
}

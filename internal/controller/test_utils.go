package controller

import (
	"github.com/yourname/openapi-aggregator-operator/pkg/swagger"
)

// TestSwaggerServer represents a test server for Swagger UI implementation.
type TestSwaggerServer struct {
	// No fields needed for this test wrapper
}

// NewTestSwaggerServer creates a new test instance of Swagger Server.
func NewTestSwaggerServer() *swagger.Server {
	return swagger.NewServer()
}

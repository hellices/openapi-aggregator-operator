package controller

import (
	observabilityv1alpha1 "github.com/yourname/openapi-aggregator-operator/api/v1alpha1"
	"github.com/yourname/openapi-aggregator-operator/pkg/swagger"
)

// TestSwaggerServer는 테스트를 위한 Swagger UI 서버 구현입니다.
type TestSwaggerServer struct {
	specs []observabilityv1alpha1.APIInfo
}

func NewTestSwaggerServer() *swagger.Server {
	return swagger.NewServer()
}

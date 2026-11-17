// Package platform is a resource detector shared by the shop's batch jobs.
// It still uses semconv v1.21.0.
package platform

import (
	"context"
	"os"

	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

// Detector reports the deployment environment from SHOP_ENV.
type Detector struct{}

// Detect implements resource.Detector.
func (Detector) Detect(context.Context) (*resource.Resource, error) {
	env := os.Getenv("SHOP_ENV")
	if env == "" {
		env = "dev"
	}
	return resource.NewWithAttributes(semconv.SchemaURL, semconv.DeploymentEnvironment(env)), nil
}

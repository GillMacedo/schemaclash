// Package telemetry builds the OpenTelemetry resource for the shop's API.
// It was written against an older SDK, and still imports semconv v1.24.0.
package telemetry

import (
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

// NewResource follows the pattern from the OpenTelemetry Go getting-started
// guide: the SDK's default resource merged with the service's attributes.
func NewResource(service, version string) (*resource.Resource, error) {
	return resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(service),
			semconv.ServiceVersion(version),
		),
	)
}

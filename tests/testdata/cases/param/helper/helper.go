// Package helper merges the default resource with the caller's.
package helper

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
)

// Build returns the default resource with a service name, in schemaURL.
func Build(schemaURL, service string) (*resource.Resource, error) {
	return resource.Merge(resource.Default(),
		resource.NewWithAttributes(schemaURL, attribute.String("service.name", service)))
}

// A local detector on the SDK's semconv version.
package main

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

type region struct{}

func (region) Detect(context.Context) (*resource.Resource, error) {
	return resource.NewWithAttributes(semconv.SchemaURL, semconv.CloudRegion("eu-west-1")), nil
}

func main() {
	res, err := resource.New(context.Background(), resource.WithDetectors(region{}), resource.WithTelemetrySDK())
	if err != nil {
		fmt.Println("conflict:", err)
		return
	}
	fmt.Println("ok", res.SchemaURL())
}

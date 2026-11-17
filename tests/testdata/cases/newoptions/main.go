// resource.New with a built-in detector and an older schema URL.
package main

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

func main() {
	res, err := resource.New(context.Background(),
		resource.WithTelemetrySDK(),
		resource.WithSchemaURL(semconv.SchemaURL),
		resource.WithAttributes(semconv.ServiceName("case")),
	)
	if err != nil {
		fmt.Println("conflict:", err)
		return
	}
	fmt.Println("ok", res.SchemaURL())
}

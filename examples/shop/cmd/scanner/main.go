// Command scanner checks stock levels every night with tracing.
package main

import (
	"context"
	"fmt"
	"log"

	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"example.com/shop/internal/platform"
)

func main() {
	ctx := context.Background()
	res, err := resource.New(ctx,
		resource.WithDetectors(platform.Detector{}),
		resource.WithTelemetrySDK(),
		resource.WithProcessPID(),
	)
	if err != nil {
		log.Fatalf("scanner: telemetry: %v", err)
	}
	tp := sdktrace.NewTracerProvider(sdktrace.WithResource(res))
	defer tp.Shutdown(ctx)
	fmt.Println("scanner: started, resource schema", res.SchemaURL())
}

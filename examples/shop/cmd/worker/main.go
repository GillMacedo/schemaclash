// Command worker processes the shop's order queue with tracing.
package main

import (
	"context"
	"fmt"
	"log"

	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func main() {
	ctx := context.Background()
	res, err := resource.New(ctx,
		resource.WithTelemetrySDK(),
		resource.WithHost(),
		resource.WithSchemaURL(semconv.SchemaURL),
		resource.WithAttributes(semconv.ServiceName("shop-worker")),
	)
	if err != nil {
		log.Fatalf("worker: telemetry: %v", err)
	}
	tp := sdktrace.NewTracerProvider(sdktrace.WithResource(res))
	defer tp.Shutdown(ctx)
	fmt.Println("worker: started, resource schema", res.SchemaURL())
}
